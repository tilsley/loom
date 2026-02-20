package yamlutil

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Parse unmarshals a YAML string into a generic map.
func Parse(content string) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	return data, nil
}

// Marshal serializes a generic map to a YAML string.
func Marshal(data map[string]interface{}) (string, error) {
	out, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}
	return string(out), nil
}

// GetMap traverses a nested map by the given keys and returns the inner map.
func GetMap(data map[string]interface{}, keys ...string) (map[string]interface{}, error) {
	current := data
	for _, key := range keys {
		val, ok := current[key]
		if !ok {
			return nil, fmt.Errorf("key %q not found", key)
		}
		next, ok := val.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("key %q is not a map", key)
		}
		current = next
	}
	return current, nil
}

// SetNested sets a value at the given key path, creating intermediate maps as needed.
func SetNested(data map[string]interface{}, value interface{}, keys ...string) {
	current := data
	for _, key := range keys[:len(keys)-1] {
		next, ok := current[key].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[key] = next
		}
		current = next
	}
	current[keys[len(keys)-1]] = value
}
