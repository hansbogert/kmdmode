package main

import (
	"regexp"
	"slices"
	"strings"
)

// A mode is what cmdmode offers for one recognised command, e.g. "kubectl get": actions rewrite
// it into another command, options toggle one of its flags. Both return the new command words.
type mode struct {
	name    string
	actions []action
	options []option
	up      func() []string // the parent menu's line, or nil at a top menu
}

type action struct {
	key   string
	label string
	apply func() []string
}

type option struct {
	key    string
	label  string
	on     bool
	toggle func() []string
}

func commandMode(cmd []string) (mode, bool) {
	if len(cmd) > 1 && cmd[0] == "kview" {
		if len(cmd) > 3 && cmd[1] == "-c" {
			return kviewMode(cmd[:3], cmd[3:])
		}
		return kviewMode(cmd[:1], cmd[1:])
	}
	return kubectlMode(cmd)
}

// kviewMode is the mode of "kview [-c COLS] CMD", as W, O and kview itself write it: Enter runs
// it, the options toggle CMD's flags with the kview prefix kept in front, and W takes the prefix
// off again. CMD's actions are left out, since they would rewrite the line into things like
// "kview kubectl logs".
func kviewMode(prefix, cmd []string) (mode, bool) {
	inner, ok := kubectlMode(cmd)
	if !ok {
		return mode{}, false
	}
	m := mode{name: "kview " + inner.name}
	m.actions = []action{{"W", "unwatch", func() []string { return cmd }}}
	for _, o := range inner.options {
		toggle := o.toggle
		o.toggle = func() []string { return append(slices.Clone(prefix), toggle()...) }
		m.options = append(m.options, o)
	}
	return m, true
}

// up returns the line of the target command's parent menu (e.g. a rollout line's get), spliced
// into the full line like run's result.
func up(words []string) ([]string, bool) {
	start, cmd, m, ok := target(words)
	if !ok || m.up == nil {
		return nil, false
	}
	out := append([]string{}, words[:start]...)
	out = append(out, m.up()...)
	return append(out, words[start+len(cmd):]...), true
}

// modeFor returns the mode of the command line's target command.
func modeFor(words []string) (mode, bool) {
	_, _, m, ok := target(words)
	return m, ok
}

// run applies the action (or, with toggle, the option) bound to key, case-insensitively, and
// splices the result back into the full line, leaving pipes, other commands and env assignments
// untouched.
func run(key string, toggle bool, words []string) ([]string, bool) {
	start, cmd, m, ok := target(words)
	if !ok {
		return nil, false
	}
	var rewrite func() []string
	if toggle {
		for _, o := range m.options {
			if strings.EqualFold(o.key, key) {
				rewrite = o.toggle
			}
		}
	} else {
		for _, a := range m.actions {
			if strings.EqualFold(a.key, key) {
				rewrite = a.apply
			}
		}
	}
	if rewrite == nil {
		return nil, false
	}
	out := append([]string{}, words[:start]...)
	out = append(out, rewrite()...)
	return append(out, words[start+len(cmd):]...), true
}

var operators = map[string]bool{
	";": true, "&&": true, "||": true, "|": true, "|&": true, "&": true, "&!": true, "&|": true,
}

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// target picks the last simple command on the line that has a mode, since that is usually the
// one being typed. It returns the command's start index, its words and its mode.
func target(words []string) (int, []string, mode, bool) {
	end := len(words)
	for i := len(words); i >= 0; i-- {
		if i > 0 && !operators[words[i-1]] {
			continue
		}
		start := i
		for start < end && assignment.MatchString(words[start]) {
			start++
		}
		if cmd := words[start:end]; len(cmd) > 0 {
			if m, ok := commandMode(cmd); ok {
				return start, cmd, m, true
			}
		}
		end = i - 1
	}
	return 0, nil, mode{}, false
}
