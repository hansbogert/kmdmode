package main

import (
	"regexp"
	"slices"
)

// A mode is what cmdmode offers for one recognised command, e.g. "kubectl get": actions rewrite
// it into another command, options toggle one of its flags. Both return the new command words.
type mode struct {
	name    string
	actions []action
	options []option
	up      func() []string // the parent menu's line, or nil at a top menu
	// retype lists another resource type where the line looks (":"), nil if it can't.
	retype  func(typ string) []string
	cluster []string // the line's cluster and identity flags, unquoted, for asking the cluster
	// context sets the line's --context (C); currentContext is the one it names, if any.
	context        func(name string) []string
	currentContext string
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
// it, the options toggle CMD's flags with the kview prefix kept in front, and W, like Esc (it is a
// menu below CMD's), takes the prefix off again. CMD's actions are left out, since they would
// rewrite the line into things like "kview kubectl logs".
func kviewMode(prefix, cmd []string) (mode, bool) {
	inner, ok := kubectlMode(cmd)
	if !ok {
		return mode{}, false
	}
	m := mode{name: "kview " + inner.name}
	m.actions = []action{{"w", "unwatch", func() []string { return cmd }}}
	m.up = func() []string { return cmd }
	m.cluster = inner.cluster
	m.currentContext = inner.currentContext
	m.context = func(name string) []string { return append(slices.Clone(prefix), inner.context(name)...) }
	if inner.retype != nil { // stays watched; -c goes, as columns differ per type
		m.retype = func(typ string) []string { return append([]string{"kview"}, inner.retype(typ)...) }
	}
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
	return spliceTarget(words, func(m mode) []string {
		if m.up == nil {
			return nil
		}
		return m.up()
	})
}

// retype rewrites the target command to list typ, or the built-in type keyed typ (":" menu). A
// single character is only ever a key: no type is that short, so an unlisted one is no type.
func retype(typ string, words []string) ([]string, bool) {
	if len(typ) == 1 {
		i := slices.IndexFunc(resourceKeys, func(r struct{ key, name string }) bool { return r.key == typ })
		if i < 0 {
			return nil, false
		}
		typ = resourceKeys[i].name
	}
	return spliceTarget(words, func(m mode) []string {
		if m.retype == nil {
			return nil
		}
		return m.retype(typ)
	})
}

// setContext rewrites the target command to use kubeconfig context name (C).
func setContext(name string, words []string) ([]string, bool) {
	return spliceTarget(words, func(m mode) []string {
		if m.context == nil {
			return nil
		}
		return m.context(name)
	})
}

// spliceTarget replaces the target command with what rewrite makes of its mode, if anything.
func spliceTarget(words []string, rewrite func(mode) []string) ([]string, bool) {
	start, cmd, m, ok := target(words)
	if !ok {
		return nil, false
	}
	res := rewrite(m)
	if res == nil {
		return nil, false
	}
	out := append([]string{}, words[:start]...)
	out = append(out, res...)
	return append(out, words[start+len(cmd):]...), true
}

// modeFor returns the mode of the command line's target command.
func modeFor(words []string) (mode, bool) {
	_, _, m, ok := target(words)
	return m, ok
}

// run applies the action (or, with toggle, the option) bound to key, case-sensitively (keys are
// lowercase; capitals are kept for the few that must not be hit by accident, C and Y), and
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
			if o.key == key {
				rewrite = o.toggle
			}
		}
	} else {
		for _, a := range m.actions {
			if a.key == key {
				rewrite = a.apply
			}
		}
	}
	if rewrite == nil {
		return nil, false
	}
	res := rewrite()
	if res == nil { // e.g. U on a pod with no owner
		return nil, false
	}
	out := append([]string{}, words[:start]...)
	out = append(out, res...)
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
