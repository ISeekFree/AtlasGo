package main

import (
	"os"
	"strconv"
	"time"

	atlasgrpc "github.com/ISeekFree/AtlasGo/integrations/grpc"
	atlasmongo "github.com/ISeekFree/AtlasGo/integrations/mongo"
	atlasredis "github.com/ISeekFree/AtlasGo/integrations/redis"
)

type Config struct {
	HTTPAddr string
	GRPC     *atlasgrpc.Config
	Mongo    *atlasmongo.Options
	Redis    *atlasredis.Options
}

func configFromEnv() (Config, error) {
	config, err := configFromFile(env("ATLAS_DEMO_CONFIG", "config.yaml"))
	if err != nil {
		return Config{}, err
	}
	config.HTTPAddr = env("ATLAS_DEMO_HTTP_ADDR", ":8080")
	return config, nil
}

func configFromFile(path string) (Config, error) {
	mongoConfig, err := atlasmongo.LoadConfig(path)
	if err != nil {
		return Config{}, err
	}
	redisConfig, err := atlasredis.LoadConfig(path)
	if err != nil {
		return Config{}, err
	}
	grpcConfig, err := atlasgrpc.LoadConfig(path)
	if err != nil {
		return Config{}, err
	}
	return Config{GRPC: &grpcConfig, Mongo: &mongoConfig, Redis: &redisConfig}, nil
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func shortTimeout() time.Duration {
	return 3 * time.Second
}
