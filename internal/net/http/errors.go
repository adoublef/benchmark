// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"fmt"
	"net/http"

	"go.adoublef.dev/runtime/debug"
)

// Error replies to the request with the specified error.
// If err implements [statusHandler] we use it's [statusHandler.ServeHTTP] method instead.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	debug.Printf("net/http: %T, %v = err", err, err)
	if sh, ok := err.(statusHandler); ok {
		sh.ServeHTTP(w, r)
		return
	}
	sh := statusHandler{code: http.StatusInternalServerError}
	sh.ServeHTTP(w, r)
}

func newErr(code int, format string, v ...any) error {
	return statusHandler{code: code, s: fmt.Sprintf(format, v...)}
}

type statusHandler struct {
	code int
	s    string
}

func (h statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.code < 300 {
		w.WriteHeader(h.code)
		w.Write([]byte(http.StatusText(h.code)))
		return
	}
	if h.code < 400 {
		http.Redirect(w, r, h.s, h.code)
		return
	}
	http.Error(w, h.s, h.code)
}

func (h statusHandler) Error() string {
	return fmt.Sprintf("%d: %s", h.code, h.s)
}

func (h statusHandler) String() string {
	return h.s
}
