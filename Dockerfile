FROM docker.io/golang:1.18 AS builder
WORKDIR /go/src/github.com/clyang82/multicluster-global-hub-lite
COPY . .

RUN CGO_ENABLED=0 go build -o bin/global-hub-apiserver ./cmd/server/main.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
ENV USER_UID=10001

# need to have etcd dir later
RUN mkdir /etc/etcd-server && chmod -R 777 /etc/etcd-server

COPY --from=builder /go/src/github.com/clyang82/multicluster-global-hub-lite/bin/global-hub-apiserver /
RUN microdnf update && microdnf clean all

USER ${USER_UID}