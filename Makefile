.PHONY: fix fmt vet lint test tidy

GOBIN := $(shell go env GOPATH)/bin
REGISTRY ?= quay.io/clyang82
IMAGE_TAG ?= latest
TMP_BIN ?= /tmp/cr-tests-bin
GO_TEST ?= go test -v

all: fix fmt vet lint test tidy build

docker:
	docker build ./ --tag ${REGISTRY}/multicluster-global-hub-apiserver:${IMAGE_TAG}
	docker build ./ -f Dockerfile.syncer --tag ${REGISTRY}/multicluster-global-hub-syncer:${IMAGE_TAG}

docker-push: docker
	docker push ${REGISTRY}/multicluster-global-hub-apiserver:${IMAGE_TAG}
	docker push ${REGISTRY}/multicluster-global-hub-syncer:${IMAGE_TAG}

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


setup_envtest:
	GOBIN=${TMP_BIN} go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

unit-tests-server: setup_envtest
	KUBEBUILDER_ASSETS="$(shell ${TMP_BIN}/setup-envtest use --use-env -p path)" ${GO_TEST} `go list ./server/... | grep -v test`