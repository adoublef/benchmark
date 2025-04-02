// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iot

import (
	"context"
	"errors"
	"iter"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.adoublef.dev/sync/batchque"
	"go.adoublef.dev/sync/hashque"
	"go.adoublef.dev/xiter"
	"golang.org/x/sync/errgroup"
	"tailscale.com/util/singleflight"
)

func (d *DB) Wait() {
	d.dbq.Stop()  // todo: use Wait
	d.wbq.Stop()  // todo: use Wait
	d.efbq.Stop() // todo: use Wait
}

type DB struct {
	RWC *pgxpool.Pool

	dsf singleflight.Group[ID, Device]
	dbq batchque.Group[ID, Device]
	wsf singleflight.Group[struct {
		Location
		float64
	}, []Device]
	wbq batchque.Group[struct {
		Location
		float64
	}, []Device]
	efbq batchque.Group[Device, struct{}]
	efhq hashque.Group[ID]
}

// Device returns a [Device].
func (d *DB) Device(ctx context.Context, id ID) (Device, error) {
	res := d.dsf.DoChanContext(ctx, id, func(ctx context.Context) (Device, error) {
		type Request = batchque.Request[ID, Device]
		dev, err := d.dbq.Do(ctx, id, func(ctx context.Context, r []Request) {
			// drop some dangling requests
			c1 := make(chan Request, 1)
			go func() {
				defer close(c1)
				for _, r := range r {
					select {
					case <-r.Context().Done():
					case c1 <- r:
					}
				}
			}()
			// todo: check cache
			c2 := make(chan Request, 1)
			var wg sync.WaitGroup
			for r := range c1 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// check the cache
					// if not found check in the database
					select {
					case <-r.Context().Done():
					case c2 <- r:
					}
				}()
			}
			go func() {
				wg.Wait()
				close(c2)
			}()
			var (
				rr = make([]Request, 0, len(r))
				id = make([]ID, 0, len(r))
			)
			for r := range c2 {
				select {
				case <-r.Context().Done():
				default:
					rr = append(rr, r)
					id = append(id, r.Val)
				}
			}
			seq := xiter.Zip2(d.getDevice(ctx, id...), slices.All(rr))
			for z := range seq {
				if z.Ok1 != z.Ok2 {
					return
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					var (
						r   = z.V2
						dev = z.K1
						err = z.V1
					)
					if err != nil {
						r.CancelFunc(err)
						return
					}
					select {
					case <-r.Context().Done():
					case r.C <- dev:
					}
					// populate the cache here
					// evict if we go over the size?
				}()
			}
			wg.Wait()
			// evict if we go over the size?
		})
		return dev, err
	})

	select {
	case <-ctx.Done():
		return Device{}, ctx.Err()
	case res := <-res:
		return res.Val, res.Err
	}
}

func (d *DB) getDevice(ctx context.Context, id ...ID) iter.Seq2[Device, error] {
	const query = `
select id, long, lat
from iot.device
where id = $1`
	var b pgx.Batch
	b.QueuedQueries = make([]*pgx.QueuedQuery, len(id))
	for i := range b.Len() {
		b.QueuedQueries[i] = &pgx.QueuedQuery{
			SQL:       query,
			Arguments: []any{id[i]},
		}
	}
	return func(yield func(Device, error) bool) {
		br := d.RWC.SendBatch(ctx, &b)
		defer br.Close()

		for range b.Len() {
			dev, err := func() (Device, error) {
				var dev Device
				err := br.QueryRow().Scan(&dev.ID, &dev.Loc.Longitude, &dev.Loc.Latitude)
				if err != nil {
					return Device{}, err
				}
				return dev, nil
			}()
			if !yield(dev, err) {
				return
			}
		}
	}
}

// Pin a new [Device] to the world map.
func (d *DB) Pin(ctx context.Context, loc Location) (ID, error) {
	const query = `
insert into iot.device (id, long, lat)
values ($1, $2, $3)`
	id := uuid.Must(uuid.NewV7())
	_, err := d.RWC.Exec(ctx, query, id, loc.Longitude, loc.Latitude)
	if err != nil {
		return ID{}, err
	}
	return id, nil
}

// Within returns a [Device] slice withing a radius.
func (d *DB) Within(ctx context.Context, loc Location, r float64) ([]Device, error) {
	type Key = struct {
		Location
		float64
	}
	res := d.wsf.DoChanContext(ctx, Key{loc, r}, func(ctx context.Context) ([]Device, error) {
		type Request = batchque.Request[Key, []Device]
		dev, err := d.wbq.Do(ctx, Key{loc, r}, func(ctx context.Context, r []Request) {
			// drop some dangling requests
			c1 := make(chan Request, 1)
			go func() {
				defer close(c1)
				for _, r := range r {
					select {
					case <-r.Context().Done():
					case c1 <- r:
					}
				}
			}()
			// todo: check cache
			c2 := make(chan Request, 1)
			var wg sync.WaitGroup
			for r := range c1 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// check the cache
					// if not found check in the database
					select {
					case <-r.Context().Done():
					case c2 <- r:
					}
				}()
			}
			go func() {
				wg.Wait()
				close(c2)
			}()
			var (
				rr = make([]Request, 0, len(r))
				ll = make([]Location, 0, len(r))
				rd = make([]float64, 0, len(r))
			)
			for r := range c2 {
				select {
				case <-r.Context().Done():
				default:
					rr = append(rr, r)
					ll = append(ll, r.Val.Location)
					rd = append(rd, r.Val.float64)
				}
			}
			seq := xiter.Zip2(d.withinRadius(ctx, ll, rd), slices.All(rr))
			for z := range seq {
				if z.Ok1 != z.Ok2 {
					return
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					var (
						r   = z.V2
						ii  = z.K1
						err = z.V1
					)
					if err != nil {
						r.CancelFunc(err)
						return
					}
					// query for data
					g, gCtx := errgroup.WithContext(r.Context())
					var ci = make(chan ID, 1)
					g.Go(func() error {
						defer close(ci)
						for _, id := range ii {
							select {
							case <-gCtx.Done():
								return gCtx.Err()
							case ci <- id:
							}
						}
						return nil
					})

					var c3 = make(chan Device, 1)
					for id := range ci {
						g.Go(func() error {
							d, err := d.Device(gCtx, id)
							if err != nil {
								return err
							}
							select {
							case <-gCtx.Done():
								return gCtx.Err()
							case c3 <- d:
							}
							return nil
						})
					}
					go func() {
						g.Wait()
						close(c3)
					}()
					var dd = make([]Device, 0, len(ii))
					for d := range c3 {
						dd = append(dd, d)
					}
					if err := g.Wait(); err != nil {
						r.CancelFunc(err)
						return
					}
					// return ids to caller
					select {
					case <-r.Context().Done():
					case r.C <- dd:
					}
				}()
			}
			wg.Wait()
			// evict if we go over the size?
		})
		return dev, err
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-res:
		return res.Val, res.Err
	}
}

func (d *DB) withinRadius(ctx context.Context, loc []Location, r []float64) iter.Seq2[[]ID, error] {
	const query = `
select id
from iot.device
where st_dwithin(st_makepoint(long, lat), st_makepoint($2, $3), $1)`

	var b pgx.Batch
	b.QueuedQueries = make([]*pgx.QueuedQuery, len(loc))
	for i := range b.Len() {
		b.QueuedQueries[i] = &pgx.QueuedQuery{
			SQL:       query,
			Arguments: []any{r[i], loc[i].Longitude, loc[i].Latitude},
		}
	}
	return func(yield func([]ID, error) bool) {
		br := d.RWC.SendBatch(ctx, &b)
		defer br.Close()

		for range b.Len() {
			dd, err := func() ([]ID, error) {
				rr, err := br.Query()
				if err != nil {
					return nil, err
				}
				defer rr.Close()
				var dd []ID
				for rr.Next() {
					var d ID
					err := rr.Scan(&d)
					if err != nil {
						return nil, err
					}
					dd = append(dd, d)
				}
				if err := rr.Err(); err != nil {
					return nil, err
				}
				return dd, nil
			}()
			if !yield(dd, err) {
				return
			}
		}
	}
}

// EditFunc
func (d *DB) EditFunc(ctx context.Context, id ID, f func(*Device) error) error {
	c1 := make(chan error, 1)
	err := d.efhq.DoContext(ctx, id, func() {
		defer close(c1)
		// get the device
		dev, err := d.Device(ctx, id)
		if err != nil {
			c1 <- err
			return
		}
		if err := f(&dev); err != nil {
			c1 <- err
			return
		}
		type Request = batchque.Request[Device, struct{}]
		_, err = d.efbq.Do(ctx, dev, func(ctx context.Context, r []Request) {
			// drop some dangling requests
			c2 := make(chan Request, 1)
			go func() {
				defer close(c2)
				for _, r := range r {
					select {
					case <-r.Context().Done():
					case c2 <- r:
					}
				}
			}()
			// todo: check cache
			c3 := make(chan Request, 1)
			var wg sync.WaitGroup
			for r := range c2 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// check the cache
					// if not found check in the database
					select {
					case <-r.Context().Done():
					case c3 <- r:
					}
				}()
			}
			go func() {
				wg.Wait()
				close(c3)
			}()
			var (
				rr = make([]Request, 0, len(r))
				dd = make([]Device, 0, len(r))
			)
			for r := range c3 {
				select {
				case <-r.Context().Done():
				default:
					rr = append(rr, r)
					dd = append(dd, r.Val)
				}
			}
			seq := xiter.Zip(d.editDevice(ctx, dd...), slices.Values(rr))
			for z := range seq {
				if z.Ok1 != z.Ok2 {
					return
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					var (
						r   = z.V2
						err = z.V1
					)
					if err != nil {
						r.CancelFunc(err)
						return
					}
					// populate the cache here
					// evict if we go over the size?
				}()
			}
			wg.Wait()
			// evict if we go over the size?
		})
		c1 <- err
	})
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-c1:
		return err
	}
}

func (d *DB) editDevice(ctx context.Context, dev ...Device) iter.Seq[error] {
	const query = `
update iot.device set long = $2, lat = $3
where id = $1`
	var b pgx.Batch
	b.QueuedQueries = make([]*pgx.QueuedQuery, len(dev))
	for i := range b.Len() {
		b.QueuedQueries[i] = &pgx.QueuedQuery{
			SQL:       query,
			Arguments: []any{dev[i].ID, dev[i].Loc.Longitude, dev[i].Loc.Latitude},
		}
	}
	return func(yield func(error) bool) {
		br := d.RWC.SendBatch(ctx, &b)
		defer br.Close()

		for range b.Len() {
			err := func() error {
				rs, err := br.Exec()
				if err != nil {
					return err
				} else if rs.RowsAffected() < 1 {
					return ErrNotExist
				}
				return nil
			}()
			if !yield(err) {
				return
			}
		}
	}
}

var (
	ErrInvalid          = errors.New("invalid argument")
	ErrPermission       = errors.New("permission denied")
	ErrExist            = errors.New("already exists")
	ErrNotExist         = errors.New("does not exist")
	ErrClosed           = errors.New("already closed")
	ErrDeadlineExceeded = errors.New("i/o timeout")
)
