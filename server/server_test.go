package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/k3s-io/kine/pkg/endpoint"
)

func TestEndpointListen(t *testing.T) {
	etcdConfig, err := endpoint.Listen(context.TODO(), endpoint.Config{
		Endpoint: "postgres://hoh-process-user:pGFCVv%40uP%5BQgE7fr%28%5EQ%7B6%3C5%29@hoh-pgbouncer.hoh-postgres.svc:5432/experimental",
	})
	if err != nil {
		t.Errorf("failed to create etcdConfig: %s", err)
	}
	fmt.Print(etcdConfig)
}
