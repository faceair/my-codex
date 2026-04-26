package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	maxPlanAge           = 24 * time.Hour
	maxScannerTokenSize  = 2 * 1024 * 1024
	reverseReadBlockSize = 64 * 1024
)

var (
	openItemRE    = regexp.MustCompile(`(?m)^\s*[-*]\s*\[( |>)\]\s+(.+)$`)
	finalStatusRE = regexp.MustCompile(`(?i)final status\s*[:：]\s*(\S+)`)
	planRefRE     = regexp.MustCompile(`(?i)(?P<path>(?:[A-Za-z]:)?[^\s\x60'\"]*?\.codex/plans/(?P<slug1>[a-z0-9][a-z0-9-]*)\.md)|(?P<rel>plans/(?P<slug2>[a-z0-9][a-z0-9-]*)\.md)|#\s*Plan:\s*(?P<slug3>[a-z0-9][a-z0-9-]*)`)
)

func main() {
	os.Exit(RunStopGuard(os.Stdin, os.Stdout, os.Stderr))
}

func RunStopGuard(stdin io.Reader, stdout, stderr io.Writer) int {
	decision, err := evaluateStopGuard(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to evaluate stop guard with context: %v\n", err)
		return 1
	}
	if decision == nil {
		return 0
	}
	encoded, err := json.Marshal(decision)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to encode stop guard decision with context: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, string(encoded))
	return 0
}

type stopGuardDecision struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type hookPayload struct {
	HookEventName  string `json:"hook_event_name"`
	Cwd            string `json:"cwd"`
	TranscriptPath string `json:"transcript_path"`
}

type transcriptText struct {
	Kind string
	Text string
}

func evaluateStopGuard(stdin io.Reader) (*stopGuardDecision, error) {
	payload, err := loadHookPayload(stdin)
	if err != nil {
		return nil, err
	}
	if payload.HookEventName != "Stop" || payload.Cwd == "" || payload.TranscriptPath == "" {
		return nil, nil
	}
	transcriptPath := filepath.Clean(payload.TranscriptPath)
	isSubagent, err := isSubagentSession(transcriptPath)
	if err != nil || isSubagent {
		return nil, nil
	}
	activePlanPath, err := latestResolvedPlanPathFromTranscript(filepath.Clean(payload.Cwd), transcriptPath)
	if err != nil || activePlanPath == "" {
		return nil, nil
	}
	openCount, examples, ok := planSummary(activePlanPath)
	if !ok {
		return nil, nil
	}
	reason := fmt.Sprintf("继续当前任务。检测到当前活跃计划 %s 仍有 %d 个未完成项。优先完成当前 active milestone 或明确记录 blocker 后再停止。未完成项示例：%s", filepath.Base(activePlanPath), openCount, strings.Join(examples, "；"))
	return &stopGuardDecision{Decision: "block", Reason: reason}, nil
}

func loadHookPayload(reader io.Reader) (hookPayload, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return hookPayload{}, fmt.Errorf("read hook payload: %w", err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return hookPayload{}, nil
	}
	var payload hookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return hookPayload{}, fmt.Errorf("decode hook payload: %w", err)
	}
	return payload, nil
}

func isSubagentSession(transcriptPath string) (bool, error) {
	file, err := os.Open(transcriptPath)
	if err != nil {
		return false, err
	}
	defer file.Close()
	scanner := newScanner(file)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	var line map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
		return false, err
	}
	payload, _ := line["payload"].(map[string]any)
	source, _ := payload["source"].(map[string]any)
	_, exists := source["subagent"]
	return exists, nil
}

func latestResolvedPlanPathFromTranscript(cwd, transcriptPath string) (string, error) {
	for _, allowedKinds := range [][]string{{"agent_message", "assistant"}, {"function_call", "function_call_output"}} {
		resolvedPlanPath := ""
		err := reverseScanJSONLLines(transcriptPath, func(line []byte) bool {
			for _, item := range transcriptTextsFromLine(line) {
				if !containsKind(allowedKinds, item.Kind) {
					continue
				}
				path, slug, ok := latestPlanRefFromText(item.Text)
				if !ok {
					continue
				}
				candidate, err := resolvePlanPath(cwd, path, slug)
				if err != nil || candidate == "" {
					continue
				}
				resolvedPlanPath = candidate
				return false
			}
			return true
		})
		if err != nil {
			return "", err
		}
		if resolvedPlanPath != "" {
			return resolvedPlanPath, nil
		}
	}
	return "", nil
}

func transcriptTextsFromLine(raw []byte) []transcriptText {
	var line map[string]any
	if err := json.Unmarshal(raw, &line); err != nil {
		return nil
	}
	payload, _ := line["payload"].(map[string]any)
	var results []transcriptText
	switch line["type"] {
	case "event_msg":
		if payload["type"] == "agent_message" {
			if message, ok := payload["message"].(string); ok {
				results = append(results, transcriptText{Kind: "agent_message", Text: message})
			}
		}
	case "response_item":
		if payload["type"] == "function_call_output" {
			if output, ok := payload["output"].(string); ok {
				results = append(results, transcriptText{Kind: "function_call_output", Text: output})
			}
		}
		if payload["type"] == "function_call" {
			if arguments, ok := payload["arguments"].(string); ok {
				results = append(results, transcriptText{Kind: "function_call", Text: arguments})
			}
		}
		if payload["role"] == "assistant" {
			if content, ok := payload["content"].([]any); ok {
				parts := make([]string, 0, len(content))
				for _, item := range content {
					if entry, ok := item.(map[string]any); ok {
						if text, ok := entry["text"].(string); ok {
							parts = append(parts, text)
						} else if text, ok := entry["content"].(string); ok {
							parts = append(parts, text)
						}
					}
				}
				if len(parts) > 0 {
					results = append(results, transcriptText{Kind: "assistant", Text: strings.Join(parts, "\n")})
				}
			}
		}
	}
	return results
}

func containsKind(kinds []string, target string) bool {
	for _, kind := range kinds {
		if kind == target {
			return true
		}
	}
	return false
}

func latestPlanRefFromText(text string) (string, string, bool) {
	matches := planRefRE.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return "", "", false
	}
	match := matches[len(matches)-1]
	values := map[string]string{}
	for index, name := range planRefRE.SubexpNames() {
		if name == "" || index >= len(match) {
			continue
		}
		values[name] = match[index]
	}
	slug := values["slug1"]
	if slug == "" {
		slug = values["slug2"]
	}
	if slug == "" {
		slug = values["slug3"]
	}
	if slug == "" {
		return "", "", false
	}
	return values["path"], slug, true
}

func reverseScanJSONLLines(path string, visit func([]byte) bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	position := info.Size()
	var tail []byte
	for position > 0 {
		start := position - reverseReadBlockSize
		if start < 0 {
			start = 0
		}
		chunk := make([]byte, position-start)
		if _, err := file.ReadAt(chunk, start); err != nil {
			return err
		}
		data := append(chunk, tail...)
		parts := bytes.Split(data, []byte{'\n'})
		for index := len(parts) - 1; index >= 1; index-- {
			line := bytes.TrimSpace(parts[index])
			if len(line) == 0 {
				continue
			}
			if !visit(line) {
				return nil
			}
		}
		tail = append(tail[:0], parts[0]...)
		position = start
	}
	line := bytes.TrimSpace(tail)
	if len(line) > 0 {
		visit(line)
	}
	return nil
}

func newScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScannerTokenSize)
	return scanner
}

func resolvePlanPath(cwd, transcriptPlanPath, transcriptSlug string) (string, error) {
	planDirs := planDirsFor(cwd)
	allowed := map[string]struct{}{}
	for _, dir := range planDirs {
		allowed[dir] = struct{}{}
	}
	if transcriptPlanPath != "" {
		candidates := []string{transcriptPlanPath}
		if !filepath.IsAbs(transcriptPlanPath) {
			candidates = append(candidates, filepath.Join(cwd, transcriptPlanPath))
		}
		for _, candidate := range candidates {
			resolved, err := filepath.Abs(candidate)
			if err != nil {
				continue
			}
			if _, err := os.Stat(resolved); err == nil {
				if _, ok := allowed[filepath.Dir(resolved)]; ok {
					return resolved, nil
				}
			}
		}
	}
	if transcriptSlug != "" {
		for _, dir := range planDirs {
			candidate := filepath.Join(dir, transcriptSlug+".md")
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", nil
}

func planDirsFor(cwd string) []string {
	var candidates []string
	base := filepath.Clean(cwd)
	if filepath.Base(base) == ".codex" {
		candidates = append(candidates, filepath.Join(base, "plans"))
	}
	candidates = append(candidates, filepath.Join(base, ".codex", "plans"))
	candidates = append(candidates, filepath.Join(base, "plans"))
	unique := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		resolved, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		unique = append(unique, resolved)
	}
	return unique
}

func planSummary(planPath string) (int, []string, bool) {
	info, err := os.Stat(planPath)
	if err != nil {
		return 0, nil, false
	}
	if time.Since(info.ModTime()) > maxPlanAge {
		return 0, nil, false
	}
	content, err := os.ReadFile(planPath)
	if err != nil {
		return 0, nil, false
	}
	text := string(content)
	if status := finalStatusRE.FindStringSubmatch(text); len(status) == 2 {
		s := strings.ToLower(strings.TrimSpace(status[1]))
		if s == "done" || s == "cancelled" || s == "completed" {
			return 0, nil, false
		}
	}
	matches := openItemRE.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return 0, nil, false
	}
	examples := make([]string, 0, 3)
	for _, match := range matches {
		if len(match) >= 3 {
			examples = append(examples, strings.TrimSpace(match[2]))
			if len(examples) == 3 {
				break
			}
		}
	}
	return len(matches), examples, true
}
