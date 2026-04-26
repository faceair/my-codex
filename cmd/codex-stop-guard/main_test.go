package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStopGuardBlocksWhenOpenPlanExists(t *testing.T) {
	tempDir := t.TempDir()
	planDir := filepath.Join(tempDir, ".codex", "plans")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("create plan dir: %v", err)
	}
	planPath := filepath.Join(planDir, "2026-04-26T14-24-09-migrate-sync-and-hooks-to-go.md")
	planContent := strings.TrimSpace(`
# Plan: 2026-04-26T14-24-09-migrate-sync-and-hooks-to-go

## Meta
- status: in_progress

## Milestones
- [>] M1. 明确 Go CLI 架构与跨平台兼容策略
- [ ] M2. 实现 Go 版工具与本地兼容逻辑
`) + "\n"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	transcriptLine := map[string]any{
		"type": "response_item",
		"payload": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"text": planPath}},
		},
	}
	encodedLine, _ := json.Marshal(transcriptLine)
	if err := os.WriteFile(transcriptPath, append(encodedLine, '\n'), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	payload := map[string]any{
		"hook_event_name": "Stop",
		"cwd":             tempDir,
		"transcript_path": transcriptPath,
	}
	rawPayload, _ := json.Marshal(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := RunStopGuard(bytes.NewReader(rawPayload), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected stop guard exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	var decision map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &decision); err != nil {
		t.Fatalf("decode stop guard output: %v, raw=%s", err, stdout.String())
	}
	if decision["decision"] != "block" {
		t.Fatalf("expected stop guard to block, got %#v", decision)
	}
}
