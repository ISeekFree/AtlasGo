package main

import "testing"

func TestConfigFromFileLoadsMongoRedisAndGRPCSections(t *testing.T) {
	t.Setenv("CLAW_MONGO_ENABLED", "false")
	t.Setenv("CLAW_REDIS_ENABLED", "false")
	t.Setenv("CLAW_DEMO_REDIS_HOST", "redis.demo.internal")

	config, err := configFromFile("config.yaml")
	if err != nil {
		t.Fatalf("configFromFile() error = %v", err)
	}
	if config.Mongo == nil || config.Mongo.IsEnabled() {
		t.Fatalf("mongo config = %#v", config.Mongo)
	}
	if config.Redis == nil || config.Redis.IsEnabled() {
		t.Fatalf("redis config = %#v", config.Redis)
	}
	if config.Redis.Host != "redis.demo.internal" || config.Redis.Port != 6379 {
		t.Fatalf("redis endpoint = %s:%d", config.Redis.Host, config.Redis.Port)
	}
	if config.Redis.Pool.MaxActive != 8 || config.Redis.Pool.MaxIdle != 8 {
		t.Fatalf("redis pool = %#v", config.Redis.Pool)
	}
	if config.GRPC == nil || config.GRPC.Server.ListenAddress() != "127.0.0.1:19090" {
		t.Fatalf("grpc server config = %#v", config.GRPC)
	}
	if got := config.GRPC.Client.Channels["local"].Target; got != "127.0.0.1:19090" {
		t.Fatalf("grpc local target = %q", got)
	}
	if got := config.GRPC.Client.Channels["account-service"].Target; got != "127.0.0.1:19091" {
		t.Fatalf("grpc account-service target = %q", got)
	}
	if got := config.GRPC.Client.Channels["order-service"].MaxInboundMessageSize; got != 8388608 {
		t.Fatalf("grpc order-service max inbound size = %d", got)
	}
}
