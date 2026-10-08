package syncer

import (
	"os"
	"path/filepath"

	"github.com/Nahaq789/ccsync/internal/pathnorm"
	"github.com/Nahaq789/ccsync/internal/session"
	"github.com/Nahaq789/ccsync/internal/store"
)

func Pull(cfg Config) (Result, error) {
	var done, conflicts []string

	if err := store.Update(cfg.StoreDir); err != nil {
		return returnErr(err)
	}
	items, err := Diff(cfg)
	if err != nil {
		return returnErr(err)
	}
	for _, it := range items {
		if it.State == StoreOnly || it.State == StoreAhead {
			meta, data, err := store.Read(cfg.StoreDir, cfg.Key, it.ID)
			if err != nil {
				return returnErr(err)
			}
			dn := pathnorm.Denormalize(data, cfg.Root)
			path := writePath(meta.RelCwd, it, cfg)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return returnErr(err)
			}
			if err := os.WriteFile(path, dn, 0o600); err != nil {
				return returnErr(err)
			}
			if err := os.Chtimes(path, meta.UpdatedAt, meta.UpdatedAt); err != nil {
				return returnErr(err)
			}

			done = append(done, it.ID)
		}
		if it.State == Conflict {
			conflicts = append(conflicts, it.ID)
		}
	}
	return Result{Done: done, Conflicts: conflicts}, nil
}

func writePath(rel string, it Item, cfg Config) string {
	if it.Local != nil {
		return filepath.Join(it.Local.Dir, it.ID+".jsonl")
	}
	p := filepath.Join(cfg.Root, rel)
	encoded := session.EncodePath(p)
	return filepath.Join(cfg.ProjectsDir, encoded, it.ID+".jsonl")
}

func returnErr(err error) (Result, error) {
	return Result{}, err
}
