package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

var managedRootKeys = []string{"model_instructions_file"}

func readTOMLFile(path string) (map[string]any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read TOML file %s: %w", path, err)
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return map[string]any{}, nil
	}
	var document map[string]any
	if err := toml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("decode TOML file %s: %w", path, err)
	}
	if document == nil {
		return map[string]any{}, nil
	}
	return document, nil
}

func writeTOMLFile(path string, document map[string]any) error {
	content, err := marshalTOMLDocument(document)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write TOML file %s: %w", path, err)
	}
	return nil
}

func extractManagedDocument(source map[string]any) map[string]any {
	managed := map[string]any{}
	for _, key := range managedRootKeys {
		if value, ok := source[key]; ok {
			managed[key] = deepCopy(value)
		}
	}
	if features, ok := source["features"].(map[string]any); ok {
		if value, ok := features["codex_hooks"]; ok {
			managed["features"] = map[string]any{"codex_hooks": deepCopy(value)}
		}
	}
	if agents, ok := source["agents"].(map[string]any); ok {
		if reviewer, ok := agents["reviewer"].(map[string]any); ok {
			if value, ok := reviewer["config_file"]; ok {
				managed["agents"] = map[string]any{"reviewer": map[string]any{"config_file": deepCopy(value)}}
			}
		}
	}
	return managed
}

func mergeManagedDocument(local, managed map[string]any) map[string]any {
	merged, _ := deepCopy(local).(map[string]any)
	if merged == nil {
		merged = map[string]any{}
	}
	stripManagedKeys(merged)
	deepMerge(merged, managed)
	pruneEmptyTables(merged)
	return merged
}

func stripManagedKeys(document map[string]any) {
	for _, key := range managedRootKeys {
		delete(document, key)
	}
	if features, ok := document["features"].(map[string]any); ok {
		delete(features, "codex_hooks")
		if len(features) == 0 {
			delete(document, "features")
		}
	}
	if agents, ok := document["agents"].(map[string]any); ok {
		if reviewer, ok := agents["reviewer"].(map[string]any); ok {
			delete(reviewer, "config_file")
			if len(reviewer) == 0 {
				delete(agents, "reviewer")
			}
		}
		if len(agents) == 0 {
			delete(document, "agents")
		}
	}
}

func deepMerge(destination, source map[string]any) {
	for key, value := range source {
		if valueMap, ok := value.(map[string]any); ok {
			current, ok := destination[key].(map[string]any)
			if !ok {
				destination[key] = deepCopy(valueMap)
				continue
			}
			deepMerge(current, valueMap)
			continue
		}
		destination[key] = deepCopy(value)
	}
}

func pruneEmptyTables(document map[string]any) {
	for key, value := range document {
		child, ok := value.(map[string]any)
		if !ok {
			continue
		}
		pruneEmptyTables(child)
		if len(child) == 0 {
			delete(document, key)
		}
	}
}

func deepCopy(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		copied := make(map[string]any, len(typed))
		for key, child := range typed {
			copied[key] = deepCopy(child)
		}
		return copied
	case []any:
		copied := make([]any, len(typed))
		for i, child := range typed {
			copied[i] = deepCopy(child)
		}
		return copied
	default:
		return typed
	}
}

func marshalTOMLDocument(document map[string]any) ([]byte, error) {
	var lines []string
	writeRoot(&lines, document)
	content := strings.TrimSpace(strings.Join(lines, "\n"))
	if content == "" {
		return []byte{}, nil
	}
	return []byte(content + "\n"), nil
}

func writeRoot(lines *[]string, document map[string]any) {
	scalars, tables := splitMap(document)
	for _, key := range scalars {
		*lines = append(*lines, fmt.Sprintf("%s = %s", formatKey(key), formatValue(document[key])))
	}
	if len(scalars) > 0 && len(tables) > 0 {
		*lines = append(*lines, "")
	}
	for index, key := range tables {
		writeTable(lines, []string{key}, document[key].(map[string]any))
		if index != len(tables)-1 {
			*lines = append(*lines, "")
		}
	}
}

func writeTable(lines *[]string, path []string, table map[string]any) {
	scalars, tables := splitMap(table)
	if len(scalars) > 0 {
		formattedPath := make([]string, 0, len(path))
		for _, part := range path {
			formattedPath = append(formattedPath, formatKey(part))
		}
		*lines = append(*lines, fmt.Sprintf("[%s]", strings.Join(formattedPath, ".")))
		for _, key := range scalars {
			*lines = append(*lines, fmt.Sprintf("%s = %s", formatKey(key), formatValue(table[key])))
		}
		if len(tables) > 0 {
			*lines = append(*lines, "")
		}
	}
	for index, key := range tables {
		writeTable(lines, append(append([]string{}, path...), key), table[key].(map[string]any))
		if index != len(tables)-1 {
			*lines = append(*lines, "")
		}
	}
}

func splitMap(document map[string]any) ([]string, []string) {
	var scalars []string
	var tables []string
	for _, key := range sortedStringKeys(document) {
		if _, ok := document[key].(map[string]any); ok {
			tables = append(tables, key)
			continue
		}
		scalars = append(scalars, key)
	}
	return scalars, tables
}

func sortedStringKeys(document map[string]any) []string {
	keys := make([]string, 0, len(document))
	for key := range document {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatKey(key string) string {
	if isBareKey(key) {
		return key
	}
	return strconv.Quote(key)
}

func isBareKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func formatValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strconv.Quote(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, formatValue(item))
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
	case time.Time:
		return typed.Format(time.RFC3339)
	default:
		panic(fmt.Sprintf("unsupported TOML value type: %T", value))
	}
}
