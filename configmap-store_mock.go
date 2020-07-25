package store

import (
	"context"
	"sync"
)

// NewMockConfigMapStoreManager returns mock of ConfigMapStoreManager
func NewMockConfigMapStoreManager(ctx context.Context, namespace string) (*ConfigMapStoreManager, error) {
	localmaps := make(map[string]string, 0)
	return &ConfigMapStoreManager{
		k8sclient: nil,
		localMaps: localmaps,
		lock:      new(sync.RWMutex),
		namespace: namespace,
	}, nil
}
