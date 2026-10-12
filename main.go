// Command cmdmode rewrites a shell command line into a related command.
//
// The zsh widget splits the buffer with ${(z)BUFFER} and passes the words as arguments, so quotes
// stay literal inside each word and joining with single spaces gives back a valid command line.
//
//	cmdmode menu -- WORD...        print the mode and its actions and options, tab-separated:
//	                               "mode NAME", "action KEY LABEL", "option KEY LABEL on|off",
//	                               and "up" when it has a parent menu
//	cmdmode apply KEY -- WORD...   print the line after action KEY; exit 1 if it does not apply
//	cmdmode toggle KEY -- WORD...  print the line after toggling option KEY; exit 1 likewise
//	cmdmode up -- WORD...          print the line of the parent menu; exit 1 at a top menu
//	cmdmode pick -- WORD...        print a command for the row on stdin, picked from WORD's output
//	cmdmode select -- WORD...      like pick, for stdin's row then label lines: a get by selector
//	cmdmode faults                 filter the kubectl table on stdin to its unhealthy rows
//	cmdmode columns COL,...        keep only these columns (and NAMESPACE, NAME) of that table
//	cmdmode columns --list [COL,...]  print its columns as "NAME on|off" for a picker
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
		if m.up != nil {
			fmt.Println("up")
		}
	case len(args) >= 3 && (args[0] == "apply" || args[0] == "toggle") && args[2] == "--":
		out, ok := run(args[1], args[0] == "toggle", args[3:])
		if !ok {
			os.Exit(1)
		}
		fmt.Println(strings.Join(out, " "))
	case len(args) >= 2 && args[0] == "up" && args[1] == "--":
		out, ok := up(args[2:])
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
	case len(args) >= 2 && args[0] == "select" && args[1] == "--":
		in, _ := io.ReadAll(os.Stdin)
		row, labels, _ := strings.Cut(string(in), "\n")
		out, ok := kubectlSelect(args[2:], row, strings.Fields(labels))
		if !ok {
			os.Exit(1)
		}
		fmt.Println(strings.Join(out, " "))
	case len(args) >= 2 && args[0] == "columns" && args[1] == "--list":
		var current []string
		if len(args) > 2 && args[2] != "" {
			current = strings.Split(args[2], ",")
		}
		if err := listColumns(os.Stdin, os.Stdout, current); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case len(args) == 2 && args[0] == "columns":
		if err := columns(os.Stdin, os.Stdout, strings.Split(args[1], ",")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case len(args) == 1 && args[0] == "faults":
		if err := faults(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: cmdmode menu|up|pick|select -- WORD... | cmdmode apply|toggle KEY -- WORD... | cmdmode faults | cmdmode columns [--list] COL,...")
		os.Exit(2)
	}
}
