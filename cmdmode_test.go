package main

import (
	"strings"
	"testing"
)

func TestApply(t *testing.T) {
	cases := []struct {
		key, in, want string // want "" means the key must not apply
	}{
		{"H", "kubectl get pods -n ns pod1", "kubectl logs -n ns pod1"},
		{"H", "kubectl -n ns get pod pod1", "kubectl logs -n ns pod1"},
		{"H", "kubectl get po pod1 --context prod -o wide", "kubectl logs --context prod pod1"},
		{"H", "kubectl get pod/pod1 -nns", "kubectl logs -nns pod1"},
		{"H", "kubectl get pods --namespace=ns -l app=api", "kubectl logs --namespace=ns -l app=api"},
		{"H", "k get pods 'pod1'", "k logs 'pod1'"},
		{"H", "KUBECONFIG=x kubectl get pods p | less", "KUBECONFIG=x kubectl logs p | less"},
		{"H", "cd /tmp && kubectl get pods p", "cd /tmp && kubectl logs p"},
		{"H", "kubectl get pods", ""},
		{"H", "kubectl get pods -A -l app=api", ""},
		{"H", "kubectl get pods a b", ""},
		{"H", "kubectl get deploy d", ""},
		{"H", "ls -l", ""},
		{"D", "kubectl get deploy d -n ns -o yaml", "kubectl describe -n ns deploy d"},
		{"D", "kubectl get pods -A -l app=api", "kubectl describe -A -l app=api pods"},
		{"X", "kubectl get pods p", ""},
		{"h", "kubectl get pods p", "kubectl logs p"},
		{"d", "kubectl get pods p", "kubectl describe pods p"},
		{"H", "kubectl get -f pods.yaml", ""},
		{"D", "kubectl get -f pods.yaml -o name", "kubectl describe -f pods.yaml"},
		{"D", "kubectl get -R -f dir deploy", "kubectl describe -R -f dir deploy"},
		{"H", "kubectl get pods p -w --show-labels --sort-by .x", "kubectl logs p"},
		{"H", "kubectl get pods p --kuberc k --proxy-url u", "kubectl logs --kuberc k --proxy-url u p"},
	}
	for _, c := range cases {
		out, ok := apply(c.key, strings.Fields(c.in))
		got := strings.Join(out, " ")
		if c.want == "" {
			if ok {
				t.Errorf("%s on %q: want no action, got %q", c.key, c.in, got)
			}
			continue
		}
		if !ok || got != c.want {
			t.Errorf("%s on %q: got %q (ok=%v), want %q", c.key, c.in, got, ok, c.want)
		}
	}
}
