package store

import (
	"bytes"
	"fmt"
	"os/exec"
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
