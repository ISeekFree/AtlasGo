package redis

import (
	"testing"
	"time"
)

func TestKeyBuilderOf(t *testing.T) {
	builder := NewKeyBuilder("claw")
	if got := builder.Of("demo"); got != "claw:demo" {
		t.Fatalf("key = %q, want claw:demo", got)
	}
}

func TestParseConfigReadsClawRedisAndEnvironment(t *testing.T) {
	t.Setenv("REDIS_TEST_HOST", "redis.internal")

	config, err := ParseConfig([]byte(`
claw:
  redis:
    enabled: true
    host: ${REDIS_TEST_HOST:127.0.0.1}
    port: 6380
    database: 2
    username: app
    password: secret
    key-prefix: claw:test
    connect-timeout: 10s
    timeout: 5s
    pool:
      max-active: 20
      max-idle: 8
      min-idle: 2
      max-wait: 3s
`))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if !config.IsEnabled() || config.Host != "redis.internal" || config.Port != 6380 || config.DB != 2 {
		t.Fatalf("config = %#v", config)
	}
	merged := mergeOptions(config)
	if merged.Addr != "redis.internal:6380" {
		t.Fatalf("addr = %q", merged.Addr)
	}
	if merged.DialTimeout != 10*time.Second || merged.ReadTimeout != 5*time.Second || merged.WriteTimeout != 5*time.Second {
		t.Fatalf("timeouts = dial %s read %s write %s", merged.DialTimeout, merged.ReadTimeout, merged.WriteTimeout)
	}
	if merged.PoolSize != 20 || merged.MaxIdleConns != 8 || merged.MinIdleConns != 2 || merged.PoolTimeout != 3*time.Second {
		t.Fatalf("pool options = %#v", merged)
	}
}

func TestParseConfigSupportsDisabledRedis(t *testing.T) {
	config, err := ParseConfig([]byte("claw:\n  redis:\n    enabled: false\n"))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if config.IsEnabled() {
		t.Fatal("redis should be disabled")
	}
}
