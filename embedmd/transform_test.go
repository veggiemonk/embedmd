package embedmd

import "testing"

func TestExcludeLines(t *testing.T) {
	tc := []struct {
		name                     string
		in                       string
		excludeStart, excludeEnd bool
		out                      string
	}{
		{
			name: "no exclusion",
			in:   "line1\nline2\nline3\n", out: "line1\nline2\nline3\n",
		},
		{
			name: "exclude start",
			in:   "// start\nline1\nline2\n", excludeStart: true, out: "line1\nline2\n",
		},
		{
			name: "exclude end",
			in:   "line1\nline2\n// end\n", excludeEnd: true, out: "line1\nline2\n",
		},
		{
			name: "exclude both",
			in:   "// start\nline1\nline2\n// end\n", excludeStart: true, excludeEnd: true, out: "line1\nline2\n",
		},
		{
			name: "exclude both leaves nothing",
			in:   "// start\n// end\n", excludeStart: true, excludeEnd: true, out: "",
		},
		{
			name: "single line exclude start",
			in:   "only\n", excludeStart: true, out: "",
		},
		{
			name: "no trailing newline exclude end",
			in:   "line1\nline2", excludeEnd: true, out: "line1\n",
		},
		{
			name: "no trailing newline exclude both",
			in:   "start\nmiddle\nend", excludeStart: true, excludeEnd: true, out: "middle\n",
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			got := excludeLines([]byte(tt.in), tt.excludeStart, tt.excludeEnd)
			if string(got) != tt.out {
				t.Errorf("case [%s]: expected %q; got %q", tt.name, tt.out, string(got))
			}
		})
	}
}

func TestTrimTrailingBlankLines(t *testing.T) {
	tc := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "no trailing blanks",
			in:   "line1\nline2\n", out: "line1\nline2\n",
		},
		{
			name: "one trailing blank",
			in:   "line1\nline2\n\n", out: "line1\nline2\n",
		},
		{
			name: "multiple trailing blanks",
			in:   "line1\n\n\n\n", out: "line1\n",
		},
		{
			name: "trailing whitespace-only lines",
			in:   "line1\n  \n\t\n", out: "line1\n",
		},
		{
			name: "all blank lines",
			in:   "\n\n\n", out: "",
		},
		{
			name: "preserves internal blank lines",
			in:   "line1\n\nline2\n\n", out: "line1\n\nline2\n",
		},
		{
			name: "empty input",
			in:   "", out: "",
		},
		{
			name: "single line no newline",
			in:   "hello", out: "hello\n",
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			got := trimTrailingBlankLines([]byte(tt.in))
			if string(got) != tt.out {
				t.Errorf("case [%s]: expected %q; got %q", tt.name, tt.out, string(got))
			}
		})
	}
}

func TestDedentBytes(t *testing.T) {
	tc := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "no indentation",
			in:   "line1\nline2\n", out: "line1\nline2\n",
		},
		{
			name: "uniform tab indent",
			in:   "\tline1\n\tline2\n", out: "line1\nline2\n",
		},
		{
			name: "uniform space indent",
			in:   "    line1\n    line2\n", out: "line1\nline2\n",
		},
		{
			name: "mixed depth keeps relative indent",
			in:   "\t\tline1\n\tline2\n\t\t\tline3\n", out: "\tline1\nline2\n\t\tline3\n",
		},
		{
			name: "blank lines ignored for min calculation",
			in:   "\tline1\n\n\tline2\n", out: "line1\n\nline2\n",
		},
		{
			name: "whitespace-only lines ignored for min calculation",
			in:   "\tline1\n  \n\tline2\n", out: "line1\n\nline2\n",
		},
		{
			name: "already no indent",
			in:   "a\n  b\n", out: "a\n  b\n",
		},
		{
			name: "empty input",
			in:   "", out: "",
		},
		{
			name: "single indented line",
			in:   "    hello\n", out: "hello\n",
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			got := dedentBytes([]byte(tt.in))
			if string(got) != tt.out {
				t.Errorf("case [%s]: expected %q; got %q", tt.name, tt.out, string(got))
			}
		})
	}
}

func TestApplySubstitutions(t *testing.T) {
	tc := []struct {
		name string
		in   string
		subs []substitution
		out  string
	}{
		{
			name: "no substitutions",
			in:   "hello world", subs: nil, out: "hello world",
		},
		{
			name: "single substitution",
			in:   "x = ELLIPSIS", subs: []substitution{{old: "ELLIPSIS", new: "..."}}, out: "x = ...",
		},
		{
			name: "multiple substitutions applied in order",
			in:   "_ = ELLIPSIS\nELLIPSIS\n",
			subs: []substitution{{old: "_ = ELLIPSIS", new: "..."}, {old: "ELLIPSIS", new: "..."}},
			out:  "...\n...\n",
		},
		{
			name: "substitution replaces all occurrences",
			in:   "A and A and A", subs: []substitution{{old: "A", new: "B"}}, out: "B and B and B",
		},
		{
			name: "no match leaves content unchanged",
			in:   "hello", subs: []substitution{{old: "MISSING", new: "X"}}, out: "hello",
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			got := applySubstitutions([]byte(tt.in), tt.subs)
			if string(got) != tt.out {
				t.Errorf("case [%s]: expected %q; got %q", tt.name, tt.out, string(got))
			}
		})
	}
}
