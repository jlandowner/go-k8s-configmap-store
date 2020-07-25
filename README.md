# go-k8s-configmap-store

`go-k8s-configmap-store` is light key-value store for Go, using Kubernetes ConfigMap.

Just run your app in k8s cluster.
There is no need any other key-value store inside or outside your cluster.

Support only for UTF-8 planetext data, not support for binary data now.

# Install

```shell
go get "github.com/jlandowner/go-k8s-configmap-store"
```

# Prerequirement

Apply ServiceAccount and ClusterRoles.

```shell
kubectl apply -f https://raw.githubusercontent.com/jlandowner/go-k8s-configmap-store/master/rbac.yaml
```

Also, your apps should run with the ServiceAccount `configmap-store`

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: XXX
spec:
  containers:
  - name: XXX
    image: XXX
  serviceAccount: go-configmap-store # Require
```

# Usage & Example

Here is an example: 

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

