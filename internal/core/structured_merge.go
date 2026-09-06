package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	if tinyDocument.Unit.Name == "workspace" && tinyDocument.Unit.Version == 2 {
		return mergeWorkspaceStorageDocuments(tiny, &tinyDocument, &sourceDocument)
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

type workspaceGlobalState struct {
	Initialized        bool            `json:"initialized"`
	WorkspaceIDs       []string        `json:"workspaceIds"`
	ArchivedSessionIDs []string        `json:"archivedSessionIds"`
	PendingMutation    json.RawMessage `json:"pendingMutation,omitempty"`
}

type workspaceRecordState struct {
	Path       string   `json:"path"`
	SessionIDs []string `json:"sessionIds"`
}

// mergeWorkspaceStorageDocuments keeps the workspace table and its global
// registry order atomic. DSH treats workspaceIds as the authoritative order;
// adding a table row without adding its id makes the whole profile unbootable.
func mergeWorkspaceStorageDocuments(originalTiny []byte, tiny, source *storageDocument) ([]byte, bool, error) {
	tinyGlobal, err := parseWorkspaceGlobal(tiny.Global, "Tiny")
	if err != nil {
		return nil, false, err
	}
	sourceGlobal, err := parseWorkspaceGlobal(source.Global, "来源")
	if err != nil {
		return nil, false, err
	}
	tinyWorkspaces, ok := tiny.Tables["workspaces"]
	if !ok {
		return nil, false, errors.New("Tiny workspace 存储缺少 workspaces 表")
	}
	sourceWorkspaces, ok := source.Tables["workspaces"]
	if !ok {
		return nil, false, errors.New("来源 workspace 存储缺少 workspaces 表")
	}
	if err = validateWorkspaceRegistry(tinyGlobal, tinyWorkspaces); err != nil {
		return nil, false, fmt.Errorf("Tiny workspace 存储不一致: %w", err)
	}
	if err = validateWorkspaceRegistry(sourceGlobal, sourceWorkspaces); err != nil {
		return nil, false, fmt.Errorf("来源 workspace 存储不一致: %w", err)
	}

	changed := false
	knownIDs := make(map[string]bool, len(tinyGlobal.WorkspaceIDs))
	pathOwners := make(map[string]string, len(tinyWorkspaces))
	sessionOwners := make(map[string]string)
	for _, id := range tinyGlobal.WorkspaceIDs {
		knownIDs[id] = true
		record, parseErr := parseWorkspaceRecord(tinyWorkspaces[id], id)
		if parseErr != nil {
			return nil, false, parseErr
		}
		pathOwners[record.Path] = id
		for _, sessionID := range record.SessionIDs {
			sessionOwners[sessionID] = id
		}
	}
	// Respect the source's authoritative order, then include any table rows
	// from a not-yet-initialized source in deterministic order.
	sourceOrder := append([]string(nil), sourceGlobal.WorkspaceIDs...)
	if !sourceGlobal.Initialized {
		var unordered []string
		for id := range sourceWorkspaces {
			if !containsString(sourceOrder, id) {
				unordered = append(unordered, id)
			}
		}
		sort.Strings(unordered)
		sourceOrder = append(sourceOrder, unordered...)
	}
	for _, id := range sourceOrder {
		raw, exists := sourceWorkspaces[id]
		if !exists {
			return nil, false, fmt.Errorf("来源 workspace 顺序引用了缺失记录 %q", id)
		}
		record, parseErr := parseWorkspaceRecord(raw, id)
		if parseErr != nil {
			return nil, false, parseErr
		}
		ownerID := id
		if !knownIDs[id] {
			ownerID = pathOwners[record.Path]
		}
		if ownerID != "" {
			// The Tiny record keeps all of its fields and session ordering. Only
			// source sessions not owned anywhere in Tiny are appended.
			updated, recordChanged, mergeErr := appendUnownedWorkspaceSessions(tinyWorkspaces[ownerID], ownerID, record.SessionIDs, sessionOwners)
			if mergeErr != nil {
				return nil, false, mergeErr
			}
			if recordChanged {
				tinyWorkspaces[ownerID] = updated
				changed = true
			}
			continue
		}
		filtered := make([]string, 0, len(record.SessionIDs))
		for _, sessionID := range record.SessionIDs {
			if _, owned := sessionOwners[sessionID]; !owned {
				filtered = append(filtered, sessionID)
				sessionOwners[sessionID] = id
			}
		}
		if len(filtered) != len(record.SessionIDs) {
			raw, err = replaceWorkspaceSessions(raw, filtered)
			if err != nil {
				return nil, false, err
			}
		}
		tinyWorkspaces[id] = append(json.RawMessage(nil), raw...)
		tinyGlobal.WorkspaceIDs = append(tinyGlobal.WorkspaceIDs, id)
		knownIDs[id] = true
		pathOwners[record.Path] = id
		changed = true
	}
	for _, archivedID := range sourceGlobal.ArchivedSessionIDs {
		if !containsString(tinyGlobal.ArchivedSessionIDs, archivedID) {
			tinyGlobal.ArchivedSessionIDs = append(tinyGlobal.ArchivedSessionIDs, archivedID)
			changed = true
		}
	}
	if err = validateWorkspaceRegistry(tinyGlobal, tinyWorkspaces); err != nil {
		return nil, false, fmt.Errorf("合并后的 workspace 存储不一致: %w", err)
	}
	if !changed {
		return originalTiny, false, nil
	}
	tiny.Global, err = json.Marshal(tinyGlobal)
	if err != nil {
		return nil, false, errors.New("无法生成 workspace 注册状态")
	}
	return marshalStorageDocument(*tiny)
}

func parseWorkspaceGlobal(raw json.RawMessage, owner string) (workspaceGlobalState, error) {
	var state workspaceGlobalState
	if json.Unmarshal(raw, &state) != nil || state.WorkspaceIDs == nil {
		return state, fmt.Errorf("%s workspace global 格式无效", owner)
	}
	// The official schema defaults this field for profiles written before the
	// archive feature existed.
	if state.ArchivedSessionIDs == nil {
		state.ArchivedSessionIDs = []string{}
	}
	return state, nil
}

func validateWorkspaceRegistry(global workspaceGlobalState, records map[string]json.RawMessage) error {
	seen := make(map[string]bool, len(global.WorkspaceIDs))
	paths := make(map[string]string, len(records))
	sessions := make(map[string]string)
	for _, id := range global.WorkspaceIDs {
		if seen[id] {
			return fmt.Errorf("注册顺序重复工作空间 %q", id)
		}
		if _, exists := records[id]; !exists {
			return fmt.Errorf("注册顺序引用了缺失工作空间 %q", id)
		}
		seen[id] = true
	}
	if global.Initialized && len(seen) != len(records) {
		for id := range records {
			if !seen[id] {
				return fmt.Errorf("工作空间 %q 未进入注册顺序", id)
			}
		}
	}
	for id, raw := range records {
		record, err := parseWorkspaceRecord(raw, id)
		if err != nil {
			return err
		}
		if holder, exists := paths[record.Path]; exists {
			return fmt.Errorf("路径 %q 同时属于工作空间 %q 和 %q", record.Path, holder, id)
		}
		paths[record.Path] = id
		for _, sessionID := range record.SessionIDs {
			if holder, exists := sessions[sessionID]; exists {
				return fmt.Errorf("会话 %q 同时属于工作空间 %q 和 %q", sessionID, holder, id)
			}
			sessions[sessionID] = id
		}
	}
	return nil
}

func parseWorkspaceRecord(raw json.RawMessage, id string) (workspaceRecordState, error) {
	var record workspaceRecordState
	if json.Unmarshal(raw, &record) != nil || record.Path == "" || record.SessionIDs == nil {
		return record, fmt.Errorf("工作空间 %q 记录格式无效", id)
	}
	return record, nil
}

func appendUnownedWorkspaceSessions(raw json.RawMessage, ownerID string, sourceIDs []string, owners map[string]string) (json.RawMessage, bool, error) {
	record, err := parseWorkspaceRecord(raw, ownerID)
	if err != nil {
		return nil, false, err
	}
	changed := false
	for _, sessionID := range sourceIDs {
		if _, exists := owners[sessionID]; exists {
			continue
		}
		record.SessionIDs = append(record.SessionIDs, sessionID)
		owners[sessionID] = ownerID
		changed = true
	}
	if !changed {
		return raw, false, nil
	}
	updated, err := replaceWorkspaceSessions(raw, record.SessionIDs)
	return updated, true, err
}

func replaceWorkspaceSessions(raw json.RawMessage, sessions []string) (json.RawMessage, error) {
	var record map[string]json.RawMessage
	if json.Unmarshal(raw, &record) != nil || record == nil {
		return nil, errors.New("workspace 记录格式无效")
	}
	encoded, err := json.Marshal(sessions)
	if err != nil {
		return nil, err
	}
	record["sessionIds"] = encoded
	return json.Marshal(record)
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func marshalStorageDocument(document storageDocument) ([]byte, bool, error) {
	output, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, false, errors.New("无法生成合并后的存储 JSON")
	}
	return append(output, '\n'), true, nil
}

// repairWorkspaceRegistryFile repairs the legacy inconsistency introduced by
// v0.2.12: workspace rows were added while their ids were omitted from
// workspaceIds. Distinct rows join the order; duplicate-path rows are safely
// coalesced under Tiny's existing record after a byte-exact backup is written.
func repairWorkspaceRegistryFile(path string) (int, error) {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var document storageDocument
	if json.Unmarshal(contents, &document) != nil || !validStorageDocument(document) {
		return 0, errors.New("workspace 存储 JSON 格式无效")
	}
	if document.Unit.Name != "workspace" || document.Unit.Version != 2 {
		return 0, nil
	}
	global, err := parseWorkspaceGlobal(document.Global, "Tiny")
	if err != nil {
		return 0, err
	}
	if !global.Initialized || !isJSONNull(global.PendingMutation) {
		// DSH itself owns bootstrap and pending two-write mutation recovery.
		return 0, nil
	}
	records, ok := document.Tables["workspaces"]
	if !ok {
		return 0, errors.New("Tiny workspace 存储缺少 workspaces 表")
	}
	// Refuse broader corruption. This compatibility repair only appends table
	// ids absent from an otherwise valid order and leaves all records untouched.
	seen := make(map[string]bool, len(global.WorkspaceIDs))
	for _, id := range global.WorkspaceIDs {
		if seen[id] {
			return 0, fmt.Errorf("workspace 注册顺序重复工作空间 %q", id)
		}
		if _, exists := records[id]; !exists {
			return 0, fmt.Errorf("workspace 注册顺序引用了缺失工作空间 %q", id)
		}
		seen[id] = true
	}
	var missing []string
	for id := range records {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return 0, nil
	}
	sort.Strings(missing)
	pathOwners := make(map[string]string, len(records))
	sessionOwners := make(map[string]string)
	for _, id := range global.WorkspaceIDs {
		record, parseErr := parseWorkspaceRecord(records[id], id)
		if parseErr != nil {
			return 0, parseErr
		}
		if holder, exists := pathOwners[record.Path]; exists {
			return 0, fmt.Errorf("路径 %q 同时属于工作空间 %q 和 %q", record.Path, holder, id)
		}
		pathOwners[record.Path] = id
		for _, sessionID := range record.SessionIDs {
			if holder, exists := sessionOwners[sessionID]; exists {
				return 0, fmt.Errorf("会话 %q 同时属于工作空间 %q 和 %q", sessionID, holder, id)
			}
			sessionOwners[sessionID] = id
		}
	}
	for _, id := range missing {
		raw := records[id]
		record, parseErr := parseWorkspaceRecord(raw, id)
		if parseErr != nil {
			return 0, parseErr
		}
		if ownerID := pathOwners[record.Path]; ownerID != "" {
			updated, changed, mergeErr := appendUnownedWorkspaceSessions(records[ownerID], ownerID, record.SessionIDs, sessionOwners)
			if mergeErr != nil {
				return 0, mergeErr
			}
			if changed {
				records[ownerID] = updated
			}
			delete(records, id)
			continue
		}
		filtered := make([]string, 0, len(record.SessionIDs))
		for _, sessionID := range record.SessionIDs {
			if _, exists := sessionOwners[sessionID]; exists {
				continue
			}
			filtered = append(filtered, sessionID)
			sessionOwners[sessionID] = id
		}
		if len(filtered) != len(record.SessionIDs) {
			records[id], err = replaceWorkspaceSessions(raw, filtered)
			if err != nil {
				return 0, err
			}
		}
		pathOwners[record.Path] = id
		global.WorkspaceIDs = append(global.WorkspaceIDs, id)
	}
	if err = validateWorkspaceRegistry(global, records); err != nil {
		return 0, fmt.Errorf("无法安全修复 workspace 存储: %w", err)
	}
	document.Global, err = json.Marshal(global)
	if err != nil {
		return 0, err
	}
	output, _, err := marshalStorageDocument(document)
	if err != nil {
		return 0, err
	}
	// Keep the exact pre-repair document beside the storage file. This is user
	// data, not a disposable test artifact, and makes the compatibility repair
	// recoverable without depending on the settings window being able to boot.
	backup := path + ".tiny-v0.2.12-recovery"
	if _, statErr := os.Stat(backup); os.IsNotExist(statErr) {
		if err = AtomicWrite(backup, contents, 0600); err != nil {
			return 0, fmt.Errorf("保存 workspace 修复备份: %w", err)
		}
	} else if statErr != nil {
		return 0, fmt.Errorf("检查 workspace 修复备份: %w", statErr)
	}
	if err = AtomicWrite(path, output, 0600); err != nil {
		return 0, err
	}
	return len(missing), nil
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
