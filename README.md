# go-k8s-configmap-store

`go-k8s-configmap-store` is light key-value store for Go, using Kubernetes ConfigMap.
Just your app is running in k8s cluster, there is no need any other key-value store inside or outside your cluster.

Support only for UTF-8 planetext data, not support for binary data now.

# Install

```shell
go get -u "github.com/jlandowner/go-k8s-configmap-store"
```

# Prerequirement

Apply ServiceAccount and ClusterRoles.

```shell
kubectl apply -f https://raw.githubusercontent.com/jlandowner/go-k8s-configmap-store/master/rbac.yaml
```

Also, your image should run with the ServiceAccount `go-configmap-store`

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

```go
package main

import (
	"context"
	"log"

	kcs "github.com/jlandowner/go-k8s-configmap-store"
)

func main() {
	stop := make(chan struct{})
	defer close(stop)

    // Create store manager
	manager, err := kcs.NewConfigMapStoreManager(stop, "default")
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()
    // Create new ConfigMap store
	exampleMap, err := manager.CreateNewMapStore(ctx, "example")
	if err != nil {
		log.Fatalln(err)
	}

    // Upsert key-value data in the ConfigMap store
	exampleMap.Upsert(ctx, "hello", "world")

    // Commit change
	if err := manager.Commit(ctx, exampleMap); err != nil {
		log.Println(err)
	}

    // Get value by given key
	val, ok = exampleMap.Get("hello")
	if ok {
		log.Println(val)
	} else {
		log.Fatalln("Failed to get")
	}
}
```

