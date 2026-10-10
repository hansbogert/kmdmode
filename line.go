package main

import (
	"regexp"
	"strings"
)

// An action rewrites one simple command (no env prefix, no operators) into another.
type action struct {
	key   string
	label string
	apply func(cmd []string) []string
}

// actionsFor returns every rule's actions for one simple command.
func actionsFor(words []string) []action {
	_, cmd, ok := target(words)
	if !ok {
		return nil
	}
	return commandActions(cmd)
}

func commandActions(cmd []string) []action {
	return kubectlActions(cmd)
}

// apply runs the action bound to key (case-insensitively) on the target command and splices the
// result back into the full line, leaving pipes, other commands and env assignments untouched.
func apply(key string, words []string) ([]string, bool) {
	start, cmd, ok := target(words)
	if !ok {
		return nil, false
	}
	for _, a := range commandActions(cmd) {
		if strings.EqualFold(a.key, key) {
			out := append([]string{}, words[:start]...)
			out = append(out, a.apply(cmd)...)
			return append(out, words[start+len(cmd):]...), true
		}
	}
	return nil, false
}

var operators = map[string]bool{
	";": true, "&&": true, "||": true, "|": true, "|&": true, "&": true, "&!": true, "&|": true,
}

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// target picks the last simple command on the line that has any action, since that is usually
// the one being typed. It returns the command's start index and its words.
func target(words []string) (int, []string, bool) {
	end := len(words)
	for i := len(words); i >= 0; i-- {
		if i > 0 && !operators[words[i-1]] {
			continue
		}
		start := i
		for start < end && assignment.MatchString(words[start]) {
			start++
		}
		if cmd := words[start:end]; len(cmd) > 0 && len(commandActions(cmd)) > 0 {
			return start, cmd, true
		}
		end = i - 1
	}
	return 0, nil, false
}
