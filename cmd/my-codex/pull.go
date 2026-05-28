package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	managedassets "github.com/faceair/my-codex"
)

const (
	legacyHookBinaryName = "codex-stop-guard"
	legacyStopStatusText = "Checking unfinished plan items"
)

type PullOptions struct {
	DestRoot        string
	Runner          CommandRunner
	Platform        Platform
	ManagedAssetsFS fs.FS
}

func runPull(options PullOptions, stdoutWriter, stderrWriter io.Writer) error {
	if options.Platform.GOOS == "" {
		options.Platform = CurrentPlatform()
	}
	if options.ManagedAssetsFS == nil {
		options.ManagedAssetsFS = managedassets.FS
	}
	destRoot := filepath.Clean(options.DestRoot)
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return fmt.Errorf("create codex root %s: %w", destRoot, err)
	}
	if err := copyDirFromFS(options.ManagedAssetsFS, "agents", filepath.Join(destRoot, "agents")); err != nil {
		return err
	}
	if err := copyDirFromFS(options.ManagedAssetsFS, "skills", filepath.Join(destRoot, "skills")); err != nil {
		return err
	}
	if err := copyDirFromFS(options.ManagedAssetsFS, "instructions", filepath.Join(destRoot, "instructions")); err != nil {
		return err
	}
	if err := cleanupLegacyPrompts(destRoot); err != nil {
		return err
	}
	if err := cleanupLegacyStopHook(destRoot); err != nil {
		return err
	}
	configPath := filepath.Join(destRoot, "config.toml")
	if err := pullManagedConfig(options.ManagedAssetsFS, configPath); err != nil {
		return err
	}
	if err := cleanupLegacyHookConfig(configPath, filepath.Join(destRoot, "hooks.json")); err != nil {
		return err
	}
	fmt.Fprintf(stdoutWriter, "Pulled into: %s\n", destRoot)
	fmt.Fprintln(stdoutWriter, "Updated:")
	fmt.Fprintf(stdoutWriter, "  - %s\n", filepath.Join(destRoot, "agents"))
	fmt.Fprintf(stdoutWriter, "  - %s\n", filepath.Join(destRoot, "skills"))
	fmt.Fprintf(stdoutWriter, "  - %s (synced from embedded release assets)\n", filepath.Join(destRoot, "instructions"))
	fmt.Fprintf(stdoutWriter, "  - %s (repo-managed goals config merged in; other local config preserved)\n", filepath.Join(destRoot, "config.toml"))
	fmt.Fprintf(stdoutWriter, "Cleaned legacy prompt and Stop hook files when present.\n")
	_ = stderrWriter
	return nil
}

func cleanupLegacyPrompts(destRoot string) error {
	// commit-and-push 已迁移到 skill；只清理旧 repo-managed prompt，保留用户自定义 prompts。
	promptsDir := filepath.Join(destRoot, "prompts")
	for _, name := range []string{"commit-and-push.md", "simplify-memory.md"} {
		path := filepath.Join(promptsDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove legacy prompt file %s: %w", path, err)
		}
	}
	if err := removeDirIfEmpty(promptsDir); err != nil {
		return err
	}
	return nil
}

func cleanupLegacyStopHook(destRoot string) error {
	// 迁移到原生 goal continuation 后，本地旧 Stop hook 必须被移除，
	// 但用户自定义的非 managed hooks 仍应保留。
	if err := cleanupLegacyHooksJSON(filepath.Join(destRoot, "hooks.json")); err != nil {
		return err
	}
	hooksDir := filepath.Join(destRoot, "hooks")
	for _, name := range []string{legacyHookBinaryName, legacyHookBinaryName + ".exe", "stop_continue_if_todo.py", ".DS_Store"} {
		path := filepath.Join(hooksDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove legacy hook file %s: %w", path, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(hooksDir, "__pycache__")); err != nil {
		return fmt.Errorf("remove legacy hook cache dir %s: %w", filepath.Join(hooksDir, "__pycache__"), err)
	}
	if err := removeDirIfEmpty(hooksDir); err != nil {
		return err
	}
	return nil
}

func cleanupLegacyHooksJSON(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read legacy hooks.json %s: %w", path, err)
	}
	document, err := decodeJSONDocument(raw)
	if err != nil {
		return fmt.Errorf("decode legacy hooks.json %s: %w", path, err)
	}
	changed := removeManagedStopHooks(document)
	if !changed {
		return nil
	}
	if hooksDocumentIsEmpty(document) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove empty legacy hooks.json %s: %w", path, err)
		}
		return nil
	}
	formatted, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode legacy hooks.json %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(formatted, '\n'), 0o644); err != nil {
		return fmt.Errorf("write cleaned legacy hooks.json %s: %w", path, err)
	}
	return nil
}

func decodeJSONDocument(raw []byte) (map[string]any, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}, nil
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	if document == nil {
		return map[string]any{}, nil
	}
	return document, nil
}

func removeManagedStopHooks(document map[string]any) bool {
	hooks, ok := document["hooks"].(map[string]any)
	if !ok {
		return false
	}
	stopEntries, ok := hooks["Stop"].([]any)
	if !ok {
		return false
	}
	changed := false
	keptEntries := make([]any, 0, len(stopEntries))
	for _, entryValue := range stopEntries {
		entry, ok := entryValue.(map[string]any)
		if !ok {
			keptEntries = append(keptEntries, entryValue)
			continue
		}
		hookList, ok := entry["hooks"].([]any)
		if !ok {
			keptEntries = append(keptEntries, entryValue)
			continue
		}
		keptHooks := make([]any, 0, len(hookList))
		for _, hookValue := range hookList {
			hookMap, ok := hookValue.(map[string]any)
			if ok && isLegacyManagedStopHook(hookMap) {
				changed = true
				continue
			}
			keptHooks = append(keptHooks, hookValue)
		}
		if len(keptHooks) == 0 {
			changed = true
			continue
		}
		entry["hooks"] = keptHooks
		keptEntries = append(keptEntries, entry)
	}
	if len(keptEntries) == 0 {
		delete(hooks, "Stop")
	} else {
		hooks["Stop"] = keptEntries
	}
	if len(hooks) == 0 {
		delete(document, "hooks")
	}
	return changed
}

func isLegacyManagedStopHook(hookMap map[string]any) bool {
	typeValue, _ := hookMap["type"].(string)
	if typeValue != "command" {
		return false
	}
	statusMessage, _ := hookMap["statusMessage"].(string)
	if statusMessage == legacyStopStatusText {
		return true
	}
	command, _ := hookMap["command"].(string)
	lower := strings.ToLower(command)
	return strings.Contains(lower, "stop_continue_if_todo") || strings.Contains(lower, legacyHookBinaryName)
}

func hooksDocumentIsEmpty(document map[string]any) bool {
	hooks, ok := document["hooks"].(map[string]any)
	if !ok || len(hooks) == 0 {
		return true
	}
	for _, value := range hooks {
		entries, ok := value.([]any)
		if !ok || len(entries) > 0 {
			return false
		}
	}
	return true
}

func removeDirIfEmpty(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open directory %s before empty cleanup: %w", path, err)
	}
	defer file.Close()
	_, err = file.Readdirnames(1)
	if err == nil {
		return nil
	}
	if err != io.EOF {
		return fmt.Errorf("read directory %s before empty cleanup: %w", path, err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove empty directory %s: %w", path, err)
	}
	return nil
}

func cleanupLegacyHookConfig(configPath, legacyHooksJSONPath string) error {
	// hook 状态表是 Codex 对具体 hooks.json 的运行时记录；删除 managed
	// hooks.json 后只清掉对应条目，避免影响其他仓库自己的 hooks。
	document, err := readTOMLFile(configPath)
	if err != nil {
		return err
	}
	hooks, ok := document["hooks"].(map[string]any)
	if !ok {
		return nil
	}
	state, ok := hooks["state"].(map[string]any)
	if !ok {
		return nil
	}
	changed := false
	for key := range state {
		if legacyHookStateKeyMatches(key, legacyHooksJSONPath) {
			delete(state, key)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if len(state) == 0 {
		delete(hooks, "state")
	}
	if len(hooks) == 0 {
		delete(document, "hooks")
	}
	return writeTOMLFile(configPath, document)
}

func legacyHookStateKeyMatches(key, legacyHooksJSONPath string) bool {
	prefixes := []string{
		legacyHooksJSONPath + ":",
		filepath.ToSlash(legacyHooksJSONPath) + ":",
		strings.ReplaceAll(legacyHooksJSONPath, "/", `\`) + ":",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func pullManagedConfig(sourceFS fs.FS, destination string) error {
	embeddedConfig, err := fs.ReadFile(sourceFS, "config.toml")
	if err != nil {
		return fmt.Errorf("read embedded config.toml: %w", err)
	}
	tempEmbedded, err := os.CreateTemp("", "my-codex-config-*.toml")
	if err != nil {
		return fmt.Errorf("create temp embedded config copy: %w", err)
	}
	tempEmbeddedPath := tempEmbedded.Name()
	defer os.Remove(tempEmbeddedPath)
	if _, err := tempEmbedded.Write(embeddedConfig); err != nil {
		tempEmbedded.Close()
		return fmt.Errorf("write temp embedded config copy: %w", err)
	}
	tempEmbedded.Close()
	localDocument, err := readTOMLFile(destination)
	if err != nil {
		return err
	}
	embeddedDocument, err := readTOMLFile(tempEmbeddedPath)
	if err != nil {
		return err
	}
	return writeTOMLFile(destination, mergeManagedDocument(localDocument, extractManagedDocument(embeddedDocument)))
}
