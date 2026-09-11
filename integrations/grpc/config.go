package atlasgrpc

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var environmentPlaceholder = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::([^}]*))?}`)

type Config struct {
	Server ServerOptions `yaml:"server"`
	Client ClientOptions `yaml:"client"`
}

type ServerOptions struct {
	Addr string           `yaml:"addr"`
	Host string           `yaml:"host"`
	Port int              `yaml:"port"`
	Auth ServerAuthConfig `yaml:"auth"`
}

type ServerAuthConfig struct {
	Required   bool   `yaml:"required"`
	InnerToken string `yaml:"inner-token"`
}

func (s ServerOptions) ListenAddress() string {
	if strings.TrimSpace(s.Addr) != "" {
		return s.Addr
	}
	host := s.Host
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(s.Port))
}

func (s ServerOptions) AuthOptions() ServerAuthOptions {
	return ServerAuthOptions{Required: s.Auth.Required, InnerToken: s.Auth.InnerToken}
}

// LoadConfig reads the framework.grpc section from a YAML project configuration.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return ParseConfig(data)
}

// ParseConfig parses either a full project YAML document containing framework.grpc,
// or a YAML document whose root is already the gRPC configuration.
func ParseConfig(data []byte) (Config, error) {
	expanded := environmentPlaceholder.ReplaceAllStringFunc(string(data), func(value string) string {
		parts := environmentPlaceholder.FindStringSubmatch(value)
		if configured, ok := os.LookupEnv(parts[1]); ok {
			return configured
		}
		return parts[2]
	})

	var document yaml.Node
	if err := yaml.Unmarshal([]byte(expanded), &document); err != nil {
		return Config{}, fmt.Errorf("parse grpc config: %w", err)
	}
	node := documentRoot(&document)
	if framework := mappingValue(node, "framework"); framework != nil {
		if grpcNode := mappingValue(framework, "grpc"); grpcNode != nil {
			node = grpcNode
		} else {
			return Config{}, fmt.Errorf("parse grpc config: framework.grpc section is missing")
		}
	}
	if node == nil || node.Kind != yaml.MappingNode {
		return Config{}, fmt.Errorf("parse grpc config: expected a mapping")
	}
	var config Config
	if err := node.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("parse grpc config: %w", err)
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
