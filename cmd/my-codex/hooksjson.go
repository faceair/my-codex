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

func mergeManagedHookJSON(sourceRaw, targetRaw []byte, command string) ([]byte, error) {
	sourceDocument, err := decodeHookJSON(sourceRaw)
	if err != nil {
		return nil, err
	}
	targetDocument, err := decodeHookJSON(targetRaw)
	if err != nil {
		return nil, err
	}
	managedHook, ok := extractManagedStopHook(sourceDocument)
	if !ok {
		return formatHookJSON(targetDocument)
	}
	managedHookCopy, _ := deepCopy(managedHook).(map[string]any)
	if managedHookCopy == nil {
		managedHookCopy = map[string]any{}
	}
	managedHookCopy["command"] = command
	mergeManagedStopHook(targetDocument, managedHookCopy)
	return formatHookJSON(targetDocument)
}

func rewriteHookJSONCommand(raw []byte, command string) ([]byte, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return raw, nil
	}
	document, err := decodeHookJSON(raw)
	if err != nil {
		return nil, err
	}
	replaced := replaceManagedStopHookCommand(document, command)
	if !replaced {
		return formatHookJSON(document)
	}
	return formatHookJSON(document)
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

func extractManagedStopHook(document map[string]any) (map[string]any, bool) {
	hooks, ok := document["hooks"].(map[string]any)
	if !ok {
		return nil, false
	}
	stopEntries, ok := hooks["Stop"].([]any)
	if !ok {
		return nil, false
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
			if isManagedStopHook(hookMap) {
				return hookMap, true
			}
		}
	}
	return nil, false
}

func mergeManagedStopHook(document map[string]any, managedHook map[string]any) {
	hooks, ok := document["hooks"].(map[string]any)
	if !ok || hooks == nil {
		hooks = map[string]any{}
		document["hooks"] = hooks
	}
	stopEntries, ok := hooks["Stop"].([]any)
	if !ok || stopEntries == nil {
		hooks["Stop"] = []any{map[string]any{"hooks": []any{managedHook}}}
		return
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
		for index, hookValue := range hookList {
			hookMap, ok := hookValue.(map[string]any)
			if !ok {
				continue
			}
			if isManagedStopHook(hookMap) {
				hookList[index] = managedHook
				entry["hooks"] = hookList
				return
			}
		}
	}
	if firstEntry, ok := stopEntries[0].(map[string]any); ok {
		if hookList, ok := firstEntry["hooks"].([]any); ok {
			firstEntry["hooks"] = append(hookList, managedHook)
			stopEntries[0] = firstEntry
			hooks["Stop"] = stopEntries
			return
		}
	}
	hooks["Stop"] = append(stopEntries, map[string]any{"hooks": []any{managedHook}})
}

func decodeHookJSON(raw []byte) (map[string]any, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}, nil
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("decode hooks.json: %w", err)
	}
	if document == nil {
		return map[string]any{}, nil
	}
	return document, nil
}

func formatHookJSON(document map[string]any) ([]byte, error) {
	formatted, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode hooks.json: %w", err)
	}
	return append(formatted, '\n'), nil
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
