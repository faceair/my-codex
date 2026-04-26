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
	planPath := writeOpenPlan(t, tempDir, "2026-04-26T14-24-09-migrate-sync-and-hooks-to-go.md", `
# Plan: 2026-04-26T14-24-09-migrate-sync-and-hooks-to-go

## Meta
- status: in_progress

## Milestones
- [>] M1. 明确 Go CLI 架构与跨平台兼容策略
- [ ] M2. 实现 Go 版工具与本地兼容逻辑
`)
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	writeTranscriptLine(t, transcriptPath, map[string]any{
		"type": "response_item",
		"payload": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"text": planPath}},
		},
	})
	assertStopGuardBlocks(t, tempDir, transcriptPath)
}

func TestStopGuardBlocksAfterLargeCompactedLineAndLargeSessionMeta(t *testing.T) {
	tempDir := t.TempDir()
	planPath := writeOpenPlan(t, tempDir, "2026-04-26T23-15-20-register-flow-with-icloud-and-historical-cleanup.md", `
# Plan: 2026-04-26T23-15-20-register-flow-with-icloud-and-historical-cleanup

## Meta
- status: in_progress

## Milestones
- [>] M1. 补齐注册链路缺口并收敛 iCloud provider 语义
- [ ] M2. 完成注册主流程真实验证
`)
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	writeTranscriptLine(t, transcriptPath, map[string]any{
		"type": "session_meta",
		"payload": map[string]any{
			"source":            map[string]any{},
			"base_instructions": map[string]any{"text": strings.Repeat("A", maxScannerTokenSize/2)},
		},
	})
	appendTranscriptLine(t, transcriptPath, map[string]any{
		"type": "compacted",
		"payload": map[string]any{
			"message": "",
			"replacement_history": []map[string]any{{
				"type":    "message",
				"role":    "assistant",
				"content": []map[string]any{{"type": "output_text", "text": strings.Repeat("B", maxScannerTokenSize)}},
			}},
		},
	})
	appendTranscriptLine(t, transcriptPath, map[string]any{
		"type": "response_item",
		"payload": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"text": planPath}},
		},
	})
	assertStopGuardBlocks(t, tempDir, transcriptPath)
}

func TestStopGuardIgnoresTrailingPlanRefsFromOtherRepos(t *testing.T) {
	tempDir := t.TempDir()
	planPath := writeOpenPlan(t, tempDir, "2026-04-26T23-15-20-register-flow-with-icloud-and-historical-cleanup.md", `
# Plan: 2026-04-26T23-15-20-register-flow-with-icloud-and-historical-cleanup

## Meta
- status: in_progress

## Milestones
- [>] M1. 补齐注册链路缺口并收敛 iCloud provider 语义
- [ ] M2. 完成注册主流程真实验证
`)
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	writeTranscriptLine(t, transcriptPath, map[string]any{
		"type": "response_item",
		"payload": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"text": planPath}},
		},
	})
	appendTranscriptLine(t, transcriptPath, map[string]any{
		"type": "response_item",
		"payload": map[string]any{
			"type":   "function_call_output",
			"output": "Chunk ID: noise\nOutput:\n/Users/faceair/Developer/other/.codex/plans/2026-04-17T02-12-00-hotmail-oauth-airouter.md",
		},
	})
	assertStopGuardBlocks(t, tempDir, transcriptPath)
}

func TestStopGuardFallsBackToFunctionCallWhenAssistantDidNotMentionPlan(t *testing.T) {
	tempDir := t.TempDir()
	_ = writeOpenPlan(t, tempDir, "2026-04-26T23-15-20-register-flow-with-icloud-and-historical-cleanup.md", `
# Plan: 2026-04-26T23-15-20-register-flow-with-icloud-and-historical-cleanup

## Meta
- status: in_progress

## Milestones
- [>] M1. 补齐注册链路缺口并收敛 iCloud provider 语义
- [ ] M2. 完成注册主流程真实验证
`)
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	writeTranscriptLine(t, transcriptPath, map[string]any{
		"type": "response_item",
		"payload": map[string]any{
			"type":      "function_call",
			"arguments": `{"cmd":"cat > .codex/plans/2026-04-26T23-15-20-register-flow-with-icloud-and-historical-cleanup.md"}`,
		},
	})
	assertStopGuardBlocks(t, tempDir, transcriptPath)
}

func writeOpenPlan(t *testing.T, tempDir, name, body string) string {
	t.Helper()
	planDir := filepath.Join(tempDir, ".codex", "plans")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("create plan dir: %v", err)
	}
	planPath := filepath.Join(planDir, name)
	planContent := strings.TrimSpace(body) + "\n"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}
	return planPath
}

func writeTranscriptLine(t *testing.T, path string, value map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode transcript line: %v", err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		t.Fatalf("write transcript line: %v", err)
	}
}

func appendTranscriptLine(t *testing.T, path string, value map[string]any) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open transcript for append: %v", err)
	}
	defer file.Close()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode transcript line: %v", err)
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		t.Fatalf("append transcript line: %v", err)
	}
}

func assertStopGuardBlocks(t *testing.T, cwd, transcriptPath string) {
	t.Helper()
	payload := map[string]any{
		"hook_event_name": "Stop",
		"cwd":             cwd,
		"transcript_path": transcriptPath,
	}
	rawPayload, _ := json.Marshal(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := RunStopGuard(bytes.NewReader(rawPayload), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected stop guard exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	assertBlockedDecision(t, stdout.Bytes())
}

func assertBlockedDecision(t *testing.T, raw []byte) {
	t.Helper()
	var decision map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &decision); err != nil {
		t.Fatalf("decode stop guard output: %v, raw=%s", err, string(raw))
	}
	if decision["decision"] != "block" {
		t.Fatalf("expected stop guard to block, got %#v", decision)
	}
}
