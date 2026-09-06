package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Structured imports merge user-owned records while preserving every value
// that already exists in Tiny. This is intentionally limited to known DSH
// formats; arbitrary session and attachment files remain path-level Tiny-wins.
const structuredImportLimit = 64 << 20

func structuredImportKind(rel string) string {
	slash := filepath.ToSlash(rel)
	switch slash {
	case "settings.yaml":
		return "settings"
	case ".credentials.yaml":
		return "credentials"
	}
	if strings.HasPrefix(slash, "storages/") && strings.Count(slash, "/") == 1 && strings.HasSuffix(slash, ".json") {
		return "storage"
	}
	return ""
}

func mergeStructuredImport(source, destination, rel string) ([]byte, bool, error) {
	kind := structuredImportKind(rel)
	if kind == "" {
		return nil, false, errors.New("该路径不支持按条目合并")
	}
	sourceBytes, err := readStructuredImportFile(source)
	if err != nil {
		return nil, false, fmt.Errorf("读取来源 %s: %w", filepath.ToSlash(rel), err)
	}
	destinationBytes, err := readStructuredImportFile(destination)
	if err != nil {
		return nil, false, fmt.Errorf("读取 Tiny %s: %w", filepath.ToSlash(rel), err)
	}
	switch kind {
	case "settings":
		return mergeYAMLDocuments(destinationBytes, sourceBytes, false)
	case "credentials":
		return mergeYAMLDocuments(destinationBytes, sourceBytes, true)
	case "storage":
		return mergeStorageDocuments(destinationBytes, sourceBytes)
	default:
		panic("unreachable structured import kind")
	}
}

func readStructuredImportFile(name string) ([]byte, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if info.Size() > structuredImportLimit {
		return nil, errors.New("文件超过 64 MiB 安全限制")
	}
	return os.ReadFile(name)
}

func mergeYAMLDocuments(tiny, source []byte, credentials bool) ([]byte, bool, error) {
	var tinyDocument, sourceDocument yaml.Node
	if err := yaml.Unmarshal(tiny, &tinyDocument); err != nil {
		return nil, false, errors.New("Tiny YAML 配置格式无效")
	}
	if err := yaml.Unmarshal(source, &sourceDocument); err != nil {
		return nil, false, errors.New("来源 YAML 配置格式无效")
	}
	tinyRoot, err := yamlMappingRoot(&tinyDocument)
	if err != nil {
		return nil, false, fmt.Errorf("Tiny YAML: %w", err)
	}
	sourceRoot, err := yamlMappingRoot(&sourceDocument)
	if err != nil {
		return nil, false, fmt.Errorf("来源 YAML: %w", err)
	}
	added := 0
	if credentials {
		// Credential records are atomic secrets. Add missing refs/records, but
		// never combine fields inside a record that Tiny already owns.
		for _, section := range []string{"refs", "records"} {
			sourceSection := yamlMapValue(sourceRoot, section)
			if sourceSection == nil {
				continue
			}
			if sourceSection.Kind != yaml.MappingNode {
				return nil, false, fmt.Errorf("来源凭据字段 %s 不是映射", section)
			}
			tinySection := yamlMapValue(tinyRoot, section)
			if tinySection == nil {
				yamlMapAppend(tinyRoot, section, sourceSection)
				added += len(sourceSection.Content) / 2
				continue
			}
			if tinySection.Kind != yaml.MappingNode {
				continue // Tiny owns an incompatible existing value.
			}
			added += mergeYAMLMap(tinySection, sourceSection, false)
		}
		// Preserve Tiny's schema fields; only fill missing top-level metadata.
		for index := 0; index+1 < len(sourceRoot.Content); index += 2 {
			key := sourceRoot.Content[index].Value
			if key == "refs" || key == "records" || yamlMapValue(tinyRoot, key) != nil {
				continue
			}
			yamlMapAppend(tinyRoot, key, sourceRoot.Content[index+1])
			added++
		}
	} else {
		added = mergeYAMLMap(tinyRoot, sourceRoot, true)
	}
	if added == 0 {
		return tiny, false, nil
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err = encoder.Encode(&tinyDocument); err != nil {
		return nil, false, errors.New("无法生成合并后的 YAML 配置")
	}
	if err = encoder.Close(); err != nil {
		return nil, false, errors.New("无法完成合并后的 YAML 配置")
	}
	return output.Bytes(), true, nil
}

func yamlMappingRoot(document *yaml.Node) (*yaml.Node, error) {
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("根节点必须是映射")
	}
	return document.Content[0], nil
}

func yamlMapValue(mapping *yaml.Node, key string) *yaml.Node {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func yamlMapAppend(mapping *yaml.Node, key string, value *yaml.Node) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		cloneYAMLNode(value),
	)
}

func mergeYAMLMap(tiny, source *yaml.Node, recursive bool) int {
	added := 0
	for index := 0; index+1 < len(source.Content); index += 2 {
		key := source.Content[index].Value
		sourceValue := source.Content[index+1]
		tinyValue := yamlMapValue(tiny, key)
		if tinyValue == nil {
			yamlMapAppend(tiny, key, sourceValue)
			added++
			continue
		}
		if recursive && tinyValue.Kind == yaml.MappingNode && sourceValue.Kind == yaml.MappingNode {
			added += mergeYAMLMap(tinyValue, sourceValue, true)
		}
	}
	return added
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	clone := *node
	clone.Content = make([]*yaml.Node, len(node.Content))
	for index, child := range node.Content {
		clone.Content[index] = cloneYAMLNode(child)
	}
	return &clone
}

type storageDocument struct {
	Unit struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
	} `json:"unit"`
	Global json.RawMessage                       `json:"global"`
	Tables map[string]map[string]json.RawMessage `json:"tables"`
}

func mergeStorageDocuments(tiny, source []byte) ([]byte, bool, error) {
	var tinyDocument, sourceDocument storageDocument
	if json.Unmarshal(tiny, &tinyDocument) != nil || !validStorageDocument(tinyDocument) {
		return nil, false, errors.New("Tiny 存储 JSON 格式无效")
	}
	if json.Unmarshal(source, &sourceDocument) != nil || !validStorageDocument(sourceDocument) {
		return nil, false, errors.New("来源存储 JSON 格式无效")
	}
	if tinyDocument.Unit != sourceDocument.Unit {
		return nil, false, errors.New("来源与 Tiny 存储单元版本不一致")
	}
	changed := false
	if isJSONNull(tinyDocument.Global) && !isJSONNull(sourceDocument.Global) {
		tinyDocument.Global = append(json.RawMessage(nil), sourceDocument.Global...)
		changed = true
	}
	for table, sourceRecords := range sourceDocument.Tables {
		tinyRecords, ok := tinyDocument.Tables[table]
		if !ok {
			tinyRecords = make(map[string]json.RawMessage)
			tinyDocument.Tables[table] = tinyRecords
		}
		for key, record := range sourceRecords {
			if _, exists := tinyRecords[key]; exists {
				continue
			}
			tinyRecords[key] = append(json.RawMessage(nil), record...)
			changed = true
		}
	}
	if !changed {
		return tiny, false, nil
	}
	output, err := json.MarshalIndent(tinyDocument, "", "  ")
	if err != nil {
		return nil, false, errors.New("无法生成合并后的存储 JSON")
	}
	return append(output, '\n'), true, nil
}

func validStorageDocument(document storageDocument) bool {
	if document.Unit.Name == "" || document.Unit.Version < 1 || document.Tables == nil {
		return false
	}
	for _, records := range document.Tables {
		if records == nil {
			return false
		}
	}
	return true
}

func isJSONNull(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}
