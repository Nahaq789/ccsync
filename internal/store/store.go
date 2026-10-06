package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func RepoKey(remoteURL string) (string, error) {
	var host string
	var path string

	s := strings.TrimSpace(remoteURL)
	_, rest, hasScheme := strings.Cut(s, "://")

	if hasScheme {
		if before, p, found := strings.Cut(rest, "/"); found {
			host = before
			path = p
		}
		if _, a, found := strings.Cut(host, "@"); found {
			host = a
		}
		if b, _, found := strings.Cut(host, ":"); found {
			host = b
		}
	}

	if !hasScheme {
		if before, p, found := strings.Cut(s, ":"); found {
			host = before
			path = p
		}
		if _, a, found := strings.Cut(host, "@"); found {
			host = a
		}
	}

	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	key := strings.ToLower(host + "/" + path)

	for p := range strings.SplitSeq(key, "/") {
		if p == "" || p == "." || p == ".." {
			return "", fmt.Errorf("リモートURLを解釈できません: %q", remoteURL)
		}
	}
	return key, nil
}

type Meta struct {
	ID        string    `json:"id"`
	RelCwd    string    `json:"rel_cwd"`
	Hash      string    `json:"hash"`
	Machine   string    `json:"machine"`
	UpdatedAt time.Time `json:"updated_at"`
}

func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func Write(storeDir, key string, meta Meta, data []byte) error {
	if invalidID(meta.ID) {
		return fmt.Errorf("セッションIDが不正です: %q", meta.ID)
	}

	dir := filepath.Join(storeDir, "repos", key, meta.ID)
	jsonl := filepath.Join(dir, "session.jsonl")
	metaJson := filepath.Join(dir, "meta.json")

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(jsonl, data, 0o600); err != nil {
		return err
	}
	mj, pErr := json.MarshalIndent(meta, "", "  ")
	if pErr != nil {
		return pErr
	}
	if err := os.WriteFile(metaJson, mj, 0o600); err != nil {
		return err
	}

	return nil
}

func Read(storeDir, key, id string) (Meta, []byte, error) {
	if invalidID(id) {
		return Meta{}, nil, fmt.Errorf("セッションIDが不正です: %q", id)
	}

	dir := filepath.Join(storeDir, "repos", key, id)
	metaJson := filepath.Join(dir, "meta.json")
	jsonl := filepath.Join(dir, "session.jsonl")

	// data.json
	data, err := os.ReadFile(metaJson)
	if err != nil {
		return Meta{}, nil, err
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return Meta{}, nil, err
	}

	// session.jsonl
	sj, err := os.ReadFile(jsonl)
	if err != nil {
		return Meta{}, nil, err
	}

	return m, sj, nil
}

func List(storeDir, key string) ([]Meta, error) {
	pDir := filepath.Join(storeDir, "repos", key)
	entries, err := os.ReadDir(pDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var metas []Meta
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}

		path := filepath.Join(pDir, ent.Name(), "meta.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var m Meta
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}

		if ent.Name() != m.ID {
			continue
		}

		metas = append(metas, m)
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].ID < metas[j].ID
	})

	return metas, nil
}

func invalidID(id string) bool {
	return id == "" || id == "." || id == ".." || strings.Contains(id, "/")
}
