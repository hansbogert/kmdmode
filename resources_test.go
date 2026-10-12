package main

import (
	"strings"
	"testing"
)

func TestRetype(t *testing.T) {
	cases := []struct{ typ, in, want string }{
		{"c", "kubectl get -n ns pods -l app=x -o wide", "kubectl get -n ns -o wide configmaps"},
		{"E", "kubectl logs --context c -n ns p -f", "kubectl get --context c -n ns secrets"},
		{"s", "kubectl get pods -A --sort-by .x", "kubectl get -A statefulsets"},
		{"a", "kubectl rollout restart -n ns deploy/d", "kubectl get -n ns daemonsets"},
		{"D", "kubectl get -n ns pods p", "kubectl get -n ns deployments"},
		{"certificates.cert-manager.io", "kubectl get pods -n ns", "kubectl get -n ns certificates.cert-manager.io"},
		{"p", "kview -c STATUS kubectl get deploy -A -o wide", "kview kubectl get -A -o wide pods"},
		{"c", "ls", ""},
	}
	for _, c := range cases {
		out, ok := retype(c.typ, strings.Fields(c.in))
		check(t, c.typ, c.in, c.want, out, ok)
	}
}

func TestAPIResourcesUsesClusterFlags(t *testing.T) {
	defer func(r func([]string) (string, error)) { runKubectl = r }(runKubectl)
	var got []string
	runKubectl = func(args []string) (string, error) { got = args; return "pods\n", nil }
	m, _ := modeFor(strings.Fields("kubectl get pods --context prod -n ns -o wide"))
	runKubectl(append(m.cluster, "api-resources"))
	if strings.Join(got, " ") != "--context prod -n ns api-resources" {
		t.Errorf("args: %q", got)
	}
}
