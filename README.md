<p>
<a href="https://godoc.org/github.com/clyang82/multicluster-global-hub-lite"><img src="https://godoc.org/github.com/clyang82/multicluster-global-hub-lite?status.svg"></a>
<a href="https://pkg.go.dev/clyang82/multicluster-global-hub-lite"><img src="https://pkg.go.dev/badge/clyang82/multicluster-global-hub-lite" alt="PkgGoDev"></a>
<a href="https://goreportcard.com/report/github.com/clyang82/multicluster-global-hub-lite"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/clyang82/multicluster-global-hub-lite" /></a>
</p>

# Global hub apiserver

Minimal embeddable Kubernetes-style apiserver

## Prerequisites

1. Connect to an OpenShift cluster
2. Install the latest [kubectl](https://kubernetes.io/docs/tasks/tools/)
3. Install the latest [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/binaries/)

## Development Prerequisites

- Go v1.18+

## Build and psuh the global-hub-apiserver and syncer image

```sh
make docker-push
```

## Start the global-hub-apiserver on an OpenShift cluster

Deploy the global-hub-apiserver
```sh
make deploy
```

## Start the syncer

1. Create secret to connect the global hub apiserver from the syncer.
```sh
oc create secret generic multicluster-global-hub-kubeconfig --from-file=kubeconfig=./deploy/server/certs/kube-aggregator.kubeconfig
```
2. Deploy the syncer
```sh
oc apply -k deploy/syncer
```