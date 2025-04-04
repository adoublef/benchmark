// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/toxics"
	. "github.com/adoublef/benchmark/internal/net/http"
	"go.adoublef.dev/net/nettest"
	"go.adoublef.dev/testing/is"
)

func Test_Decode(t *testing.T) {

	t.Run("OK", func(t *testing.T) {

		c, _ := newDecodeClient(t, DefaultMaxBytes, 0)

		_, sc, _, err := doRequest[any](c, "POST /", strings.NewReader(`{"username":"username","password":"password"}`), ctJSON)
		is.OK(t, err)                         // POST /
		is.Equal(t, sc, http.StatusNoContent) // got!=want
	})

	t.Run("ErrRequestEntityTooLarge", func(t *testing.T) {

		c, _ := newDecodeClient(t, 1, 0)

		_, sc, _, err := doRequest[any](c, "POST /", strings.NewReader(`{"username":"username","password":"password"}`), ctJSON)
		is.OK(t, err)
		is.Equal(t, sc, http.StatusRequestEntityTooLarge)
	})

	t.Run("ErrRequestTimeout", func(t *testing.T) {

		c, p := newDecodeClient(t, DefaultMaxBytes, 1)

		toxic, err := p.AddToxic("bandwidth", true, &toxics.BandwidthToxic{Rate: 1})
		is.OK(t, err)
		t.Cleanup(func() { p.RemoveToxic(toxic) })

		t.Logf("%v := p.AddToxic(bandwidth)", err)

		s := fmt.Sprintf(`{"username":%q,"password":"password"}`, strings.Repeat("username", 1<<10))
		_, sc, _, err := doRequest[any](c, "POST /", strings.NewReader(s), ctJSON)
		is.OK(t, err)
		is.Equal(t, sc, http.StatusRequestTimeout)
	})

	t.Run("ErrBadRequest", func(t *testing.T) {

		c, _ := newDecodeClient(t, DefaultMaxBytes, 0)

		type testcase struct {
			body   string
			detail string
		}

		for name, tc := range map[string]testcase{
			"Syntax": {
				body:   `{"username:"user"}`,
				detail: "invalid character 'u' at position 13",
			},
			"Syntax2": {
				body:   `<"username:"user"}`,
				detail: "invalid character '<' at position 1",
			},
			"Unmarshal": {
				body:   `{"username":1,"password":"pass"}`,
				detail: `unexpected number for field "username" at position 13`,
			},
			"Unmarshal2": {
				body:   `"username:"user"}`,
				detail: "unexpected string for field \"\" at position 11",
				// skip:   true,
			},
			"UnknownField": {
				body:   `{"never":"user"}`,
				detail: `unknown field "never"`,
			},
			"Stream": {
				body: `{"username":"username","password":"password"}{}`,
			},
		} {
			t.Run(name, func(t *testing.T) {

				_, sc, _, err := doRequest[any](c, "POST /", strings.NewReader(tc.body), ctJSON)
				is.OK(t, err)
				is.Equal(t, sc, http.StatusBadRequest)
				// todo: read the error to see if it matches
			})
		}
	})
}

func newDecodeClient(tb testing.TB, sz int, d time.Duration) (*http.Client, *nettest.Proxy) {
	handleTest := func() http.HandlerFunc {
		type payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		return func(w http.ResponseWriter, r *http.Request) {
			_, err := Decode[payload](w, r, sz, d)
			if err != nil {
				Error(w, r, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /{$}", handleTest())

	return newClientHTTP(tb, mux)
}
