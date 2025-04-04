// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"cmp"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/adoublef/benchmark/internal/iot"
	"go.adoublef.dev/xiter"
	olog "go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

const scopeName = "github.com/adoublef/iot/internal/net/http"

var (
	tracer = otel.Tracer(scopeName)
	_      = otel.Meter(scopeName)
	_      = olog.NewLogger(scopeName)
)

const (
	DefaultReadTimeout    = 100 * time.Second // Cloudflare's default read request timeout of 100s
	DefaultWriteTimeout   = 30 * time.Second  // Cloudflare's default write request timeout of 30s
	DefaultIdleTimeout    = 900 * time.Second // Cloudflare's default write request timeout of 900s
	DefaultMaxHeaderBytes = 32 * (1 << 10)
	DefaultMaxBytes       = 1 << 20 // Cloudflare's free tier limits of 100mb
)

type (
	Server = http.Server
	Client = http.Client
)

var (
	ErrServerClosed = http.ErrServerClosed
)

func Handler(iotdb *iot.DB) http.Handler {
	mux := http.NewServeMux()
	handle := func(pattern string, h http.Handler) {
		h = otelhttp.WithRouteTag(pattern, h)
		mux.Handle(pattern, h)
	}

	handle("GET /ping", statusHandler{code: 204})
	handle("POST /devices", handleRegister(iotdb))
	handle("GET /devices", handleSearch(iotdb))

	h := otelhttp.NewHandler(mux, "Http")
	return h
}

func handleRegister(iotdb *iot.DB) http.HandlerFunc {
	type request struct {
		Tag  *iot.Tag `json:"assetTag"`
		Long float64  `json:"longitude"`
		Lat  float64  `json:"latitude"`
	}
	type response struct {
		ID iot.ID `json:"deviceId"`
	}
	parse := func(w http.ResponseWriter, r *http.Request) (iot.Tag, iot.Location, error) {
		v, err := Decode[request](w, r, 0, 0)
		if err != nil {
			return "", iot.Location{}, err
		}
		if v.Tag == nil {
			return "", iot.Location{}, newErr(http.StatusBadRequest, "assetTag is required")
		}
		return *v.Tag, iot.Location{Longitude: v.Long, Latitude: v.Lat}, nil
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "http.register")
		defer span.End()

		tag, loc, err := parse(w, r)
		if err != nil {
			Error(w, r, err)
			return
		}

		de, err := iotdb.Pin(ctx, loc, tag)
		if err != nil {
			Error(w, r, err)
			return
		}
		respond(w, r, response{de}, http.StatusCreated)
	}
}

func handleSearch(iotdb *iot.DB) http.HandlerFunc {
	type device struct {
		ID  iot.ID       `json:"deviceId"`
		Tag iot.Tag      `json:"assetTag"`
		Loc iot.Location `json:"location"`
	}
	type response struct {
		Devices []device `json:"devices,omitempty"`
	}
	parse := func(_ http.ResponseWriter, r *http.Request) (radius float64, loc iot.Location, err error) {
		{
			query := cmp.Or(r.URL.Query().Get("r"), "0") // should impose a size?
			radius, err = strconv.ParseFloat(query, 64)
			if err != nil {
				return
			}
		}
		{
			query := cmp.Or(r.URL.Query().Get("long"), "0")
			loc.Longitude, err = strconv.ParseFloat(query, 64)
			if err != nil {
				return
			}
		}
		{
			query := cmp.Or(r.URL.Query().Get("lat"), "0")
			loc.Latitude, err = strconv.ParseFloat(query, 64)
			if err != nil {
				return
			}
		}
		return
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "http.search")
		defer span.End()

		d, loc, err := parse(w, r)
		if err != nil {
			Error(w, r, err)
			return
		}

		dd, err := iotdb.Radius(ctx, loc, d)
		if err != nil {
			Error(w, r, err)
			return
		}

		devs := xiter.Reduce(func(sum []device, dev iot.Device) []device {
			d := device{ID: dev.ID, Tag: dev.Tag, Loc: dev.Loc}
			return append(sum, d)
		}, []device(nil), slices.Values(dd))
		respond(w, r, response{devs}, http.StatusOK)
	}
}
