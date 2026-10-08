package syncer

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
