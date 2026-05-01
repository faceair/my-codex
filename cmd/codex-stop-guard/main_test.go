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

func TestStopGuardRequestsReviewerWhenRepeatedSimilarOutputs(t *testing.T) {
	tempDir := t.TempDir()
	planPath := writeOpenPlan(t, tempDir, "2026-05-01T14-55-06-align-stop-guard-with-python-hook.md", `
# Plan: 2026-05-01T14-55-06-align-stop-guard-with-python-hook

## Meta
- status: in_progress

## Milestones
- [>] M1. 补齐重复输出保护
- [ ] M2. 增加回归测试
`)
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	writeAssistantMessage(t, transcriptPath, planPath)
	appendAssistantMessage(t, transcriptPath, repeatedProgressMessage("继续执行重复路径", "第一次"))
	appendAssistantMessage(t, transcriptPath, repeatedProgressMessage("继续执行重复路径", "第二次"))

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
	reason, _ := decision["reason"].(string)
	if decision["decision"] != "block" || !strings.Contains(reason, "reviewer") || !strings.Contains(reason, "高相似重复输出") {
		t.Fatalf("expected repeated-loop reviewer block, got %#v", decision)
	}
}

func TestStopGuardAllowsAfterRepeatedSimilarOutputsReachLimit(t *testing.T) {
	tempDir := t.TempDir()
	planPath := writeOpenPlan(t, tempDir, "2026-05-01T14-55-06-align-stop-guard-with-python-hook.md", `
# Plan: 2026-05-01T14-55-06-align-stop-guard-with-python-hook

## Meta
- status: in_progress

## Milestones
- [>] M1. 补齐重复输出保护
- [ ] M2. 增加回归测试
`)
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	writeAssistantMessage(t, transcriptPath, planPath)
	appendAssistantMessage(t, transcriptPath, repeatedProgressMessage("继续执行重复路径", "第一次"))
	appendAssistantMessage(t, transcriptPath, repeatedProgressMessage("继续执行重复路径", "第二次"))
	appendAssistantMessage(t, transcriptPath, repeatedProgressMessage("继续执行重复路径", "第三次"))

	assertStopGuardAllows(t, tempDir, transcriptPath)
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

func writeAssistantMessage(t *testing.T, path, text string) {
	t.Helper()
	writeTranscriptLine(t, path, map[string]any{
		"type": "response_item",
		"payload": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"text": text}},
		},
	})
}

func appendAssistantMessage(t *testing.T, path, text string) {
	t.Helper()
	appendTranscriptLine(t, path, map[string]any{
		"type": "response_item",
		"payload": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"text": text}},
		},
	})
}

func repeatedProgressMessage(prefix, suffix string) string {
	return prefix + "：我会继续沿着当前执行路径检查同一批证据，并重复说明这些步骤还没有完成，需要继续当前任务直到验证闭环。这里保留足够长的稳定文本，用来模拟真实长输出中只有少量词变化但整体内容高度相似的重复回路。" + suffix
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

func assertStopGuardAllows(t *testing.T, cwd, transcriptPath string) {
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
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("expected stop guard to allow without output, got %s", stdout.String())
	}
}
