package main

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// What kubectl port-forward reaches, to the short name f writes it with: a pod, a service, or a
// workload (kubectl forwards to one of its pods).
var forwardKinds = map[string]string{
	"pod": "pod", "pods": "pod", "po": "pod",
	"svc": "svc", "service": "svc", "services": "svc",
	"deploy": "deploy", "deployment": "deploy", "deployments": "deploy",
	"sts": "sts", "statefulset": "sts", "statefulsets": "sts",
	"ds": "ds", "daemonset": "ds", "daemonsets": "ds",
	"rs": "rs", "replicaset": "rs", "replicasets": "rs",
}

// forwardTarget returns, as TYPE/NAME, the one resource the line names that port-forward can
// reach.
func (k kubectlCmd) forwardTarget() (string, bool) {
	ref, ok := singleResource(k)
	if !ok && (k.sub == "rollout" || k.sub == "scale") {
		if refs, one := k.workloads(); one && len(refs) == 1 {
			ref, ok = refs[0], true
		}
	}
	if !ok {
		return "", false
	}
	typ, name, _ := strings.Cut(ref, "/")
	typ, _, _ = strings.Cut(typ, ".") // deployments.apps
	kind, known := forwardKinds[typ]
	return kind + "/" + name, known && name != ""
}

// forwardAction is f: a port-forward to the line's resource, of the ports it declares, asked of
// the cluster on the keypress. Without declared ports the line ends in a space, for typing them.
func (k kubectlCmd) forwardAction() (action, bool) {
	target, ok := k.forwardTarget()
	if !ok {
		return action{}, false
	}
	return action{"f", "forward", func() []string {
		ports, err := k.forwardPorts(target)
		if err != nil {
			clusterErr = err
			return nil
		}
		line := append(k.without(k.nonGlobalLongs()...).rebuild("port-forward", target), ports...)
		if len(ports) == 0 {
			line = append(line, "")
		}
		return line
	}}, true
}

// forwardPorts returns the port-forward arguments for the ports ref declares: a service's own
// ports, else its containers' (a workload's, from its pod template). A port below 1024 is
// forwarded from 8000 above it (80 from 8080), so it needs no root; one that would clash is left
// to kubectl to pick (":80").
func (k kubectlCmd) forwardPorts(ref string) ([]string, error) {
	kind, _, _ := strings.Cut(ref, "/")
	path := "{.spec.template.spec.containers[*].ports[*].containerPort}"
	switch kind {
	case "pod":
		path = "{.spec.containers[*].ports[*].containerPort}"
	case "svc":
		path = "{.spec.ports[*].port}"
	}
	args := append(k.clusterFlags(), "--request-timeout=5s", "get", ref, "-o", "jsonpath="+path)
	out, err := runKubectl(args)
	if err != nil {
		return nil, err
	}
	var ports []string
	var remotes []int
	for _, f := range strings.Fields(out) {
		remote, err := strconv.Atoi(f)
		if err != nil || slices.Contains(remotes, remote) {
			continue // a port declared for TCP and UDP alike
		}
		remotes = append(remotes, remote)
	}
	for _, remote := range remotes {
		local := remote
		if local < 1024 {
			local += 8000
		}
		switch {
		case local == remote:
			ports = append(ports, strconv.Itoa(remote))
		case slices.Contains(remotes, local): // 80 beside 8080: 8080 is taken
			ports = append(ports, ":"+strconv.Itoa(remote))
		default:
			ports = append(ports, strconv.Itoa(local)+":"+strconv.Itoa(remote))
		}
	}
	return ports, nil
}

// forwardResource returns the get arguments for what a port-forward line forwards to: "pod p"
// for a bare pod name, else its TYPE/NAME.
func forwardResource(k kubectlCmd) ([]string, bool) {
	if k.sub != "port-forward" || len(k.pos) == 0 {
		return nil, false
	}
	ref := bare(k.pos[0])
	typ, name, slashed := strings.Cut(ref, "/")
	if !slashed {
		return []string{"pod", ref}, true
	}
	if _, ok := forwardKinds[strings.SplitN(typ, ".", 2)[0]]; !ok || name == "" {
		return nil, false
	}
	return []string{ref}, true
}

// cursorMark in a word of an action's output is where the cursor goes, for an action that hands
// the keyboard back mid-line; kmdmode apply removes it and prints its position.
const cursorMark = "\x00"

var portSpec = regexp.MustCompile(`^(?:(\d*):)?(\d+)$`)

// localPortAction is p on a port-forward line: the first port's local side emptied, with the
// cursor there to type another ("8080:80" becomes "|:80").
func (k kubectlCmd) localPortAction() (action, bool) {
	if _, ok := forwardResource(k); !ok {
		return action{}, false
	}
	inFlag := func(i int) bool {
		return slices.ContainsFunc(k.flags, func(f kflag) bool { return f.start <= i && i < f.end })
	}
	for i := k.subIdx + 2; i < k.end; i++ { // past the target
		m := portSpec.FindStringSubmatch(bare(k.words[i]))
		if m == nil || inFlag(i) {
			continue
		}
		return action{"p", "local port", func() []string {
			return k.splice(i, i+1, []string{cursorMark + ":" + m[2]})
		}}, true
	}
	return action{}, false
}
