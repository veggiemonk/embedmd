// Copyright 2026 Julien Bisconti and the embedmd fork contributors.
//
// Written for the github.com/veggiemonk/embedmd fork.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to writing, software distributed
// under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
// CONDITIONS OF ANY KIND, either express or implied.
//
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"strconv"
	"strings"
)

// contextLines is the number of unchanged lines shown around a change.
const contextLines = 3

// maxEditDistance limits the search for the shortest edit script. Beyond it
// the two texts have little in common, and a coarse answer costs far less
// memory than an exact one.
const maxEditDistance = 2000

// diff returns the difference between a and b in the unified format that
// diff -u produces, or an empty string when they are equal.
//
// Lines keep their own terminator, so a CRLF file reads correctly. A last
// line without a terminator is marked the way diff(1) marks it.
func diff(a, b string) string {
	if a == b {
		return ""
	}
	return unified(editScript(splitLines(a), splitLines(b)))
}

// splitLines cuts s into lines, each keeping its terminator. A last line
// without one is kept as it is.
func splitLines(s string) []string {
	var lines []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return append(lines, s)
		}
		lines, s = append(lines, s[:i+1]), s[i+1:]
	}
	return lines
}

// An edit is one line of the result: kept (' '), removed ('-'), or added
// ('+').
type edit struct {
	kind byte
	text string
}

// editScript returns the operations that turn a into b, in order. It uses the
// greedy algorithm of Myers (1986): walk the edit graph one edit distance at a
// time, keep each step, then read the path back from the end.
func editScript(a, b []string) []edit {
	n, m := len(a), len(b)
	maxD := min(n+m, maxEditDistance)

	v := make([]int, 2*maxD+2)
	offset := maxD
	trace := make([][]int, 0, maxD+1)

	for d := 0; d <= maxD; d++ {
		// Keep the state reached with d-1 edits. Only the diagonals in
		// [-d, d] can be read back later, so only they are kept.
		trace = append(trace, append([]int(nil), v[offset-d:offset+d+1]...))

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1] // the step came down: a line was added.
			} else {
				x = v[offset+k-1] + 1 // the step came right: a line was removed.
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] { // follow the equal lines.
				x, y = x+1, y+1
			}
			v[offset+k] = x
			if x >= n && y >= m {
				return backtrack(trace, a, b)
			}
		}
	}

	// Too far apart to be worth a line by line answer: replace the lot.
	script := make([]edit, 0, n+m)
	for _, line := range a {
		script = append(script, edit{'-', line})
	}
	for _, line := range b {
		script = append(script, edit{'+', line})
	}
	return script
}

// backtrack reads the recorded steps from the end back to the start and
// returns the operations in order.
func backtrack(trace [][]int, a, b []string) []edit {
	var reversed []edit
	x, y := len(a), len(b)

	for d := len(trace) - 1; d >= 0; d-- {
		if d == 0 {
			// Whatever is left is the run of equal lines that starts the
			// two texts.
			for x > 0 && y > 0 {
				x, y = x-1, y-1
				reversed = append(reversed, edit{' ', a[x]})
			}
			break
		}

		// v holds the diagonals from -d to d, so diagonal k sits at k+d.
		v, k := trace[d], x-y
		prevK := k - 1
		if k == -d || (k != d && v[k-1+d] < v[k+1+d]) {
			prevK = k + 1
		}
		prevX := v[prevK+d]
		prevY := prevX - prevK

		for x > prevX && y > prevY { // the run of equal lines.
			x, y = x-1, y-1
			reversed = append(reversed, edit{' ', a[x]})
		}
		if x > prevX {
			x--
			reversed = append(reversed, edit{'-', a[x]})
		} else {
			y--
			reversed = append(reversed, edit{'+', b[y]})
		}
	}

	script := make([]edit, len(reversed))
	for i, e := range reversed {
		script[len(reversed)-1-i] = e
	}
	return script
}

// position is the line number of an operation in each of the two texts,
// counted from one.
type position struct{ a, b int }

// unified writes the operations as a unified diff: one block for each group
// of changes, with contextLines unchanged lines around it. Two groups closer
// than that share a block.
func unified(script []edit) string {
	positions := make([]position, len(script))
	at := position{1, 1}
	for i, e := range script {
		positions[i] = at
		switch e.kind {
		case ' ':
			at.a, at.b = at.a+1, at.b+1
		case '-':
			at.a++
		case '+':
			at.b++
		}
	}

	var out strings.Builder
	for i := 0; i < len(script); {
		if script[i].kind == ' ' {
			i++
			continue
		}

		start := max(i-contextLines, 0)
		end := i + 1
		for {
			// Count the equal lines that follow this change.
			gap, j := 0, end
			for j < len(script) && script[j].kind == ' ' {
				gap, j = gap+1, j+1
			}
			if j < len(script) && gap <= 2*contextLines {
				end = j + 1 // another change is near: keep the gap as context.
				continue
			}
			end += min(gap, contextLines)
			break
		}

		writeHunk(&out, script[start:end], positions[start])
		i = end
	}
	return out.String()
}

func writeHunk(out *strings.Builder, hunk []edit, at position) {
	countA, countB := 0, 0
	for _, e := range hunk {
		switch e.kind {
		case ' ':
			countA, countB = countA+1, countB+1
		case '-':
			countA++
		case '+':
			countB++
		}
	}
	fmt.Fprintf(out, "@@ -%s +%s @@\n", lineRange(at.a, countA), lineRange(at.b, countB))

	for _, e := range hunk {
		out.WriteByte(e.kind)
		out.WriteString(e.text)
		if !strings.HasSuffix(e.text, "\n") {
			out.WriteString("\n\\ No newline at end of file\n")
		}
	}
}

// lineRange writes the start and the length of a block the way diff(1) does:
// the length is left out when it is one, and an empty block points at the
// line before it.
func lineRange(start, count int) string {
	switch count {
	case 0:
		return strconv.Itoa(start-1) + ",0"
	case 1:
		return strconv.Itoa(start)
	default:
		return strconv.Itoa(start) + "," + strconv.Itoa(count)
	}
}
