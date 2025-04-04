// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/adoublef/benchmark/internal/database/cockroach"
	"github.com/adoublef/benchmark/internal/iot"
	. "github.com/adoublef/benchmark/internal/net/http"
	"github.com/testcontainers/testcontainers-go"
	"go.adoublef.dev/net/http/httputil"
	"go.adoublef.dev/net/nettest"
	"go.adoublef.dev/testing/is"
)

var acceptAll = func(r *http.Request) { r.Header.Set("Accept", "*/*") }
var ctJSON = func(r *http.Request) { r.Header.Set("Content-Type", "application/json") }

func doRequest[V any](c *http.Client, pattern string, body io.Reader, funcs ...func(*http.Request)) (V, int, http.Header, error) {
	var v V
	method, _, url, err := httputil.ParsePattern(pattern)
	if err != nil {
		return v, 0, nil, fmt.Errorf("cannot parse request pattern %q: %w", pattern, err)
	}

	if body != nil && method == "" {
		method = http.MethodPost
	} else if body == nil && method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return v, 0, nil, fmt.Errorf("cannot create request: %w", err)
	}
	for _, f := range funcs {
		f(req)
	}

	res, err := c.Do(req)
	if err != nil {
		return v, 0, nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusMultipleChoices || res.StatusCode == http.StatusNoContent || res.Body == nil {
		return v, res.StatusCode, res.Header, nil
	}
	var payload V
	err = json.NewDecoder(res.Body).Decode(&payload)
	return payload, res.StatusCode, res.Header, err
}

func newClientHTTP(tb testing.TB, h http.Handler) (*http.Client, *nettest.Proxy) {
	tb.Helper()
	ts := httptest.NewUnstartedServer(h)
	ts.Config.MaxHeaderBytes = DefaultMaxHeaderBytes
	// note: the client panics if readTimeout is less than the test timeout
	// is this a non-issue?
	ts.Config.ReadTimeout = DefaultReadTimeout
	ts.Config.WriteTimeout = DefaultWriteTimeout
	ts.Config.IdleTimeout = DefaultIdleTimeout
	// CipherSuites is a list of enabled TLS 1.0–1.2 cipher suites.
	// The order of the list is ignored.
	// Note that TLS 1.3 ciphersuites are not configurable.
	// ts.Config.TLSConfig.CipherSuites
	ts.StartTLS()
	tb.Cleanup(func() { ts.Close() })

	proxy := nettest.NewProxy("HTTP_"+tb.Name(), strings.TrimPrefix(ts.URL, "https://"))
	tb.Cleanup(func() { proxy.Close() })

	if tp, ok := ts.Client().Transport.(*http.Transport); ok {
		tp.DisableCompression = true
	}
	tc := httputil.WithTransport(ts.Client(), "https://"+proxy.Listen())
	return tc, proxy
}

func newDB(tb testing.TB) *iot.DB {
	tb.Helper()

	p, m, pr, err := container.Pool(tb.Context(), "Test")
	is.OK(tb, err) // container.Pool(ctx)
	tb.Cleanup(func() { p.Close() })
	tb.Cleanup(func() { pr.Close() })

	is.OK(tb, m.MigrateUp(tb.Context())) // m.MigrateUp(ctx)
	tb.Cleanup(func() { m.MigrateDown(context.Background()) })

	return &iot.DB{RWC: p}
}

func TestMain(m *testing.M) {
	err := setup(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	err = cleanup(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	os.Exit(code)
}

var container *cockroach.Container

// setup initialises containers within the pacakge.
func setup(ctx context.Context) (err error) {
	container, err = cockroach.Run(ctx, "")
	if err != nil {
		return
	}
	return
}

// cleanup stops all running containers for the pacakge.
func cleanup(ctx context.Context) (err error) {
	var cc = []testcontainers.Container{container}
	for _, c := range cc {
		if c != nil {
			err = errors.Join(err, c.Terminate(ctx))
		}
	}
	return err
}
