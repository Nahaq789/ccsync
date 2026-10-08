package syncer

import "bytes"

func Compare(local, stored []byte) State {
	if bytes.Equal(local, stored) {
		return Synced
	}
	if bytes.HasPrefix(local, stored) {
		return LocalAhead
	}
	if bytes.HasPrefix(stored, local) {
		return StoreAhead
	}
	return Conflict
}
