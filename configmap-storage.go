package storage

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
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

// ConfigMapStorageClient is a manager of configmaps
type ConfigMapStorageClient struct {
	k8sclient *kubernetes.Clientset
	LocalMaps map[string]*MapStorage
	namespace string
	lock      *sync.RWMutex
}

// MapStorage is a configmap as table
type MapStorage struct {
	configMap *corev1.ConfigMap
	lock      *sync.RWMutex
}

// NewConfigMapStorageClient returns ConfigMapStorageClient
func NewConfigMapStorageClient(stopCh <-chan struct{}, namespace string) (*ConfigMapStorageClient, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	tables := make(map[string]*MapStorage, 0)

	factory := informers.NewSharedInformerFactory(client, time.Minute)
	listener := factory.Core().V1().ConfigMaps().Lister()
	labelSelector := labels.NewSelector()
	labelSelector, _ = labels.Parse(namePrefix + "/type in table")

	resync := func() {
		ret, err := listener.List(labelSelector)
		if err != nil {
			return
		}
		syncLocalMap(tables, ret)
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

	return &ConfigMapStorageClient{
		k8sclient: client,
		LocalMaps: tables,
		lock:      new(sync.RWMutex),
		namespace: namespace,
	}, nil
}

// CreateNewMapStorage creates new configmap as managed table
func (c *ConfigMapStorageClient) CreateNewMapStorage(ctx context.Context, name string) (*MapStorage, error) {
	_, exist := c.LocalMaps[name]
	if exist {
		return nil, fmt.Errorf("ConfigMap %s has already exist in cluster", name)
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	cm := &corev1.ConfigMap{}
	cm.SetName(namePrefix + "." + name)
	cm.SetLabels(getLabels())

	res, err := c.k8sclient.CoreV1().ConfigMaps(c.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	table := &MapStorage{res, new(sync.RWMutex)}
	c.LocalMaps[name] = table

	return table, nil
}

// DeleteMapStorage removes configmaptable
func (c *ConfigMapStorageClient) DeleteMapStorage(ctx context.Context, name string) error {
	table, exist := c.LocalMaps[name]
	if !exist {
		return fmt.Errorf("MapStorage %s do not exist in cluster", name)
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	err := c.k8sclient.CoreV1().ConfigMaps(table.configMap.Namespace).Delete(ctx, table.configMap.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	delete(c.LocalMaps, name)
	return nil
}

// GetMapStorage returns value by given key
func (c *ConfigMapStorageClient) GetMapStorage(name string) (*MapStorage, error) {
	table, exist := c.LocalMaps[name]
	if !exist {
		return nil, fmt.Errorf("MapStorage %s do not exist in cluster", name)
	}
	return table, nil
}

// Upsert update or insert value by given key
func (t *MapStorage) Upsert(key, value string) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.configMap.Data[key] = value
}

// Delete remove given key
func (t *MapStorage) Delete(key string) {
	t.lock.Lock()
	defer t.lock.Unlock()

	delete(t.configMap.Data, key)
}

// Get returns value by given key
func (t *MapStorage) Get(key string) (string, bool) {
	val, ok := t.configMap.Data[key]
	return val, ok
}

func syncLocalMap(localMap map[string]*MapStorage, syncList []*corev1.ConfigMap) {
	for _, cm := range syncList {
		namekey := extractBaseName(cm.Name)
		table, exist := localMap[namekey]
		if exist {
			table.lock.Lock()
			localMap[namekey] = &MapStorage{cm, new(sync.RWMutex)}
			table.lock.Unlock()
		} else {
			localMap[namekey] = &MapStorage{cm, new(sync.RWMutex)}
		}
	}
}

func getLabels() map[string]string {
	labels := make(map[string]string, 0)
	labels[namePrefix+"/type"] = "table"
	labels[namePrefix+"/locked"] = "false"
	return labels
}

func extractBaseName(name string) string {
	sp := strings.Split(name, ".")
	return sp[len(sp)-1]
}
