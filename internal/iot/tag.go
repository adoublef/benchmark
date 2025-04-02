// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iot

import (
	"database/sql/driver"
	"fmt"
)

func (n Tag) String() string   { return string(n) }
func (t Tag) GoString() string { return `Tag("` + string(t) + `")` }

// Tag are unique identifiers. Formatted values using
// lowercase letters and integers.
type Tag string

// Scan implements sql.Scanner interface
func (t *Tag) Scan(value any) (err error) {
	if value == nil {
		*t = ""
		return nil
	}
	switch v := value.(type) {
	case string:
		*t, err = ParseTag(v)
	default:
		return fmt.Errorf("cannot scan type %T into iot.Tag", value)
	}
	return err
}

// Value implements driver.Valuer interface
func (t Tag) Value() (driver.Value, error) {
	if len(t) == 0 {
		return nil, nil
	}
	return t.String(), nil
}

func (t Tag) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t *Tag) UnmarshalText(text []byte) (err error) {
	*t, err = ParseTag(string(text))
	if err != nil {
		return fmt.Errorf("cannot unmarshal into iot.Tag: %w", err)
	}
	return nil
}

func ParseTag(s string) (Tag, error) {
	// format size references:
	// - [8-40](https://en.wikipedia.org/wiki/Tracking_number.)
	// - [9-27](https://www.postoffice.co.uk/track-trace)
	if n := len(s); n < 8 || n > 40 {
		return "", fmt.Errorf("invalid length %d: %w", n, ErrInvalid)
	}
	// must contain an integer
	// cannot have letters following an integer
	var n bool
	for i, r := range s {
		// is letter? has a number already been spotted?
		if r >= '0' && r <= '9' {
			n = true
			continue
		}
		if r >= 'a' && r <= 'z' && !n {
			continue
		}
		return "", fmt.Errorf("invalid character %q at position %d: %w", r, i, ErrInvalid)
	}
	if !n { // no number exists
		return "", fmt.Errorf("number not found: %w", ErrInvalid)
	}
	return Tag(s), nil
}
