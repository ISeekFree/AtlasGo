package mongo

import (
	"context"
	"fmt"
	"reflect"

	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Registry struct {
	clients          map[string]*mongodriver.Client
	databases        map[string]*mongodriver.Database
	datastores       map[string]resolvedDatastore
	entities         map[reflect.Type]resolvedEntity
	defaultCluster   string
	defaultDatastore string
}

func Connect(ctx context.Context, input Options) (*Registry, error) {
	config, err := resolveOptions(input)
	if err != nil {
		return nil, err
	}
	if !config.enabled {
		return nil, ErrDisabled
	}
	entities, err := resolveEntities(input.Entities, config.datastores)
	if err != nil {
		return nil, err
	}

	clients := make(map[string]*mongodriver.Client, len(config.clusters))
	for _, name := range sortedKeys(config.clusters) {
		client, connectErr := connectCluster(ctx, config.clusters[name])
		if connectErr != nil {
			_ = closeClients(clients, context.Background())
			return nil, fmt.Errorf("connect mongo cluster %s: %w", name, connectErr)
		}
		clients[name] = client
	}

	databases := make(map[string]*mongodriver.Database, len(config.datastores))
	for name, datastore := range config.datastores {
		databases[name] = clients[datastore.cluster].Database(datastore.database)
	}
	registry := &Registry{
		clients: clients, databases: databases, datastores: config.datastores, entities: entities,
		defaultCluster: config.defaultCluster, defaultDatastore: config.defaultDatastore,
	}
	if err := registry.ensureEntityIndexes(ctx); err != nil {
		_ = registry.Close(context.Background())
		return nil, err
	}
	return registry, nil
}

func ConnectConfig(ctx context.Context, path string, entities ...EntityMapping) (*Registry, error) {
	config, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	config.Entities = append(config.Entities, entities...)
	return Connect(ctx, config)
}

func (r *Registry) Client(names ...string) *mongodriver.Client {
	name := r.defaultCluster
	if len(names) > 0 && names[0] != "" {
		name = names[0]
	}
	if client, ok := r.clients[name]; ok {
		return client
	}
	return r.clients[r.defaultCluster]
}

func (r *Registry) RequireClient(name string) (*mongodriver.Client, error) {
	if name == "" {
		name = r.defaultCluster
	}
	client, ok := r.clients[name]
	if !ok {
		return nil, ErrMissingCluster(name)
	}
	return client, nil
}

func (r *Registry) Database(names ...string) *mongodriver.Database {
	name := r.defaultDatastore
	if len(names) > 0 && names[0] != "" {
		name = names[0]
	}
	if database, ok := r.databases[name]; ok {
		return database
	}
	return r.databases[r.defaultDatastore]
}

func (r *Registry) RequireDatabase(name string) (*mongodriver.Database, error) {
	if name == "" {
		name = r.defaultDatastore
	}
	database, ok := r.databases[name]
	if !ok {
		return nil, ErrMissingDatastore(name)
	}
	return database, nil
}

func (r *Registry) Collection(collection string, datastore ...string) *mongodriver.Collection {
	return r.Database(datastore...).Collection(collection)
}

func (r *Registry) Close(ctx context.Context) error {
	return closeClients(r.clients, ctx)
}

func connectCluster(ctx context.Context, cluster resolvedCluster) (*mongodriver.Client, error) {
	connectCtx, cancel := context.WithTimeout(ctx, cluster.timeout)
	defer cancel()

	client, err := mongodriver.Connect(connectCtx, options.Client().ApplyURI(cluster.uri))
	if err != nil {
		return nil, err
	}
	if cluster.ping {
		if err := client.Ping(connectCtx, readpref.Primary()); err != nil {
			_ = client.Disconnect(context.Background())
			return nil, err
		}
	}
	return client, nil
}

func closeClients(clients map[string]*mongodriver.Client, ctx context.Context) error {
	var first error
	seen := map[*mongodriver.Client]struct{}{}
	for _, client := range clients {
		if client == nil {
			continue
		}
		if _, ok := seen[client]; ok {
			continue
		}
		seen[client] = struct{}{}
		if err := client.Disconnect(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

var ErrDisabled = fmt.Errorf("mongo integration is disabled")

type MissingClusterError struct{ Name string }

func ErrMissingCluster(name string) MissingClusterError { return MissingClusterError{Name: name} }

func (e MissingClusterError) Error() string { return "missing mongo cluster: " + e.Name }

type MissingDatastoreError struct{ Name string }

func ErrMissingDatastore(name string) MissingDatastoreError { return MissingDatastoreError{Name: name} }

func (e MissingDatastoreError) Error() string { return "missing mongo datastore: " + e.Name }
