// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iot_test

import (
	"context"
	"testing"

	. "github.com/adoublef/benchmark/internal/iot"
	"go.adoublef.dev/testing/is"
	"golang.org/x/sync/errgroup"
)

func TestDB_Within(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		var (
			ctx = t.Context()
			d   = newDB(t)
		)
		t.Cleanup(func() { d.Wait() })

		type testcase struct {
			name string
			loc  Location
		}

		{
			tt := []testcase{
				{
					name: "zero",
				},
				{
					name: "long=1,lat=1",
					loc:  Location{1, 1},
				},
				{
					name: "long=2,lat=3",
					loc:  Location{2, 3},
				},
			}

			g, ctx := errgroup.WithContext(ctx)
			c := make(chan testcase, 1)
			g.Go(func() error {
				defer close(c)
				for _, tc := range tt {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case c <- tc:
					}
				}
				return nil
			})

			for tc := range c {
				g.Go(func() error {
					_, err := d.Pin(ctx, tc.loc)
					return err
				})
			}
			is.OK(t, g.Wait()) // g.Wait
		}

		// === RUN   TestDB_Within
		// === RUN   TestDB_Within/OK
		// --- PASS: TestDB_Within (0.16s)
		// --- PASS: TestDB_Within/OK (0.16s)
		// === RUN   TestDB_Within
		// === RUN   TestDB_Within/OK
		// --- PASS: TestDB_Within (0.26s)
		//     --- PASS: TestDB_Within/OK (0.26s)

		// does not work with zero?
		dd, err := d.Within(ctx, Location{0, 0}, 4)
		is.OK(t, err) // d.Within(ctx, r)
		is.Equal(t, len(dd), 3)
	})
}

func TeestDB_Edit(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		var (
			ctx = t.Context()
			d   = newDB(t)
		)
		t.Cleanup(func() { d.Wait() })

		four, err := d.Pin(ctx, Location{2, 3})
		is.OK(t, err) // d.Pin(ctx, loc={2,3})

		err = d.EditFunc(ctx, four, func(d *Device) error { d.Loc = Location{-2, 3}; return nil })
		is.OK(t, err) // d.Edit(ctx, dev={id=four, loc={-2,-3}})

		dev, err := d.Device(ctx, four)
		is.OK(t, err) // d.Device(ctx, id=four)

		is.Equal(t, dev.Loc, Location{-2, -3}) // location change
	})
}

func newDB(tb testing.TB) *DB {
	tb.Helper()

	p, m, err := container.Pool(tb.Context())
	is.OK(tb, err) // container.Pool(ctx)
	tb.Cleanup(func() { p.Close() })

	is.OK(tb, m.MigrateUp(tb.Context())) // m.MigrateUp(ctx)
	tb.Cleanup(func() { m.MigrateDown(context.Background()) })

	return &DB{RWC: p}
}
