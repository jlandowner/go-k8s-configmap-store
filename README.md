# go-k8s-configmap-store

`go-k8s-configmap-store` is light key-value store for Go, using Kubernetes ConfigMap.

Support only for UTF-8 planetext data, not support for binary data.
And it does NOT lock any ConfigMaps in the cluster.

# Install

```shell
go get "github.com/jlandowner/go-k8s-configmap-store"
```

# Prerequirement

Apply ServiceAccount and ClusterRoles.

```shell
kubectl apply -f https://raw.githubusercontent.com/jlandowner/go-k8s-configmap-store/master/rbac.yaml
```

Then your apps should run with the ServiceAccount `configmap-store`

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: XXX
spec:
  containers:
  - name: XXX
    image: XXX
  serviceAccount: configmap-store # Require
```

# Usage example

```go
package main

import (
	"context"
	"log"

	kcs "github.com/jlandowner/go-k8s-configmap-store"
)

func main() {
	ctx := context.Background()

	// Create store manager
	man, err := kcs.NewConfigMapStoreManager(ctx, "default")
	if err != nil {
		panic(err)
	}

	// Create new ConfigMap store
	exampleMap, err := man.NewMapStore(ctx, "example")
	if err != nil {
		panic(err)
	}

	log.Println(exampleMap.GetConfigMap().Name)

	// Upsert key-value data in the ConfigMap store
	err = exampleMap.Upsert(ctx, "hello", "world")
	if err != nil {
		panic(err)
	}

	// Get value by given key
	val, err := exampleMap.Get(ctx, "hello")
	if err != nil {
		panic(err)
	}

	log.Println("OK", val)
}
```

