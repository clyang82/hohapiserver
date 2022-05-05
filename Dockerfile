FROM golang:1.17 as builder
WORKDIR /workspace

# Copy the sources
COPY ./ ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o hohapiserver ./

FROM gcr.io/distroless/base:debug
WORKDIR /
COPY --from=builder /workspace/hohapiserver .
ENTRYPOINT ["/hohapiserver"]