package mongo

import (
	"testing"
	"time"
)

func TestParseConfigSupportsSingleClusterMultipleDatastores(t *testing.T) {
	t.Setenv("MONGO_PRIMARY_URI", "mongodb://mongo-primary:27017")

	config, err := ParseConfig([]byte(`
framework:
  mongo:
    enabled: true
    timeout: 2s
    auto-index: true
    clusters:
      primary:
        uri: ${MONGO_PRIMARY_URI:mongodb://127.0.0.1:27017}
        auto-index: false
        datastores:
          catalog:
            database: atlas_catalog
          audit:
            database: atlas_audit
            auto-index: true
`))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	resolved, err := resolveOptions(config)
	if err != nil {
		t.Fatalf("resolveOptions() error = %v", err)
	}
	if len(resolved.clusters) != 1 {
		t.Fatalf("clusters = %d, want 1", len(resolved.clusters))
	}
	if got := resolved.clusters["primary"].uri; got != "mongodb://mongo-primary:27017" {
		t.Fatalf("primary URI = %q", got)
	}
	if got := resolved.clusters["primary"].timeout; got != 2*time.Second {
		t.Fatalf("primary timeout = %s", got)
	}
	if got := resolved.datastores["catalog"].database; got != "atlas_catalog" {
		t.Fatalf("catalog database = %q", got)
	}
	if resolved.datastores["catalog"].autoIndex {
		t.Fatal("catalog auto-index should inherit false from cluster")
	}
	if !resolved.datastores["audit"].autoIndex {
		t.Fatal("audit auto-index override should be true")
	}
}

func TestResolveOptionsRejectsDuplicateNestedDatastoreNames(t *testing.T) {
	_, err := resolveOptions(Options{
		Clusters: map[string]ClusterOptions{
			"primary": {
				URI: "mongodb://primary",
				Datastores: map[string]DatastoreOptions{
					"shared": {Database: "primary"},
				},
			},
			"secondary": {
				URI: "mongodb://secondary",
				Datastores: map[string]DatastoreOptions{
					"shared": {Database: "secondary"},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate datastore error")
	}
}
