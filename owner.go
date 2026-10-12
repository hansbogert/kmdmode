package main

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// Owner kinds, to the short name used in TYPE/NAME.
var ownerKinds = map[string]string{
	"ReplicaSet": "rs", "Deployment": "deploy", "StatefulSet": "sts", "DaemonSet": "ds",
	"Job": "job", "CronJob": "cronjob",
}

// runKubectl runs kubectl and returns its stdout; tests replace it with a fake cluster.
var runKubectl = func(args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	return string(out), err
}

// singlePod returns the pod a line names, if it names exactly one: "get/describe/delete pod NAME",
// "... pod/NAME" or "logs NAME".
func singlePod(k kubectlCmd) (string, bool) {
	if k.has("--all-namespaces", "--selector", "--filename", "--kustomize") {
		return "", false
	}
	switch k.sub {
	case "get", "describe", "delete":
		switch {
		case len(k.pos) == 2 && podTypes[bare(k.pos[0])]:
			return bare(k.pos[1]), true
		case len(k.pos) == 1:
			if typ, name, ok := strings.Cut(bare(k.pos[0]), "/"); ok && podTypes[typ] && name != "" {
				return name, true
			}
		}
	case "logs":
		if len(k.pos) > 0 {
			typ, name, ok := strings.Cut(bare(k.pos[0]), "/")
			if !ok {
				return typ, true
			} else if podTypes[typ] && name != "" {
				return name, true
			}
		}
	}
	return "", false
}

// owner asks the cluster for the top controller of pod NAME, following ownerReferences up
// (pod -> ReplicaSet -> Deployment, pod -> Job -> CronJob) with the line's cluster flags, and
// returns a get of it. Each call costs a second or more (auth), so it stops at kinds that have no
// owner, and reads a Deployment straight off its ReplicaSet's name: "<deployment>-<hash>", with
// the hash in the pod's pod-template-hash label. A deployment's pod takes one call.
func (k kubectlCmd) owner(pod string) ([]string, bool) {
	// Only the cluster and identity flags apply to the lookups and the result.
	notGlobal := func(f kflag) bool { _, _, known := lookup("", f.long); return !known }
	k.flags = slices.DeleteFunc(slices.Clone(k.flags), notGlobal)
	flags := k.clusterFlags()
	ref, found := "pod/"+pod, false
	for range 3 {
		args := append(append([]string{}, flags...), "--request-timeout=5s", "get", ref, "-o",
			"jsonpath={.metadata.ownerReferences[0].kind}/{.metadata.ownerReferences[0].name}"+
				"/{.metadata.labels['pod-template-hash']}")
		out, err := runKubectl(args)
		if err != nil { // e.g. no access to the next owner up: stop at the last one found
			break
		}
		parts := strings.SplitN(strings.TrimSpace(out), "/", 3)
		if len(parts) < 2 {
			break
		}
		short, known := ownerKinds[parts[0]]
		if !known || parts[1] == "" {
			break
		}
		ref, found = short+"/"+parts[1], true
		if hash := "-" + parts[len(parts)-1]; short == "rs" && len(parts) == 3 && hash != "-" &&
			strings.HasSuffix(parts[1], hash) {
			ref = "deploy/" + strings.TrimSuffix(parts[1], hash)
			break
		}
		if short != "rs" && short != "job" { // deployments, statefulsets, daemonsets, cronjobs
			break
		}
	}
	if !found {
		return nil, false
	}
	return k.rebuild("get", ref), true
}
