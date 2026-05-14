package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunSyncDefaultsCommitGeneratorToCodexCLI(t *testing.T) {
	tempDir := t.TempDir()
	repoRoot := filepath.Join(tempDir, "repo")
	sourceRoot := filepath.Join(tempDir, ".codex")
	for _, dir := range []string{repoRoot, sourceRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create dir %s: %v", dir, err)
		}
	}
	mustWriteFile(t, filepath.Join(sourceRoot, "agents", "reviewer.toml"), "name = \"reviewer\"\n", 0o644)
	mustWriteFile(t, filepath.Join(sourceRoot, "prompts", "commit-and-push.md"), "Write a commit message.\n", 0o644)
	mustWriteFile(t, filepath.Join(sourceRoot, "instructions", "main.md"), "# main\n", 0o644)
	mustWriteFile(t, filepath.Join(sourceRoot, "config.toml"), "model_instructions_file = \"instructions/main.md\"\n", 0o644)

	oldLookPath := execLookPath
	execLookPath = func(file string) (string, error) {
		if file == "git" || file == codexBinaryName {
			return filepath.Join(tempDir, file), nil
		}
		return "", fmt.Errorf("missing command %s", file)
	}
	defer func() { execLookPath = oldLookPath }()

	runner := &syncDefaultCodexRunner{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runSync(SyncOptions{
		RepoRoot:   repoRoot,
		SourceRoot: sourceRoot,
		Runner:     runner,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("runSync returned error: %v, stderr=%s", err, stderr.String())
	}
	if !runner.sawCodexExec {
		t.Fatalf("expected sync to generate commit message with %q exec, commands=%v", codexBinaryName, runner.commands)
	}
}

type syncDefaultCodexRunner struct {
	commands     [][]string
	sawCodexExec bool
}

func (r *syncDefaultCodexRunner) Run(command []string, options RunOptions) (RunResult, error) {
	r.commands = append(r.commands, append([]string{}, command...))
	if reflect.DeepEqual(command, []string{"git", "rev-parse", "--is-inside-work-tree"}) {
		return RunResult{Stdout: "true\n"}, nil
	}
	if len(command) >= 2 && command[0] == "git" && command[1] == "add" {
		return RunResult{}, nil
	}
	if len(command) >= 3 && command[0] == "git" && command[1] == "diff" && command[2] == "--cached" {
		if containsString(command, "--quiet") {
			return RunResult{ReturnCode: 1}, nil
		}
		return RunResult{Stdout: "diff --git a/instructions/main.md b/instructions/main.md\n"}, nil
	}
	if len(command) >= 2 && command[0] == codexBinaryName && command[1] == "exec" {
		outputPath := outputPathFromCodexCommand(command)
		if outputPath == "" {
			return RunResult{}, fmt.Errorf("missing -o output path in command: %s", strings.Join(command, " "))
		}
		if err := os.WriteFile(outputPath, []byte("chore: sync codex assets\n"), 0o644); err != nil {
			return RunResult{}, fmt.Errorf("write generated commit message %s: %w", outputPath, err)
		}
		r.sawCodexExec = true
		return RunResult{}, nil
	}
	if reflect.DeepEqual(command, []string{"git", "commit", "-m", "chore: sync codex assets"}) {
		return RunResult{}, nil
	}
	if reflect.DeepEqual(command, []string{"git", "push"}) {
		return RunResult{}, nil
	}
	if reflect.DeepEqual(command, []string{"git", "ls-files", "--error-unmatch", "--", "hooks.json"}) {
		return RunResult{ReturnCode: 1}, nil
	}
	return RunResult{}, fmt.Errorf("unexpected command: %s", strings.Join(command, " "))
}

func outputPathFromCodexCommand(command []string) string {
	for index, arg := range command {
		if arg == "-o" && index+1 < len(command) {
			return command[index+1]
		}
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
