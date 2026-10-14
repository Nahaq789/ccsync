package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	c, err := config.Load(filepath.Join(home, ".ccsync", "config.json"))
	if err != nil {
		return err
	}
	cfg, err := buildConfig(home, cwd, c)
	if err != nil {
		return err
	}
	if err := store.Clone(c.Remote, cfg.StoreDir); err != nil {
		return err
	}

	if cmd == "push" {
		r, err := syncer.Push(cfg)
		if err != nil {
			return err
		}
		return printResult(out, cmd, r)
	}

	if cmd == "pull" {
		r, err := syncer.Pull(cfg)
		if err != nil {
			return err
		}
		return printResult(out, cmd, r)
	}

	if cmd == "status" {
		return status(cfg, out)
	}

	return nil
}

func printResult(out io.Writer, cmd string, res syncer.Result) error {
	if len(res.Done) == 0 {
		if _, err := fmt.Fprintf(out, "%s: 対象はありません\n", cmd); err != nil {
			return err
		}
	}
	if len(res.Done) >= 1 {
		if _, err := fmt.Fprintf(out, "%s: %d 件 %s\n", cmd, len(res.Done), strings.Join(res.Done, ", ")); err != nil {
			return err
		}
	}

	if len(res.Conflicts) == 0 {
		return nil
	}
	if len(res.Conflicts) >= 1 {
		if _, err := fmt.Fprintf(out, "競合: %d 件(両方のPCで進んでいるため、スキップしました) %s\n", len(res.Conflicts), strings.Join(res.Conflicts, ", ")); err != nil {
			return err
		}
		return fmt.Errorf("競合が %d 件あります", len(res.Conflicts))
	}
	return nil
}

func status(cfg syncer.Config, out io.Writer) error {
	if err := store.Update(cfg.StoreDir); err != nil {
		return err
	}

	items, err := syncer.Diff(cfg)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(out, "%s\n", cfg.Key); err != nil {
		return err
	}
	if len(items) == 0 {
		if _, err := fmt.Fprintln(out, "  セッションはありません"); err != nil {
			return err
		}
		return nil
	}
	for _, it := range items {
		if _, err := fmt.Fprintf(out, "  %s %s\n", it.ID, stateLabel(it.State)); err != nil {
			return err
		}
	}
	return nil
}

func stateLabel(s syncer.State) string {
	switch s {
	case syncer.Synced:
		return "同期済み"
	case syncer.LocalOnly:
		return "手元だけ"
	case syncer.StoreOnly:
		return "ストアだけ"
	case syncer.LocalAhead:
		return "手元が新しい"
	case syncer.StoreAhead:
		return "ストアが新しい"
	case syncer.Conflict:
		return "競合"
	default:
		return ""
	}
}
