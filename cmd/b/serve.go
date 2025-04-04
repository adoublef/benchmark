// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os/signal"
	"time"

	"github.com/adoublef/benchmark/internal/iot"
	"github.com/adoublef/benchmark/internal/net/http"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

type serve struct {
	http struct {
		addr            string        // --http-address
		gracefulTimeout time.Duration // HTTP_GRACEFUL_TIMEOUT
	}
	db struct {
		url string // --database-url
	}
	logLevel slog.Level // debug or info
}

func (c *serve) parse(args []string, getenv func(string) string, stdin io.Reader) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.StringVar(&c.http.addr, "http-address", ":8080", "http address")
	fs.StringVar(&c.db.url, "database-url", "", "database url") // cannot be empty
	// debug
	debug := fs.Bool("debug", false, "debug mode")
	err := fs.Parse(args)
	if err != nil {
		return err
	} else if fs.NArg() > 0 {
		fs.Usage()
		return flag.ErrHelp
	}
	if *debug {
		c.logLevel = slog.LevelDebug
	} else {
		c.logLevel = slog.LevelInfo
	}
	c.http.gracefulTimeout, err = time.ParseDuration(cmp.Or(getenv("HTTP_GRACEFUL_TIMEOUT"), "5s"))
	if err != nil {
		return err
	}
	return nil
}

func (c *serve) run(ctx context.Context, stderr, stdout io.Writer) error {
	// create a logger that will be used on start up.
	// note: this may be needed throughout the application too.
	log := newLogger(stdout, c.logLevel)

	ctx, cancel := signal.NotifyContext(ctx, unix.SIGINT, unix.SIGKILL, unix.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, c.db.url)
	if err != nil {
		return fmt.Errorf("cannot connect to postgres: %w", err)
	}
	defer pool.Close() // can be added to a group cleanup function

	var (
		iotdb = &iot.DB{RWC: pool}
	)

	s := &http.Server{
		Addr:        c.http.addr,
		BaseContext: func(l net.Listener) context.Context { return ctx },
		Handler:     http.Handler(iotdb),
	}
	s.RegisterOnShutdown(cancel)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() (err error) {
		log.Printf("Http server listening at address %s", s.Addr)
		if s.TLSConfig == nil {
			err = s.ListenAndServe()
		} else {
			err = s.ListenAndServeTLS("", "")
		}
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		} else if err != nil {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), c.http.gracefulTimeout)
		defer cancel()

		return s.Shutdown(ctx)
	})

	return g.Wait()
}

func newLogger(w io.Writer, logLevel slog.Level) *log.Logger {
	o := &slog.HandlerOptions{
		Level: logLevel,
	}
	return slog.NewLogLogger(slog.NewTextHandler(w, o), logLevel)
}
