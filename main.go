package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Nahaq789/ccsync/internal/config"
	"github.com/Nahaq789/ccsync/internal/gitcmd"
	"github.com/Nahaq789/ccsync/internal/store"
	"github.com/Nahaq789/ccsync/internal/syncer"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ccsync:", err)
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ccsync:", err)
		os.Exit(1)
	}

	if err := run(os.Args[1:], home, cwd, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ccsync:", err)
		os.Exit(1)
	}

}

func detectRepo(dir string) (root, key string, err error) {
	root, err = gitcmd.Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", fmt.Errorf("git リポジトリの中で実行してください: %w", err)
	}
	url, err := gitcmd.Run(root, "remote", "get-url", "origin")
	if err != nil {
		return "", "", fmt.Errorf("origin が設定されていません: %w", err)
	}

	key, err = store.RepoKey(url)
	if err != nil {
		return "", "", err
	}
	return root, key, nil
}

func buildConfig(home, cwd string, c config.Config) (syncer.Config, error) {
	r, k, err := detectRepo(cwd)
	if err != nil {
		return syncer.Config{}, err
	}
	return syncer.Config{
		ProjectsDir: filepath.Join(home, ".claude", "projects"),
		StoreDir:    filepath.Join(home, ".ccsync", "store"),
		Root:        r,
		Key:         k,
		Machine:     c.Machine,
	}, nil
}

func run(args []string, home, cwd string, out io.Writer) error {
	if len(args) == 0 {
		return nil // help
	}
	a := args[0]
	if a == "init" {
		if err := runInit(args[1:], home, out); err != nil {
			return err
		}
	}
	if a == "push" || a == "pull" || a == "status" {
		if err := runSync(a, home, cwd, out); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out, "不明なコマンドです: %s", a); err != nil {
		return err
	}
	return nil
}

func runInit(args []string, home string, out io.Writer) error {
	set := flag.NewFlagSet("init", flag.ContinueOnError)
	return nil
}

func runSync(cmd, home, cwd string, out io.Writer) error {
	return nil
}

func printResult(out io.Writer, cmd string, res syncer.Result) error {
	return nil
}

func status(cfg syncer.Config, out io.Writer) error {
	return nil
}

func stateLabel(s syncer.State) string {
	return ""
}
