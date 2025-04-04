// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// source - https://hub.docker.com/r/postgis/postgis/tags
package postgis

import (
	"cmp"
	"context"
	"embed"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.adoublef.dev/net/nettest"
	"maragu.dev/migrate"
)

// https://testcontainers.com/modules/postgis/?language=go
const DefaultImage = "postgis/postgis@sha256:a941c72f9e58dda53184bab8d58de20d6501bb8df9cad47776d6f4f1010334ea"

type (
	Pool      = pgxpool.Pool
	Container struct{ *postgres.PostgresContainer }
)

// Run a new [Container] instance for postgres.
func Run(ctx context.Context, image string) (*Container, error) {
	c, err := postgres.Run(ctx,
		cmp.Or(image, DefaultImage),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second)),
	)
	return &Container{c}, err
}

// New creates a new [Pool].
func (c *Container) Pool(ctx context.Context, name string) (*pgxpool.Pool, Migrator, *nettest.Proxy, error) {
	uri, err := c.ConnectionString(ctx) // application_name
	if err != nil {
		return nil, nil, nil, err
	}
	// postgres://postgres:postgres@127.0.0.1:58859/postgres
	u, err := url.Parse(uri)
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

// cleanup

//go:embed all:*.sql
var embedFS embed.FS

type Migrator interface {
	MigrateDown(ctx context.Context) (err error)
	MigrateUp(ctx context.Context) (err error)
}
