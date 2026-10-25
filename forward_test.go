package main

import (
	"strings"
	"testing"
)

func TestForward(t *testing.T) {
	ports := map[string]string{ // what the jsonpath of declared ports prints
		"svc/web":      "80 9090",
		"pod/p":        "5432",
		"deploy/web":   "80 8080",
		"pod/db-0":     "53 53", // TCP and UDP
		"sts/db":       "5432",
		"pod/bare":     "",
		"deploy/proxy": "443",
	}
	defer func(r func([]string) (string, error)) { runKubectl = r }(runKubectl)
	var seen []string
	runKubectl = fakeCluster(t, ports, &seen)

	for _, c := range []struct{ in, want string }{
		{"kubectl get -n ns svc web", "kubectl port-forward -n ns svc/web 8080:80 9090"},
		{"kubectl get pods p -o wide", "kubectl port-forward pod/p 5432"},
		{"kubectl describe --context c deployments.apps/web", "kubectl --context c port-forward deploy/web :80 8080"},
		{"kubectl logs -n ns db-0 -f --tail=10", "kubectl port-forward -n ns pod/db-0 8053:53"},
		{"kubectl scale -n ns sts db --replicas=2", "kubectl port-forward -n ns sts/db 5432"},
		{"kubectl rollout status deploy/proxy", "kubectl port-forward deploy/proxy 8443:443"},
		{"kubectl get pods bare", "kubectl port-forward pod/bare "}, // no ports declared: type them
		{"kubectl get pods missing", ""},
		{"kubectl get configmap cfg", ""},
		{"kubectl get pods -l app=web", ""},
		{"kubectl get svc", ""},
	} {
		clusterErr = nil
		out, ok := run("f", false, strings.Fields(c.in))
		check(t, "f", c.in, c.want, out, ok)
	}
	clusterErr = nil
	if _, ok := run("f", false, strings.Fields("kubectl get pods missing")); ok || clusterErr == nil {
		t.Errorf("a failed lookup should say why: ok=%v, clusterErr=%v", ok, clusterErr)
	}
}

func TestForwardLine(t *testing.T) {
	for in, want := range map[string]string{
		"kubectl port-forward -n ns svc/web 8080:80":         "kubectl get -n ns svc/web",
		"kubectl port-forward -n ns web-1 5432":              "kubectl get -n ns pod web-1",
		"kubectl port-forward --address 0.0.0.0 deploy/d 80": "kubectl get deploy/d",
	} {
		if out, ok := up(strings.Fields(in)); !ok || strings.Join(out, " ") != want {
			t.Errorf("up %s: got %q, want %q", in, out, want)
		}
	}
	out, ok := run("d", false, strings.Fields("kubectl port-forward -n ns svc/web 8080:80"))
	check(t, "d", "port-forward", "kubectl describe -n ns svc/web", out, ok)
}

func TestLocalPort(t *testing.T) {
	for in, want := range map[string]string{
		"kubectl port-forward -n ns svc/web 8080:80 9090":          "kubectl port-forward -n ns svc/web |:80 9090",
		"kubectl port-forward --pod-running-timeout 80 pod/p 5432": "kubectl port-forward --pod-running-timeout 80 pod/p |:5432",
		"kubectl port-forward pod/p :5432":                         "kubectl port-forward pod/p |:5432",
		"kubectl port-forward pod/p":                               "", // no port to change
		"kubectl get pods p":                                       "",
	} {
		out, ok := run("p", false, strings.Fields(in))
		got := strings.ReplaceAll(strings.Join(out, " "), cursorMark, "|")
		if got != want || ok != (want != "") {
			t.Errorf("p on %q: got %q, want %q", in, got, want)
		}
	}
}
