package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	managedassets "github.com/faceair/my-codex"
)

type PullOptions struct {
	DestRoot        string
	HookBinaryPath  string
	Runner          CommandRunner
	Platform        Platform
	ManagedAssetsFS fs.FS
}

func runPull(options PullOptions, stdoutWriter, stderrWriter io.Writer) error {
	if options.Platform.GOOS == "" {
		options.Platform = CurrentPlatform()
	}
	if options.ManagedAssetsFS == nil {
		options.ManagedAssetsFS = managedassets.FS
	}
	destRoot := filepath.Clean(options.DestRoot)
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return fmt.Errorf("create codex root %s: %w", destRoot, err)
	}
	hookBinaryPath, cleanup, err := resolveHookBinary(options)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := copyDirFromFS(options.ManagedAssetsFS, "agents", filepath.Join(destRoot, "agents")); err != nil {
		return err
	}
	if err := copyDirFromFS(options.ManagedAssetsFS, "prompts", filepath.Join(destRoot, "prompts")); err != nil {
		return err
	}
	if err := copyDirFromFS(options.ManagedAssetsFS, "instructions", filepath.Join(destRoot, "instructions")); err != nil {
		return err
	}
	if err := copyDirFromFS(options.ManagedAssetsFS, "hooks", filepath.Join(destRoot, "hooks")); err != nil {
		return err
	}
	installedHookPath := filepath.Join(destRoot, "hooks", options.Platform.HookBinaryFilename())
	if err := os.MkdirAll(filepath.Dir(installedHookPath), 0o755); err != nil {
		return fmt.Errorf("create hooks dir for %s: %w", installedHookPath, err)
	}
	if err := installBinary(hookBinaryPath, installedHookPath); err != nil {
		return fmt.Errorf("install hook binary to %s: %w", installedHookPath, err)
	}
	if err := pullManagedHooksJSON(options.ManagedAssetsFS, filepath.Join(destRoot, "hooks.json"), installedHookPath); err != nil {
		return err
	}
	if err := pullManagedConfig(options.ManagedAssetsFS, filepath.Join(destRoot, "config.toml")); err != nil {
		return err
	}
	fmt.Fprintf(stdoutWriter, "Pulled into: %s\n", destRoot)
	fmt.Fprintln(stdoutWriter, "Updated:")
	fmt.Fprintf(stdoutWriter, "  - %s\n", filepath.Join(destRoot, "agents"))
	fmt.Fprintf(stdoutWriter, "  - %s\n", filepath.Join(destRoot, "prompts"))
	fmt.Fprintf(stdoutWriter, "  - %s (synced from embedded release assets)\n", filepath.Join(destRoot, "instructions"))
	fmt.Fprintf(stdoutWriter, "  - %s (legacy hook scripts replaced; hook binary installed here)\n", filepath.Join(destRoot, "hooks"))
	fmt.Fprintf(stdoutWriter, "  - %s (managed stop hook command adapted for local platform)\n", filepath.Join(destRoot, "hooks.json"))
	fmt.Fprintf(stdoutWriter, "  - %s (repo-managed whitelist merged in; other local config preserved)\n", filepath.Join(destRoot, "config.toml"))
	fmt.Fprintf(stdoutWriter, "  - %s\n", installedHookPath)
	_ = stderrWriter
	return nil
}

func resolveHookBinary(options PullOptions) (string, func(), error) {
	if options.HookBinaryPath != "" {
		return filepath.Clean(options.HookBinaryPath), func() {}, nil
	}
	currentExecutable, err := os.Executable()
	if err != nil {
		return "", func() {}, fmt.Errorf("resolve current executable: %w", err)
	}
	sibling := filepath.Join(filepath.Dir(currentExecutable), options.Platform.HookBinaryFilename())
	if _, err := os.Stat(sibling); err == nil {
		return sibling, func() {}, nil
	}
	if resolved, err := execLookPath(options.Platform.HookBinaryFilename()); err == nil {
		return resolved, func() {}, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", func() {}, fmt.Errorf("resolve current working directory: %w", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
		tempFile, err := os.CreateTemp("", "codex-stop-guard-*")
		if err != nil {
			return "", func() {}, fmt.Errorf("create temp hook binary: %w", err)
		}
		tempPath := tempFile.Name()
		tempFile.Close()
		if options.Platform.IsWindows() {
			tempPath += ".exe"
		}
		runner := options.Runner
		if runner == nil {
			runner = ExecRunner{}
		}
		if _, err := runner.Run([]string{"go", "build", "-o", tempPath, "./cmd/codex-stop-guard"}, RunOptions{Cwd: cwd}); err != nil {
			return "", func() {}, err
		}
		return tempPath, func() { _ = os.Remove(tempPath) }, nil
	}
	return "", func() {}, fmt.Errorf("hook binary not found next to my-codex and no local repo build fallback available")
}

func pullManagedHooksJSON(sourceFS fs.FS, destination, installedHookPath string) error {
	raw, err := fs.ReadFile(sourceFS, "hooks.json")
	if err != nil {
		return fmt.Errorf("read embedded hooks.json: %w", err)
	}
	adapted, err := adaptLocalHookJSON(raw, installedHookPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(destination, adapted, 0o644); err != nil {
		return fmt.Errorf("write local hooks.json %s: %w", destination, err)
	}
	return nil
}

func pullManagedConfig(sourceFS fs.FS, destination string) error {
	embeddedConfig, err := fs.ReadFile(sourceFS, "config.toml")
	if err != nil {
		return fmt.Errorf("read embedded config.toml: %w", err)
	}
	tempEmbedded, err := os.CreateTemp("", "my-codex-config-*.toml")
	if err != nil {
		return fmt.Errorf("create temp embedded config copy: %w", err)
	}
	tempEmbeddedPath := tempEmbedded.Name()
	defer os.Remove(tempEmbeddedPath)
	if _, err := tempEmbedded.Write(embeddedConfig); err != nil {
		tempEmbedded.Close()
		return fmt.Errorf("write temp embedded config copy: %w", err)
	}
	tempEmbedded.Close()
	localDocument, err := readTOMLFile(destination)
	if err != nil {
		return err
	}
	embeddedDocument, err := readTOMLFile(tempEmbeddedPath)
	if err != nil {
		return err
	}
	return writeTOMLFile(destination, mergeManagedDocument(localDocument, extractManagedDocument(embeddedDocument)))
}
