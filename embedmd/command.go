// Copyright 2016 Google Inc. All rights reserved.
// Copyright 2026 Julien Bisconti and the embedmd fork contributors.
//
// Changed in the github.com/veggiemonk/embedmd fork.
// See the NOTICE file for the list of changes.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to writing, software distributed
// under the License is distributed on a "AS IS" BASIS, WITHOUT WARRANTIES OR
// CONDITIONS OF ANY KIND, either express or implied.
//
// See the License for the specific language governing permissions and
// limitations under the License.

package embedmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type substitution struct {
	old, new string
}

type command struct {
	path, lang    string
	start, end    *string
	excludeStart  bool
	excludeEnd    bool
	dedent        bool
	trim          bool
	substitutions []substitution
}

func parseCommand(s string) (*command, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return nil, errors.New("argument list should be in parenthesis")
	}

	args, err := fields(s[1 : len(s)-1])
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, errors.New("missing file name")
	}

	cmd := &command{path: args[0]}
	args = args[1:]
	if len(args) > 0 && !isRegexpOrOption(args[0]) {
		cmd.lang, args = args[0], args[1:]
	} else {
		ext := filepath.Ext(cmd.path[1:])
		if len(ext) == 0 {
			return nil, errors.New("language is required when file has no extension")
		}
		cmd.lang = ext[1:]
	}

	// Consume regexp arguments (starting with / or !/ or bare $).
	for len(args) > 0 {
		arg := args[0]
		isRegexp := strings.HasPrefix(arg, "/")
		isExclude := strings.HasPrefix(arg, "!/")
		isDollar := arg == "$"
		isExcludeDollar := arg == "!$"

		if isExcludeDollar {
			return nil, errors.New("exclude (!) cannot be used with $")
		}

		// $ means "to the end of the file", so it only makes sense as the
		// second range argument. Without this check the start slot stays
		// empty while the end slot is filled, and extract dereferences nil.
		if isDollar && cmd.start == nil {
			return nil, errors.New("$ must follow a start regexp")
		}

		if !isRegexp && !isExclude && !isDollar {
			break
		}

		if cmd.start == nil && !isDollar {
			re := arg
			if isExclude {
				re = arg[1:]
				cmd.excludeStart = true
			}
			cmd.start = &re
		} else if cmd.end == nil {
			re := arg
			if isExclude {
				re = arg[1:]
				cmd.excludeEnd = true
			}
			cmd.end = &re
		} else {
			return nil, errors.New("too many arguments")
		}
		args = args[1:]
	}

	// Single regexp with exclude is useless (would produce empty output).
	if cmd.start != nil && cmd.end == nil && cmd.excludeStart {
		return nil, errors.New("exclude (!) cannot be used with a single regexp")
	}

	// Consume options: dedent, trim, s/old/new/.
	for _, arg := range args {
		switch {
		case arg == "dedent":
			cmd.dedent = true
		case arg == "trim":
			cmd.trim = true
		case strings.HasPrefix(arg, "s/"):
			sub, err := parseSubstitution(arg)
			if err != nil {
				return nil, err
			}
			cmd.substitutions = append(cmd.substitutions, sub)
		default:
			return nil, fmt.Errorf("unknown option %q", arg)
		}
	}

	return cmd, nil
}

func parseSubstitution(s string) (substitution, error) {
	// s is already validated as a complete s/old/new/ token by fields().
	inner := s[2 : len(s)-1] // strip "s/" and trailing "/"
	idx := strings.Index(inner, "/")
	if idx < 0 {
		return substitution{}, fmt.Errorf("invalid substitution %q", s)
	}
	return substitution{old: inner[:idx], new: inner[idx+1:]}, nil
}

// isRegexpOrOption reports whether arg looks like a regexp (/.../, !/.../, $)
// or an option (dedent, trim, s/.../) rather than a language identifier.
func isRegexpOrOption(arg string) bool {
	return arg[0] == '/' || arg[0] == '!' || arg == "$" ||
		strings.HasPrefix(arg, "s/") || arg == "dedent" || arg == "trim"
}

// fields returns a list of the groups of text separated by blanks,
// keeping all text surrounded by / as a group.
// It also handles !/regexp/ (exclude) and s/old/new/ (substitution) tokens.
func fields(s string) ([]string, error) {
	var args []string

	for s = strings.TrimSpace(s); len(s) > 0; s = strings.TrimSpace(s) {
		regexpStart := s[0] == '/'
		excludeRegexp := len(s) > 1 && s[0] == '!' && s[1] == '/'
		substStart := len(s) > 1 && s[0] == 's' && s[1] == '/'

		switch {
		case regexpStart, excludeRegexp:
			offset := 0
			if excludeRegexp {
				offset = 1
			}
			sep := nextSlash(s[offset+1:])
			if sep < 0 {
				return nil, errors.New("unbalanced /")
			}
			args, s = append(args, s[:offset+sep+2]), s[offset+sep+2:]
		case substStart:
			sep1 := nextSlash(s[2:])
			if sep1 < 0 {
				return nil, errors.New("unbalanced / in substitution")
			}
			sep2 := nextSlash(s[2+sep1+1:])
			if sep2 < 0 {
				return nil, errors.New("unbalanced / in substitution")
			}
			end := 2 + sep1 + 1 + sep2 + 1
			args, s = append(args, s[:end]), s[end:]
		default:
			sep := strings.IndexByte(s[1:], ' ')
			if sep < 0 {
				return append(args, s), nil
			}
			args, s = append(args, s[:sep+1]), s[sep+1:]
		}
	}

	return args, nil
}

// nextSlash will find the index of the next unescaped slash in a string.
func nextSlash(s string) int {
	for sep := 0; ; sep++ {
		i := strings.IndexByte(s[sep:], '/')
		if i < 0 {
			return -1
		}
		sep += i
		if sep == 0 || s[sep-1] != '\\' {
			return sep
		}
	}
}
