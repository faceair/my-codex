package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunSyncStagesCommitsAndPushes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("integration test currently exercises shell-based fake codex on non-Windows platforms")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tempDir := t.TempDir()
	remoteRoot := filepath.Join(tempDir, "remote.git")
	repoRoot := filepath.Join(tempDir, "repo")
	sourceRoot := filepath.Join(tempDir, ".codex")
	codexPath := filepath.Join(tempDir, "fake-codex")

	runGit(t, tempDir, "git", "init", "--bare", remoteRoot)
	runGit(t, tempDir, "git", "init", repoRoot)
	runGit(t, repoRoot, "git", "config", "user.name", "Test User")
	runGit(t, repoRoot, "git", "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# temp repo\n"), 0o644); err != nil {
		t.Fatalf("write repo README: %v", err)
	}
	runGit(t, repoRoot, "git", "add", "README.md")
	runGit(t, repoRoot, "git", "commit", "-m", "chore: initial")
	runGit(t, repoRoot, "git", "branch", "-M", "main")
	runGit(t, repoRoot, "git", "remote", "add", "origin", remoteRoot)
	runGit(t, repoRoot, "git", "push", "-u", "origin", "main")

	mustWriteFile(t, filepath.Join(sourceRoot, "agents", "reviewer.toml"), "name = \"reviewer\"\n", 0o644)
	mustWriteFile(t, filepath.Join(sourceRoot, "skills", "commit-and-push", "SKILL.md"), "# Commit and Push\n", 0o644)
	mustWriteFile(t, filepath.Join(sourceRoot, "instructions", "main.md"), "# main\n", 0o644)
	mustWriteFile(t, filepath.Join(sourceRoot, "config.toml"), strings.TrimSpace(`
model = "ignored"
model_instructions_file = "instructions/main.md"

[features]
goals = true
codex_hooks = true
chatty_output = true

[agents.reviewer]
config_file = "agents/reviewer.toml"
model = "ignored"
`)+"\n", 0o644)
	mustWriteFile(t, codexPath, "#!/usr/bin/env sh\nset -eu\nout=\"\"\nprev=\"\"\nfor arg in \"$@\"; do\n  if [ \"$prev\" = \"-o\" ]; then\n    out=\"$arg\"\n  fi\n  prev=\"$arg\"\ndone\nprintf 'chore: sync codex assets\\n' > \"$out\"\n", 0o755)
	mustWriteFile(t, filepath.Join(repoRoot, "instructions", "legacy.md"), "legacy\n", 0o644)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runSync(SyncOptions{
		RepoRoot:    repoRoot,
		SourceRoot:  sourceRoot,
		CodexBinary: codexPath,
		Version:     "test",
		Runner:      ExecRunner{},
	}, &stdout, &stderr); err != nil {
		t.Fatalf("runSync returned error: %v, stderr=%s", err, stderr.String())
	}

	configContent, err := os.ReadFile(filepath.Join(repoRoot, "config.toml"))
	if err != nil {
		t.Fatalf("read synced config.toml: %v", err)
	}
	configText := string(configContent)
	for _, unexpected := range []string{"chatty_output", "codex_hooks", "model = \"ignored\""} {
		if strings.Contains(configText, unexpected) {
			t.Fatalf("expected synced config.toml to keep goals whitelist only, found %q in:\n%s", unexpected, configText)
		}
	}
	for _, expected := range []string{"model_instructions_file = \"instructions/main.md\"", "goals = true", "config_file = \"agents/reviewer.toml\""} {
		if !strings.Contains(configText, expected) {
			t.Fatalf("expected synced config.toml to include %q, got:\n%s", expected, configText)
		}
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("expected sync not to create hooks.json, err=%v", err)
	}
	for _, target := range []string{
		filepath.Join(repoRoot, "instructions", "legacy.md"),
	} {
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("expected legacy file to be preserved at %s: %v", target, err)
		}
	}

	headCommit := strings.TrimSpace(runGit(t, repoRoot, "git", "log", "-1", "--pretty=%s"))
	if headCommit != "chore: sync codex assets" {
		t.Fatalf("unexpected sync commit subject: %q", headCommit)
	}
	localHead := strings.TrimSpace(runGit(t, repoRoot, "git", "rev-parse", "HEAD"))
	remoteHead := strings.TrimSpace(runGit(t, repoRoot, "git", "rev-parse", "origin/main"))
	if localHead != remoteHead {
		t.Fatalf("expected pushed head to match origin/main, local=%s remote=%s", localHead, remoteHead)
	}
}

func mustWriteFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func runGit(t *testing.T, cwd string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: cwd=%s command=%s %s output=%s err=%v", cwd, name, strings.Join(args, " "), string(output), err)
	}
	return string(output)
}
