package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type SyncOptions struct {
	RepoRoot    string
	SourceRoot  string
	CodexBinary string
	Version     string
	Runner      CommandRunner
}

func RunSync(options SyncOptions, stdout, stderr io.Writer) error {
	return runSync(options, stdout, stderr)
}

func runSync(options SyncOptions, stdoutWriter, stderrWriter io.Writer) error {
	repoRoot := filepath.Clean(options.RepoRoot)
	sourceRoot := filepath.Clean(options.SourceRoot)
	if options.Runner == nil {
		options.Runner = ExecRunner{}
	}
	if options.CodexBinary == "" {
		options.CodexBinary = managedBinaryName
	}
	if err := ensureCommandExists("git"); err != nil {
		return err
	}
	gitCheck, err := options.Runner.Run([]string{"git", "rev-parse", "--is-inside-work-tree"}, RunOptions{Cwd: repoRoot})
	if err != nil {
		return err
	}
	if strings.TrimSpace(gitCheck.Stdout) != "true" {
		return fmt.Errorf("current directory is not a git repository: %s", repoRoot)
	}
	for _, required := range []string{"agents", "config.toml", "prompts"} {
		candidate := filepath.Join(sourceRoot, required)
		if _, err := os.Stat(candidate); err != nil {
			return fmt.Errorf("missing source path: %s", candidate)
		}
	}
	if err := copyDirFromOS(filepath.Join(sourceRoot, "agents"), filepath.Join(repoRoot, "agents")); err != nil {
		return err
	}
	if err := copyDirFromOS(filepath.Join(sourceRoot, "prompts"), filepath.Join(repoRoot, "prompts")); err != nil {
		return err
	}
	if err := syncOptionalDir(filepath.Join(sourceRoot, "instructions"), filepath.Join(repoRoot, "instructions")); err != nil {
		return err
	}
	if err := syncOptionalDir(filepath.Join(sourceRoot, "hooks"), filepath.Join(repoRoot, "hooks")); err != nil {
		return err
	}
	if err := syncHooksJSONForRepo(filepath.Join(sourceRoot, "hooks.json"), filepath.Join(repoRoot, "hooks.json")); err != nil {
		return err
	}
	if err := syncManagedConfig(filepath.Join(sourceRoot, "config.toml"), filepath.Join(repoRoot, "config.toml")); err != nil {
		return err
	}
	syncTargets := []string{"agents", "prompts", "instructions", "config.toml"}
	for _, optionalTarget := range []string{"hooks", "hooks.json"} {
		present, err := trackedOrPresent(repoRoot, optionalTarget, options.Runner)
		if err != nil {
			return err
		}
		if present {
			syncTargets = append(syncTargets, optionalTarget)
		}
	}
	if _, err := options.Runner.Run(append([]string{"git", "add", "-A", "--"}, syncTargets...), RunOptions{Cwd: repoRoot}); err != nil {
		return err
	}
	diffQuiet, err := options.Runner.Run(append([]string{"git", "diff", "--cached", "--quiet", "--"}, syncTargets...), RunOptions{Cwd: repoRoot, AllowedReturnCodes: []int{0, 1}})
	if err != nil {
		return err
	}
	if diffQuiet.ReturnCode == 0 {
		fmt.Fprintf(stdoutWriter, "Synced to: %s\n", repoRoot)
		fmt.Fprintln(stdoutWriter, "No changes in sync targets. Skip commit and push.")
		return nil
	}
	if err := ensureCommandExists(options.CodexBinary); err != nil {
		return err
	}
	commitMessage, err := generateCommitMessage(repoRoot, options.CodexBinary, options.Runner, syncTargets)
	if err != nil {
		return err
	}
	if _, err := options.Runner.Run([]string{"git", "commit", "-m", commitMessage}, RunOptions{Cwd: repoRoot}); err != nil {
		return err
	}
	if _, err := options.Runner.Run([]string{"git", "push"}, RunOptions{Cwd: repoRoot}); err != nil {
		return err
	}
	fmt.Fprintf(stdoutWriter, "Synced to: %s\n", repoRoot)
	fmt.Fprintln(stdoutWriter, "Updated files:")
	fmt.Fprintf(stdoutWriter, "  - %s\n", filepath.Join(repoRoot, "agents"))
	fmt.Fprintf(stdoutWriter, "  - %s\n", filepath.Join(repoRoot, "prompts"))
	fmt.Fprintf(stdoutWriter, "  - %s (synced when source exists, removed when absent)\n", filepath.Join(repoRoot, "instructions"))
	fmt.Fprintf(stdoutWriter, "  - %s (synced when source exists, removed when absent; legacy Python hook files are no longer required)\n", filepath.Join(repoRoot, "hooks"))
	fmt.Fprintf(stdoutWriter, "  - %s (managed stop hook command normalized to macOS-style repo path)\n", filepath.Join(repoRoot, "hooks.json"))
	fmt.Fprintf(stdoutWriter, "  - %s (whitelist only: model_instructions_file, [features].codex_hooks, [agents.reviewer].config_file)\n", filepath.Join(repoRoot, "config.toml"))
	fmt.Fprintln(stdoutWriter, "Committed and pushed with message:")
	fmt.Fprintln(stdoutWriter, commitMessage)
	_ = stderrWriter
	return nil
}

func syncOptionalDir(source, destination string) error {
	if _, err := os.Stat(source); err == nil {
		return copyDirFromOS(source, destination)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat optional dir %s: %w", source, err)
	}
	if err := os.RemoveAll(destination); err != nil {
		return fmt.Errorf("remove optional dir %s: %w", destination, err)
	}
	return nil
}

func syncHooksJSONForRepo(source, destination string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		if os.IsNotExist(err) {
			if removeErr := os.Remove(destination); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove repo hooks.json %s: %w", destination, removeErr)
			}
			return nil
		}
		return fmt.Errorf("read source hooks.json %s: %w", source, err)
	}
	normalized, err := normalizeRepoHookJSON(raw)
	if err != nil {
		return err
	}
	if err := os.WriteFile(destination, normalized, 0o644); err != nil {
		return fmt.Errorf("write repo hooks.json %s: %w", destination, err)
	}
	return nil
}

func syncManagedConfig(source, destination string) error {
	sourceDocument, err := readTOMLFile(source)
	if err != nil {
		return err
	}
	return writeTOMLFile(destination, extractManagedDocument(sourceDocument))
}
