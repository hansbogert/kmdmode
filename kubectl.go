package main

import (
	"regexp"
	"slices"
	"strings"
)

var kubectlNames = set("kubectl", "k", "kubecolor")

var subcommand = regexp.MustCompile(`^[a-z][a-z-]*$`)

// Flag tables from kubectl v1.37.1: "short long kind" per flag, short "-" when there is none.
// Regenerate a subcommand's table from the "--long=default:" lines of "kubectl <sub> --help";
// the global one comes from "kubectl options".
var kubectlFlagTables = map[string]flagTable{
	"": parseFlagTable(`
		- --as value; - --as-group value; - --as-uid value; - --as-user-extra value
		- --cache-dir value; - --certificate-authority value; - --client-certificate value
		- --client-key value; - --cluster value; - --context value; - --disable-compression bool
		- --insecure-skip-tls-verify bool; - --kubeconfig value; - --kuberc value
		- --log-flush-frequency value; - --match-server-version bool; -n --namespace value
		- --password value; - --profile value; - --profile-output value; - --proxy-url value
		- --request-timeout value; -s --server value; - --tls-server-name value; - --token value
		- --user value; - --username value; -v --v value; - --vmodule value
		- --warnings-as-errors bool`),
	"get": parseFlagTable(`
		-A --all-namespaces bool; - --allow-missing-template-keys bool; - --chunk-size value
		- --field-selector value; -f --filename value; - --ignore-not-found bool
		-k --kustomize value; -L --label-columns value; - --no-headers bool; -o --output value
		- --output-watch-events bool; - --raw value; -R --recursive bool; -l --selector value
		- --server-print bool; - --show-kind bool; - --show-labels bool
		- --show-managed-fields bool; - --sort-by value; - --subresource value
		- --template value; -w --watch bool; - --watch-only bool`),
	"logs": parseFlagTable(`
		- --all-containers bool; - --all-pods bool; -c --container value; -f --follow bool
		- --ignore-errors bool; - --insecure-skip-tls-verify-backend bool
		- --limit-bytes value; - --max-log-requests value; - --pod-running-timeout value
		- --prefix bool; -p --previous bool; -l --selector value; - --since value
		- --since-time value; - --tail value; - --timestamps bool`),
	"delete": parseFlagTable(`
		- --all bool; -A --all-namespaces bool; - --cascade value; - --dry-run value
		- --field-selector value; -f --filename value; - --force bool; - --grace-period value
		- --ignore-not-found bool; -i --interactive bool; -k --kustomize value; - --now bool
		-o --output value; - --raw value; -R --recursive bool; -l --selector value
		- --timeout value; - --wait bool`),
	"describe": parseFlagTable(`
		-A --all-namespaces bool; - --chunk-size value; -f --filename value
		-k --kustomize value; -R --recursive bool; -l --selector value; - --show-events bool`),
}

// The option keys offered per mode. A flag with a value is "on" only with exactly that value.
var kubectlOptions = map[string][]optionSpec{
	"get": {
		{"w", "--watch", "", []string{"-w"}},
		{"o", "--output", "wide", []string{"-o", "wide"}},
		{"y", "--output", "yaml", []string{"-o", "yaml"}},
		{"a", "--all-namespaces", "", []string{"-A"}},
		{"l", "--show-labels", "", []string{"--show-labels"}},
		{"s", "--sort-by", ".metadata.creationTimestamp", []string{"--sort-by=.metadata.creationTimestamp"}},
	},
	"logs": {
		{"f", "--follow", "", []string{"-f"}},
		{"p", "--previous", "", []string{"-p"}},
		{"t", "--timestamps", "", []string{"--timestamps"}},
		{"c", "--all-containers", "", []string{"--all-containers"}},
		{"x", "--prefix", "", []string{"--prefix"}},
		{"n", "--tail", "100", []string{"--tail=100"}},
		{"s", "--since", "1h", []string{"--since=1h"}},
	},
	"describe": {
		{"e", "--show-events", "false", []string{"--show-events=false"}},
	},
}

var podTypes = set("pods", "pod", "po")

type flagTable struct {
	short map[string]string // "-f" -> "--follow"
	value map[string]bool   // long name -> takes a value
}

func parseFlagTable(spec string) flagTable {
	t := flagTable{short: map[string]string{}, value: map[string]bool{}}
	for _, entry := range strings.FieldsFunc(spec, func(r rune) bool { return r == ';' || r == '\n' }) {
		f := strings.Fields(entry)
		if len(f) != 3 {
			continue
		}
		if f[0] != "-" {
			t.short[f[0]] = f[1]
		}
		t.value[f[1]] = f[2] == "value"
	}
	return t
}

type optionSpec struct {
	key   string
	long  string
	value string // "" for a boolean flag
	words []string
}

// A parsed flag, spanning cmd[start:end].
type kflag struct {
	long       string
	start, end int
}

type kubectlCmd struct {
	words []string
	sub   string
	flags []kflag
	pos   []string // positional args after the subcommand
	end   int      // where new flags go: before "--", else the end
}

// lookup resolves a flag word to its long name using the subcommand's table, then the global one,
// so "-f" reads as --filename on get but --follow on logs.
func lookup(sub, name string) (long string, takesValue, known bool) {
	for _, t := range []flagTable{kubectlFlagTables[sub], kubectlFlagTables[""]} {
		if l, ok := t.short[name]; ok {
			name = l
		}
		if v, ok := t.value[name]; ok {
			return name, v, true
		}
	}
	return name, false, false
}

func parseKubectl(cmd []string) (kubectlCmd, bool) {
	if len(cmd) == 0 || !kubectlNames[bare(cmd[0])] {
		return kubectlCmd{}, false
	}
	k := kubectlCmd{words: cmd, end: len(cmd)}
	for i := 1; i < len(cmd); i++ {
		w := cmd[i]
		switch {
		case w == "--":
			k.end = i
			k.pos = append(k.pos, cmd[i+1:]...)
			i = len(cmd)
		case strings.HasPrefix(w, "-") && len(w) > 1:
			// Values may be attached: --ns=x, -nx, -n=x.
			name, attached := w[:2], len(w) > 2
			if strings.HasPrefix(w, "--") {
				name, _, attached = strings.Cut(w, "=")
			}
			long, takesValue, _ := lookup(k.sub, name)
			f := kflag{long: long, start: i, end: i + 1}
			if takesValue && !attached && i+1 < len(cmd) {
				i++
				f.end = i + 1
			}
			k.flags = append(k.flags, f)
		case k.sub == "":
			k.sub = bare(w)
			if !subcommand.MatchString(k.sub) { // e.g. "kubectl() {", a function definition
				return kubectlCmd{}, false
			}
		default:
			k.pos = append(k.pos, w)
		}
	}
	return k, true
}

func (k kubectlCmd) find(longs ...string) (kflag, bool) {
	for _, f := range k.flags {
		for _, l := range longs {
			if f.long == l {
				return f, true
			}
		}
	}
	return kflag{}, false
}

func (k kubectlCmd) has(longs ...string) bool {
	_, ok := k.find(longs...)
	return ok
}

func (k kubectlCmd) value(f kflag) string {
	if f.end-f.start == 2 {
		return k.words[f.start+1]
	}
	w := k.words[f.start]
	if strings.HasPrefix(w, "--") {
		_, v, _ := strings.Cut(w, "=")
		return v
	}
	return strings.TrimPrefix(w[2:], "=")
}

// rebuild turns the command into "argv0 sub <flags> args...", keeping each flag the target
// subcommand also has. A short form that means something else there is spelled out in full.
func (k kubectlCmd) rebuild(sub string, args ...string) []string {
	out := []string{k.words[0], sub}
	for _, f := range k.flags {
		if _, takesValue, ok := lookup(sub, f.long); ok {
			w := k.words[f.start]
			if short := w[:2]; !strings.HasPrefix(w, "--") {
				if l, _, _ := lookup(sub, short); l != f.long {
					w = f.long
					if takesValue {
						w += "=" + k.value(f)
					}
					out = append(out, w)
					continue
				}
			}
			out = append(out, k.words[f.start:f.end]...)
		}
	}
	return append(out, args...)
}

// dropFlags returns the command's words minus the flags with these long names, in place.
func (k kubectlCmd) dropFlags(longs ...string) []string {
	drop := set(longs...)
	var out []string
	next := 0
	for _, f := range k.flags {
		if drop[f.long] {
			out = append(out, k.words[next:f.start]...)
			next = f.end
		}
	}
	return append(out, k.words[next:]...)
}

// without returns the command minus the flags with these long names.
func (k kubectlCmd) without(longs ...string) kubectlCmd {
	drop := set(longs...)
	k.flags = slices.DeleteFunc(slices.Clone(k.flags), func(f kflag) bool { return drop[f.long] })
	return k
}

func (k kubectlCmd) splice(start, end int, with []string) []string {
	out := append([]string{}, k.words[:start]...)
	out = append(out, with...)
	return append(out, k.words[end:]...)
}

func kubectlMode(cmd []string) (mode, bool) {
	k, ok := parseKubectl(cmd)
	if !ok {
		return mode{}, false
	}
	m := mode{name: strings.TrimSpace(bare(k.words[0]) + " " + k.sub)}
	switch k.sub {
	case "get", "describe", "delete":
		if logArgs, ok := podLogTarget(k); ok {
			m.actions = append(m.actions, action{"L", "logs", func() []string {
				return k.rebuild("logs", logArgs...)
			}})
		}
		for _, v := range resourceVerbs {
			if v.sub == k.sub || len(k.pos) == 0 && !k.has("--filename", "--kustomize") ||
				v.sub == "delete" && !namesResources(k) {
				continue
			}
			m.actions = append(m.actions, action{v.key, v.sub, func() []string {
				// delete's -o only takes "name"; get's -o wide or yaml would fail there.
				return k.without("--output").rebuild(v.sub, k.pos...)
			}})
		}
		if namesResources(k) {
			m.actions = append(m.actions, workloadActions(k, k.pos)...)
		}
	case "rollout", "scale":
		if refs, ok := k.workloads(); ok {
			for _, v := range resourceVerbs[:2] { // get, describe; not delete from here
				m.actions = append(m.actions, action{v.key, v.sub, func() []string {
					return k.rebuild(v.sub, refs...)
				}})
			}
			if len(refs) == 1 {
				m.actions = append(m.actions, action{"L", "logs", func() []string {
					return k.rebuild("logs", refs...)
				}})
			}
			m.actions = append(m.actions, workloadActions(k, refs)...)
		}
	case "logs":
		if args, ok := logsResource(k); ok {
			for _, v := range resourceVerbs {
				// "logs deploy/d" reads a pod of d; deleting d itself is too far a leap.
				if v.sub == "delete" && args[0] != "pod" {
					continue
				}
				m.actions = append(m.actions, action{v.key, v.sub, func() []string {
					return k.rebuild(v.sub, args...)
				}})
			}
			if !k.has("--selector") {
				m.actions = append(m.actions, workloadActions(k, args)...)
			}
		}
	}
	if k.sub == "get" || k.sub == "describe" {
		m.actions = append(m.actions, action{"W", "watch", func() []string {
			return append([]string{"kview"}, k.dropFlags("--watch", "--watch-only")...)
		}})
	}
	m.up = k.parent()
	m.retype = k.retype
	m.cluster = k.clusterFlags()
	m.context = k.withContext
	if f, ok := k.find("--context"); ok {
		m.currentContext = bare(k.value(f))
	}
	if ref, ok := singleResource(k); ok {
		// Resolved on the keypress, so the menu itself never waits on the cluster.
		m.actions = append(m.actions, action{"U", "owner", func() []string {
			out, _ := k.owner(ref)
			return out
		}})
	}
	m.actions = append(m.actions, action{"O", "overview", k.overview})
	for _, o := range kubectlOptions[k.sub] {
		opt := k.option(o)
		if o.long == "--all-namespaces" && !opt.on && k.sub == "get" && len(k.pos) > 0 &&
			(len(k.pos) > 1 || strings.Contains(k.pos[0], "/")) {
			// kubectl can't get a named resource across all namespaces, and ignores -n with -A,
			// so turning -A on lists the type everywhere instead.
			opt.toggle = func() []string {
				typ, _, _ := strings.Cut(k.pos[0], "/")
				return append(k.without("--namespace").rebuild("get", typ), "-A")
			}
		}
		m.options = append(m.options, opt)
	}
	return m, true
}

// The built-in resource types the ":" menu offers, by key ("." picks any type the cluster serves).
var resourceKeys = []struct{ key, name string }{
	{"p", "pods"}, {"d", "deployments"}, {"s", "statefulsets"}, {"a", "daemonsets"},
	{"c", "configmaps"}, {"e", "secrets"},
}

// retype lists another resource type where the line looks: the cluster flags, -n or -A, and -o
// carry over; names, selectors and the rest belong to the old type.
func (k kubectlCmd) retype(typ string) []string {
	k.flags = slices.DeleteFunc(slices.Clone(k.flags), func(f kflag) bool {
		_, _, global := lookup("", f.long)
		return !global && f.long != "--all-namespaces" && f.long != "--output"
	})
	return k.rebuild("get", typ)
}

// withContext sets --context name, replacing the line's own or adding it after kubectl, where every
// subcommand takes it.
func (k kubectlCmd) withContext(name string) []string {
	if f, ok := k.find("--context"); ok {
		return k.splice(f.start, f.end, []string{"--context", name})
	}
	return k.splice(1, 1, []string{"--context", name})
}

// clusterFlags returns the line's global flags (cluster, identity, namespace), unquoted for exec.
func (k kubectlCmd) clusterFlags() []string {
	var flags []string
	for _, f := range k.flags {
		if _, _, global := lookup("", f.long); global {
			for _, w := range k.words[f.start:f.end] {
				flags = append(flags, bare(w))
			}
		}
	}
	return flags
}

// parent is the menu above the line's: the get of the resource that a describe, delete, logs,
// rollout or scale line acts on. A get line, a list and anything else are top menus (nil).
func (k kubectlCmd) parent() func() []string {
	var args []string
	switch k.sub {
	case "describe", "delete":
		if !namesResources(k) {
			return nil
		}
		args = k.pos
	case "logs":
		a, ok := logsResource(k)
		if !ok {
			return nil
		}
		args = a
	case "rollout", "scale":
		refs, ok := k.workloads()
		if !ok {
			return nil
		}
		args = refs
	default:
		return nil
	}
	return func() []string { return k.without("--output").rebuild("get", args...) }
}

// Subcommands that address resources alike ("TYPE NAME..." or "TYPE/NAME"), so a line converts
// between them. I is describe as in "inspect", leaving D for delete.
var resourceVerbs = []struct{ key, sub string }{{"G", "get"}, {"I", "describe"}, {"D", "delete"}}

// namesResources reports whether the line names specific resources. Delete is offered only then:
// a selector, -A, a file or a bare type would make it delete everything that matches.
func namesResources(k kubectlCmd) bool {
	if k.has("--all", "--all-namespaces", "--selector", "--field-selector", "--filename",
		"--kustomize") {
		return false
	}
	return len(k.pos) >= 2 || len(k.pos) == 1 && strings.Contains(k.pos[0], "/")
}

// Workload kinds by every name kubectl accepts for them, to the short name used in TYPE/NAME.
var workloadKinds = map[string]string{
	"deploy": "deploy", "deployment": "deploy", "deployments": "deploy",
	"sts": "sts", "statefulset": "sts", "statefulsets": "sts",
	"ds": "ds", "daemonset": "ds", "daemonsets": "ds",
	"rs": "rs", "replicaset": "rs", "replicasets": "rs",
}

// What "kubectl rollout restart" and "kubectl scale" accept: daemonsets have no replicas, and a
// replicaset is restarted through its deployment.
var (
	restartable = set("deploy", "sts", "ds")
	scalable    = set("deploy", "sts", "rs")
	anyWorkload = set("deploy", "sts", "ds", "rs")
)

// workloadRefs turns named resources ("TYPE NAME..." or "TYPE/NAME") into TYPE/NAME refs, if all
// are of a kind in allowed.
func workloadRefs(pos []string, allowed map[string]bool) ([]string, bool) {
	kindOf := func(typ string) (string, bool) {
		typ, _, _ = strings.Cut(bare(typ), ".") // deployments.apps
		kind, ok := workloadKinds[typ]
		return kind, ok && allowed[kind]
	}
	var refs []string
	switch {
	case len(pos) >= 1 && strings.Contains(pos[0], "/"): // TYPE/NAME...
		for _, p := range pos {
			typ, name, ok := strings.Cut(bare(p), "/")
			kind, known := kindOf(typ)
			if !ok || !known || name == "" {
				return nil, false
			}
			refs = append(refs, kind+"/"+name)
		}
	case len(pos) >= 2:
		kind, known := kindOf(pos[0])
		if !known {
			return nil, false
		}
		for _, name := range pos[1:] {
			refs = append(refs, kind+"/"+name)
		}
	default:
		return nil, false
	}
	return refs, true
}

// workloadActions offers R (rollout restart), S (scale) and T (rollout status, which follows a
// rollout until it is done) for named workloads, leaving out the one the line already is. S ends
// the line at "--replicas=", for the count to be typed.
func workloadActions(k kubectlCmd, pos []string) []action {
	rolloutVerb := ""
	if k.sub == "rollout" && len(k.pos) > 0 {
		rolloutVerb = bare(k.pos[0])
	}
	var acts []action
	if refs, ok := workloadRefs(pos, restartable); ok && rolloutVerb != "restart" {
		acts = append(acts, action{"R", "restart", func() []string {
			return k.rebuild("rollout restart", refs...)
		}})
	}
	if refs, ok := workloadRefs(pos, scalable); ok && k.sub != "scale" {
		acts = append(acts, action{"S", "scale", func() []string {
			return append(k.rebuild("scale", refs...), "--replicas=")
		}})
	}
	if refs, ok := workloadRefs(pos, restartable); ok && rolloutVerb != "status" {
		acts = append(acts, action{"T", "status", func() []string {
			return k.rebuild("rollout status", refs...)
		}})
	}
	return acts
}

// workloads returns the TYPE/NAME refs a rollout or scale line names ("rollout VERB REF..." or
// "scale REF...").
func (k kubectlCmd) workloads() ([]string, bool) {
	pos := k.pos
	if k.sub == "rollout" {
		if len(pos) == 0 {
			return nil, false
		}
		pos = pos[1:]
	}
	return workloadRefs(pos, anyWorkload)
}

// Subcommands whose first argument is a resource type (or TYPE/NAME).
var typedSubs = set("get", "describe", "delete", "edit", "label", "annotate", "patch", "scale")

// overview is the k9s-style list: every resource of the line's type across all namespaces, wide,
// in kview (the plugin's live, scrollable view). It is the same view from any line: only the flags
// that pick the cluster and identity (--context, --kubeconfig, --as, ...) carry over; names,
// selectors, sorting and the rest reset.
func (k kubectlCmd) overview() []string {
	typ := "pods"
	if typedSubs[k.sub] && len(k.pos) > 0 {
		typ, _, _ = strings.Cut(k.pos[0], "/")
	} else if refs, ok := k.workloads(); ok && (k.sub == "rollout" || k.sub == "scale") {
		typ, _, _ = strings.Cut(refs[0], "/")
	}
	k.flags = slices.DeleteFunc(slices.Clone(k.flags), func(f kflag) bool {
		_, global := kubectlFlagTables[""].value[f.long]
		return !global || f.long == "--namespace"
	})
	return append(append([]string{"kview"}, k.rebuild("get", typ)...), "-A", "-o", "wide")
}

// kubectlPick turns a row picked from "kubectl get TYPE ..." output into a get of that one
// resource: NAMESPACE and NAME come from the row's leading columns (NAMESPACE only with -A),
// replacing -A with -n NS and dropping output, watch, sort and selector flags.
func kubectlPick(cmd []string, row string) ([]string, bool) {
	k, ns, typ, name, ok := pickedResource(cmd, row)
	if !ok {
		return nil, false
	}
	// One named resource: nothing to sort, and kubectl rejects a name alongside a selector.
	k = k.without("--sort-by", "--selector", "--field-selector")
	if typ == "" { // TYPE/NAME, as "get all" lists them
		return append(append(k.rebuild("get"), ns...), name), true
	}
	return append(append(k.rebuild("get"), ns...), typ, name), true
}

// kubectlSelect is kubectlPick for labels chosen from the picked resource: a get of everything of
// its type in its namespace carrying all of them, replacing any selector the list had.
func kubectlSelect(cmd []string, row string, labels []string) ([]string, bool) {
	k, ns, typ, name, ok := pickedResource(cmd, row)
	if !ok || len(labels) == 0 {
		return nil, false
	}
	if typ == "" {
		typ, _, _ = strings.Cut(name, "/")
	}
	k = k.without("--selector")
	return append(append(k.rebuild("get"), ns...), typ, "-l", strings.Join(labels, ",")), true
}

// pickedResource parses a row of "kubectl get" output: the command minus output and scope flags,
// the "-n NS" to replace -A with, and the resource's TYPE and NAME (TYPE empty when the row names
// it as TYPE/NAME).
func pickedResource(cmd []string, row string) (k kubectlCmd, ns []string, typ, name string, ok bool) {
	k, ok = parseKubectl(cmd)
	fields := strings.Fields(row)
	if !ok || k.sub != "get" || len(fields) == 0 {
		return k, nil, "", "", false
	}
	k = k.without("--output", "--watch", "--watch-only")
	if k.has("--all-namespaces") {
		if len(fields) < 2 {
			return k, nil, "", "", false
		}
		ns, fields = []string{"-n", fields[0]}, fields[1:]
		k = k.without("--all-namespaces", "--namespace")
	}
	if strings.Contains(fields[0], "/") {
		return k, ns, "", fields[0], true
	}
	typ = "pods"
	if len(k.pos) > 0 {
		typ, _, _ = strings.Cut(k.pos[0], "/")
	}
	return k, ns, typ, fields[0], true
}

// option toggles a flag: on removes it, a different value is replaced, absent is added.
func (k kubectlCmd) option(o optionSpec) option {
	f, present := k.find(o.long)
	on := present && (o.value == "" && k.value(f) != "false" || o.value != "" && k.value(f) == o.value)
	return option{o.key, strings.Join(o.words, " "), on, func() []string {
		switch {
		case on:
			return k.splice(f.start, f.end, nil)
		case present:
			return k.splice(f.start, f.end, o.words)
		default:
			return k.splice(k.end, k.end, o.words)
		}
	}}
}

// podLogTarget returns the logs arguments for a get/describe that names exactly one pod, or for
// pods picked by a label selector. "kubectl logs" cannot read resources from files or across all
// namespaces, so --filename, --kustomize and --all-namespaces rule it out.
func podLogTarget(k kubectlCmd) ([]string, bool) {
	if k.has("--all-namespaces", "--filename", "--kustomize") {
		return nil, false
	}
	switch {
	case len(k.pos) == 2 && podTypes[bare(k.pos[0])]:
		return []string{k.pos[1]}, true
	case len(k.pos) == 1 && podTypes[bare(k.pos[0])] && k.has("--selector"):
		return nil, true
	case len(k.pos) == 1:
		if typ, name, ok := strings.Cut(bare(k.pos[0]), "/"); ok && podTypes[typ] && name != "" {
			return []string{name}, true
		}
	}
	return nil, false
}

// logsResource maps "logs NAME", "logs TYPE/NAME" or "logs -l SEL" to get/describe arguments.
func logsResource(k kubectlCmd) ([]string, bool) {
	switch {
	case len(k.pos) > 0 && strings.Contains(k.pos[0], "/"):
		return k.pos[:1], true
	case len(k.pos) > 0:
		return []string{"pod", k.pos[0]}, true
	case k.has("--selector"):
		return []string{"pods"}, true
	}
	return nil, false
}

// bare strips one layer of matching quotes, for comparing a word against a keyword.
func bare(w string) string {
	if len(w) >= 2 && (w[0] == '\'' || w[0] == '"') && w[len(w)-1] == w[0] {
		return w[1 : len(w)-1]
	}
	return w
}

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}
