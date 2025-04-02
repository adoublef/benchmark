package iotas

import (
	"errors"
	"reflect"
	"strings"
)

// match reports whether s1 and s2 match ignoring case.
// It is assumed s1 and s2 are the same length.
func match(s1, s2 string) bool {
	for i := 0; i < len(s1); i++ {
		c1 := s1[i]
		c2 := s2[i]
		if c1 != c2 {
			// Switch to lower-case; 'a'-'A' is known to be a single bit.
			c1 |= 'a' - 'A'
			c2 |= 'a' - 'A'
			if c1 != c2 || c1 < 'a' || c1 > 'z' {
				return false
			}
		}
	}
	return true
}

// Lookup
func Lookup(tab []string, val string) (int, string, error) {
	for i, v := range tab {
		if len(val) >= len(v) && match(val[0:len(v)], v) {
			return i, val[len(v):], nil
		}
	}
	return -1, val, errors.New("bad value for field")
}

// Parse
func Parse[T ~uint8](tab []string, val string, offset int) (T, error) {
	c, _, err := Lookup(tab, val)
	if err != nil {
		return 0, errors.New("bad value")
	}
	return T(c + offset), nil
}

// Format formats v into the tail of buf.
// It returns the string where the output begins.
func Format[T ~uint8](buf []byte, v T) string {
	if buf == nil {
		buf = make([]byte, 20)
	}
	w := len(buf)
	if v == 0 {
		w--
		buf[w] = '0'
	} else {
		for v > 0 {
			w--
			buf[w] = byte(v%10) + '0'
			v /= 10
		}
	}
	t := reflect.TypeOf(v).Name()
	if dot := strings.LastIndex(t, "."); dot != -1 {
		t = t[dot+1:]
	}
	return "%!" + t + "(" + string(buf[w:]) + ")"
}
