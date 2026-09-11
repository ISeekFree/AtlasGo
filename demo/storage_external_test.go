package main

import (
	"context"
	"os"
	"testing"
	"time"

	atlasmongo "github.com/ISeekFree/AtlasGo/integrations/mongo"
	atlasredis "github.com/ISeekFree/AtlasGo/integrations/redis"
)

const (
	defaultExternalRedisHost     = "r-bp1qbhon70753useripd.redis.rds.aliyuncs.com"
	defaultExternalRedisPassword = "qYYeNBu6xpsZrnDuLuwEAcvde"
	defaultExternalRedisDB       = 1
	defaultExternalMongoURI      = "mongodb://root:iseekliqNoegkpsZrnqlchwE72we073mc@115.29.220.3:16673/admin?replicaSet=rs0&directConnection=true"
	defaultExternalMongoDatabase = "atlas-sdk-demo"
)

func TestDemoExternalRedisAndMongoBasicOperations(t *testing.T) {
	if os.Getenv("ATLAS_DEMO_EXTERNAL_TEST") != "1" {
		t.Skip("set ATLAS_DEMO_EXTERNAL_TEST=1 to run Redis/Mongo external connection checks")
	}

	config := externalStorageConfig()
	engine, cleanup, err := newDemoApp(context.Background(), config)
	if err != nil {
		t.Fatalf("newDemoApp() error = %v", err)
	}
	defer cleanup(context.Background())

	redisResp := request(engine, "/demo/redis/basic", "")
	if redisResp.Code != 0 {
		t.Fatalf("redis basic code = %d msg = %s", redisResp.Code, redisResp.Msg)
	}
	redisData := redisResp.Data.(map[string]any)
	if redisData["value"] != "atlas-sdk-demo" {
		t.Fatalf("unexpected redis value: %#v", redisData)
	}

	mongoResp := request(engine, "/demo/mongo/basic", "")
	if mongoResp.Code != 0 {
		t.Fatalf("mongo basic code = %d msg = %s", mongoResp.Code, mongoResp.Msg)
	}
	mongoData := mongoResp.Data.(map[string]any)
	if mongoData["name"] != "atlas-sdk-demo" {
		t.Fatalf("unexpected mongo value: %#v", mongoData)
	}
}

func externalStorageConfig() Config {
	return Config{
		Mongo: &atlasmongo.Options{
			Enabled:  atlasmongo.Bool(true),
			URI:      env("ATLAS_DEMO_MONGO_URI", defaultExternalMongoURI),
			Database: env("ATLAS_DEMO_MONGO_DATABASE", defaultExternalMongoDatabase),
			Timeout:  shortTimeout(),
			Datastores: map[string]atlasmongo.DatastoreOptions{
				"demo": {
					Cluster:  atlasmongo.DefaultCluster,
					Database: env("ATLAS_DEMO_MONGO_DATABASE", defaultExternalMongoDatabase),
				},
			},
		},
		Redis: &atlasredis.Options{
			Enabled:   atlasredis.Bool(true),
			Host:      env("ATLAS_DEMO_REDIS_HOST", defaultExternalRedisHost),
			Port:      envInt("ATLAS_DEMO_REDIS_PORT", 6379),
			Password:  env("ATLAS_DEMO_REDIS_PASSWORD", defaultExternalRedisPassword),
			DB:        envInt("ATLAS_DEMO_REDIS_DATABASE", defaultExternalRedisDB),
			KeyPrefix: env("ATLAS_DEMO_REDIS_PREFIX", "atlas:demo"),
			Pool: atlasredis.PoolOptions{
				MaxActive: envInt("ATLAS_DEMO_REDIS_MAX_ACTIVE", 1000),
				MaxIdle:   envInt("ATLAS_DEMO_REDIS_MAX_IDLE", 2),
				MinIdle:   envInt("ATLAS_DEMO_REDIS_MIN_IDLE", 100),
				MaxWait:   time.Duration(envInt("ATLAS_DEMO_REDIS_MAX_WAIT_SECONDS", 8)) * time.Second,
			},
		},
	}
}
