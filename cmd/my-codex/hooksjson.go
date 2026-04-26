package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

const stopStatusMessage = "Checking unfinished plan items"

func normalizeRepoHookJSON(raw []byte) ([]byte, error) {
	return rewriteHookJSONCommand(raw, RepoHookCommand())
}

func adaptLocalHookJSON(raw []byte, hookBinaryPath string) ([]byte, error) {
	return rewriteHookJSONCommand(raw, LocalHookCommand(hookBinaryPath))
}

func rewriteHookJSONCommand(raw []byte, command string) ([]byte, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return raw, nil
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("decode hooks.json: %w", err)
	}
	replaced := replaceManagedStopHookCommand(document, command)
	if !replaced {
		return json.MarshalIndent(document, "", "  ")
	}
	formatted, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode hooks.json: %w", err)
	}
	return append(formatted, '\n'), nil
}

func replaceManagedStopHookCommand(document map[string]any, command string) bool {
	hooks, ok := document["hooks"].(map[string]any)
	if !ok {
		return false
	}
	stopEntries, ok := hooks["Stop"].([]any)
	if !ok {
		return false
	}
	for _, entryValue := range stopEntries {
		entry, ok := entryValue.(map[string]any)
		if !ok {
			continue
		}
		hookList, ok := entry["hooks"].([]any)
		if !ok {
			continue
		}
		for _, hookValue := range hookList {
			hookMap, ok := hookValue.(map[string]any)
			if !ok {
				continue
			}
			if !isManagedStopHook(hookMap) {
				continue
			}
			hookMap["command"] = command
			return true
		}
	}
	return false
}

func isManagedStopHook(hookMap map[string]any) bool {
	typeValue, _ := hookMap["type"].(string)
	if typeValue != "command" {
		return false
	}
	statusMessage, _ := hookMap["statusMessage"].(string)
	if statusMessage == stopStatusMessage {
		return true
	}
	command, _ := hookMap["command"].(string)
	lower := strings.ToLower(command)
	return strings.Contains(lower, "stop_continue_if_todo") || strings.Contains(lower, hookBinaryName)
}
