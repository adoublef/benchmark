// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"encoding/json"
	"net/http"

	"go.adoublef.dev/runtime/debug"
)

func respond[V any](w http.ResponseWriter, _ *http.Request, v V, code int) {
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		debug.Printf(`%v = json.NewEncoder(w).Encode(%#v)`, err, v)
	}
}
