// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iot

import (
	"database/sql/driver"
	"fmt"

	"github.com/adoublef/benchmark/internal/iotas"
	"github.com/google/uuid"
)

type Device struct {
	ID    ID
	Loc   Location
	Tag   Tag // Tag AT00000001
	State State
}

type ID = uuid.UUID // alias

type Location struct {
	Longitude float64
	Latitude  float64
}

func (st State) String() string {
	if Created <= st && st <= Created {
		return devStates[st]
	}
	return iotas.Format(nil, st)
}

// - [Fly Machines](https://fly.io/docs/machines/machine-states/)
// - [Fly Volumes](https://fly.io/docs/volumes/volume-states/)
type State uint8

const (
	Created State = iota // Initial state for a new [Device].
)

// Scan implements sql.Scanner interface
func (st *State) Scan(value any) (err error) {
	if value == nil {
		*st = 0
		return nil
	}
	switch v := value.(type) {
	case string:
		*st, err = ParseState(v)
	default:
		return fmt.Errorf("cannot scan type %T into iot.Status", value)
	}
	return err
}

// Value implements driver.Valuer interface
func (st State) Value() (driver.Value, error) {
	// or use binary values instead?
	return st.String(), nil
}

func (st *State) UnmarshalText(p []byte) (err error) {
	*st, err = ParseState(string(p))
	if err != nil {
		return fmt.Errorf("cannot unmarshal into iot.State: %w", err)
	}
	return
}

func (st State) MarshalText() ([]byte, error) {
	return []byte(st.String()), nil
}

func ParseState(s string) (State, error) {
	return iotas.Parse[State](devStates, s, 0)
}

var devStates = []string{
	"Created",
}
