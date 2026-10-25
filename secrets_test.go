package main

import (
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	dec := []string{"-o", "'" + decodeTemplate + "'"} // as zsh splits it: one word
	words := func(line string, extra ...string) []string { return append(strings.Fields(line), extra...) }
	for _, c := range []struct {
		in   []string
		want string
	}{
		{words("kubectl get -n ns secret db"), "kubectl get -n ns secret db " + strings.Join(dec, " ")},
		{words("kubectl describe -n ns secrets/db"), "kubectl get -n ns secrets/db " + strings.Join(dec, " ")},
		{words("kubectl get secret db -o yaml"), "kubectl get secret db " + strings.Join(dec, " ")},
		{words("kubectl get secret db", dec...), "kubectl get secret db"}, // v again: back
		{words("kubectl get secrets"), ""}, // a list: alt-d in kview instead
		{words("kubectl get pod p"), ""},
		{words("kubectl get secret db -l app=web"), ""},
	} {
		out, ok := run("v", false, c.in)
		if got := strings.Join(out, " "); got != c.want || ok != (c.want != "") {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestKviewLayout(t *testing.T) {
	for in, want := range map[string]string{
		"kubectl get pods -A -o wide":   "table",
		"kubectl get secrets -A":        "table decode",
		"kubectl get secret db":         "table decode",
		"kubectl get pod p -o yaml":     "document",
		"kubectl get secret db -o yaml": "document decode",
		"kubectl describe secret db":    "document decode",
		"kubectl get secret db -o name": "document decode",
		"kubectl get":                   "table",
		"zsh -c date":                   "document",
	} {
		if got := strings.Join(kviewLayout(strings.Fields(in)), " "); got != want {
			t.Errorf("%s: got %q, want %q", in, got, want)
		}
	}
	if got := kviewLayout([]string{"kubectl", "get", "secret", "db", "-o", decodeTemplate}); strings.Join(got, " ") != "document decode" {
		t.Errorf("decoded view: got %q", got)
	}
}
