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

		// ディレクトリ名完全一致
		exact := ent.Name() == encoded
		// ディレクトリ名前方一致
		prefixed := strings.HasPrefix(ent.Name(), encoded+"-")
		if !exact && !prefixed {
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

			// jsonlのパス
			path := filepath.Join(dir, f.Name())
			cwd, err := ReadCwd(path)
			if err != nil {
				return nil, err
			}

			rel, ok := relCwd(cwd, root, exact)
			if !ok {
				continue
			}

			id := strings.TrimSuffix(f.Name(), ext)
			info, err := f.Info()
			if err != nil {
				return nil, err
			}

			sessions = append(sessions, Session{
				ID:      id,
				Dir:     dir,
				RelCwd:  rel,
				ModTime: info.ModTime(),
			})
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ID < sessions[j].ID
	})
	return sessions, nil
}

func relCwd(cwd, root string, exact bool) (string, bool) {
	if cwd == root {
		return ".", true
	}
	if cwd == "" && exact {
		return ".", true
	}
	if after, ok := strings.CutPrefix(cwd, root+"/"); ok {
		return after, true
	}
	return "", false
}
