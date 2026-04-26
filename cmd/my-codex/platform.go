package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	managedBinaryName = "my-codex"
	hookBinaryName    = "codex-stop-guard"
)

type Platform struct {
	GOOS string
}

func CurrentPlatform() Platform {
	return Platform{GOOS: runtime.GOOS}
}

func (p Platform) IsWindows() bool {
	return p.GOOS == "windows"
}

func (p Platform) ManagedBinaryFilename() string {
	if p.IsWindows() {
		return managedBinaryName + ".exe"
	}
	return managedBinaryName
}

func (p Platform) HookBinaryFilename() string {
	if p.IsWindows() {
		return hookBinaryName + ".exe"
	}
	return hookBinaryName
}

func DefaultCodexHome() (string, error) {
	if override := os.Getenv("CODEX_HOME"); override != "" {
		return filepath.Clean(override), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

func GoBinDir() (string, error) {
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		return filepath.Clean(gobin), nil
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve GOPATH home: %w", err)
		}
		gopath = filepath.Join(home, "go")
	}
	return filepath.Join(gopath, "bin"), nil
}

func RepoHookCommand() string {
	return fmt.Sprintf("\"$HOME/.codex/hooks/%s\"", hookBinaryName)
}

func LocalHookCommand(binaryPath string) string {
	return fmt.Sprintf("\"%s\"", binaryPath)
}
