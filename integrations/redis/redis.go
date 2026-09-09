package redis

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Options struct {
	Enabled        *bool         `yaml:"enabled"`
	Addr           string        `yaml:"addr"`
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	Username       string        `yaml:"username"`
	Password       string        `yaml:"password"`
	DB             int           `yaml:"database"`
	KeyPrefix      string        `yaml:"key-prefix"`
	ClientName     string        `yaml:"client-name"`
	ConnectTimeout time.Duration `yaml:"connect-timeout"`
	Timeout        time.Duration `yaml:"timeout"`
	Pool           PoolOptions   `yaml:"pool"`
	PoolSize       int           `yaml:"pool-size"`
	MinIdleConns   int           `yaml:"min-idle-conns"`
	MaxIdleConns   int           `yaml:"max-idle-conns"`
	PoolTimeout    time.Duration `yaml:"pool-timeout"`
	DialTimeout    time.Duration `yaml:"dial-timeout"`
	ReadTimeout    time.Duration `yaml:"read-timeout"`
	WriteTimeout   time.Duration `yaml:"write-timeout"`
}

type PoolOptions struct {
	MaxActive int           `yaml:"max-active"`
	MaxIdle   int           `yaml:"max-idle"`
	MinIdle   int           `yaml:"min-idle"`
	MaxWait   time.Duration `yaml:"max-wait"`
}

func DefaultOptions() Options {
	return Options{
		Enabled:      Bool(true),
		Addr:         "127.0.0.1:6379",
		Host:         "127.0.0.1",
		Port:         6379,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

func Bool(value bool) *bool {
	return &value
}

func (o Options) IsEnabled() bool {
	return o.Enabled == nil || *o.Enabled
}

func NewClient(input Options) *goredis.Client {
	options := mergeOptions(input)
	return goredis.NewClient(&goredis.Options{
		Addr:         options.Addr,
		Username:     options.Username,
		Password:     options.Password,
		DB:           options.DB,
		ClientName:   options.ClientName,
		PoolSize:     options.PoolSize,
		MinIdleConns: options.MinIdleConns,
		MaxIdleConns: options.MaxIdleConns,
		PoolTimeout:  options.PoolTimeout,
		DialTimeout:  options.DialTimeout,
		ReadTimeout:  options.ReadTimeout,
		WriteTimeout: options.WriteTimeout,
	})
}

func Ping(ctx context.Context, client *goredis.Client) error {
	return client.Ping(ctx).Err()
}

type KeyBuilder struct {
	Prefix string
}

func NewKeyBuilder(prefix string) KeyBuilder {
	return KeyBuilder{Prefix: prefix}
}

func (k KeyBuilder) Of(key string) string {
	prefix := strings.TrimSpace(k.Prefix)
	if prefix == "" {
		return key
	}
	if strings.HasSuffix(prefix, ":") {
		return prefix + key
	}
	return prefix + ":" + key
}

func mergeOptions(input Options) Options {
	defaults := DefaultOptions()
	defaults.Enabled = input.Enabled
	if input.Addr != "" {
		defaults.Addr = input.Addr
	} else if input.Host != "" || input.Port > 0 {
		host := input.Host
		if host == "" {
			host = defaults.Host
		}
		port := input.Port
		if port <= 0 {
			port = defaults.Port
		}
		defaults.Addr = net.JoinHostPort(host, strconv.Itoa(port))
	}
	if input.Host != "" {
		defaults.Host = input.Host
	}
	if input.Port > 0 {
		defaults.Port = input.Port
	}
	defaults.Username = input.Username
	defaults.Password = input.Password
	defaults.DB = input.DB
	defaults.KeyPrefix = input.KeyPrefix
	defaults.ClientName = input.ClientName
	defaults.Pool = input.Pool
	defaults.PoolSize = firstNonZero(input.PoolSize, input.Pool.MaxActive)
	defaults.MinIdleConns = firstNonZero(input.MinIdleConns, input.Pool.MinIdle)
	defaults.MaxIdleConns = firstNonZero(input.MaxIdleConns, input.Pool.MaxIdle)
	if input.PoolTimeout != 0 {
		defaults.PoolTimeout = input.PoolTimeout
	} else if input.Pool.MaxWait != 0 {
		defaults.PoolTimeout = input.Pool.MaxWait
	}
	if input.DialTimeout != 0 {
		defaults.DialTimeout = input.DialTimeout
	} else if input.ConnectTimeout != 0 {
		defaults.DialTimeout = input.ConnectTimeout
	}
	if input.ReadTimeout != 0 {
		defaults.ReadTimeout = input.ReadTimeout
	} else if input.Timeout != 0 {
		defaults.ReadTimeout = input.Timeout
	}
	if input.WriteTimeout != 0 {
		defaults.WriteTimeout = input.WriteTimeout
	} else if input.Timeout != 0 {
		defaults.WriteTimeout = input.Timeout
	}
	return defaults
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
