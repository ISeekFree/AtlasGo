package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

type indexedAccount struct {
	TenantID  string `bson:"tenantId" mongo:"index=uk_tenant_email,unique"`
	Email     string `bson:"email" mongo:"index=uk_tenant_email,unique,order=-1"`
	Status    string `bson:"status" mongo:"index,order=-1"`
	ExpiresAt int64  `bson:"expiresAt" mongo:"index=ttl_account,expire=3600"`
}

type loginRecord struct {
	AccountID string `bson:"accountId"`
	LoginAt   int64  `bson:"loginAt"`
}

func (loginRecord) MongoIndexes() []IndexDefinition {
	return []IndexDefinition{
		CompoundIndex("idx_login_account_time", Asc("accountId"), Desc("loginAt")),
	}
}

type salesOrder struct {
	TenantID string `bson:"tenantId"`
	OrderNo  string `bson:"orderNo"`
}

func (salesOrder) MongoIndexes() []IndexDefinition {
	return []IndexDefinition{
		CompoundIndex("uk_sales_order_tenant_no", Asc("tenantId"), Asc("orderNo")).WithUnique(),
	}
}

func (*indexedAccount) MongoDatastore() string  { return "accounts" }
func (*indexedAccount) MongoCollection() string { return "account" }

func TestIndexesForBuildsCompoundAndTTLIndexes(t *testing.T) {
	models, err := IndexesFor(indexedAccount{})
	if err != nil {
		t.Fatalf("IndexesFor() error = %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("models = %d, want 3", len(models))
	}

	compound, ok := models[0].Keys.(bson.D)
	if !ok {
		t.Fatalf("compound keys type = %T", models[0].Keys)
	}
	if len(compound) != 2 || compound[0].Key != "tenantId" || compound[1].Key != "email" {
		t.Fatalf("compound keys = %#v", compound)
	}
	if compound[0].Value != int32(1) || compound[1].Value != int32(-1) {
		t.Fatalf("compound key directions = %#v", compound)
	}
	if models[0].Options.Name == nil || *models[0].Options.Name != "uk_tenant_email" {
		t.Fatalf("compound name = %#v", models[0].Options.Name)
	}
	if models[0].Options.Unique == nil || !*models[0].Options.Unique {
		t.Fatal("compound index should be unique")
	}

	status := models[1].Keys.(bson.D)
	if got := status[0].Value; got != int32(-1) {
		t.Fatalf("status order = %#v", got)
	}
	if models[2].Options.ExpireAfterSeconds == nil || *models[2].Options.ExpireAfterSeconds != 3600 {
		t.Fatalf("TTL = %#v", models[2].Options.ExpireAfterSeconds)
	}
}

func TestResolveEntitiesUsesEntityMetadata(t *testing.T) {
	entities, err := resolveEntities([]EntityMapping{{Model: indexedAccount{}}}, map[string]resolvedDatastore{
		"accounts": {cluster: "primary", database: "accounts"},
	})
	if err != nil {
		t.Fatalf("resolveEntities() error = %v", err)
	}
	typeOf, _ := entityType(indexedAccount{})
	entity := entities[typeOf]
	if entity.datastore != "accounts" || entity.collection != "account" {
		t.Fatalf("entity mapping = %#v", entity)
	}
}

func TestIndexesForBuildsEntityLevelCompoundIndex(t *testing.T) {
	models, err := IndexesFor(loginRecord{})
	if err != nil {
		t.Fatalf("IndexesFor() error = %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("models = %d, want 1", len(models))
	}
	keys := models[0].Keys.(bson.D)
	if len(keys) != 2 || keys[0] != (bson.E{Key: "accountId", Value: int32(1)}) || keys[1] != (bson.E{Key: "loginAt", Value: int32(-1)}) {
		t.Fatalf("compound keys = %#v", keys)
	}
	if models[0].Options.Name == nil || *models[0].Options.Name != "idx_login_account_time" {
		t.Fatalf("index name = %#v", models[0].Options.Name)
	}
}

func TestIndexesForBuildsUniqueEntityLevelCompoundIndex(t *testing.T) {
	models, err := IndexesFor(salesOrder{})
	if err != nil {
		t.Fatalf("IndexesFor() error = %v", err)
	}
	if len(models) != 1 || models[0].Options.Unique == nil || !*models[0].Options.Unique {
		t.Fatalf("unique compound index = %#v", models)
	}
}

func TestIndexesForRejectsUnknownTagOption(t *testing.T) {
	type broken struct {
		Value string `bson:"value" mongo:"index,wat"`
	}
	if _, err := IndexesFor(broken{}); err == nil {
		t.Fatal("expected invalid tag error")
	}
}
