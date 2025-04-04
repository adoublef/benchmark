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

	. "github.com/adoublef/benchmark/internal/net/http"
	"go.adoublef.dev/net/nettest"
	"go.adoublef.dev/testing/is"
	"golang.org/x/sync/errgroup"
)

func TestHandler_register(t *testing.T) {

	t.Run("OK", func(t *testing.T) {
		t.Run("OK", func(t *testing.T) {
			c, _ := newHandlerClient(t)

			_, sc, _, err := doRequest[any](c, "POST /devices", strings.NewReader(`{"assetTag":"at00000001"}`), ctJSON, acceptAll)
			is.OK(t, err) // return file upload response
			is.Equal(t, sc, http.StatusCreated)
		})
	})
}

func TestHandler_search(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		c, _ := newHandlerClient(t)

		body := strings.NewReader(`{"assetTag":"at00000001"}`)
		_, sc, _, err := doRequest[any](c, "POST /devices", body, ctJSON, acceptAll)
		is.OK(t, err) // upload a device
		is.Equal(t, sc, http.StatusCreated)
		// get all devices within a range

		res, sc, _, err := doRequest[struct{ Devices []any }](c, "GET /devices", nil, ctJSON, acceptAll)
		is.OK(t, err) // return all the points
		is.Equal(t, sc, http.StatusOK)
		is.Equal(t, len(res.Devices), 1)
	})
}

func TestHandler_device(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		ctx := t.Context()
		c, _ := newHandlerClient(t)

		// create N-devices
		{
			const N = 10
			g, ctx := errgroup.WithContext(ctx)
			for i := range N {
				g.Go(func() error {
					body := strings.NewReader(fmt.Sprintf(`{"assetTag":"at%08d"}`, i))
					err := func() error {
						r, err := http.NewRequestWithContext(ctx, "POST", "/devices", body)
						if err != nil {
							return err
						}
						ctJSON(r)
						acceptAll(r)
						res, err := c.Do(r)
						if err != nil {
							return err
						} else if res.StatusCode != http.StatusCreated {
							return fmt.Errorf("expected 201 code")
						}
						// todo: get the uuid that was returned.
						return nil
					}()
					if err != nil {
						return err
					}
					return nil
				})
			}
			// note: we could also search for each device that is added.
			is.OK(t, g.Wait())
		}

		res, sc, _, err := doRequest[struct{ Devices []any }](c, "GET /devices", nil, ctJSON, acceptAll)
		is.OK(t, err) // return all the points
		is.Equal(t, sc, http.StatusOK)
		is.Equal(t, len(res.Devices), 10)
	})
}

func TestHandler_ping(t *testing.T) {

	t.Run("OK", func(t *testing.T) {

		c, _ := newHandlerClient(t)
		// send a ping
		_, sc, _, err := doRequest[any](c, "GET /ping", nil)
		is.OK(t, err)                         // GET /ping
		is.Equal(t, sc, http.StatusNoContent) // got!=want
	})
}

func newHandlerClient(tb testing.TB) (*http.Client, *nettest.Proxy) {
	tb.Helper()

	var (
		db = newDB(tb)
	)

	h := Handler(db)
	return newClientHTTP(tb, h)
}
