// Command cmdmode rewrites a shell command line into a related command.
//
// The zsh widget splits the buffer with ${(z)BUFFER} and passes the words as arguments, so quotes
// stay literal inside each word and joining with single spaces gives back a valid command line.
//
//	cmdmode menu -- WORD...        print the mode and its actions and options, tab-separated:
//	                               "mode NAME", "action KEY LABEL", "option KEY LABEL on|off"
//	cmdmode apply KEY -- WORD...   print the line after action KEY; exit 1 if it does not apply
//	cmdmode toggle KEY -- WORD...  print the line after toggling option KEY; exit 1 likewise
//	cmdmode pick -- WORD...        print a command for the row on stdin, picked from WORD's output
//	cmdmode faults                 filter the kubectl table on stdin to its unhealthy rows
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	switch {
	case len(args) >= 2 && args[0] == "menu" && args[1] == "--":
		m, ok := modeFor(args[2:])
		if !ok {
			return
		}
		fmt.Printf("mode\t%s\n", m.name)
		for _, a := range m.actions {
			fmt.Printf("action\t%s\t%s\n", a.key, a.label)
		}
		for _, o := range m.options {
			fmt.Printf("option\t%s\t%s\t%s\n", o.key, o.label, map[bool]string{true: "on", false: "off"}[o.on])
		}
	case len(args) >= 3 && (args[0] == "apply" || args[0] == "toggle") && args[2] == "--":
		out, ok := run(args[1], args[0] == "toggle", args[3:])
		if !ok {
			os.Exit(1)
		}
		fmt.Println(strings.Join(out, " "))
	case len(args) >= 2 && args[0] == "pick" && args[1] == "--":
		row, _ := io.ReadAll(os.Stdin)
		out, ok := kubectlPick(args[2:], string(row))
		if !ok {
			os.Exit(1)
		}
		fmt.Println(strings.Join(out, " "))
	case len(args) == 1 && args[0] == "faults":
		if err := faults(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: cmdmode menu|pick -- WORD... | cmdmode apply|toggle KEY -- WORD... | cmdmode faults")
		os.Exit(2)
	}
}
