package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

func TestCleanOptionalPathPreservesEmptyString(t *testing.T) {
	if got := cleanOptionalPath(""); got != "" {
		t.Fatalf("expected empty optional path to stay empty, got %q", got)
	}
	if got := cleanOptionalPath("./tmp/foo"); got != filepath.Clean("./tmp/foo") {
		t.Fatalf("expected non-empty path to be cleaned, got %q", got)
	}
}

func TestRepoHookCommandUsesMacStylePath(t *testing.T) {
	if got := RepoHookCommand(); got != "\"$HOME/.codex/hooks/codex-stop-guard\"" {
		t.Fatalf("unexpected repo hook command: %s", got)
	}
}

func TestLocalHookCommandQuotesBinary(t *testing.T) {
	binary := filepath.Join("C:", "Users", "faceair", ".codex", "hooks", platformHookBinaryName(runtime.GOOS))
	got := LocalHookCommand(binary)
	if got == binary {
		t.Fatalf("expected quoted local hook command, got %s", got)
	}
}

func TestNormalizeRepoHookJSONRewritesLegacyPythonCommand(t *testing.T) {
	raw := []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"/usr/bin/python3 \"$HOME/.codex/hooks/stop_continue_if_todo.py\"","timeout":10,"statusMessage":"Checking unfinished plan items"}]}]}}`)
	normalized, err := normalizeRepoHookJSON(raw)
	if err != nil {
		t.Fatalf("normalizeRepoHookJSON returned error: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(normalized, &document); err != nil {
		t.Fatalf("decode normalized hooks json: %v", err)
	}
	command := extractStopHookCommand(t, document)
	if command != RepoHookCommand() {
		t.Fatalf("expected repo hook command %q, got %q", RepoHookCommand(), command)
	}
}

func TestPullInstallsHookBinaryAndAdaptsHookJSON(t *testing.T) {
	tempDir := t.TempDir()
	destRoot := filepath.Join(tempDir, ".codex")
	legacyHook := filepath.Join(destRoot, "hooks", "stop_continue_if_todo.py")
	if err := os.MkdirAll(filepath.Dir(legacyHook), 0o755); err != nil {
		t.Fatalf("create legacy hooks dir: %v", err)
	}
	if err := os.WriteFile(legacyHook, []byte("print('legacy')\n"), 0o644); err != nil {
		t.Fatalf("write legacy hook: %v", err)
	}
	hookBinary := filepath.Join(tempDir, platformHookBinaryName(runtime.GOOS))
	if runtime.GOOS == "windows" {
		if err := os.WriteFile(hookBinary, []byte("@echo off\r\n"), 0o644); err != nil {
			t.Fatalf("write fake hook binary: %v", err)
		}
	} else {
		if err := os.WriteFile(hookBinary, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write fake hook binary: %v", err)
		}
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runPull(PullOptions{DestRoot: destRoot, HookBinaryPath: hookBinary, Platform: CurrentPlatform(), ManagedAssetsFS: managedassets.FS}, &stdout, &stderr); err != nil {
		t.Fatalf("runPull returned error: %v", err)
	}
	installedHookPath := filepath.Join(destRoot, "hooks", platformHookBinaryName(runtime.GOOS))
	if _, err := os.Stat(installedHookPath); err != nil {
		t.Fatalf("expected installed hook binary at %s: %v", installedHookPath, err)
	}
	if _, err := os.Stat(legacyHook); !os.IsNotExist(err) {
		t.Fatalf("expected legacy hook to be removed, stat err=%v", err)
	}
	hooksJSON, err := os.ReadFile(filepath.Join(destRoot, "hooks.json"))
	if err != nil {
		t.Fatalf("read adapted hooks.json: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(hooksJSON, &document); err != nil {
		t.Fatalf("decode adapted hooks.json: %v", err)
	}
	if command := extractStopHookCommand(t, document); !strings.Contains(command, platformHookBinaryName(runtime.GOOS)) {
		t.Fatalf("expected local hook command to contain installed hook binary, got %q", command)
	}
	config, err := readTOMLFile(filepath.Join(destRoot, "config.toml"))
	if err != nil {
		t.Fatalf("read pulled config.toml: %v", err)
	}
	if config["model_instructions_file"] != "instructions/main.md" {
		t.Fatalf("unexpected pulled model_instructions_file: %#v", config["model_instructions_file"])
	}
}

func TestPullOnWindowsUsesExeHookPath(t *testing.T) {
	tempDir := t.TempDir()
	destRoot := filepath.Join(tempDir, ".codex")
	hookBinary := filepath.Join(tempDir, platformHookBinaryName("windows"))
	if err := os.WriteFile(hookBinary, []byte("windows-binary"), 0o644); err != nil {
		t.Fatalf("write fake windows hook binary: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runPull(PullOptions{DestRoot: destRoot, HookBinaryPath: hookBinary, Platform: Platform{GOOS: "windows"}, ManagedAssetsFS: managedassets.FS}, &stdout, &stderr); err != nil {
		t.Fatalf("runPull returned error: %v", err)
	}
	installedHookPath := filepath.Join(destRoot, "hooks", platformHookBinaryName("windows"))
	if _, err := os.Stat(installedHookPath); err != nil {
		t.Fatalf("expected installed windows hook binary at %s: %v", installedHookPath, err)
	}
	hooksJSON, err := os.ReadFile(filepath.Join(destRoot, "hooks.json"))
	if err != nil {
		t.Fatalf("read adapted hooks.json: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(hooksJSON, &document); err != nil {
		t.Fatalf("decode adapted hooks.json: %v", err)
	}
	command := extractStopHookCommand(t, document)
	if !strings.Contains(command, "codex-stop-guard.exe") {
		t.Fatalf("expected windows hook command to target .exe binary, got %q", command)
	}
}

func TestSyncManagedConfigWritesWhitelistOnly(t *testing.T) {
	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source.toml")
	destination := filepath.Join(tempDir, "destination.toml")
	content := strings.TrimSpace(`
model = "gpt-5.4"
service_tier = "fast"
model_instructions_file = "instructions/main.md"
model_provider = "quotio"

[features]
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
	if len(features) != 1 || features["codex_hooks"] != true {
		t.Fatalf("unexpected features table: %#v", features)
	}
	reviewer := document["agents"].(map[string]any)["reviewer"].(map[string]any)
	if len(reviewer) != 1 || reviewer["config_file"] != "agents/reviewer.toml" {
		t.Fatalf("unexpected reviewer config: %#v", reviewer)
	}
}

func extractStopHookCommand(t *testing.T, document map[string]any) string {
	t.Helper()
	hooks := document["hooks"].(map[string]any)
	stop := hooks["Stop"].([]any)
	entry := stop[0].(map[string]any)
	hookList := entry["hooks"].([]any)
	hook := hookList[0].(map[string]any)
	return hook["command"].(string)
}

func platformHookBinaryName(goos string) string {
	if goos == "windows" {
		return hookBinaryName + ".exe"
	}
	return hookBinaryName
}
