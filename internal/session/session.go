package session

import (
	"bytes"
	"time"
)

type Session struct {
	ID      string
	Dir     string
	RelCwd  string
	ModTime time.Time
}

func EncodePath(p string) string {
	b := []byte(p)
	var out bytes.Buffer
	for _, c := range b {
		if isAlnum(c) {
			out.WriteByte(c)
			continue
		}
		out.WriteByte('-')
	}

	return out.String()
}

func ReadCwd(path string) (string, error) {
	return "", nil
}

func Find(pDir, root string) ([]Session, error) {
	return nil, nil
}

func isAlnum(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	}
	return false
}
