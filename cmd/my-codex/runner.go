package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type RunOptions struct {
	Cwd                string
	Input              string
	AllowedReturnCodes []int
}

type RunResult struct {
	Stdout     string
	Stderr     string
	ReturnCode int
}

type CommandRunner interface {
	Run(command []string, options RunOptions) (RunResult, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(command []string, options RunOptions) (RunResult, error) {
	if len(command) == 0 {
		return RunResult{}, fmt.Errorf("empty command")
	}
	cmd := exec.Command(command[0], command[1:]...)
	if options.Cwd != "" {
		cmd.Dir = options.Cwd
	}
	if options.Input != "" {
		cmd.Stdin = strings.NewReader(options.Input)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := RunResult{Stdout: stdout.String(), Stderr: stderr.String(), ReturnCode: 0}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if ok := AsExitError(err, &exitErr); ok {
		result.ReturnCode = exitErr.ExitCode()
		if isAllowedCode(result.ReturnCode, options.AllowedReturnCodes) {
			return result, nil
		}
		return result, fmt.Errorf("command failed with context: cwd=%s, command=%s, returncode=%d, stdout=%q, stderr=%q", filepath.Clean(cmd.Dir), strings.Join(command, " "), result.ReturnCode, result.Stdout, result.Stderr)
	}
	return result, fmt.Errorf("command failed with context: cwd=%s, command=%s, stdout=%q, stderr=%q, err=%w", filepath.Clean(cmd.Dir), strings.Join(command, " "), result.Stdout, result.Stderr, err)
}

func isAllowedCode(code int, allowed []int) bool {
	if len(allowed) == 0 {
		return code == 0
	}
	for _, candidate := range allowed {
		if code == candidate {
			return true
		}
	}
	return false
}

func AsExitError(err error, target **exec.ExitError) bool {
	if err == nil {
		return false
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	*target = exitErr
	return true
}
