package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func copyDirFromOS(source, destination string) error {
	if err := os.RemoveAll(destination); err != nil {
		return fmt.Errorf("remove destination dir %s: %w", destination, err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create destination dir %s: %w", destination, err)
	}
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("rel path for %s: %w", path, err)
		}
		if rel == "." {
			return nil
		}
		if shouldIgnorePath(rel, d) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		target := filepath.Join(destination, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target, 0o644)
	})
}

func copyDirFromFS(sourceFS fs.FS, source, destination string) error {
	if err := os.RemoveAll(destination); err != nil {
		return fmt.Errorf("remove destination dir %s: %w", destination, err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create destination dir %s: %w", destination, err)
	}
	return fs.WalkDir(sourceFS, source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, source)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		if shouldIgnoreEmbeddedPath(rel, d) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		target := filepath.Join(destination, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := fs.ReadFile(sourceFS, path)
		if err != nil {
			return fmt.Errorf("read embedded file %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create parent dir for %s: %w", target, err)
		}
		return os.WriteFile(target, content, 0o644)
	})
}

func copyOptionalFile(source, destination string) error {
	if _, err := os.Stat(source); err == nil {
		return copyFile(source, destination, 0o644)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat optional file %s: %w", source, err)
	}
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove optional file %s: %w", destination, err)
	}
	return nil
}

func copyOptionalEmbeddedFile(sourceFS fs.FS, source, destination string) error {
	content, err := fs.ReadFile(sourceFS, source)
	if err != nil {
		if isNotExistFS(err) {
			if removeErr := os.Remove(destination); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove optional file %s: %w", destination, removeErr)
			}
			return nil
		}
		return fmt.Errorf("read embedded optional file %s: %w", source, err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", destination, err)
	}
	return os.WriteFile(destination, content, 0o644)
}

func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source file %s: %w", source, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", destination, err)
	}
	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create destination file %s: %w", destination, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s to %s: %w", source, destination, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close destination file %s: %w", destination, err)
	}
	if err := os.Chmod(destination, mode); err != nil {
		return fmt.Errorf("chmod destination file %s: %w", destination, err)
	}
	return nil
}

func installBinary(source, destination string) error {
	absSource, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("abs source %s: %w", source, err)
	}
	absDestination, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("abs destination %s: %w", destination, err)
	}
	if absSource == absDestination {
		return nil
	}
	mode := os.FileMode(0o755)
	return copyFile(absSource, absDestination, mode)
}

func shouldIgnorePath(rel string, d fs.DirEntry) bool {
	name := d.Name()
	if name == "__pycache__" {
		return true
	}
	if !d.IsDir() {
		ext := strings.ToLower(filepath.Ext(name))
		return ext == ".pyc" || ext == ".pyo"
	}
	return false
}

func shouldIgnoreEmbeddedPath(rel string, d fs.DirEntry) bool {
	return shouldIgnorePath(filepath.FromSlash(rel), d)
}

func isNotExistFS(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "file does not exist")
}

func trackedOrPresent(repoRoot, relativePath string, runner CommandRunner) (bool, error) {
	if _, err := os.Stat(filepath.Join(repoRoot, relativePath)); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat tracked candidate %s: %w", relativePath, err)
	}
	result, err := runner.Run([]string{"git", "ls-files", "--error-unmatch", "--", relativePath}, RunOptions{Cwd: repoRoot, AllowedReturnCodes: []int{0, 1}})
	if err != nil {
		return false, err
	}
	return result.ReturnCode == 0, nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
