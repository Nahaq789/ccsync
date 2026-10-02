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

		justBefore := j == 0 || !isPathChar(data[j-1])
		after := end == len(data) || !isPathChar(data[end])
		if justBefore && after {
			out.WriteString(Placeholder)
		} else {
			out.Write(old)
		}
		i = end
	}
}

func Denormalize(data []byte, root string) []byte {
	return bytes.ReplaceAll(data, []byte(Placeholder), []byte(root))
}

func isPathChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '.', b == '_', b == '-':
		return true
	}
	return false
}
