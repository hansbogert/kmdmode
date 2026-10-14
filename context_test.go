package main

import (
	"strings"
	"testing"
)

func TestSetContext(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"prod", "kubectl get -n ns pods", "kubectl --context prod get -n ns pods"},
		{"prod", "kubectl get pods --context test -o wide", "kubectl get pods --context prod -o wide"},
		{"prod", "kubectl logs --context=test p", "kubectl logs --context prod p"},
		{"prod", "kview -c STATUS kubectl get pods -A", "kview -c STATUS kubectl --context prod get pods -A"},
		{"prod", "kubectl -n ns", "kubectl --context prod -n ns"},
		{"prod", "ls", ""},
	}
	for _, c := range cases {
		out, ok := setContext(c.name, strings.Fields(c.in))
		check(t, "C", c.in, c.want, out, ok)
	}
}

func TestContextKeyIsCaseSensitive(t *testing.T) {
	// C is handled by the zsh picker, not as an action, so lowercase c applies nothing.
	if _, ok := run("c", false, strings.Fields("kubectl get pods")); ok {
		t.Error("lowercase c applied an action")
	}
	m, _ := modeFor(strings.Fields("kubectl get pods --context=test"))
	if m.currentContext != "test" {
		t.Errorf("currentContext: %q", m.currentContext)
	}
}
