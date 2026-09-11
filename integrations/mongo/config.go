package mongo

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/yaml.v3"
)

const DefaultDatastore = "default"
const DefaultCluster = "default"

var environmentPlaceholder = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::([^}]*))?}`)

type Options struct {
	Enabled    *bool                       `yaml:"enabled"`
	URI        string                      `yaml:"uri"`
	Database   string                      `yaml:"database"`
	Timeout    time.Duration               `yaml:"timeout"`
	Ping       bool                        `yaml:"ping"`
	AutoIndex  *bool                       `yaml:"auto-index"`
	Clusters   map[string]ClusterOptions   `yaml:"clusters"`
	Datastores map[string]DatastoreOptions `yaml:"datastores"`
	Entities   []EntityMapping             `yaml:"-"`
}

type ClusterOptions struct {
	URI        string                      `yaml:"uri"`
	Timeout    time.Duration               `yaml:"timeout"`
	Ping       bool                        `yaml:"ping"`
	Database   string                      `yaml:"database"`
	AutoIndex  *bool                       `yaml:"auto-index"`
	Datastores map[string]DatastoreOptions `yaml:"datastores"`
}

type DatastoreOptions struct {
	Cluster   string `yaml:"cluster"`
	Database  string `yaml:"database"`
	AutoIndex *bool  `yaml:"auto-index"`
}

func DefaultOptions() Options {
	return Options{
		Enabled:   Bool(true),
		URI:       "mongodb://127.0.0.1:27017",
		Database:  "atlas",
		Timeout:   10 * time.Second,
		Ping:      false,
		AutoIndex: Bool(true),
	}
}

func Bool(value bool) *bool {
	return &value
}

func (o Options) IsEnabled() bool {
	return boolValue(o.Enabled, true)
}

func (o Options) ClientOptions() *options.ClientOptions {
	uri := o.URI
	if uri == "" {
		uri = DefaultOptions().URI
	}
	return options.Client().ApplyURI(uri)
}

// LoadConfig reads the framework.mongo section from a YAML project configuration.
func LoadConfig(path string) (Options, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Options{}, err
	}
	return ParseConfig(data)
}

// ParseConfig parses either a full project YAML document containing framework.mongo,
// or a YAML document whose root is already the Mongo configuration.
func ParseConfig(data []byte) (Options, error) {
	expanded := environmentPlaceholder.ReplaceAllStringFunc(string(data), func(value string) string {
		parts := environmentPlaceholder.FindStringSubmatch(value)
		if configured, ok := os.LookupEnv(parts[1]); ok {
			return configured
		}
		return parts[2]
	})

	var document yaml.Node
	if err := yaml.Unmarshal([]byte(expanded), &document); err != nil {
		return Options{}, fmt.Errorf("parse mongo config: %w", err)
	}
	node := documentRoot(&document)
	if framework := mappingValue(node, "framework"); framework != nil {
		if mongoNode := mappingValue(framework, "mongo"); mongoNode != nil {
			node = mongoNode
		} else {
			return Options{}, fmt.Errorf("parse mongo config: framework.mongo section is missing")
		}
	}
	if node == nil || node.Kind != yaml.MappingNode {
		return Options{}, fmt.Errorf("parse mongo config: expected a mapping")
	}
	var config Options
	if err := node.Decode(&config); err != nil {
		return Options{}, fmt.Errorf("parse mongo config: %w", err)
	}
	return config, nil
}

func documentRoot(document *yaml.Node) *yaml.Node {
	if document == nil {
		return nil
	}
	if document.Kind == yaml.DocumentNode && len(document.Content) > 0 {
		return document.Content[0]
	}
	return document
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

type resolvedOptions struct {
	enabled          bool
	clusters         map[string]resolvedCluster
	datastores       map[string]resolvedDatastore
	defaultCluster   string
	defaultDatastore string
}

type resolvedCluster struct {
	uri     string
	timeout time.Duration
	ping    bool
}

type resolvedDatastore struct {
	cluster   string
	database  string
	autoIndex bool
}

func resolveOptions(input Options) (resolvedOptions, error) {
	defaults := DefaultOptions()
	rootURI := firstNonBlank(input.URI, defaults.URI)
	rootDatabase := firstNonBlank(input.Database, defaults.Database)
	rootTimeout := input.Timeout
	if rootTimeout <= 0 {
		rootTimeout = defaults.Timeout
	}
	rootPing := input.Ping
	rootAutoIndex := boolValue(input.AutoIndex, true)

	resolved := resolvedOptions{
		enabled:    boolValue(input.Enabled, true),
		clusters:   make(map[string]resolvedCluster),
		datastores: make(map[string]resolvedDatastore),
	}
	createDefault := len(input.Clusters) == 0 || input.URI != "" || input.Database != "" || input.Clusters[DefaultCluster].URI != ""
	for _, datastore := range input.Datastores {
		if datastore.Cluster == "" || datastore.Cluster == DefaultCluster {
			createDefault = true
		}
	}
	if createDefault {
		resolved.clusters[DefaultCluster] = resolvedCluster{uri: rootURI, timeout: rootTimeout, ping: rootPing}
		resolved.defaultCluster = DefaultCluster
		resolved.datastores[DefaultDatastore] = resolvedDatastore{
			cluster: DefaultCluster, database: rootDatabase, autoIndex: rootAutoIndex,
		}
		resolved.defaultDatastore = DefaultDatastore
	}

	clusterNames := sortedKeys(input.Clusters)
	for _, name := range clusterNames {
		if strings.TrimSpace(name) == "" {
			return resolvedOptions{}, fmt.Errorf("mongo cluster name cannot be empty")
		}
		cluster := input.Clusters[name]
		uri := firstNonBlank(cluster.URI, rootURI)
		if name != DefaultCluster && cluster.URI == "" {
			return resolvedOptions{}, fmt.Errorf("mongo cluster %s must define uri", name)
		}
		timeout := cluster.Timeout
		if timeout <= 0 {
			timeout = rootTimeout
		}
		resolved.clusters[name] = resolvedCluster{uri: uri, timeout: timeout, ping: cluster.Ping}
		if resolved.defaultCluster == "" {
			resolved.defaultCluster = name
		}
		clusterAutoIndex := boolValue(cluster.AutoIndex, rootAutoIndex)
		for _, datastoreName := range sortedKeys(cluster.Datastores) {
			datastore := cluster.Datastores[datastoreName]
			if datastore.Cluster != "" && datastore.Cluster != name {
				return resolvedOptions{}, fmt.Errorf("mongo datastore %s is nested under cluster %s but references cluster %s", datastoreName, name, datastore.Cluster)
			}
			database := firstNonBlank(datastore.Database, cluster.Database, rootDatabase)
			if err := addResolvedDatastore(&resolved, datastoreName, resolvedDatastore{
				cluster: name, database: database,
				autoIndex: boolValue(datastore.AutoIndex, clusterAutoIndex),
			}); err != nil {
				return resolvedOptions{}, err
			}
		}
	}

	for _, name := range sortedKeys(input.Datastores) {
		datastore := input.Datastores[name]
		clusterName := firstNonBlank(datastore.Cluster, DefaultCluster)
		cluster, ok := input.Clusters[clusterName]
		if !ok && clusterName != DefaultCluster {
			return resolvedOptions{}, ErrMissingCluster(clusterName)
		}
		if _, ok := resolved.clusters[clusterName]; !ok {
			return resolvedOptions{}, ErrMissingCluster(clusterName)
		}
		inheritedAutoIndex := rootAutoIndex
		inheritedDatabase := rootDatabase
		if clusterName != DefaultCluster || cluster.URI != "" {
			inheritedAutoIndex = boolValue(cluster.AutoIndex, rootAutoIndex)
			inheritedDatabase = firstNonBlank(cluster.Database, rootDatabase)
		}
		if err := addResolvedDatastore(&resolved, name, resolvedDatastore{
			cluster: clusterName, database: firstNonBlank(datastore.Database, inheritedDatabase),
			autoIndex: boolValue(datastore.AutoIndex, inheritedAutoIndex),
		}); err != nil {
			return resolvedOptions{}, err
		}
	}
	if len(resolved.clusters) == 0 {
		return resolvedOptions{}, fmt.Errorf("mongo configuration has no clusters")
	}
	return resolved, nil
}

func addResolvedDatastore(config *resolvedOptions, name string, datastore resolvedDatastore) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("mongo datastore name cannot be empty")
	}
	if _, exists := config.datastores[name]; exists {
		return fmt.Errorf("duplicate mongo datastore name: %s", name)
	}
	if strings.TrimSpace(datastore.database) == "" {
		return fmt.Errorf("mongo datastore %s must define database", name)
	}
	config.datastores[name] = datastore
	if config.defaultDatastore == "" {
		config.defaultDatastore = name
	}
	return nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
