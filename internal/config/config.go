package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type Config struct {
	Remote  string `json:"remote"`
	Machine string `json:"machine"`
}

func Save(path string, c Config) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	return nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Config{}, fmt.Errorf("設定がありません。先に ccsync init を実行してください: %s", path)
		}
		return Config{}, err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}

	if c.Remote == "" || c.Machine == "" {
		return Config{}, errors.New("Remote もしくは Machine が空です")
	}
	return c, nil
}
