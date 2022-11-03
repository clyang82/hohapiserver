<p>
<a href="https://godoc.org/github.com/clyang82/multicluster-global-hub-lite"><img src="https://godoc.org/github.com/clyang82/multicluster-global-hub-lite?status.svg"></a>
<a href="https://pkg.go.dev/clyang82/multicluster-global-hub-lite"><img src="https://pkg.go.dev/badge/clyang82/multicluster-global-hub-lite" alt="PkgGoDev"></a>
<a href="https://goreportcard.com/report/github.com/clyang82/multicluster-global-hub-lite"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/clyang82/multicluster-global-hub-lite" /></a>
</p>

# Global hub apiserver

Minimal embeddable Kubernetes-style apiserver that supports CustomResourceDefitions

## Prerequisites

- kubectl binary

## Development Prerequisites

- Go v1.15+

## Build the global-hub-apiserver and syncer

```sh
make build
```

## Start the global-hub-apiserver

```sh
bin/global-hub-apiserver
```

