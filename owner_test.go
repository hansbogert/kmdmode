package main

import (
	"errors"
	"strings"
	"testing"
)

// fakeCluster answers owner lookups from a map of "get REF" to the jsonpath output.
func fakeCluster(t *testing.T, owners map[string]string, seen *[]string) func([]string) (string, error) {
	return func(args []string) (string, error) {
		*seen = append(*seen, strings.Join(args, " "))
		for i, a := range args {
			if a == "get" && i+1 < len(args) {
				if out, ok := owners[args[i+1]]; ok {
					return out, nil
				}
				return "", errors.New("NotFound")
			}
		}
		t.Fatalf("no get in %q", args)
		return "", nil
	}
}

func TestOwner(t *testing.T) {
	owners := map[string]string{
		"pod/web-7d9f8-x2k":  "ReplicaSet/web-7d9f8/7d9f8",
		"pod/db-0":           "StatefulSet/db/",
		"pod/backup-29-abcd": "Job/backup-29/",
		"job/backup-29":      "CronJob/backup/",
		"pod/solo-x":         "ReplicaSet/solo/",
		"rs/solo":            "//",
		"pod/bare":           "//",
	}
	defer func(r func([]string) (string, error)) { runKubectl = r }(runKubectl)
	var seen []string
	runKubectl = fakeCluster(t, owners, &seen)

	cases := []struct{ in, want string }{
		{"kubectl get -n ns pods web-7d9f8-x2k -o wide", "kubectl get -n ns deploy/web"},
		{"kubectl logs --context c -n ns db-0 -f", "kubectl get --context c -n ns sts/db"},
		{"kubectl describe -n ns pod/backup-29-abcd", "kubectl get -n ns cronjob/backup"},
		{"kubectl get pods solo-x", "kubectl get rs/solo"},
		{"kubectl get pods bare", ""},
		{"kubectl get pods missing", ""},
	}
	for _, c := range cases {
		out, ok := run("U", false, strings.Fields(c.in))
		check(t, "U", c.in, c.want, out, ok)
	}
	// A deployment's pod takes one call, a statefulset's one, a cronjob's two.
	if n := len(seen); n != 1+1+2+2+1+1 {
		t.Errorf("lookups: got %d: %q", n, seen)
	}
	if !strings.HasPrefix(seen[0], "-n ns --request-timeout=5s get pod/web-7d9f8-x2k -o jsonpath=") {
		t.Errorf("first lookup: %q", seen[0])
	}
}

func TestOwnerOffered(t *testing.T) {
	for in, want := range map[string]bool{
		"kubectl get pods p":        true,
		"kubectl get pod/p -n ns":   true,
		"kubectl logs p":            true,
		"kubectl describe pods p":   true,
		"kubectl get pods":          false,
		"kubectl get pods a b":      false,
		"kubectl get pods -A p":     false,
		"kubectl get deploy d":      false,
		"kubectl logs deploy/d":     false,
		"kubectl get pods -l app=x": false,
	} {
		m, _ := modeFor(strings.Fields(in))
		got := false
		for _, a := range m.actions {
			got = got || a.key == "U"
		}
		if got != want {
			t.Errorf("%q: U offered %v, want %v", in, got, want)
		}
	}
}
