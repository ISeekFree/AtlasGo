package mongo

import (
	"context"
	"testing"
)

func TestConnectBuildsMultiClusterRegistry(t *testing.T) {
	registry, err := Connect(context.Background(), Options{
		URI:      "mongodb://127.0.0.1:27017",
		Database: "default_db",
		Clusters: map[string]ClusterOptions{
			"orders": {
				URI:      "mongodb://127.0.0.1:27018",
				Database: "orders_default",
			},
		},
		Datastores: map[string]DatastoreOptions{
			"order-read": {
				Cluster:  "orders",
				Database: "orders_read",
			},
		},
	})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer registry.Close(context.Background())

	if registry.Client() == nil {
		t.Fatalf("default client is nil")
	}
	if registry.Client("orders") == nil {
		t.Fatalf("orders client is nil")
	}
	if got := registry.Database().Name(); got != "default_db" {
		t.Fatalf("default database = %q, want default_db", got)
	}
	if got := registry.Database("order-read").Name(); got != "orders_read" {
		t.Fatalf("order-read database = %q, want orders_read", got)
	}
}

func TestConnectRejectsMissingCluster(t *testing.T) {
	_, err := Connect(context.Background(), Options{
		Datastores: map[string]DatastoreOptions{
			"broken": {Cluster: "missing"},
		},
	})
	if err == nil {
		t.Fatalf("expected missing cluster error")
	}
}
