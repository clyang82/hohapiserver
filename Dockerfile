FROM quay.io/bitnami/golang:1.17 AS builder
WORKDIR /go/src/github.com/clyang82/hohapiserver
COPY . .

RUN make build --warn-undefined-variables

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
ENV USER_UID=10001

# need to have etcd dir later
RUN mkdir /etc/etcd-server && chmod -R 777 /etc/etcd-server

COPY --from=builder /go/src/github.com/clyang82/hohapiserver/hohapiserver /
RUN microdnf update && microdnf clean all

USER ${USER_UID}