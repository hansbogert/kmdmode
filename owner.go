package main

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// Owner kinds cmdmode knows by a short name, to that name; any other kind is written as
// kind.group (e.g. certificate.cert-manager.io), which kubectl resolves for any CRD.
var ownerKinds = map[string]string{
	"ReplicaSet": "rs", "Deployment": "deploy", "StatefulSet": "sts", "DaemonSet": "ds",
	"Job": "job", "CronJob": "cronjob",
}

// Kinds that are not owned by anything in practice, so the walk ends there without asking.
var topKinds = set("deploy", "sts", "ds", "cronjob")

// runKubectl runs kubectl and returns its stdout; tests replace it with a fake cluster.
var runKubectl = func(args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	return string(out), err
}

// singleResource returns the TYPE/NAME a line names, if it names exactly one resource:
// "get/describe/delete TYPE NAME", "... TYPE/NAME", or "logs NAME" (a pod).
func singleResource(k kubectlCmd) (string, bool) {
	if k.has("--all-namespaces", "--selector", "--field-selector", "--filename", "--kustomize") {
		return "", false
	}
	switch k.sub {
	case "get", "describe", "delete":
		switch {
		case len(k.pos) == 2 && !strings.Contains(k.pos[0], ","):
			return bare(k.pos[0]) + "/" + bare(k.pos[1]), true
		case len(k.pos) == 1:
			if typ, name, ok := strings.Cut(bare(k.pos[0]), "/"); ok && typ != "" && name != "" {
				return typ + "/" + name, true
			}
		}
	case "logs":
		if len(k.pos) > 0 {
			if typ, name, ok := strings.Cut(bare(k.pos[0]), "/"); !ok {
				return "pod/" + typ, true
			} else if podTypes[typ] && name != "" {
				return "pod/" + name, true
			}
		}
	}
	return "", false
}

// owner asks the cluster for the top owner of ref (TYPE/NAME), following ownerReferences up
// (pod -> ReplicaSet -> Deployment, pod -> Job -> CronJob, Secret -> its operator's object, ...)
// with the line's cluster flags, and returns a get of it. Each call costs a second or more
// (auth), so it stops at kinds that have no owner, and reads a Deployment straight off its
// ReplicaSet's name: "<deployment>-<hash>", with the hash in the pod's pod-template-hash label.
// A deployment's pod takes one call.
func (k kubectlCmd) owner(ref string) ([]string, bool) {
	// Only the cluster and identity flags apply to the lookups and the result.
	notGlobal := func(f kflag) bool { _, _, known := lookup("", f.long); return !known }
	k.flags = slices.DeleteFunc(slices.Clone(k.flags), notGlobal)
	flags := k.clusterFlags()
	found := false
	for range 4 {
		args := append(append([]string{}, flags...), "--request-timeout=5s", "get", ref, "-o",
			"jsonpath={.metadata.ownerReferences[0].kind}|{.metadata.ownerReferences[0].apiVersion}"+
				"|{.metadata.ownerReferences[0].name}|{.metadata.labels['pod-template-hash']}")
		out, err := runKubectl(args)
		if err != nil { // e.g. no access to the next owner up: stop at the last one found
			break
		}
		parts := strings.Split(strings.TrimSpace(out), "|")
		if len(parts) < 4 || parts[0] == "" || parts[2] == "" {
			break
		}
		kind, apiVersion, name, hash := parts[0], parts[1], parts[2], parts[3]
		short, known := ownerKinds[kind]
		if group, _, grouped := strings.Cut(apiVersion, "/"); !known && grouped {
			short = strings.ToLower(kind) + "." + group
		} else if !known {
			short = strings.ToLower(kind) // core group, e.g. Node
		}
		ref, found = short+"/"+name, true
		if short == "rs" && hash != "" && strings.HasSuffix(name, "-"+hash) {
			ref = "deploy/" + strings.TrimSuffix(name, "-"+hash)
			break
		}
		if topKinds[short] {
			break
		}
	}
	if !found {
		return nil, false
	}
	return k.rebuild("get", ref), true
}
