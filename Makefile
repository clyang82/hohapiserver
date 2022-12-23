.PHONY: fix fmt vet lint test tidy deploy

SHELL := /bin/bash

GOBIN := $(shell go env GOPATH)/bin
REGISTRY ?= quay.io/clyang82
IMAGE_TAG ?= latest
KUBECTL ?= oc
KUSTOMIZE ?= kustomize
TMP_BIN ?= /tmp/cr-tests-bin
GO_TEST ?= go test -v

all: fix fmt vet lint test tidy build

unit-tests-server: setup_envtest
	KUBEBUILDER_ASSETS="$(shell ${TMP_BIN}/setup-envtest use --use-env -p path)" ${GO_TEST} `go list ./server/... | grep -v test`

docker:
	docker build ./ --tag ${REGISTRY}/multicluster-global-hub-apiserver:${IMAGE_TAG}
	docker build ./ -f Dockerfile.syncer --tag ${REGISTRY}/multicluster-global-hub-syncer:${IMAGE_TAG}

docker-push: docker
	docker push ${REGISTRY}/multicluster-global-hub-apiserver:${IMAGE_TAG}
	docker push ${REGISTRY}/multicluster-global-hub-syncer:${IMAGE_TAG}

build:
	CGO_ENABLED=0 go build -o bin/global-hub-apiserver ./cmd/server/main.go
	CGO_ENABLED=0 go build -o bin/syncer ./cmd/syncer/main.go

deploy:
	cp ./deploy/server/deployment.yaml ./deploy/server/deployment.yaml.tmp
	cp ./deploy/server/kustomization.yaml ./deploy/server/kustomization.yaml.tmp
	cd ./deploy/server && . generate_certs.sh && ${KUSTOMIZE} edit set image quay.io/clyang82/multicluster-global-hub-apiserver=${REGISTRY}/multicluster-global-hub-apiserver:${IMAGE_TAG} && cd ../..
	sed -e 's,API_HOST,'${API_HOST}',' ./deploy/server/deployment.yaml
	${KUSTOMIZE} build ./deploy/server | ${KUBECTL} apply -f -
	mv ./deploy/server/deployment.yaml.tmp ./deploy/server/deployment.yaml
	mv ./deploy/server/kustomization.yaml.tmp ./deploy/server/kustomization.yaml

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

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONTROLLER_TOOLS_VERSION ?= v0.9.2
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

manifests: controller-gen ## Generate CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd paths="./apis/..." output:crd:artifacts:config=server/manifests

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object paths="./apis/..."
