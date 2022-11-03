.PHONY: fix fmt vet lint test tidy

GOBIN := $(shell go env GOPATH)/bin
REGISTRY ?= quay.io/clyang82
IMAGE_TAG ?= latest


all: fix fmt vet lint test tidy build

docker:
	docker build ./ --tag ${REGISTRY}/multicluster-global-hub-apiserver:${IMAGE_TAG}
	docker build ./ -f Dockerfile.syncer --tag ${REGISTRY}/multicluster-global-hub-syncer:${IMAGE_TAG}

build:
	CGO_ENABLED=0 go build -o bin/global-hub-apiserver ./cmd/server/main.go
	CGO_ENABLED=0 go build -o bin/syncer ./cmd/syncer/main.go

fix:
	go fix ./...

fmt:
	test -z $(go fmt ./tools/...)

tidy:
	go mod tidy

lint:
	(which golangci-lint || go get github.com/golangci/golangci-lint/cmd/golangci-lint)
	$(GOBIN)/golangci-lint run ./...

test:
	go test -cover ./...

vet:
	go vet ./...

clean:
	rm -rf apiserver.local.config default.etcd bin/