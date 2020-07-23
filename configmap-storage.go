package storage

import (
	"context"
	"fmt"
	"log"
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
)

const (
	namePrefix = "storage.k8s.jlandowner.com"
)

// ConfigMapStorageManager is a manager of configmaps
type ConfigMapStorageManager struct {
	k8sclient *kubernetes.Clientset
	LocalMaps map[string]*MapStorage
	namespace string
	lock      *sync.RWMutex
}

// MapStorage is a configmap as m
type MapStorage struct {
	configMap *corev1.ConfigMap
	lock      *sync.RWMutex
}

// NewConfigMapStorageManager returns ConfigMapStorageManager
func NewConfigMapStorageManager(stopCh <-chan struct{}, namespace string) (*ConfigMapStorageManager, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	maps := make(map[string]*MapStorage, 0)

	factory := informers.NewSharedInformerFactory(client, time.Minute)
	listener := factory.Core().V1().ConfigMaps().Lister()
	labelSelector := getLabelSelector()

	resync := func() {
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
		informer.Run(stopCh)
		log.Println("informer stopped")
	}()

	return &ConfigMapStorageManager{
		k8sclient: client,
		LocalMaps: maps,
		lock:      new(sync.RWMutex),
		namespace: namespace,
	}, nil
}

// CreateNewMapStorage creates new configmap as managed map
func (c *ConfigMapStorageManager) CreateNewMapStorage(ctx context.Context, name string) (*MapStorage, error) {
	_, exist := c.LocalMaps[name]
	if exist {
		return nil, fmt.Errorf("ConfigMap %s has already exist in cluster", name)
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	cm := &corev1.ConfigMap{}
	cm.SetName(namePrefix + "." + name)
	cm.SetLabels(getLabels())
	cm.Data = map[string]string{"dummy": time.Now().String()}

	res, err := c.k8sclient.CoreV1().ConfigMaps(c.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if apierrs.IsAlreadyExists(err) {
		return c.GetMapStorage(name)
	}
	if err != nil {
		return nil, err
	}

	m := &MapStorage{configMap: res, lock: new(sync.RWMutex)}
	c.LocalMaps[name] = m

	return m, nil
}

// DeleteMapStorage removes configmap
func (c *ConfigMapStorageManager) DeleteMapStorage(ctx context.Context, name string) error {
	m, exist := c.LocalMaps[name]
	if !exist {
		return fmt.Errorf("MapStorage %s do not exist in cluster", name)
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

// GetMapStorage returns value by given key
func (c *ConfigMapStorageManager) GetMapStorage(name string) (*MapStorage, error) {
	m, exist := c.LocalMaps[name]
	if !exist {
		return nil, fmt.Errorf("MapStorage %s do not exist in cluster", name)
	}
	return m, nil
}

// Commit commit the local change
func (c *ConfigMapStorageManager) Commit(ctx context.Context, m *MapStorage) error {
	ret, err := c.k8sclient.CoreV1().ConfigMaps(m.configMap.Namespace).Update(ctx, m.configMap, metav1.UpdateOptions{})
	syncLocalMap(c.LocalMaps, []*corev1.ConfigMap{ret})
	return err
}

// Upsert update or insert value by given key
func (m *MapStorage) Upsert(ctx context.Context, key, value string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.configMap.Data != nil {
		m.configMap.Data[key] = value
	} else {
		m.configMap.Data = map[string]string{key: value}
	}
}

// Delete remove given key
func (m *MapStorage) Delete(ctx context.Context, key string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.configMap.Data, key)
}

// Get returns value by given key
func (m *MapStorage) Get(key string) (string, bool) {
	val, ok := m.configMap.Data[key]
	return val, ok
}

func syncLocalMap(localMap map[string]*MapStorage, syncList []*corev1.ConfigMap) {
	for _, cm := range syncList {
		namekey := extractBaseName(cm.Name)
		m, exist := localMap[namekey]
		if exist {
			m.lock.Lock()
			m.configMap = cm
			m.lock.Unlock()
		} else {
			localMap[namekey] = &MapStorage{configMap: cm, lock: new(sync.RWMutex)}
		}
	}
}

func getLabelSelector() labels.Selector {
	labelSelector, err := labels.Parse(namePrefix + "/type in m")
	if err != nil {
		return labels.NewSelector()
	}
	return labelSelector
}

func getLabels() map[string]string {
	labels := make(map[string]string, 0)
	labels[namePrefix+"/type"] = "m"
	labels[namePrefix+"/locked"] = "false"
	return labels
}

func extractBaseName(name string) string {
	sp := strings.Split(name, ".")
	return sp[len(sp)-1]
}
