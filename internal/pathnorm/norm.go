package pathnorm

import "bytes"

const Placeholder = "\x01CCSYNC_ROOT\x01"

func Normalize(data []byte, root string) []byte {
	if root == "" {
		return data
	}

	old := []byte(root)
	var out bytes.Buffer

	i := 0
	for {
		j := bytes.Index(data[i:], old)
		if j < 0 {
			out.Write(data[i:])
			return out.Bytes()
		}
		j += i
		end := j + len(old)
		out.Write(data[i:j])

		if j == 0 || isPathChar(data[j-1]) {
		}

		i = end

	}

	return bytes.ReplaceAll(data, []byte(root), []byte(Placeholder))
}

func Denormalize(data []byte, root string) []byte {
	return bytes.ReplaceAll(data, []byte(Placeholder), []byte(root))
}

func isPathChar(b byte) bool {
	return true
}
