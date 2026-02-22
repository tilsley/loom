package yamlutil

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ParseNode parses a YAML string into a yaml.Node, preserving key order and document structure.
// Use this (with MarshalNode) when round-tripping existing documents.
func ParseNode(content string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0], nil
	}
	return &doc, nil
}

// MarshalNode serializes a yaml.Node to YAML with 2-space indentation.
func MarshalNode(node *yaml.Node) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}
	return buf.String(), nil
}

// GetMappingNode traverses a yaml.Node tree by mapping keys and returns the inner mapping node.
// Returns an error if any key is missing or a node is not a mapping.
func GetMappingNode(node *yaml.Node, keys ...string) (*yaml.Node, error) {
	current := node
	for _, key := range keys {
		if current.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("expected mapping node at %q, got kind %v", key, current.Kind)
		}
		found := false
		for i := 0; i+1 < len(current.Content); i += 2 {
			if current.Content[i].Value == key {
				current = current.Content[i+1]
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("key %q not found", key)
		}
	}
	return current, nil
}

// EnsureMappingNode traverses a yaml.Node tree by keys, creating missing intermediate
// mapping nodes as needed, and returns the final mapping node.
func EnsureMappingNode(node *yaml.Node, keys ...string) *yaml.Node {
	current := node
	for _, key := range keys {
		current = getOrCreateMapping(current, key)
	}
	return current
}

// SetScalar sets a scalar value at the given key in a mapping node.
// Updates the value in place if the key exists; appends a new pair if it does not.
func SetScalar(mapping *yaml.Node, key string, value interface{}) {
	strVal := fmt.Sprintf("%v", value)
	tag := scalarTag(value)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1].Value = strVal
			mapping.Content[i+1].Tag = tag
			mapping.Content[i+1].Kind = yaml.ScalarNode
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: strVal, Tag: tag},
	)
}

// DeleteKey removes a key-value pair from a mapping node. No-op if the key is not found.
func DeleteKey(mapping *yaml.Node, key string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

// SetNestedValue sets a scalar value at a nested key path in a yaml.Node tree.
// Traverses existing mapping nodes and creates missing intermediate mappings as needed.
func SetNestedValue(node *yaml.Node, value interface{}, keys ...string) {
	current := node
	for _, key := range keys[:len(keys)-1] {
		current = getOrCreateMapping(current, key)
	}
	SetScalar(current, keys[len(keys)-1], value)
}

// Parse unmarshals a YAML string into a generic map.
// Suitable for creating new documents; for round-tripping existing files use ParseNode.
func Parse(content string) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	return data, nil
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

// Marshal serializes a generic map to a YAML string with 2-space indentation.
// Suitable for creating new documents; for round-tripping existing files use ParseNode + MarshalNode.
func Marshal(data map[string]interface{}) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(data); err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}
	return buf.String(), nil
}

func getOrCreateMapping(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	child := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		child,
	)
	return child
}

func scalarTag(v interface{}) string {
	switch v.(type) {
	case bool:
		return "!!bool"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "!!int"
	case float32, float64:
		return "!!float"
	default:
		return "!!str"
	}
}
