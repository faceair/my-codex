package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Run(args []string, stdout, stderr io.Writer, version string) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "sync":
		return runSyncCLI(args[1:], stdout, stderr, version)
	case "pull":
		return runPullCLI(args[1:], stdout, stderr)
	case "version", "--version", "-v":
		_, _ = fmt.Fprintln(stdout, version)
		return 0
	case "help", "--help", "-h":
		printUsage(stdout)
		return 0
	default:
		printUsage(stderr)
		return 2
	}
}

func runSyncCLI(args []string, stdout, stderr io.Writer, version string) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cwd, _ := os.Getwd()
	defaultHome, _ := DefaultCodexHome()
	repoRoot := fs.String("repo-root", cwd, "Repository root to sync into")
	sourceRoot := fs.String("source-root", defaultHome, "Local .codex root to sync from")
	codexBinary := fs.String("codex-bin", envOrDefault("CODEX_BIN", managedBinaryName), "Codex executable used for commit message generation")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := runSync(SyncOptions{RepoRoot: *repoRoot, SourceRoot: *sourceRoot, CodexBinary: *codexBinary, Version: version, Runner: ExecRunner{}}, stdout, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, "Failed to sync codex with context: %v\n", err)
		return 1
	}
	return 0
}

func runPullCLI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	fs.SetOutput(stderr)
	defaultHome, _ := DefaultCodexHome()
	destRoot := fs.String("dest-root", defaultHome, "Local .codex root to populate")
	hookBinary := fs.String("hook-binary", envOrDefault("MY_CODEX_HOOK_BINARY", ""), "Path to codex-stop-guard binary (defaults to sibling binary or local build fallback)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := runPull(PullOptions{DestRoot: filepath.Clean(*destRoot), HookBinaryPath: cleanOptionalPath(*hookBinary), Runner: ExecRunner{}, Platform: CurrentPlatform()}, stdout, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, "Failed to pull codex with context: %v\n", err)
		return 1
	}
	return 0
}

func cleanOptionalPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  my-codex sync [--repo-root PATH] [--source-root PATH] [--codex-bin PATH]")
	_, _ = fmt.Fprintln(w, "  my-codex pull [--dest-root PATH] [--hook-binary PATH]")
	_, _ = fmt.Fprintln(w, "  my-codex version")
}
