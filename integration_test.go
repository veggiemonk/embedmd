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
// under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
// CONDITIONS OF ANY KIND, either express or implied.
//
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestIntegration builds the binary from the current source and runs it over
// sample/docs.md. The sample embeds one file over HTTP, so the test needs
// network access and is skipped with -short.
func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test needs network access")
	}

	bin := filepath.Join(t.TempDir(), "embedmd")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("could not build embedmd (%v): %s", err, out)
	}

	cmd := exec.Command(bin, "docs.md")
	cmd.Dir = "sample"
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("could not process file (%v): %s", err, got)
	}
	wants, err := os.ReadFile(filepath.Join("sample", "result.md"))
	if err != nil {
		t.Fatalf("could not read result: %v", err)
	}
	if string(got) != string(wants) {
		t.Fatalf("got bad result (compared to result.md):\n%s", got)
	}
}
