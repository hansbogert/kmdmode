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
		"pods/web-7d9f8-x2k": "ReplicaSet|apps/v1|web-7d9f8|7d9f8",
		"pod/db-0":           "StatefulSet|apps/v1|db|",
		"pod/backup-29-abcd": "Job|batch/v1|backup-29|",
		"job/backup-29":      "CronJob|batch/v1|backup|",
		"pods/solo-x":        "ReplicaSet|apps/v1|solo|",
		"rs/solo":            "|||",
		"pods/bare":          "|||",
		"secrets/ch":         "ClickHouseCluster|clickhouse.com/v1alpha1|analytics|",
		"clickhousecluster.clickhouse.com/analytics": "|||",
		"certificates.cert-manager.io/tls":           "|||",
		"pods/static-node-a":                         "Node|v1|node-a|",
		"node/node-a":                                "|||",
	}
	defer func(r func([]string) (string, error)) { runKubectl = r }(runKubectl)
	var seen []string
	runKubectl = fakeCluster(t, owners, &seen)

	cases := []struct{ in, want string }{
		{"kubectl get -n ns pods web-7d9f8-x2k -o wide", "kubectl get -n ns deploy/web"},
		{"kubectl logs --context c -n ns db-0 -f", "kubectl --context c get -n ns sts/db"},
		{"kubectl describe -n ns pod/backup-29-abcd", "kubectl get -n ns cronjob/backup"},
		{"kubectl get pods solo-x", "kubectl get rs/solo"},
		{"kubectl get -n clickhouse secrets ch", "kubectl get -n clickhouse clickhousecluster.clickhouse.com/analytics"},
		{"kubectl get pods static-node-a", "kubectl get node/node-a"},
		{"kubectl get pods bare", ""},
		{"kubectl get certificates.cert-manager.io tls", ""},
		{"kubectl get pods missing", ""},
	}
	for _, c := range cases {
		seen = nil
		out, ok := run("u", false, strings.Fields(c.in))
		check(t, "u", c.in, c.want, out, ok)
		// A deployment's pod takes one call.
		if c.want == "kubectl get -n ns deploy/web" && len(seen) != 1 {
			t.Errorf("deployment pod: %d lookups: %q", len(seen), seen)
		}
	}
}

func TestOwnerOffered(t *testing.T) {
	for in, want := range map[string]bool{
		"kubectl get pods p":                           true,
		"kubectl get pod/p -n ns":                      true,
		"kubectl logs p":                               true,
		"kubectl describe pods p":                      true,
		"kubectl get secrets s -n ns":                  true,
		"kubectl get certificates.cert-manager.io/tls": true,
		"kubectl get deploy d":                         true,
		"kubectl get pods":                             false,
		"kubectl get pods a b":                         false,
		"kubectl get pods -A p":                        false,
		"kubectl logs deploy/d":                        false,
		"kubectl get pods -l app=x":                    false,
		"kubectl get pods,svc p":                       false,
	} {
		m, _ := modeFor(strings.Fields(in))
		got := false
		for _, a := range m.actions {
			got = got || a.key == "u"
		}
		if got != want {
			t.Errorf("%q: U offered %v, want %v", in, got, want)
		}
	}
}
