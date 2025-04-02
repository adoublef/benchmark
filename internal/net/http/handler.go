// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import "net/http"

type (
	Server = http.Server
)

var (
	ErrServerClosed = http.ErrServerClosed
)

func Handler() http.Handler {
	mux := http.NewServeMux()
	handleFunc := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, h)
	}

	handleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	return mux
}
