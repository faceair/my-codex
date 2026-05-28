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
		options.CodexBinary = codexBinaryName
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
	for _, required := range []string{"agents", "config.toml", "skills"} {
		candidate := filepath.Join(sourceRoot, required)
		if _, err := os.Stat(candidate); err != nil {
			return fmt.Errorf("missing source path: %s", candidate)
		}
	}
	if err := copyDirFromOS(filepath.Join(sourceRoot, "agents"), filepath.Join(repoRoot, "agents")); err != nil {
		return err
	}
	if err := copyDirFromOS(filepath.Join(sourceRoot, "skills"), filepath.Join(repoRoot, "skills")); err != nil {
		return err
	}
	if err := syncOptionalDir(filepath.Join(sourceRoot, "instructions"), filepath.Join(repoRoot, "instructions")); err != nil {
		return err
	}
	if err := syncManagedConfig(filepath.Join(sourceRoot, "config.toml"), filepath.Join(repoRoot, "config.toml")); err != nil {
		return err
	}
	syncTargets := []string{"agents", "skills", "instructions", "config.toml"}
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
	fmt.Fprintf(stdoutWriter, "  - %s\n", filepath.Join(repoRoot, "skills"))
	fmt.Fprintf(stdoutWriter, "  - %s (incremental sync when source exists; existing destination files are preserved)\n", filepath.Join(repoRoot, "instructions"))
	fmt.Fprintf(stdoutWriter, "  - %s (whitelist only: model_instructions_file, [features].goals, [agents.reviewer].config_file)\n", filepath.Join(repoRoot, "config.toml"))
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
	return nil
}

func syncManagedConfig(source, destination string) error {
	sourceDocument, err := readTOMLFile(source)
	if err != nil {
		return err
	}
	return writeTOMLFile(destination, extractManagedDocument(sourceDocument))
}
