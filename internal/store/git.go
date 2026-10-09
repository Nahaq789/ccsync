package store

import (
	"os"
	"path/filepath"

	"github.com/Nahaq789/ccsync/internal/gitcmd"
)

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
		_, gitErr := gitcmd.Run(parentDir, "clone", url, storeDir)
		if gitErr != nil {
			return gitErr
		}
	}
	return nil

}

func Update(storeDir string) error {
	if _, err := gitcmd.Run(storeDir, "fetch", "origin"); err != nil {
		return err
	}
	if _, err := gitcmd.Run(storeDir, "reset", "--hard", "@{upstream}"); err != nil {
		return err
	}
	if _, err := gitcmd.Run(storeDir, "clean", "-fd"); err != nil {
		return err
	}
	return nil
}

func CommitPush(storeDir, message string) (bool, error) {
	if _, err := gitcmd.Run(storeDir, "add", "-A"); err != nil {
		return false, err
	}

	m, err := gitcmd.Run(storeDir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	if m == "" {
		return false, nil
	}

	if _, err := gitcmd.Run(storeDir, "commit", "-m", message); err != nil {
		return false, err
	}

	if _, err := gitcmd.Run(storeDir, "push"); err != nil {
		return false, err
	}

	return true, nil
}
