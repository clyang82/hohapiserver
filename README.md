<p>
<a href="https://godoc.org/github.com/clyang82/multicluster-global-hub-lite"><img src="https://godoc.org/github.com/clyang82/multicluster-global-hub-lite?status.svg"></a>
<a href="https://pkg.go.dev/clyang82/multicluster-global-hub-lite"><img src="https://pkg.go.dev/badge/clyang82/multicluster-global-hub-lite" alt="PkgGoDev"></a>
<a href="https://goreportcard.com/report/github.com/clyang82/multicluster-global-hub-lite"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/clyang82/multicluster-global-hub-lite" /></a>
</p>

# Global hub apiserver

Minimal embeddable Kubernetes-style apiserver

## Prerequisites

- kubectl binary

## Development Prerequisites

- Go v1.18+

## Build the global-hub-apiserver and syncer

```sh
make build
```

## Start the global-hub-apiserver on an OpenShift cluster

Deploy the global-hub-apiserver
```sh
make deploy/server
```

## Start the syncer

1. Generate the kubeconfig to connect the global hub apiserver.
```sh
SECRETNAME=`oc get sa multicluster-global-hub-apiserver-sa -ojsonpath="{.secrets[0].name}"`
TOKEN=`oc get secret $SECRETNAME -ojsonpath="{.data.token}" | base64 -d`
APISERVER=`oc get route multicluster-global-hub-apiserver -ojsonpath="{.spec.host}"`
oc --kubeconfig /tmp/kubeconfig config set-credentials apiserver-user --token=$TOKEN
oc --kubeconfig /tmp/kubeconfig config set-cluster multicluster-global-hub-apiserver --server=https://$APISERVER --insecure-skip-tls-verify=true
oc --kubeconfig /tmp/kubeconfig config set-context global-hub-apiserver --user=apiserver-user --cluster=multicluster-global-hub-apiserver
oc --kubeconfig /tmp/kubeconfig config use-context global-hub-apiserver
```
2. Create secret to connect the global hub apiserver from the syncer.
```sh
oc create secret generic multicluster-global-hub-kubeconfig --from-file=kubeconfig=/tmp/kubeconfig
```
3. Deploy the syncer
```sh
make deploy/syncer
```