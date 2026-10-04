package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
func Write(storeDir, key string, meta Meta, data []byte) error
func Read(storeDIr, key, id string) (Meta, []byte, error)
func List(storeDir, key string) ([]Meta, error)
