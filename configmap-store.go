package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"k8s.io/klog/v2"
)

const (
	namePrefix = "store.k8s.jlandowner.com"
)

// ConfigMapStoreManager is a manager of ConfigMaps
type ConfigMapStoreManager struct {
	k8sclient *kubernetes.Clientset
	LocalMaps map[string]*MapStore
	namespace string
	lock      *sync.RWMutex
}

// MapStore has the ConfigMap and methods to CRUD to the ConfigMap's Data
type MapStore struct {
	configMap *corev1.ConfigMap
	lock      *sync.RWMutex
}

// NewConfigMapStoreManager returns ConfigMapStoreManager
func NewConfigMapStoreManager(stopCh <-chan struct{}, namespace string) (*ConfigMapStoreManager, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	maps := make(map[string]*MapStore, 0)

	factory := informers.NewSharedInformerFactory(client, time.Minute)
	listener := factory.Core().V1().ConfigMaps().Lister()
	labelSelector := getLabelSelector()

	resync := func() {
		klog.Infoln("Syncing local maps")
		ret, err := listener.List(labelSelector)
		if err != nil {
			return
		}
		syncLocalMap(maps, ret)
	}

	informer := factory.Core().V1().ConfigMaps().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			resync()
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			resync()
		},
		DeleteFunc: func(obj interface{}) {
			resync()
		},
	})

	go func() {
		klog.Infoln("informer starting")
		informer.Run(stopCh)
		klog.Infoln("informer stopped")
	}()

	return &ConfigMapStoreManager{
		k8sclient: client,
		LocalMaps: maps,
		lock:      new(sync.RWMutex),
		namespace: namespace,
	}, nil
}

// CreateNewMapStore creates new ConfigMap as managed map
func (c *ConfigMapStoreManager) CreateNewMapStore(ctx context.Context, name string) (*MapStore, error) {
	_, exist := c.LocalMaps[name]
	if exist {
		return nil, fmt.Errorf("ConfigMap %s has already exist in cluster", name)
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	cm := &corev1.ConfigMap{}
	cm.SetName(namePrefix + "." + name)
	cm.SetLabels(getLabels())
	cm.Data = map[string]string{"DATE": time.Now().String()}

	res, err := c.k8sclient.CoreV1().ConfigMaps(c.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if apierrs.IsAlreadyExists(err) {
		return c.GetMapStore(name)
	}
	if err != nil {
		return nil, err
	}

	m := &MapStore{configMap: res, lock: new(sync.RWMutex)}
	c.LocalMaps[name] = m

	return m, nil
}

// DeleteMapStore removes ConfigMap
func (c *ConfigMapStoreManager) DeleteMapStore(ctx context.Context, name string) error {
	m, exist := c.LocalMaps[name]
	if !exist {
		return fmt.Errorf("MapStore %s do not exist in cluster", name)
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	err := c.k8sclient.CoreV1().ConfigMaps(m.configMap.Namespace).Delete(ctx, m.configMap.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	delete(c.LocalMaps, name)
	return nil
}

// GetMapStore returns value by given key
func (c *ConfigMapStoreManager) GetMapStore(name string) (*MapStore, error) {
	m, exist := c.LocalMaps[name]
	if !exist {
		return nil, fmt.Errorf("MapStore %s do not exist in cluster", name)
	}
	return m, nil
}

// Commit commit the local change
func (c *ConfigMapStoreManager) Commit(ctx context.Context, m *MapStore) error {
	ret, err := c.k8sclient.CoreV1().ConfigMaps(m.configMap.Namespace).Update(ctx, m.configMap, metav1.UpdateOptions{})
	syncLocalMap(c.LocalMaps, []*corev1.ConfigMap{ret})
	return err
}

// Upsert update or insert value by given key
func (m *MapStore) Upsert(ctx context.Context, key, value string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.configMap.Data != nil {
		m.configMap.Data[key] = value
	} else {
		m.configMap.Data = map[string]string{key: value}
	}
}

// Delete remove the given key
func (m *MapStore) Delete(ctx context.Context, key string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.configMap.Data, key)
}

// Get returns value by given key
func (m *MapStore) Get(key string) (string, bool) {
	val, ok := m.configMap.Data[key]
	return val, ok
}

// GetConfigMap returns corev1.ConfigMap of MapStore
func (m *MapStore) GetConfigMap() corev1.ConfigMap {
	return *m.configMap
}

func syncLocalMap(localMap map[string]*MapStore, syncList []*corev1.ConfigMap) {
	// sync Add or Update
	for _, cm := range syncList {
		namekey := extractBaseName(cm.Name)
		m, exist := localMap[namekey]
		if exist {
			m.lock.Lock()
			m.configMap = cm
			m.lock.Unlock()
		} else {
			localMap[namekey] = &MapStore{configMap: cm, lock: new(sync.RWMutex)}
		}
	}

	// sync Delete
	for _, lm := range localMap {
		foundInCluster := false
		for _, cm := range syncList {
			if lm.configMap.Name == cm.Name {
				foundInCluster = true
			}
		}
		if !foundInCluster {
			delete(localMap, extractBaseName(lm.configMap.Name))
		}
	}
}

func getLabelSelector() labels.Selector {
	labelSelector, err := labels.Parse(namePrefix + "/managed in true")
	if err != nil {
		return labels.NewSelector()
	}
	return labelSelector
}

func getLabels() map[string]string {
	labels := make(map[string]string, 0)
	labels[namePrefix+"/managed"] = "true"
	return labels
}

func extractBaseName(name string) string {
	sp := strings.Split(name, ".")
	return sp[len(sp)-1]
}
