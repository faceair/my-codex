package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ensureCommandExists(command string) error {
	if filepath.IsAbs(command) {
		if _, err := os.Stat(command); err != nil {
			return fmt.Errorf("required command not found: %s", command)
		}
		return nil
	}
	if _, err := execLookPath(command); err != nil {
		return fmt.Errorf("required command not found: %s", command)
	}
	return nil
}

var execLookPath = func(file string) (string, error) {
	return exec.LookPath(file)
}

func findCommitPromptFile(promptsDir string) (string, error) {
	for _, candidate := range []string{"commit-and-push.md", "smart-commit.md"} {
		path := filepath.Join(promptsDir, candidate)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("missing commit prompt in %s: expected commit-and-push.md or smart-commit.md", promptsDir)
}

func generateCommitMessage(repoRoot, codexBinary string, runner CommandRunner, syncTargets []string) (string, error) {
	diffResult, err := runner.Run(append([]string{"git", "diff", "--cached", "--"}, syncTargets...), RunOptions{Cwd: repoRoot})
	if err != nil {
		return "", err
	}
	promptFile, err := findCommitPromptFile(filepath.Join(repoRoot, "prompts"))
	if err != nil {
		return "", err
	}
	promptContent, err := os.ReadFile(promptFile)
	if err != nil {
		return "", fmt.Errorf("read commit prompt %s: %w", promptFile, err)
	}
	promptText := strings.Join([]string{
		strings.TrimSpace(string(promptContent)),
		"",
		"Automation-specific overrides:",
		"- Do not run git status, git diff, git add, git commit, or git push.",
		"- The staged diff is provided below, so do not ask for more input.",
		"- Output only the git commit message text.",
		"- The first line must be the summary line.",
		"- If a body is needed, separate it from the summary with a blank line.",
		"- Do not include code fences, quotes, markdown, or explanations.",
		"",
		"Staged diff:",
		diffResult.Stdout,
	}, "\n")
	tempFile, err := os.CreateTemp("", "codex-commit-message-*")
	if err != nil {
		return "", fmt.Errorf("create temp commit message output file: %w", err)
	}
	outputPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(outputPath)
	if _, err := runner.Run([]string{codexBinary, "exec", "--color", "never", "-o", outputPath, "-"}, RunOptions{Cwd: repoRoot, Input: promptText}); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("read generated commit message %s: %w", outputPath, err)
	}
	trimmed := trimBlankLines(strings.Split(strings.ReplaceAll(string(raw), "\r", ""), "\n"))
	if len(trimmed) == 0 {
		return "", fmt.Errorf("failed to generate commit message from codex output")
	}
	return strings.Join(trimmed, "\n"), nil
}

func trimBlankLines(lines []string) []string {
	start := 0
	end := len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}
