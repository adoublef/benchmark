// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iot

import "github.com/google/uuid"

type Device struct {
	ID  ID
	Loc Location
	// Tag AT00000001
}

type ID = uuid.UUID // alias

type Location struct {
	Longitude float64
	Latitude  float64
}
