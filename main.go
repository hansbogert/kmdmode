// Command cmdmode rewrites a shell command line into a related command.
//
// The zsh widget splits the buffer with ${(z)BUFFER} and passes the words as arguments, so quotes
// stay literal inside each word and joining with single spaces gives back a valid command line.
//
//	cmdmode hints -- WORD...      print "KEY<TAB>label" per available action
//	cmdmode apply KEY -- WORD...  print the rewritten line; exit 1 if KEY does not apply
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	switch {
	case len(args) >= 2 && args[0] == "hints" && args[1] == "--":
		for _, a := range actionsFor(args[2:]) {
			fmt.Printf("%s\t%s\n", a.key, a.label)
		}
	case len(args) >= 3 && args[0] == "apply" && args[2] == "--":
		out, ok := apply(args[1], args[3:])
		if !ok {
			os.Exit(1)
		}
		fmt.Println(strings.Join(out, " "))
	default:
		fmt.Fprintln(os.Stderr, "usage: cmdmode hints -- WORD... | cmdmode apply KEY -- WORD...")
		os.Exit(2)
	}
}
