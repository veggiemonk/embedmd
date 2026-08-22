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
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Fetcher provides an abstraction on a file system.
// The Fetch function is called anytime some content needs to be fetched.
// For now this includes files and URLs.
// The first parameter is the base directory that could be used to resolve
// relative paths. This base directory will be ignored for absolute paths,
// such as URLs.
// Implementations should stop their work and return an error when the given
// context is done.
//
// The default implementation keeps local reads inside the base directory. A
// custom implementation is responsible for its own limits: embedmd applies
// none on its behalf.
type Fetcher interface {
	Fetch(ctx context.Context, dir, path string) ([]byte, error)
}

// maxResponseSize is the largest response body an HTTP fetch accepts.
const maxResponseSize = 10 << 20 // 10 MiB

// httpTimeout stops the process from hanging on a slow or unresponsive
// server. It applies to the whole request, headers and body.
const httpTimeout = 10 * time.Second

// fetcher is the default Fetcher. A nil client means the default one.
type fetcher struct{ client *http.Client }

// httpClient returns the client to use. Building one on demand keeps the
// package free of shared mutable state, and a fetch costs far more than the
// allocation.
func (f fetcher) httpClient() *http.Client {
	if f.client != nil {
		return f.client
	}
	return &http.Client{Timeout: httpTimeout}
}

// readLocalFile reads path, resolved against the base directory dir.
//
// When dir is set, the read stays inside dir: os.OpenInRoot refuses "..", an
// absolute path, and a symbolic link that points out of dir. A string compare
// on the cleaned path cannot do the last one.
//
// When dir is empty there is no boundary to keep, so the path is read as
// given. Callers that process untrusted markdown must set a base directory
// with WithBaseDir.
func readLocalFile(dir, path string) ([]byte, error) {
	name := filepath.FromSlash(path)
	if dir == "" {
		return os.ReadFile(name)
	}
	f, err := os.OpenInRoot(dir, name)
	if err != nil {
		// Drop the *fs.PathError: the caller already names the path.
		if perr, ok := errors.AsType[*fs.PathError](err); ok {
			return nil, perr.Err
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

func (f fetcher) Fetch(ctx context.Context, dir, path string) ([]byte, error) {
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		return readLocalFile(dir, path)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	res, err := f.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", res.Status)
	}
	// Read one byte past the limit. io.LimitReader alone stops at the limit
	// and reports no error, so an oversize response would enter the document
	// truncated, and look complete.
	b, err := io.ReadAll(io.LimitReader(res.Body, maxResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxResponseSize {
		return nil, fmt.Errorf("response is larger than the limit of %d bytes", maxResponseSize)
	}
	return b, nil
}
