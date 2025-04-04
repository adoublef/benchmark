// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// source - https://hub.docker.com/r/cockroachdb/cockroach/tags
package cockroach

import (
	"cmp"
	"context"
	"embed"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go/modules/cockroachdb"
	"go.adoublef.dev/net/nettest"
	"maragu.dev/migrate"
)

const DefaultImage = "cockroachdb/cockroach@sha256:89863535ea38412901f0dc0a91e86155e273e3e3098f4ed9b5b9d460da0dc7d7"

type (
	Pool      = pgxpool.Pool
	Container struct {
		*cockroachdb.CockroachDBContainer
	}
)

// Run a new [Container] instance for postgres.
func Run(ctx context.Context, image string) (*Container, error) {
	c, err := cockroachdb.Run(ctx,
		cmp.Or(image, DefaultImage),
		cockroachdb.WithInsecure(),
	)
	return &Container{c}, err
}

// New creates a new [Pool].
func (c *Container) Pool(ctx context.Context, name string) (*pgxpool.Pool, Migrator, *nettest.Proxy, error) {
	cf, err := c.ConnectionConfig(ctx) // application_name
	if err != nil {
		return nil, nil, nil, err
	}
	// postgres://postgres:postgres@127.0.0.1:58859/postgres
	u, err := url.Parse(cf.ConnString())
	if err != nil {
		return nil, nil, nil, err
	}
	// connect via the proxy
	proxy := nettest.NewProxy(name, u.Host)
	u.Host = proxy.Listen()
	// debug.Printf("%q, %q = uri, proxy", uri, proxy.Listen())
	p, err := pgxpool.New(ctx, u.String())
	if err != nil {
		return nil, nil, nil, err
	}
	d := stdlib.OpenDBFromPool(p)
	m := migrate.New(migrate.Options{DB: d, FS: embedFS})
	return p, m, proxy, nil
}

//go:embed all:*.sql
var embedFS embed.FS

type Migrator interface {
	MigrateDown(ctx context.Context) (err error)
	MigrateUp(ctx context.Context) (err error)
}
