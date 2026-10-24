// Command kmdmode rewrites a shell command line into a related command.
//
// The zsh widget splits the buffer with ${(z)BUFFER} and passes the words as arguments, so quotes
// stay literal inside each word and joining with single spaces gives back a valid command line.
//
//	kmdmode menu -- WORD...        print the mode and its actions and options, tab-separated:
//	                               "mode NAME", "action KEY LABEL", "option KEY LABEL on|off",
//	                               "up" when it has a parent menu, "nocontext" when the line
//	                               names no --context, "verb IDX" for the index of the verb word;
//	                               actions that change the verb end in a "verb" field
//	kmdmode apply KEY -- WORD...   print the line after action KEY; exit 1 if it does not apply
//	kmdmode toggle KEY -- WORD...  print the line after toggling option KEY; exit 1 likewise
//	kmdmode up -- WORD...          print the line of the parent menu; exit 1 at a top menu
//	kmdmode retype TYPE|KEY -- WORD...  print the line listing another resource type (":" menu)
//	kmdmode api-resources -- WORD...    print every listable resource type of the line's cluster
//	kmdmode contexts -- WORD...    print the context the line uses, then every kubeconfig context
//	kmdmode context NAME -- WORD...  print the line using context NAME (C)
//	kmdmode pick -- WORD...        print a command for the row on stdin, picked from WORD's output
//	kmdmode select -- WORD...      like pick, for stdin's row then label lines: a get by selector
//	kmdmode faults                 filter the kubectl table on stdin to its unhealthy rows
//	kmdmode columns COL,...        keep only these columns (and NAMESPACE, NAME) of that table
//	kmdmode columns --list [COL,...]  print its columns as "NAME on|off" for a picker
//	kmdmode order name             sort that table's rows by namespace and name (kubectl's order)
//	kmdmode init zsh               print the zsh plugin, for `source <(kmdmode init zsh)` in .zshrc
//	kmdmode kview [-c COL,...] CMD...  the live view (the plugin's kview runs this)
//	kmdmode glue                   print the version of the embedded zsh code
//	kmdmode version                print the release version
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
	case len(args) == 2 && args[0] == "init" && args[1] == "zsh":
		initZsh()
	case len(args) >= 1 && args[0] == "kview":
		if err := runKview(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case len(args) == 1 && args[0] == "glue":
		fmt.Println(glueVersion())
	case len(args) == 1 && (args[0] == "version" || args[0] == "--version"):
		fmt.Println(releaseVersion())
	case len(args) >= 2 && args[0] == "menu" && args[1] == "--":
		m, ok := modeFor(args[2:])
		if !ok {
			return
		}
		fmt.Printf("mode\t%s\n", m.name)
		for _, a := range m.actions {
			if verbKeys[a.key] {
				fmt.Printf("action\t%s\t%s\tverb\n", a.key, a.label)
			} else {
				fmt.Printf("action\t%s\t%s\n", a.key, a.label)
			}
		}
		if i, ok := verbWord(args[2:]); ok {
			fmt.Printf("verb\t%d\n", i)
		}
		for _, o := range m.options {
			fmt.Printf("option\t%s\t%s\t%s\n", o.key, o.label, map[bool]string{true: "on", false: "off"}[o.on])
		}
		if m.up != nil {
			fmt.Println("up")
		}
		if m.context != nil { // C, case-sensitive: the zsh side runs the picker
			fmt.Println("action\tC\tcontext")
			if m.currentContext == "" {
				fmt.Println("nocontext") // so entering kmdmode asks for one first
			}
		}
		if m.retype != nil {
			for _, r := range resourceKeys {
				fmt.Printf("resource\t%s\t%s\n", r.key, r.name)
			}
			fmt.Printf("resource\t.\tany…\n")
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
	case len(args) >= 3 && args[0] == "retype" && args[2] == "--":
		out, ok := retype(args[1], args[3:])
		if !ok {
			os.Exit(1)
		}
		fmt.Println(strings.Join(out, " "))
	case len(args) >= 2 && args[0] == "api-resources" && args[1] == "--":
		m, ok := modeFor(args[2:])
		if !ok {
			os.Exit(1)
		}
		// Discovery lists built-ins and every CRD the apiextensions API has registered; -o name
		// qualifies the latter (certificates.cert-manager.io).
		out, err := runKubectl(append(m.cluster, "--request-timeout=10s", "api-resources", "--verbs=list", "-o", "name"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(out)
	case len(args) >= 3 && args[0] == "context" && args[2] == "--":
		out, ok := setContext(args[1], args[3:])
		if !ok {
			os.Exit(1)
		}
		fmt.Println(strings.Join(out, " "))
	case len(args) >= 2 && args[0] == "contexts" && args[1] == "--":
		m, ok := modeFor(args[2:])
		if !ok {
			os.Exit(1)
		}
		current := m.currentContext
		if current == "" {
			out, _ := runKubectl(append(m.cluster, "config", "current-context"))
			current = strings.TrimSpace(out)
		}
		names, err := runKubectl(append(m.cluster, "config", "get-contexts", "-o", "name"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(current)
		fmt.Print(names)
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
	case len(args) == 2 && args[0] == "order" && args[1] == "name":
		if err := orderByName(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case len(args) == 1 && args[0] == "faults":
		if err := faults(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: kmdmode init zsh | kmdmode kview CMD... | kmdmode version")
		fmt.Fprintln(os.Stderr, "internal, for the zsh plugin: kmdmode menu|up|api-resources|contexts|pick|select -- WORD... | kmdmode retype TYPE|context NAME -- WORD... | kmdmode apply|toggle KEY -- WORD... | kmdmode faults | kmdmode columns [--list] COL,... | kmdmode order name")
		os.Exit(2)
	}
}
