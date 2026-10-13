package main

import (
	"errors"
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
		return errors.New("使い方: ccsync init --machine <名前> <リモートURL> | push | pull | status")
	}
	a := args[0]
	if a == "init" {
		return runInit(args[1:], home, out)
	}
	if a == "push" || a == "pull" || a == "status" {
		return runSync(a, home, cwd, out)
	}
	return fmt.Errorf("不明なコマンドです: %s", a)
}

func runInit(args []string, home string, out io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	machine := fs.String("machine", "", "このPCの名前")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 || *machine == "" {
		return fmt.Errorf("使い方: ccsync init --machine <名前> <リモートURL>")
	}

	if err := store.Clone(fs.Arg(0), filepath.Join(home, ".ccsync", "store")); err != nil {
		return err
	}

	if err := config.Save(filepath.Join(home, ".ccsync", "config.json"), config.Config{Remote: fs.Arg(0), Machine: *machine}); err != nil {
		return err
	}

	_, m := fmt.Fprintf(out, "初期化しました: %s (%s)\n", *machine, fs.Arg(0))
	return m
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
