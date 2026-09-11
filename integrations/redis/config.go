package redis

import (
	"fmt"
	"os"
	"regexp"

	goredis "github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

var environmentPlaceholder = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::([^}]*))?}`)
var ErrDisabled = fmt.Errorf("redis integration is disabled")

// LoadConfig reads the framework.redis section from a YAML project configuration.
func LoadConfig(path string) (Options, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Options{}, err
	}
	return ParseConfig(data)
}

// ParseConfig parses either a full project YAML document containing framework.redis,
// or a YAML document whose root is already the Redis configuration.
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
		return Options{}, fmt.Errorf("parse redis config: %w", err)
	}
	node := documentRoot(&document)
	if framework := mappingValue(node, "framework"); framework != nil {
		if redisNode := mappingValue(framework, "redis"); redisNode != nil {
			node = redisNode
		} else {
			return Options{}, fmt.Errorf("parse redis config: framework.redis section is missing")
		}
	}
	if node == nil || node.Kind != yaml.MappingNode {
		return Options{}, fmt.Errorf("parse redis config: expected a mapping")
	}
	var config Options
	if err := node.Decode(&config); err != nil {
		return Options{}, fmt.Errorf("parse redis config: %w", err)
	}
	return config, nil
}

func NewClientFromConfig(path string) (*goredis.Client, error) {
	config, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	if !config.IsEnabled() {
		return nil, ErrDisabled
	}
	return NewClient(config), nil
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
