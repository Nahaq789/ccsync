package syncer

import (
	"fmt"

	"github.com/Nahaq789/ccsync/internal/store"
)

func Push(cfg Config) (Result, error) {
	var done []string
	var conflicts []string
	err := store.Update(cfg.StoreDir)
	if err != nil {
		return Result{}, err
	}

	items, err := Diff(cfg)
	if err != nil {
		return Result{}, err
	}

	for _, it := range items {
		if it.State == LocalOnly || it.State == LocalAhead {
			data, err := readLocal(*it.Local, cfg.Root)
			if err != nil {
				return Result{}, err
			}
			meta := store.Meta{
				ID:        it.ID,
				RelCwd:    it.Local.RelCwd,
				Hash:      store.Hash(data),
				Machine:   cfg.Machine,
				UpdatedAt: it.Local.ModTime,
			}
			if err := store.Write(cfg.StoreDir, cfg.Key, meta, data); err != nil {
				return Result{}, err
			}
			done = append(done, it.ID)
		}
		if it.State == Conflict {
			conflicts = append(conflicts, it.ID)
		}
	}

	if len(done) != 0 {
		msg := fmt.Sprintf("ccsync: %s から %d 件を push", cfg.Machine, len(done))
		_, err := store.CommitPush(cfg.StoreDir, msg)
		if err != nil {
			return Result{}, err
		}
	}
	return Result{
		Done:      done,
		Conflicts: conflicts,
	}, nil
}
