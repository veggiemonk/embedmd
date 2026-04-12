package embedmd

import "bytes"

func excludeLines(b []byte, exclStart, exclEnd bool) []byte {
	if len(b) == 0 {
		return b
	}
	lines := bytes.SplitAfter(b, []byte("\n"))
	// SplitAfter may produce an empty trailing element if b ends with \n
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if exclStart && len(lines) > 0 {
		lines = lines[1:]
	}
	if exclEnd && len(lines) > 0 {
		last := lines[len(lines)-1]
		// If last line doesn't end with \n, just drop it.
		// If it does, drop it. But we need the previous line to end with \n.
		lines = lines[:len(lines)-1]
		// If last line didn't end with \n and the new last line also doesn't,
		// ensure trailing newline on new last line.
		_ = last
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
	lines := bytes.SplitAfter(b, []byte("\n"))
	// Remove empty trailing element from SplitAfter
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	// Remove trailing blank/whitespace-only lines
	for len(lines) > 0 {
		last := lines[len(lines)-1]
		trimmed := bytes.TrimRight(last, " \t\n\r")
		if len(trimmed) == 0 {
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
	lines := bytes.SplitAfter(b, []byte("\n"))
	// Remove empty trailing element from SplitAfter
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}

	// Find common whitespace prefix among non-blank lines
	var prefix []byte
	first := true
	for _, line := range lines {
		content := bytes.TrimRight(line, "\n")
		if len(bytes.TrimSpace(content)) == 0 {
			continue // skip blank/whitespace-only lines
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

	// Strip common prefix from each line
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
