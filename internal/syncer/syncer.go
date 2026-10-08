package syncer

import (
	"github.com/Nahaq789/ccsync/internal/session"
	"github.com/Nahaq789/ccsync/internal/store"
)

type Config struct {
	ProjectsDir string
	StoreDir    string
	Root        string
	Key         string
	Machine     string
}

type State int

const (
	Synced State = iota
	LocalOnly
	StoreOnly
	LocalAhead
	StoreAhead
	Conflict
)

type Item struct {
	ID     string
	State  State
	Local  *session.Session
	Remote *store.Meta
}

type Result struct {
	Done      []string
	Conflicts []string
}
