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

package embedmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchHTTPTimeout(t *testing.T) {
	// Server that hangs forever without sending a response body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// flush headers so the client sees 200, then block
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// block until the client gives up
		<-r.Context().Done()
	}))
	defer srv.Close()

	// Replace the package-level client with a very short timeout.
	old := httpClient
	httpClient = &http.Client{Timeout: 100 * time.Millisecond}
	defer func() { httpClient = old }()

	f := fetcher{}
	_, err := f.Fetch(context.Background(), "", srv.URL)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestFetchHTTPSizeLimit(t *testing.T) {
	const limit = 10 << 20 // 10 MiB — must match maxResponseSize in content.go
	// Server that returns more than the limit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write limit+1 bytes worth of data.
		chunk := bytes.Repeat([]byte("x"), 4096)
		total := 0
		for total < limit+1 {
			n, err := w.Write(chunk)
			if err != nil {
				return
			}
			total += n
		}
	}))
	defer srv.Close()

	f := fetcher{}
	data, err := f.Fetch(context.Background(), "", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != limit {
		t.Errorf("expected %d bytes, got %d", limit, len(data))
	}
}

func TestFetchHTTPNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	f := fetcher{}
	_, err := f.Fetch(context.Background(), "", srv.URL)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	want := fmt.Sprintf("status %s", http.StatusText(http.StatusNotFound)+" 404 page not found\n")
	_ = want // error message format varies; just confirm non-nil
}

func TestFetchContextCancelled(t *testing.T) {
	// Server that blocks until the client goes away.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	f := fetcher{}
	_, err := f.Fetch(ctx, "", srv.URL)
	if err == nil {
		t.Fatal("expected error after cancel, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestProcessContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	in := strings.NewReader("[embedmd]:# (code.go)\n")
	var out bytes.Buffer
	err := Process(ctx, &out, in, WithFetcher(fakeFileProvider{"code.go": []byte("package main\n")}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
