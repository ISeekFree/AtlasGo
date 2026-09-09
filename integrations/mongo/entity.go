package mongo

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EntityMetadata lets an entity declare its datastore and collection. Explicit
// values passed to MapEntity take precedence over these methods.
type EntityMetadata interface {
	MongoDatastore() string
	MongoCollection() string
}

// EntityIndexes is the Go equivalent of an entity-level @Indexes annotation.
// It is the preferred way to declare compound indexes.
type EntityIndexes interface {
	MongoIndexes() []IndexDefinition
}

type IndexKey struct {
	Field string
	Order any
}

type IndexDefinition struct {
	Name               string
	Keys               []IndexKey
	Unique             bool
	Sparse             bool
	Hidden             bool
	ExpireAfterSeconds *int32
}

func CompoundIndex(name string, keys ...IndexKey) IndexDefinition {
	return IndexDefinition{Name: name, Keys: keys}
}

func Asc(field string) IndexKey {
	return IndexKey{Field: field, Order: int32(1)}
}

func Desc(field string) IndexKey {
	return IndexKey{Field: field, Order: int32(-1)}
}

func Text(field string) IndexKey {
	return IndexKey{Field: field, Order: "text"}
}

func Hashed(field string) IndexKey {
	return IndexKey{Field: field, Order: "hashed"}
}

func (i IndexDefinition) WithUnique() IndexDefinition {
	i.Unique = true
	return i
}

func (i IndexDefinition) WithSparse() IndexDefinition {
	i.Sparse = true
	return i
}

func (i IndexDefinition) WithHidden() IndexDefinition {
	i.Hidden = true
	return i
}

func (i IndexDefinition) WithExpireAfter(seconds int32) IndexDefinition {
	i.ExpireAfterSeconds = &seconds
	return i
}

func (i IndexDefinition) Model() (mongodriver.IndexModel, error) {
	if len(i.Keys) == 0 {
		return mongodriver.IndexModel{}, fmt.Errorf("mongo index %q must define at least one key", i.Name)
	}
	keys := make(bson.D, 0, len(i.Keys))
	for _, key := range i.Keys {
		if strings.TrimSpace(key.Field) == "" {
			return mongodriver.IndexModel{}, fmt.Errorf("mongo index %q has an empty key field", i.Name)
		}
		if err := validateIndexOrder(key.Order); err != nil {
			return mongodriver.IndexModel{}, fmt.Errorf("mongo index %q field %s: %w", i.Name, key.Field, err)
		}
		keys = append(keys, bson.E{Key: key.Field, Value: key.Order})
	}
	indexOptions := options.Index()
	if i.Name != "" {
		indexOptions.SetName(i.Name)
	}
	if i.Unique {
		indexOptions.SetUnique(true)
	}
	if i.Sparse {
		indexOptions.SetSparse(true)
	}
	if i.Hidden {
		indexOptions.SetHidden(true)
	}
	if i.ExpireAfterSeconds != nil {
		if *i.ExpireAfterSeconds < 0 {
			return mongodriver.IndexModel{}, fmt.Errorf("mongo index %q has a negative expire-after value", i.Name)
		}
		indexOptions.SetExpireAfterSeconds(*i.ExpireAfterSeconds)
	}
	return mongodriver.IndexModel{Keys: keys, Options: indexOptions}, nil
}

type EntityMapping struct {
	Model      any
	Datastore  string
	Collection string
}

func MapEntity(model any, datastore, collection string) EntityMapping {
	return EntityMapping{Model: model, Datastore: datastore, Collection: collection}
}

type resolvedEntity struct {
	typeOf     reflect.Type
	model      any
	datastore  string
	collection string
}

func resolveEntities(mappings []EntityMapping, datastores map[string]resolvedDatastore) (map[reflect.Type]resolvedEntity, error) {
	entities := make(map[reflect.Type]resolvedEntity, len(mappings))
	for _, mapping := range mappings {
		typeOf, err := entityType(mapping.Model)
		if err != nil {
			return nil, err
		}
		datastore, collection := mapping.Datastore, mapping.Collection
		if metadata, ok := entityMetadata(mapping.Model, typeOf); ok {
			if datastore == "" {
				datastore = metadata.MongoDatastore()
			}
			if collection == "" {
				collection = metadata.MongoCollection()
			}
		}
		if datastore == "" {
			datastore = DefaultDatastore
		}
		if _, ok := datastores[datastore]; !ok {
			return nil, fmt.Errorf("mongo entity %s references unknown datastore %s", typeOf, datastore)
		}
		if strings.TrimSpace(collection) == "" {
			return nil, fmt.Errorf("mongo entity %s must define collection", typeOf)
		}
		if _, exists := entities[typeOf]; exists {
			return nil, fmt.Errorf("duplicate mongo entity mapping: %s", typeOf)
		}
		entities[typeOf] = resolvedEntity{typeOf: typeOf, model: mapping.Model, datastore: datastore, collection: collection}
	}
	return entities, nil
}

func entityType(model any) (reflect.Type, error) {
	if model == nil {
		return nil, fmt.Errorf("mongo entity model cannot be nil")
	}
	typeOf := reflect.TypeOf(model)
	for typeOf.Kind() == reflect.Pointer || typeOf.Kind() == reflect.Slice || typeOf.Kind() == reflect.Array {
		typeOf = typeOf.Elem()
	}
	if typeOf.Kind() != reflect.Struct {
		return nil, fmt.Errorf("mongo entity model must be a struct, got %s", typeOf)
	}
	return typeOf, nil
}

func entityMetadata(model any, typeOf reflect.Type) (EntityMetadata, bool) {
	if metadata, ok := model.(EntityMetadata); ok {
		return metadata, true
	}
	metadata, ok := reflect.New(typeOf).Interface().(EntityMetadata)
	return metadata, ok
}

func (r *Registry) CollectionFor(model any) (*mongodriver.Collection, error) {
	entity, err := r.entity(model)
	if err != nil {
		return nil, err
	}
	return r.databases[entity.datastore].Collection(entity.collection), nil
}

func (r *Registry) EntityDatastore(model any) (string, error) {
	entity, err := r.entity(model)
	if err != nil {
		return "", err
	}
	return entity.datastore, nil
}

func (r *Registry) entity(model any) (resolvedEntity, error) {
	typeOf, err := entityType(model)
	if err != nil {
		return resolvedEntity{}, err
	}
	entity, ok := r.entities[typeOf]
	if !ok {
		return resolvedEntity{}, fmt.Errorf("mongo entity is not registered: %s", typeOf)
	}
	return entity, nil
}

func (r *Registry) ensureEntityIndexes(ctx context.Context) error {
	types := make([]reflect.Type, 0, len(r.entities))
	for typeOf := range r.entities {
		types = append(types, typeOf)
	}
	sort.Slice(types, func(i, j int) bool { return types[i].String() < types[j].String() })
	for _, typeOf := range types {
		entity := r.entities[typeOf]
		if !r.datastores[entity.datastore].autoIndex {
			continue
		}
		models, err := IndexesFor(entity.model)
		if err != nil {
			return fmt.Errorf("mongo entity %s: %w", typeOf, err)
		}
		if len(models) == 0 {
			continue
		}
		collection := r.databases[entity.datastore].Collection(entity.collection)
		if _, err := collection.Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("create indexes for mongo entity %s: %w", typeOf, err)
		}
	}
	return nil
}

// EnsureIndexes creates explicit index models idempotently on a collection.
func (r *Registry) EnsureIndexes(ctx context.Context, datastore, collection string, models ...mongodriver.IndexModel) error {
	database, err := r.RequireDatabase(datastore)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return nil
	}
	_, err = database.Collection(collection).Indexes().CreateMany(ctx, models)
	return err
}

// IndexesFor builds MongoDB index models from `mongo` field tags. Fields with
// the same explicit index name form one compound index in struct field order.
// Example: `mongo:"index=uk_tenant_email,unique,order=1"`.
func IndexesFor(model any) ([]mongodriver.IndexModel, error) {
	typeOf, err := entityType(model)
	if err != nil {
		return nil, err
	}
	builders := make(map[string]*entityIndex)
	order := make([]string, 0)
	if err := scanIndexFields(typeOf, builders, &order); err != nil {
		return nil, err
	}
	models := make([]mongodriver.IndexModel, 0, len(order))
	if provider, ok := entityIndexes(model, typeOf); ok {
		for _, definition := range provider.MongoIndexes() {
			indexModel, modelErr := definition.Model()
			if modelErr != nil {
				return nil, modelErr
			}
			models = append(models, indexModel)
		}
	}
	for _, key := range order {
		index := builders[key]
		models = append(models, mongodriver.IndexModel{Keys: index.keys, Options: index.options})
	}
	return models, nil
}

func entityIndexes(model any, typeOf reflect.Type) (EntityIndexes, bool) {
	if indexes, ok := model.(EntityIndexes); ok {
		return indexes, true
	}
	indexes, ok := reflect.New(typeOf).Interface().(EntityIndexes)
	return indexes, ok
}

type entityIndex struct {
	keys    bson.D
	options *options.IndexOptions
}

func scanIndexFields(typeOf reflect.Type, indexes map[string]*entityIndex, order *[]string) error {
	for fieldIndex := 0; fieldIndex < typeOf.NumField(); fieldIndex++ {
		field := typeOf.Field(fieldIndex)
		if field.PkgPath != "" {
			continue
		}
		bsonName, inline, skip := bsonField(field)
		if skip {
			continue
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if inline && fieldType.Kind() == reflect.Struct {
			if err := scanIndexFields(fieldType, indexes, order); err != nil {
				return err
			}
			continue
		}
		tag := strings.TrimSpace(field.Tag.Get("mongo"))
		if tag != "" && tag != "-" {
			if err := addTaggedIndex(typeOf, field, bsonName, tag, indexes, order); err != nil {
				return err
			}
		}
	}
	return nil
}

func bsonField(field reflect.StructField) (name string, inline bool, skip bool) {
	parts := strings.Split(field.Tag.Get("bson"), ",")
	if parts[0] == "-" {
		return "", false, true
	}
	name = parts[0]
	if name == "" {
		name = strings.ToLower(field.Name)
	}
	for _, option := range parts[1:] {
		if option == "inline" {
			inline = true
		}
	}
	return name, inline, false
}

func addTaggedIndex(typeOf reflect.Type, field reflect.StructField, path, tag string, indexes map[string]*entityIndex, order *[]string) error {
	parts := strings.Split(tag, ",")
	name := ""
	direction := any(int32(1))
	unique, sparse, hidden := false, false, false
	var expire *int32
	hasIndex := false
	for _, raw := range parts {
		part := strings.TrimSpace(raw)
		key, value, hasValue := strings.Cut(part, "=")
		switch key {
		case "index":
			hasIndex = true
			if hasValue {
				name = strings.TrimSpace(value)
			}
		case "name":
			name, hasIndex = strings.TrimSpace(value), true
		case "unique":
			unique, hasIndex = true, true
		case "sparse":
			sparse, hasIndex = true, true
		case "hidden":
			hidden, hasIndex = true, true
		case "order":
			parsed, err := parseIndexOrder(value)
			if err != nil {
				return fmt.Errorf("field %s.%s: %w", typeOf, field.Name, err)
			}
			direction, hasIndex = parsed, true
		case "expire":
			seconds, err := strconv.ParseInt(value, 10, 32)
			if err != nil || seconds < 0 {
				return fmt.Errorf("field %s.%s: invalid mongo index expire %q", typeOf, field.Name, value)
			}
			seconds32 := int32(seconds)
			expire, hasIndex = &seconds32, true
		case "":
		default:
			return fmt.Errorf("field %s.%s: unknown mongo index option %q", typeOf, field.Name, key)
		}
	}
	if !hasIndex {
		return nil
	}
	group := name
	if group == "" {
		group = typeOf.PkgPath() + "." + typeOf.Name() + ":" + path
	}
	index, exists := indexes[group]
	if !exists {
		index = &entityIndex{options: options.Index()}
		if name != "" {
			index.options.SetName(name)
		}
		indexes[group] = index
		*order = append(*order, group)
	}
	index.keys = append(index.keys, bson.E{Key: path, Value: direction})
	if unique {
		index.options.SetUnique(true)
	}
	if sparse {
		index.options.SetSparse(true)
	}
	if hidden {
		index.options.SetHidden(true)
	}
	if expire != nil {
		index.options.SetExpireAfterSeconds(*expire)
	}
	return nil
}

func parseIndexOrder(value string) (any, error) {
	switch strings.TrimSpace(value) {
	case "", "1", "asc", "ASC":
		return int32(1), nil
	case "-1", "desc", "DESC":
		return int32(-1), nil
	case "2d", "2dsphere", "geoHaystack", "hashed", "text":
		return strings.TrimSpace(value), nil
	default:
		return nil, fmt.Errorf("invalid mongo index order %q", value)
	}
}

func validateIndexOrder(value any) error {
	switch order := value.(type) {
	case int:
		if order == 1 || order == -1 {
			return nil
		}
	case int32:
		if order == 1 || order == -1 {
			return nil
		}
	case int64:
		if order == 1 || order == -1 {
			return nil
		}
	case string:
		switch order {
		case "2d", "2dsphere", "geoHaystack", "hashed", "text":
			return nil
		}
	}
	return fmt.Errorf("invalid mongo index order %#v", value)
}
