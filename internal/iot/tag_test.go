// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iot_test

import (
	"testing"

	. "github.com/adoublef/benchmark/internal/iot"
	"go.adoublef.dev/testing/is"
)

func TestParseTag(t *testing.T) {
	type testcase struct {
		s         string
		expectErr bool
	}
	tm := map[string]testcase{
		"Number":           {"1", true},
		"NumberMany":       {"12345678", false},
		"LetterMany":       {"abcdefgh", true},
		"LetterManyNumber": {"abcdefg1", false},
		"UpperManyNumber":  {"abcdefG1", true},
		"Ascii":            {"αβγ123", true},
	}
	for name, tc := range tm {
		t.Run(name, func(t *testing.T) {
			_, err := ParseTag(tc.s)
			is.True(t, (err != nil) == tc.expectErr)
		})
	}
}
