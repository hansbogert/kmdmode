package main

import (
	"fmt"
	"os"

	fzf "github.com/junegunn/fzf/src"
	"github.com/junegunn/fzf/src/protector"
)

// fzfVersion is the fzf built into kmdmode; kview needs 0.59+ (--wrap, trigger, --listen), newer
// than some distributions ship, so it no longer depends on the one on PATH.
const fzfVersion = "0.74.4"

// runFzf runs the built-in fzf as its own main does: `kmdmode fzf ARGS` behaves like `fzf ARGS`.
func runFzf(args []string) {
	protector.Protect()
	options, err := fzf.ParseOptions(true, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(fzf.ExitError)
	}
	switch {
	case options.Version:
		fmt.Println(fzfVersion + " (built into kmdmode)")
		return
	case options.Help:
		fmt.Print(fzf.Usage)
		return
	}
	code, err := fzf.Run(options)
	if code == fzf.ExitError && err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
