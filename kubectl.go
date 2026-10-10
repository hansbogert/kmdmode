package main

import (
	"slices"
	"strings"
)

var kubectlNames = set("kubectl", "k", "kubecolor")

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
	case "get", "describe":
		if logArgs, ok := podLogTarget(k); ok {
			m.actions = append(m.actions, action{"H", "logs", func() []string {
				return k.rebuild("logs", logArgs...)
			}})
		}
		if len(k.pos) > 0 || k.has("--filename", "--kustomize") {
			other := map[string]string{"get": "describe", "describe": "get"}[k.sub]
			m.actions = append(m.actions, action{strings.ToUpper(other[:1]), other, func() []string {
				return k.rebuild(other, k.pos...)
			}})
		}
	case "logs":
		if args, ok := logsResource(k); ok {
			for _, sub := range []string{"get", "describe"} {
				m.actions = append(m.actions, action{strings.ToUpper(sub[:1]), sub, func() []string {
					return k.rebuild(sub, args...)
				}})
			}
		}
	}
	m.actions = append(m.actions, action{"O", "overview", k.overview})
	for _, o := range kubectlOptions[k.sub] {
		m.options = append(m.options, k.option(o))
	}
	return m, true
}

// Subcommands whose first argument is a resource type (or TYPE/NAME).
var typedSubs = set("get", "describe", "delete", "edit", "label", "annotate", "patch", "scale")

// overview is the k9s-style list: every resource of the line's type across all namespaces, wide,
// in kview (the plugin's live, scrollable view). Names are dropped; selectors and the other
// global flags carry over.
func (k kubectlCmd) overview() []string {
	typ := "pods"
	if typedSubs[k.sub] && len(k.pos) > 0 {
		typ, _, _ = strings.Cut(k.pos[0], "/")
	}
	k = k.without("--output", "--watch", "--watch-only", "--filename", "--kustomize", "--recursive",
		"--raw", "--namespace", "--all-namespaces")
	return append(append([]string{"kview"}, k.rebuild("get", typ)...), "-A", "-o", "wide")
}

// kubectlPick turns a row picked from "kubectl get TYPE ..." output into a get of that one
// resource: NAMESPACE and NAME come from the row's leading columns (NAMESPACE only with -A),
// replacing -A with -n NS and dropping output and watch flags.
func kubectlPick(cmd []string, row string) ([]string, bool) {
	k, ok := parseKubectl(cmd)
	fields := strings.Fields(row)
	if !ok || k.sub != "get" || len(fields) == 0 {
		return nil, false
	}
	k = k.without("--output", "--watch", "--watch-only")
	var ns []string
	if k.has("--all-namespaces") {
		if len(fields) < 2 {
			return nil, false
		}
		ns, fields = []string{"-n", fields[0]}, fields[1:]
		k = k.without("--all-namespaces", "--namespace")
	}
	res := []string{fields[0]} // TYPE/NAME, as "get all" lists them
	if !strings.Contains(fields[0], "/") {
		typ := "pods"
		if len(k.pos) > 0 {
			typ, _, _ = strings.Cut(k.pos[0], "/")
		}
		res = []string{typ, fields[0]}
	}
	return append(append(k.rebuild("get"), ns...), res...), true
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
