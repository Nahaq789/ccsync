package syncer

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/Nahaq789/ccsync/internal/pathnorm"
	"github.com/Nahaq789/ccsync/internal/session"
	"github.com/Nahaq789/ccsync/internal/store"
)

func Diff(cfg Config) ([]Item, error) {
	var items []Item
	locals, err := session.Find(cfg.ProjectsDir, cfg.Root)
	if err != nil {
		return nil, err
	}
	stores, err := store.List(cfg.StoreDir, cfg.Key)
	if err != nil {
		return nil, err
	}
	metaByID := map[string]store.Meta{}
	for _, s := range stores {
		metaByID[s.ID] = s
	}

	for _, l := range locals {
		v, ok := metaByID[l.ID]
		if !ok {
			items = append(items, Item{
				ID:     l.ID,
				State:  LocalOnly,
				Local:  &l,
				Remote: nil,
			})
		}
		if ok {
			r, err := readLocal(l, cfg.Root)
			if err != nil {
				return nil, err
			}
			_, sr, err := store.Read(cfg.StoreDir, cfg.Key, l.ID)
			if err != nil {
				return nil, err
			}
			c := Compare(r, sr)
			items = append(items, Item{
				ID:     l.ID,
				State:  c,
				Local:  &l,
				Remote: &v,
			})
			delete(metaByID, l.ID)
		}
	}

	for _, m := range metaByID {
		items = append(items, Item{
			ID:     m.ID,
			State:  StoreOnly,
			Local:  nil,
			Remote: &m,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})

	return items, nil
}

func readLocal(s session.Session, root string) ([]byte, error) {
	path := filepath.Join(s.Dir, s.ID+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	n := pathnorm.Normalize(data, root)
	return n, nil
}
