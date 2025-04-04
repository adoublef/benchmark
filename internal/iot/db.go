// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iot

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.adoublef.dev/runtime/debug"
	"go.adoublef.dev/sync/batchque"
	"go.adoublef.dev/xiter"
	"golang.org/x/sync/errgroup"
	"tailscale.com/util/singleflight"
)

func (d *DB) Wait() {
	// d.dbq.Stop()  // todo: fix stop
	// d.wbq.Stop() // todo: fix stop
	// d.efbq.Stop() // todo: fix stop
}

type DB struct {
	RWC *pgxpool.Pool

	dsf singleflight.Group[ID, Device]
	dbq batchque.Group[ID, Device]
	pbq batchque.Group[struct {
		Location
		Tag
	}, ID]
	wsf singleflight.Group[struct {
		Location
		float64
	}, []Device]
	wbq batchque.Group[struct {
		Location
		float64
	}, []Device]
	csf singleflight.Group[struct {
		WKT string // calculated based on []Points
	}, []Device]
	cbq batchque.Group[struct {
		WKT string // calculated based on []Points
	}, []Device]
	fnbq batchque.Group[struct {
		ID
		Func func(*Device) error
	}, struct{}]
	efbq batchque.Group[Device, struct{}]
	// efhq hashque.Group[ID]
}

// Device returns a [Device].
func (d *DB) Device(ctx context.Context, id ID) (Device, error) {
	res := d.dsf.DoChanContext(ctx, id, func(ctx context.Context) (Device, error) {
		type Request = batchque.Request[ID, Device]
		dev, err := d.dbq.Do(ctx, id, func(ctx context.Context, r []Request) {
			// drop some dangling requests
			rc := make(chan Request, 1)
			go func() {
				defer close(rc)
				for _, r := range r {
					select {
					case <-r.Context().Done():
					case rc <- r:
					}
				}
			}()
			// todo: check cache
			rrc := make(chan Request, 1)
			var wg sync.WaitGroup
			for r := range rc {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// check the cache
					// if not found check in the database
					select {
					case <-r.Context().Done():
					case rrc <- r:
					}
				}()
			}
			go func() {
				wg.Wait()
				close(rrc)
			}()
			var (
				rr = make([]Request, 0, len(r))
				id = make([]ID, 0, len(r))
			)
			for r := range rrc {
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
select id, tag, long, lat, state
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
				err := br.QueryRow().Scan(&dev.ID, &dev.Tag, &dev.Loc.Longitude, &dev.Loc.Latitude, &dev.State)
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
//
// TODO: batch insertions.
func (d *DB) Pin(ctx context.Context, loc Location, tag Tag) (ID, error) {
	type Key = struct {
		Location
		Tag
	}
	type Request = batchque.Request[Key, ID]
	id, err := d.pbq.Do(ctx, Key{loc, tag}, func(ctx context.Context, r []Request) {
		// drop some dangling requests
		rc := make(chan Request, 1)
		go func() {
			defer close(rc)
			for _, r := range r {
				select {
				case <-r.Context().Done():
				case rc <- r:
				}
			}
		}()
		// collect the remainder
		var (
			rr = make([]Request, 0, len(r))
			ll = make([]Location, 0, len(r))
			tt = make([]Tag, 0, len(r))
		)
		for r := range rc {
			select {
			case <-r.Context().Done():
			default:
				rr = append(rr, r)
				ll = append(ll, r.Val.Location)
				tt = append(tt, r.Val.Tag)
			}
		}

		var wg sync.WaitGroup
		seq := xiter.Zip2(d.pin(ctx, ll, tt), slices.All(rr))
		for z := range seq {
			if z.Ok1 != z.Ok2 {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				var (
					r   = z.V2
					id  = z.K1
					err = z.V1
				)
				if err != nil {
					r.CancelFunc(err)
					return
				}
				r.C <- id
				// populate the cache here
				// evict if we go over the size?
			}()
		}
		wg.Wait()
		// evict if we go over the size?
	})
	return id, err
}

func (d *DB) pin(ctx context.Context, loc []Location, tag []Tag) iter.Seq2[ID, error] {
	// should assert that len(loc) == len(tag)
	const query = `
insert into iot.device (id, tag, long, lat, state)
values ($1, $2, $3, $4, $5)`
	var b pgx.Batch
	b.QueuedQueries = make([]*pgx.QueuedQuery, len(tag))
	for i := range b.Len() {
		b.QueuedQueries[i] = &pgx.QueuedQuery{
			SQL:       query,
			Arguments: []any{uuid.Must(uuid.NewV7()), tag[i], loc[i].Longitude, loc[i].Latitude, Created},
		}
	}
	return func(yield func(ID, error) bool) {
		br := d.RWC.SendBatch(ctx, &b)
		defer br.Close()

		for i := range b.Len() {
			id := b.QueuedQueries[i].Arguments[0].(ID)
			err := func() error {
				_, err := br.Exec()
				if err != nil {
					return err
				}
				return nil
			}()
			if !yield(id, err) {
				return
			}
		}
	}
}

// Radius returns a [Device] slice withing a radius limit.
func (d *DB) Radius(ctx context.Context, loc Location, r float64) ([]Device, error) {
	type Key = struct {
		Location
		float64
	}
	res := d.wsf.DoChanContext(ctx, Key{loc, r}, func(ctx context.Context) ([]Device, error) {
		type Request = batchque.Request[Key, []Device]
		dev, err := d.wbq.Do(ctx, Key{loc, r}, func(ctx context.Context, r []Request) {
			// drop some dangling requests
			rc := make(chan Request, 1)
			go func() {
				defer close(rc)
				for _, r := range r {
					select {
					case <-r.Context().Done():
					case rc <- r:
					}
				}
			}()
			// todo: check cache
			rrc := make(chan Request, 1)
			var wg sync.WaitGroup
			for r := range rc {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// check the cache
					// if not found check in the database
					select {
					case <-r.Context().Done():
					case rrc <- r:
					}
				}()
			}
			go func() {
				wg.Wait()
				close(rrc)
			}()
			var (
				rr = make([]Request, 0, len(r))
				ll = make([]Location, 0, len(r))
				rd = make([]float64, 0, len(r))
			)
			for r := range rrc {
				select {
				case <-r.Context().Done():
				default:
					rr = append(rr, r)
					ll = append(ll, r.Val.Location)
					rd = append(rd, r.Val.float64)
				}
			}
			// var wg sync.WaitGroup
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
					dd, err := d.By(r.Context(), ii...)
					if err != nil {
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

// Region returns a list of [Device] within a closed polygon region.
func (d *DB) Region(ctx context.Context, pp ...Point) ([]Device, error) {
	type Key = struct {
		WKT string // a slice is not comparable so a string is a compromise
	}
	// can we do this better?
	// polygon((%s))
	coord := xiter.Reduce(func(sum string, p Point) string {
		if sum == "" {
			return fmt.Sprintf("%g %g", p.X, p.Y)
		}
		return fmt.Sprintf("%s, %g %g", sum, p.X, p.Y)
	}, "", slices.Values(pp))
	wkt := fmt.Sprintf("polygon((%s))", coord)

	res := d.csf.DoChanContext(ctx, Key{wkt}, func(ctx context.Context) ([]Device, error) {
		type Request = batchque.Request[Key, []Device]
		dev, err := d.cbq.Do(ctx, Key{wkt}, func(ctx context.Context, r []Request) {
			// drop some dangling requests
			rc := make(chan Request, 1)
			go func() {
				defer close(rc)
				for _, r := range r {
					select {
					case <-r.Context().Done():
					case rc <- r:
					}
				}
			}()
			// todo: check cache
			rrc := make(chan Request, 1)
			var wg sync.WaitGroup
			for r := range rc {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// check the cache
					// if not found check in the database
					select {
					case <-r.Context().Done():
					case rrc <- r:
					}
				}()
			}
			go func() {
				wg.Wait()
				close(rrc)
			}()
			var (
				rr  = make([]Request, 0, len(r))
				wkt = make([]string, 0, len(r))
			)
			for r := range rrc {
				select {
				case <-r.Context().Done():
				default:
					rr = append(rr, r)
					wkt = append(wkt, r.Val.WKT)
				}
			}
			seq := xiter.Zip2(d.coveredBy(ctx, wkt...), slices.All(rr))
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
					dd, err := d.By(r.Context(), ii...)
					if err != nil {
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

func (d *DB) coveredBy(ctx context.Context, wkt ...string) iter.Seq2[[]ID, error] {
	const query = `
select id
from iot.device
where st_coveredby(
	st_makepoint(long, lat),
	st_geomfromtext($1)
)`

	var b pgx.Batch
	b.QueuedQueries = make([]*pgx.QueuedQuery, len(wkt))
	for i := range b.Len() {
		b.QueuedQueries[i] = &pgx.QueuedQuery{
			SQL:       query,
			Arguments: []any{wkt[i]},
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

// By returns a slice of [Device] by their [ID].
//
// TODO: preserve order
func (d *DB) By(ctx context.Context, ii ...ID) ([]Device, error) {
	g, ctx := errgroup.WithContext(ctx)
	var iic = make(chan ID, 1)
	g.Go(func() error {
		defer close(iic)
		for _, id := range ii {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case iic <- id:
			}
		}
		return nil
	})
	var ddc = make(chan Device, 1)
	for id := range iic {
		g.Go(func() error {
			d, err := d.Device(ctx, id)
			if err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ddc <- d:
			}
			return nil
		})
	}
	go func() {
		g.Wait()
		close(ddc)
	}()
	// can we make this an iterator?
	var dd = make([]Device, 0, len(ii))
	for d := range ddc {
		dd = append(dd, d)
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return dd, nil
}

// EditFunc
func (d *DB) EditFunc(ctx context.Context, id ID, f func(*Device) error) error {
	// currently this will queue and so will always make a request to the database
	// if the batch is too large. we can reduce this by batching the in-process function call
	// leading to a single get fetch for a slice of workers.
	type Key = struct {
		ID
		Func func(*Device) error
	}
	type Request = batchque.Request[Key, struct{}]
	_, err := d.fnbq.Do(ctx, Key{id, f}, func(ctx context.Context, r []Request) {
		// drop requests if no longer interested
		rc := make(chan Request, 1)
		go func() {
			defer close(rc)
			for _, r := range r {
				select {
				case <-r.Context().Done():
				case rc <- r:
				}
			}
		}()
		// partition by key
		part := make(map[ID][]Request, 1)
		for r := range rc {
			select {
			case <-r.Context().Done():
			default:
				part[r.Val.ID] = append(part[r.Val.ID], r)
			}
		}
		var wg sync.WaitGroup
		for id, r := range part {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// hashque group to be handled now
				// does this need to be locked? yes given we will introduce
				// more fine-grain edits that dont end up dumping the whole
				// back into the database. this will lead to a penality early on
				// but best to not hide the potential complexity
				// get the device
				dev, err := d.Device(ctx, id) // ok to use the parent for all here
				if err != nil {
					// can be in a different process to not block
					// but yields no real benefit
					for _, r := range r {
						r.CancelFunc(err)
					}
					return
				}
				// run the functions, for now in order.
				// but here we can look into the priority queue for ordering
				// create a context that is tied to the requests.
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				var merged []func() bool
				var count atomic.Int64
				for _, r := range r {
					// should only apply changes for clients that
					// are alive, but will need to look at handling
					// that later.
					tmp := dev
					if err := r.Val.Func(&tmp); err != nil {
						r.CancelFunc(err)
						continue
					}
					dev = tmp
					count.Add(1)
					//  If stop returns false, either the context is canceled and f has been started in its own goroutine; or f was already stopped.
					merged = append(merged, context.AfterFunc(ctx, func() {
						debug.Printf("canceled") // not being called.
						if count.Add(-1) == 0 {
							cancel()
						}
					}))
				}
				defer func() {
					for _, stop := range merged {
						stop()
					}
				}()
				// will writing a proper function make a difference.
				// persist the data on the device batch for postgres
				type Request = batchque.Request[Device, struct{}]
				// this context should be bounded by the individual requests.
				_, err = d.efbq.Do(ctx, dev, func(ctx context.Context, r []Request) {
					// drop some dangling requests
					rc := make(chan Request, 1)
					go func() {
						defer close(rc)
						for _, r := range r {
							select {
							case <-r.Context().Done():
							case rc <- r:
							}
						}
					}()
					// collect the remainder
					var (
						rr = make([]Request, 0, len(r))
						dd = make([]Device, 0, len(r))
					)
					for r := range rc {
						select {
						case <-r.Context().Done():
						default:
							rr = append(rr, r)
							dd = append(dd, r.Val)
						}
					}

					var wg sync.WaitGroup
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
							close(r.C)
							// populate the cache here
							// evict if we go over the size?
						}()
					}
					wg.Wait()
					// evict if we go over the size?
				})
				for _, r := range r {
					wg.Add(1)
					go func() {
						defer wg.Done()
						if err != nil {
							r.CancelFunc(err)
						} else {
							close(r.C)
						}
					}()
				}
			}()
		}
		wg.Wait()
	})
	return err
}

func (d *DB) editDevice(ctx context.Context, dev ...Device) iter.Seq[error] {
	const query = `
update iot.device set long = $2, lat = $3, tag = $4, state = $5
where id = $1`
	var b pgx.Batch
	b.QueuedQueries = make([]*pgx.QueuedQuery, len(dev))
	for i := range b.Len() {
		b.QueuedQueries[i] = &pgx.QueuedQuery{
			SQL:       query,
			Arguments: []any{dev[i].ID, dev[i].Loc.Longitude, dev[i].Loc.Latitude, dev[i].Tag, dev[i].State},
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
