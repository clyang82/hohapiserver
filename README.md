<p>
<a href="https://godoc.org/github.com/clyang82/hohapiserver"><img src="https://godoc.org/github.com/clyang82/hohapiserver?status.svg"></a>
<a href="https://pkg.go.dev/thetirefire/hohapiserver"><img src="https://pkg.go.dev/badge/thetirefire/hohapiserver" alt="PkgGoDev"></a>
<a href="https://goreportcard.com/report/github.com/clyang82/hohapiserver"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/clyang82/hohapiserver" /></a>
</p>

# hohapiserver

Minimal embeddable Kubernetes-style apiserver that supports CustomResourceDefitions

[Presentation/Demo for SIG API Machinery on October 7, 2020](https://www.youtube.com/watch?v=n1L5a09wWas)

[Slide deck](https://docs.google.com/presentation/d/1TfCrsBEgvyOQ1MGC7jBKTvyaelAYCZzl3udRjPlVmWg/edit?usp=sharing)

## Prerequisites

- kubectl binary

## Development Prerequisites

- Go v1.15+

## Build the hohapiserver server

```sh
make hohapiserver
```

## Start the hohapiserver server

```sh
bin/hohapiserver
```

## Do the thing

```sh
# username and password are ignored, but required for the command to complete
kubectl --server https://localhost:6443 --insecure-skip-tls-verify --username=bad --password=idea <the thing>
```
