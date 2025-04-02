// Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

type cmd interface {
	parse(args []string, getenv func(string) string, stdin io.Reader) error
	run(ctx context.Context, stderr, stdout io.Writer) error
}

func main() {
	var (
		ctx    = context.Background()
		args   = os.Args[1:]
		getenv = os.Getenv
		stdin  = os.Stdin
		stderr = os.Stderr
		stdout = os.Stdout
	)

	err := run(ctx, args, getenv, stdin, stderr, stdout)
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(2)
	} else if err != nil {
		fmt.Fprintf(stderr, "[ERR]: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, stdin io.Reader, stderr, stdout io.Writer) error {
	var arg string
	if len(args) > 0 {
		arg, args = args[0], args[1:]
	}
	cmds := map[string]cmd{
		"serve": &serve{},
	}
	c, ok := cmds[arg]
	if !ok {
		if arg == "" {
			return fmt.Errorf("missing command")
		}
		return fmt.Errorf("unknown command: %s", arg)
	}
	err := c.parse(args, getenv, stdin)
	if err != nil {
		return err
	}
	return c.run(ctx, stderr, stdout)
}
