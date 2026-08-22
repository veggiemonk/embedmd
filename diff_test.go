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
	"strings"
	"testing"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want string
	}{
		{
			name: "no difference",
			a:    "one\ntwo\n",
			b:    "one\ntwo\n",
		},
		{
			name: "both empty",
		},
		{
			name: "lines added at the end",
			a:    "one\n",
			b:    "one\ntwo\nthree\n",
			want: "@@ -1 +1,3 @@\n one\n+two\n+three\n",
		},
		{
			name: "lines removed at the end",
			a:    "one\ntwo\nthree\n",
			b:    "one\n",
			want: "@@ -1,3 +1 @@\n one\n-two\n-three\n",
		},
		{
			name: "a line changed in the middle",
			a:    "1\n2\n3\n4\n5\n6\n7\n8\n9\n",
			b:    "1\n2\n3\n4\nX\n6\n7\n8\n9\n",
			want: "@@ -2,7 +2,7 @@\n 2\n 3\n 4\n-5\n+X\n 6\n 7\n 8\n",
		},
		{
			name: "everything added",
			a:    "",
			b:    "one\n",
			want: "@@ -0,0 +1 @@\n+one\n",
		},
		{
			name: "everything removed",
			a:    "one\n",
			b:    "",
			want: "@@ -1 +0,0 @@\n-one\n",
		},
		{
			name: "a missing terminator on the last line",
			a:    "one\ntwo",
			b:    "one\ntwo\n",
			want: "@@ -1,2 +1,2 @@\n one\n-two\n\\ No newline at end of file\n+two\n",
		},
		{
			name: "CRLF is part of the line",
			a:    "one\r\ntwo\r\n",
			b:    "one\r\nTWO\r\n",
			want: "@@ -1,2 +1,2 @@\n one\r\n-two\r\n+TWO\r\n",
		},
		{
			name: "two changes far apart make two blocks",
			a:    "a\n1\n2\n3\n4\n5\n6\n7\n8\n9\nz\n",
			b:    "A\n1\n2\n3\n4\n5\n6\n7\n8\n9\nZ\n",
			want: "@@ -1,4 +1,4 @@\n-a\n+A\n 1\n 2\n 3\n@@ -8,4 +8,4 @@\n 7\n 8\n 9\n-z\n+Z\n",
		},
		{
			name: "two changes close together share a block",
			a:    "a\n1\n2\n3\n4\nz\n",
			b:    "A\n1\n2\n3\n4\nZ\n",
			want: "@@ -1,6 +1,6 @@\n-a\n+A\n 1\n 2\n 3\n 4\n-z\n+Z\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := diff(tt.a, tt.b); got != tt.want {
				t.Errorf("expected\n%q\ngot\n%q", tt.want, got)
			}
		})
	}
}

// TestDiffApplies checks the result on random pairs of texts: every removed
// line has to come from a, every added line from b, and the kept lines plus
// the removed ones have to rebuild a.
func TestDiffRebuildsBothTexts(t *testing.T) {
	tests := []struct{ a, b string }{
		{"", ""},
		{"a\n", ""},
		{"", "a\n"},
		{"a\nb\nc\n", "c\nb\na\n"},
		{"a\nb\nc\nd\ne\nf\n", "a\nc\ne\n"},
		{"a\nc\ne\n", "a\nb\nc\nd\ne\nf\n"},
		{strings.Repeat("x\n", 50), strings.Repeat("x\n", 25) + "y\n" + strings.Repeat("x\n", 25)},
	}

	for _, tt := range tests {
		script := editScript(splitLines(tt.a), splitLines(tt.b))
		var gotA, gotB strings.Builder
		for _, e := range script {
			switch e.kind {
			case ' ':
				gotA.WriteString(e.text)
				gotB.WriteString(e.text)
			case '-':
				gotA.WriteString(e.text)
			case '+':
				gotB.WriteString(e.text)
			}
		}
		if gotA.String() != tt.a {
			t.Errorf("the script does not rebuild a: expected %q; got %q", tt.a, gotA.String())
		}
		if gotB.String() != tt.b {
			t.Errorf("the script does not rebuild b: expected %q; got %q", tt.b, gotB.String())
		}
	}
}

func TestDiffTooManyChanges(t *testing.T) {
	// Beyond maxEditDistance the answer is coarse, but it still has to
	// rebuild both texts.
	a := strings.Repeat("a\n", maxEditDistance)
	b := strings.Repeat("b\n", maxEditDistance)

	script := editScript(splitLines(a), splitLines(b))
	var removed, added int
	for _, e := range script {
		switch e.kind {
		case '-':
			removed++
		case '+':
			added++
		case ' ':
			t.Fatal("expected no line to be kept")
		}
	}
	if removed != maxEditDistance || added != maxEditDistance {
		t.Errorf("expected %d removed and added; got %d and %d", maxEditDistance, removed, added)
	}
}
