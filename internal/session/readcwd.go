package session

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

func ReadCwd(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')

		var v struct {
			Cwd string `json:"cwd"`
		}

		jsonErr := json.Unmarshal(line, &v)

		if jsonErr == nil && v.Cwd != "" {
			return v.Cwd, nil
		}

		if err != nil {
			if err == io.EOF {
				return "", nil
			}
			return "", err
		}
	}
}
