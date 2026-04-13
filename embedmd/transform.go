package embedmd

import "bytes"

// splitLines splits b into lines, each retaining its trailing \n.
// Unlike bytes.SplitAfter, it never returns an empty trailing element.
func splitLines(b []byte) [][]byte {
	lines := bytes.SplitAfter(b, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func excludeLines(b []byte, exclStart, exclEnd bool) []byte {
	if len(b) == 0 {
		return b
	}
	lines := splitLines(b)
	if exclStart && len(lines) > 0 {
		lines = lines[1:]
	}
	if exclEnd && len(lines) > 0 {
		lines = lines[:len(lines)-1]
		// Ensure trailing newline on the new last line.
		if len(lines) > 0 {
			l := lines[len(lines)-1]
			if len(l) > 0 && l[len(l)-1] != '\n' {
				lines[len(lines)-1] = append(l, '\n')
			}
		}
	}
	return bytes.Join(lines, nil)
}

func trimTrailingBlankLines(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	lines := splitLines(b)
	// Remove trailing blank/whitespace-only lines.
	for len(lines) > 0 {
		last := lines[len(lines)-1]
		if len(bytes.TrimRight(last, " \t\n\r")) == 0 {
			lines = lines[:len(lines)-1]
		} else {
			break
		}
	}
	if len(lines) == 0 {
		return nil
	}
	// Ensure last line ends with newline
	last := lines[len(lines)-1]
	if len(last) > 0 && last[len(last)-1] != '\n' {
		lines[len(lines)-1] = append(last, '\n')
	}
	return bytes.Join(lines, nil)
}

func dedentBytes(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	lines := splitLines(b)

	// Find common whitespace prefix among non-blank lines.
	var prefix []byte
	first := true
	for _, line := range lines {
		content := bytes.TrimRight(line, "\n")
		if len(bytes.TrimSpace(content)) == 0 {
			continue // skip blank lines for prefix calculation
		}
		linePrefix := leadingWhitespace(content)
		if first {
			prefix = linePrefix
			first = false
		} else {
			prefix = commonPrefix(prefix, linePrefix)
		}
	}

	if len(prefix) == 0 {
		return b
	}

	// Strip common prefix from each line.
	var result []byte
	for _, line := range lines {
		content := bytes.TrimRight(line, "\n")
		if len(bytes.TrimSpace(content)) == 0 {
			// Whitespace-only line: make it empty
			result = append(result, '\n')
		} else {
			result = append(result, bytes.TrimPrefix(content, prefix)...)
			result = append(result, '\n')
		}
	}
	return result
}

func leadingWhitespace(b []byte) []byte {
	for i, c := range b {
		if c != ' ' && c != '\t' {
			return b[:i]
		}
	}
	return b
}

func commonPrefix(a, b []byte) []byte {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:n]
}

func applySubstitutions(b []byte, subs []substitution) []byte {
	for _, s := range subs {
		b = bytes.ReplaceAll(b, []byte(s.old), []byte(s.new))
	}
	return b
}
