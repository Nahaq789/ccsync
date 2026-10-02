package session

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const ext = ".jsonl"

func Find(pDir, root string) ([]Session, error) {

	entries, err := os.ReadDir(pDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []Session
	encoded := EncodePath(root)
	for _, ent := range entries {
		// ファイルはスキップ
		if !ent.IsDir() {
			continue
		}

		a := ent.Name() == encoded
		b := strings.HasPrefix(ent.Name(), encoded+"-")
		if !a && !b {
			continue
		}

		dir := filepath.Join(pDir, ent.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ext) {
				continue
			}

			abPath := filepath.Join(dir, f.Name())
			cwd, err := ReadCwd(abPath)
			if err != nil {
				return nil, err
			}

			id := strings.TrimSuffix(f.Name(), ext)
			info, err := f.Info()
			if err != nil {
				return nil, err
			}

			if cwd == root {
				sessions = append(sessions, Session{
					ID:      id,
					Dir:     dir,
					RelCwd:  ".",
					ModTime: info.ModTime(),
				})
			}

			if after, ok := strings.CutPrefix(cwd, root+"/"); ok {
				relCwd := after
				sessions = append(sessions, Session{
					ID:      id,
					Dir:     dir,
					RelCwd:  relCwd,
					ModTime: info.ModTime(),
				})
			}

			if cwd == "" && a {
				sessions = append(sessions, Session{
					ID:      id,
					Dir:     dir,
					RelCwd:  ".",
					ModTime: info.ModTime(),
				})
			}

		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ID < sessions[j].ID
	})
	return sessions, nil
}
