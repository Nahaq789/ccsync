package store

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Dir: %s, cmd: %s, stderr: %s, err: %w", dir, cmd.Args, strings.TrimSpace(stderr.String()), err)
	}

	s := stdout.String()
	result := strings.TrimSpace(s)

	return result, nil
}

func Clone(url, storeDir string) error {
	parentDir := filepath.Dir(storeDir)
	if err := os.MkdirAll(parentDir, 0o700); err != nil {
		return err
	}

	// clone先が存在するか
	path := filepath.Join(storeDir, ".git")
	_, err := os.Stat(path)
	if err != nil {

		// 実際にcloneする
		_, gitErr := runGit(parentDir, "clone", url, storeDir)
		if gitErr != nil {
			return gitErr
		}
	}
	return nil

}

func Update(storeDir string) error {
	if _, err := runGit(storeDir, "fetch", "origin"); err != nil {
		return err
	}
	if _, err := runGit(storeDir, "reset", "--hard", "@{upstream}"); err != nil {
		return err
	}
	if _, err := runGit(storeDir, "clean", "-fd"); err != nil {
		return err
	}
	return nil
}

func CommitPush(storeDir, message string) (bool, error) {
	if _, err := runGit(storeDir, "add", "-A"); err != nil {
		return false, err
	}

	m, err := runGit(storeDir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	if m == "" {
		return false, nil
	}

	if _, err := runGit(storeDir, "commit", "-m", message); err != nil {
		return false, err
	}

	if _, err := runGit(storeDir, "push"); err != nil {
		return false, err
	}

	return true, nil
}
