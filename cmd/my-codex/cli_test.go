package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	managedassets "github.com/faceair/my-codex"
)

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"unknown"}, &stdout, &stderr, "dev")
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestPullCleansManagedStopHookAndKeepsUnmanagedHooks(t *testing.T) {
	tempDir := t.TempDir()
	destRoot := filepath.Join(tempDir, ".codex")
	managedHook := filepath.Join(destRoot, "hooks", legacyHookBinaryName)
	legacyPythonHook := filepath.Join(destRoot, "hooks", "stop_continue_if_todo.py")
	unmanagedHook := filepath.Join(destRoot, "hooks", "custom.sh")
	for _, target := range []string{managedHook, legacyPythonHook, unmanagedHook} {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("create parent dir for %s: %v", target, err)
		}
		if err := os.WriteFile(target, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write hook %s: %v", target, err)
		}
	}
	existingHooksJSON := filepath.Join(destRoot, "hooks.json")
	if err := os.WriteFile(existingHooksJSON, []byte(`{
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "echo pretool"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "\"/Users/test/.codex/hooks/codex-stop-guard\"",
            "timeout": 10,
            "statusMessage": "Checking unfinished plan items"
          },
          {
            "type": "command",
            "command": "echo keep-stop"
          }
        ]
      }
    ]
  }
}`), 0o644); err != nil {
		t.Fatalf("write existing hooks.json: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runPull(PullOptions{DestRoot: destRoot, Platform: CurrentPlatform(), ManagedAssetsFS: managedassets.FS}, &stdout, &stderr); err != nil {
		t.Fatalf("runPull returned error: %v", err)
	}
	for _, removed := range []string{managedHook, legacyPythonHook} {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("expected legacy hook %s to be removed, err=%v", removed, err)
		}
	}
	if _, err := os.Stat(unmanagedHook); err != nil {
		t.Fatalf("expected unmanaged hook to be preserved: %v", err)
	}
	hooksJSON, err := os.ReadFile(existingHooksJSON)
	if err != nil {
		t.Fatalf("expected hooks.json with unmanaged hooks to remain: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(hooksJSON, &document); err != nil {
		t.Fatalf("decode cleaned hooks.json: %v", err)
	}
	hooks := document["hooks"].(map[string]any)
	if hooks["PreToolUse"] == nil {
		t.Fatalf("expected unmanaged PreToolUse hook to be preserved")
	}
	stopEntries := hooks["Stop"].([]any)
	hookList := stopEntries[0].(map[string]any)["hooks"].([]any)
	if len(hookList) != 1 || hookList[0].(map[string]any)["command"] != "echo keep-stop" {
		t.Fatalf("unexpected remaining Stop hooks: %#v", hookList)
	}
}

func TestPullRemovesHooksJSONWhenOnlyManagedStopHookExists(t *testing.T) {
	tempDir := t.TempDir()
	destRoot := filepath.Join(tempDir, ".codex")
	managedHook := filepath.Join(destRoot, "hooks", legacyHookBinaryName)
	if err := os.MkdirAll(filepath.Join(filepath.Dir(managedHook), "__pycache__"), 0o755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}
	if err := os.WriteFile(managedHook, []byte("legacy"), 0o755); err != nil {
		t.Fatalf("write managed hook: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(managedHook), ".DS_Store"), []byte("cache"), 0o644); err != nil {
		t.Fatalf("write hook ds store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(managedHook), "__pycache__", "legacy.pyc"), []byte("cache"), 0o644); err != nil {
		t.Fatalf("write hook pycache: %v", err)
	}
	hooksJSON := filepath.Join(destRoot, "hooks.json")
	if err := os.WriteFile(hooksJSON, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"$HOME/.codex/hooks/stop_continue_if_todo.py"}]}]}}`), 0o644); err != nil {
		t.Fatalf("write hooks.json: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runPull(PullOptions{DestRoot: destRoot, Platform: CurrentPlatform(), ManagedAssetsFS: managedassets.FS}, &stdout, &stderr); err != nil {
		t.Fatalf("runPull returned error: %v", err)
	}
	if _, err := os.Stat(hooksJSON); !os.IsNotExist(err) {
		t.Fatalf("expected managed-only hooks.json to be removed, err=%v", err)
	}
	if _, err := os.Stat(managedHook); !os.IsNotExist(err) {
		t.Fatalf("expected managed hook binary to be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Dir(managedHook)); !os.IsNotExist(err) {
		t.Fatalf("expected legacy-only hooks dir to be removed, err=%v", err)
	}
}

func TestPullPreservesExistingManagedDirectoriesFilesAndMergesGoalsConfig(t *testing.T) {
	tempDir := t.TempDir()
	destRoot := filepath.Join(tempDir, ".codex")
	legacyPrompt := filepath.Join(destRoot, "prompts", "commit-and-push.md")
	customPrompt := filepath.Join(destRoot, "prompts", "custom.md")
	existingSkill := filepath.Join(destRoot, "skills", "legacy", "SKILL.md")
	existingInstruction := filepath.Join(destRoot, "instructions", "legacy.md")
	existingAgent := filepath.Join(destRoot, "agents", "legacy.toml")
	for _, target := range []string{legacyPrompt, customPrompt, existingSkill, existingInstruction, existingAgent} {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("create parent dir for %s: %v", target, err)
		}
		if err := os.WriteFile(target, []byte("legacy\n"), 0o644); err != nil {
			t.Fatalf("write legacy file %s: %v", target, err)
		}
	}
	legacyHookState := filepath.Join(destRoot, "hooks.json") + ":stop:0:0"
	localConfig := strings.TrimSpace(`
model = "local"
model_instructions_file = "old.md"

[features]
hooks = true
codex_hooks = true
chatty_output = true

[agents.reviewer]
config_file = "old-reviewer.toml"
model = "local-reviewer"

[hooks.state.`+strconv.Quote(legacyHookState)+`]
seen = true

[hooks.state."/other/repo/hooks.json:stop:0:0"]
seen = true
`) + "\n"
	if err := os.WriteFile(filepath.Join(destRoot, "config.toml"), []byte(localConfig), 0o644); err != nil {
		t.Fatalf("write local config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runPull(PullOptions{DestRoot: destRoot, Platform: CurrentPlatform(), ManagedAssetsFS: managedassets.FS}, &stdout, &stderr); err != nil {
		t.Fatalf("runPull returned error: %v", err)
	}
	for _, target := range []string{customPrompt, existingSkill, existingInstruction, existingAgent} {
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("expected existing unmanaged file to be preserved at %s: %v", target, err)
		}
	}
	if _, err := os.Stat(legacyPrompt); !os.IsNotExist(err) {
		t.Fatalf("expected legacy prompt to be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "skills", "commit-and-push", "SKILL.md")); err != nil {
		t.Fatalf("expected commit-and-push skill to be pulled: %v", err)
	}
	config, err := readTOMLFile(filepath.Join(destRoot, "config.toml"))
	if err != nil {
		t.Fatalf("read pulled config.toml: %v", err)
	}
	if config["model"] != "local" {
		t.Fatalf("expected unmanaged model to be preserved, got %#v", config["model"])
	}
	if config["model_instructions_file"] != "instructions/main.md" {
		t.Fatalf("unexpected pulled model_instructions_file: %#v", config["model_instructions_file"])
	}
	features := config["features"].(map[string]any)
	if features["goals"] != true || features["chatty_output"] != true {
		t.Fatalf("unexpected merged features: %#v", features)
	}
	for _, legacyFeature := range []string{"codex_hooks", "hooks"} {
		if _, exists := features[legacyFeature]; exists {
			t.Fatalf("expected legacy %s feature to be removed, got %#v", legacyFeature, features)
		}
	}
	reviewer := config["agents"].(map[string]any)["reviewer"].(map[string]any)
	if reviewer["config_file"] != "agents/reviewer.toml" || reviewer["model"] != "local-reviewer" {
		t.Fatalf("unexpected merged reviewer config: %#v", reviewer)
	}
	hookState := config["hooks"].(map[string]any)["state"].(map[string]any)
	if _, exists := hookState[legacyHookState]; exists {
		t.Fatalf("expected stale managed hook state to be removed, got %#v", hookState)
	}
	if hookState["/other/repo/hooks.json:stop:0:0"] == nil {
		t.Fatalf("expected unrelated hook state to be preserved, got %#v", hookState)
	}
}

func TestSyncManagedConfigWritesGoalsWhitelistOnly(t *testing.T) {
	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source.toml")
	destination := filepath.Join(tempDir, "destination.toml")
	content := strings.TrimSpace(`
model = "gpt-5.4"
service_tier = "fast"
model_instructions_file = "instructions/main.md"
model_provider = "quotio"

[features]
goals = true
codex_hooks = true
chatty_output = true

[agents.reviewer]
config_file = "agents/reviewer.toml"
model = "gemini-3-flash"
`) + "\n"
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatalf("write source config: %v", err)
	}
	if err := syncManagedConfig(source, destination); err != nil {
		t.Fatalf("syncManagedConfig returned error: %v", err)
	}
	document, err := readTOMLFile(destination)
	if err != nil {
		t.Fatalf("read destination config: %v", err)
	}
	if len(document) != 3 {
		t.Fatalf("expected exactly 3 top-level keys, got %#v", document)
	}
	if _, ok := document["model_instructions_file"]; !ok {
		t.Fatalf("expected model_instructions_file to remain in destination config")
	}
	features := document["features"].(map[string]any)
	if len(features) != 1 || features["goals"] != true {
		t.Fatalf("unexpected features table: %#v", features)
	}
	reviewer := document["agents"].(map[string]any)["reviewer"].(map[string]any)
	if len(reviewer) != 1 || reviewer["config_file"] != "agents/reviewer.toml" {
		t.Fatalf("unexpected reviewer config: %#v", reviewer)
	}
}
