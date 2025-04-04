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

func TestDB_Radius(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		var (
			ctx = t.Context()
			d   = newDB(t)
		)
		t.Cleanup(func() { d.Wait() })

		type testcase struct {
			name string
			loc  Location
			tag  Tag
		}

		{
			tt := []testcase{
				{
					name: "zero",
					tag:  "00000001",
				},
				{
					name: "long=1,lat=1",
					loc:  Location{1, 1},
					tag:  "00000002",
				},
				{
					name: "long=2,lat=3",
					loc:  Location{2, 3},
					tag:  "00000003",
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
					_, err := d.Pin(ctx, tc.loc, tc.tag)
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
		dd, err := d.Radius(ctx, Location{0, 0}, 4)
		is.OK(t, err) // d.Radius(ctx, r)
		is.Equal(t, len(dd), 3)
	})
}

func TestDB_Region(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		var (
			ctx = t.Context()
			d   = newDB(t)
		)
		t.Cleanup(func() { d.Wait() })

		type testcase struct {
			name string
			loc  Location
			tag  Tag
		}

		{
			tt := []testcase{
				{
					name: "zero",
					tag:  "00000001",
				},
				{
					name: "long=1,lat=1",
					loc:  Location{1, 1},
					tag:  "00000002",
				},
				{
					name: "long=2,lat=3",
					loc:  Location{2, 3},
					tag:  "00000003",
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
					_, err := d.Pin(ctx, tc.loc, tc.tag)
					return err
				})
			}
			is.OK(t, g.Wait()) // g.Wait
		}

		// does not work with zero?
		dd, err := d.Region(ctx, Point{-1, -1}, Point{1, -1}, Point{1, 1}, Point{-1, 1}, Point{-1, -1})
		is.OK(t, err) // d.Region(ctx, r)
		is.Equal(t, len(dd), 2)
	})
}

func TestDB_Edit(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		var (
			ctx = t.Context()
			d   = newDB(t)
		)
		t.Cleanup(func() { d.Wait() })

		id, err := d.Pin(ctx, Location{2, 3}, "00000001")
		is.OK(t, err) // d.Pin(ctx, loc={2,3})

		{
			g, ctx := errgroup.WithContext(ctx)
			g.Go(func() error {
				return d.EditFunc(ctx, id, func(d *Device) error { d.Loc = Location{-2, -3}; return nil })
			})
			g.Go(func() error {
				return d.EditFunc(ctx, id, func(d *Device) error { d.Tag = "00000002"; return nil })
			})
			g.Go(func() error {
				return d.EditFunc(ctx, id, func(d *Device) error { d.State = Started; return nil })
			})
			is.OK(t, g.Wait()) // d.Edit
		}

		dev, err := d.Device(ctx, id)
		is.OK(t, err) // d.Device(ctx, id=four)

		is.Equal(t, dev.Loc, Location{-2, -3}) // location change
		is.Equal(t, dev.Tag, "00000002")       // tag changed
		is.Equal(t, dev.State, Started)        // device has started
	})
}

func newDB(tb testing.TB) *DB {
	tb.Helper()

	p, m, pr, err := container.Pool(tb.Context(), tb.Name())
	is.OK(tb, err) // container.Pool(ctx)
	tb.Cleanup(func() { p.Close() })
	tb.Cleanup(func() { pr.Close() })

	is.OK(tb, m.MigrateUp(tb.Context())) // m.MigrateUp(ctx)
	tb.Cleanup(func() { m.MigrateDown(context.Background()) })

	return &DB{RWC: p}
}
