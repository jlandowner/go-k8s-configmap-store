package store

import (
	"context"
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	namePrefix = "store.k8s.jlandowner.com"
)

// ConfigMapStoreManager is a manager of ConfigMaps
type ConfigMapStoreManager struct {
	k8sclient *kubernetes.Clientset
	localMaps map[string]string
	namespace string
	lock      *sync.RWMutex
}

// MapStore has the ConfigMap and methods to CRUD to the ConfigMap's Data
type MapStore struct {
	k8sclient *kubernetes.Clientset
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

	return &ConfigMapStoreManager{
		k8sclient: client,
		localMaps: make(map[string]string, 0),
		lock:      new(sync.RWMutex),
		namespace: namespace,
	}, nil
}

// CreateNewMapStore creates new ConfigMap as managed map
func (c *ConfigMapStoreManager) CreateNewMapStore(ctx context.Context, name string) (*MapStore, error) {
	_, exist := c.localMaps[name]
	if exist {
		return c.GetMapStore(ctx, name)
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	cm := &corev1.ConfigMap{}
	cm.SetName(namePrefix + "." + name)
	cm.SetLabels(getLabels())

	ret, err := c.k8sclient.CoreV1().ConfigMaps(c.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if apierrs.IsAlreadyExists(err) {
		return c.GetMapStore(ctx, name)
	}
	if err != nil {
		return nil, err
	}

	c.localMaps[name] = ret.Name
	return &MapStore{configMap: ret, lock: new(sync.RWMutex)}, nil
}

// DeleteMapStore removes ConfigMap
func (c *ConfigMapStoreManager) DeleteMapStore(ctx context.Context, name string) error {
	cname, exist := c.localMaps[name]
	if !exist {
		return fmt.Errorf("MapStore %s do not exist in cluster", name)
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	err := c.k8sclient.CoreV1().ConfigMaps(c.namespace).Delete(ctx, cname, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	delete(c.localMaps, name)
	return nil
}

// GetMapStore returns value by given key
func (c *ConfigMapStoreManager) GetMapStore(ctx context.Context, name string) (*MapStore, error) {
	cname, exist := c.localMaps[name]
	if !exist {
		return nil, fmt.Errorf("MapStore %s do not exist in cluster", name)
	}

	cm, err := c.k8sclient.CoreV1().ConfigMaps(c.namespace).Get(ctx, cname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return &MapStore{k8sclient: c.k8sclient, configMap: cm, lock: new(sync.RWMutex)}, nil
}

// Upsert update or insert value by given key
func (m *MapStore) Upsert(ctx context.Context, key, value string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.configMap.Data != nil {
		m.configMap.Data[key] = value
	} else {
		m.configMap.Data = map[string]string{key: value}
	}

	ret, err := m.k8sclient.CoreV1().ConfigMaps(m.configMap.Namespace).Update(ctx, m.configMap, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	m.configMap = ret
	return nil
}

// Delete remove the given key
func (m *MapStore) Delete(ctx context.Context, key string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.configMap.Data != nil {
		return fmt.Errorf("MapStore %s does not have key %s", extractBaseName(m.configMap.Name), key)
	}
	if _, exist := m.configMap.Data[key]; !exist {
		return fmt.Errorf("MapStore %s does not have key %s", extractBaseName(m.configMap.Name), key)
	}

	delete(m.configMap.Data, key)

	ret, err := m.k8sclient.CoreV1().ConfigMaps(m.configMap.Namespace).Update(ctx, m.configMap, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	m.configMap = ret
	return nil
}

// Get returns value by given key
func (m *MapStore) Get(ctx context.Context, key string) (string, error) {
	cm, err := m.k8sclient.CoreV1().ConfigMaps(m.configMap.Namespace).Get(ctx, m.configMap.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if cm.Data == nil {
		return "", fmt.Errorf("MapStore %s does not have key %s", extractBaseName(m.configMap.Name), key)
	}

	val, exist := cm.Data[key]
	if !exist {
		return "", fmt.Errorf("MapStore %s does not have key %s", extractBaseName(m.configMap.Name), key)
	}

	return val, nil
}

// GetConfigMap returns corev1.ConfigMap of MapStore
func (m *MapStore) GetConfigMap() corev1.ConfigMap {
	return *m.configMap
}

func getLabelSelector() labels.Selector {
	labelSelector, _ := labels.Parse(namePrefix + "/managed in (true)")
	return labelSelector
}

func getLabels() map[string]string {
	labels := map[string]string{namePrefix + "/managed": "true"}
	return labels
}

func extractBaseName(name string) string {
	sp := strings.Split(name, ".")
	return sp[len(sp)-1]
}
