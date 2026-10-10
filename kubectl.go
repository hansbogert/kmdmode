package main

import "strings"

var kubectlNames = set("kubectl", "k", "kubecolor")

// Flags that take a separate value word; needed to tell a flag's value from a positional arg.
var kubectlValueFlags = set(
	"-n", "--namespace", "--context", "--kubeconfig", "--cluster", "--user", "-s", "--server",
	"--token", "--as", "--as-group", "--as-uid", "--request-timeout", "--certificate-authority",
	"--client-certificate", "--client-key", "--tls-server-name", "--cache-dir", "-v", "--v",
	"-o", "--output", "-l", "--selector", "--field-selector", "--sort-by", "--template",
	"-L", "--label-columns", "--chunk-size", "--output-watch-events", "--subresource",
)

// Connection and scope flags mean the same for every subcommand, so they carry over.
var kubectlGlobalFlags = set(
	"-n", "--namespace", "--context", "--kubeconfig", "--cluster", "--user", "-s", "--server",
	"--token", "--as", "--as-group", "--as-uid", "--request-timeout", "--certificate-authority",
	"--client-certificate", "--client-key", "--tls-server-name", "--cache-dir", "-v", "--v",
	"--insecure-skip-tls-verify",
)

var podTypes = set("pods", "pod", "po")

type kflag struct {
	name  string
	words []string
}

type kubectlCmd struct {
	argv0 string
	sub   string
	flags []kflag
	pos   []string // positional args after the subcommand
}

func parseKubectl(cmd []string) (kubectlCmd, bool) {
	if len(cmd) == 0 || !kubectlNames[bare(cmd[0])] {
		return kubectlCmd{}, false
	}
	k := kubectlCmd{argv0: cmd[0]}
	var pos []string
	for i := 1; i < len(cmd); i++ {
		w := cmd[i]
		switch {
		case w == "--":
			pos = append(pos, cmd[i+1:]...)
			i = len(cmd)
		case strings.HasPrefix(w, "--"):
			name, _, hasValue := strings.Cut(w, "=")
			f := kflag{name: name, words: []string{w}}
			if !hasValue && kubectlValueFlags[name] && i+1 < len(cmd) {
				i++
				f.words = append(f.words, cmd[i])
			}
			k.flags = append(k.flags, f)
		case strings.HasPrefix(w, "-") && len(w) > 1:
			// Short flags may carry their value attached: -nfoo or -n=foo.
			name := w[:2]
			f := kflag{name: name, words: []string{w}}
			if len(w) == 2 && kubectlValueFlags[name] && i+1 < len(cmd) {
				i++
				f.words = append(f.words, cmd[i])
			}
			k.flags = append(k.flags, f)
		default:
			pos = append(pos, w)
		}
	}
	if len(pos) == 0 {
		return kubectlCmd{}, false
	}
	k.sub, k.pos = bare(pos[0]), pos[1:]
	return k, true
}

func (k kubectlCmd) has(names ...string) bool {
	for _, f := range k.flags {
		for _, n := range names {
			if f.name == n {
				return true
			}
		}
	}
	return false
}

// rebuild emits "argv0 sub <global flags> <flags named in keep> args...".
func (k kubectlCmd) rebuild(sub string, keep map[string]bool, args ...string) []string {
	out := []string{k.argv0, sub}
	for _, f := range k.flags {
		if kubectlGlobalFlags[f.name] || keep[f.name] {
			out = append(out, f.words...)
		}
	}
	return append(out, args...)
}

var selectorFlags = set("-l", "--selector")

func kubectlActions(cmd []string) []action {
	k, ok := parseKubectl(cmd)
	if !ok || k.sub != "get" || len(k.pos) == 0 {
		return nil
	}
	var acts []action

	if logArgs, ok := podLogTarget(k); ok {
		acts = append(acts, action{"H", "logs", func([]string) []string {
			return k.rebuild("logs", selectorFlags, logArgs...)
		}})
	}
	acts = append(acts, action{"D", "describe", func([]string) []string {
		return k.rebuild("describe", set("-l", "--selector", "-A", "--all-namespaces"), k.pos...)
	}})
	return acts
}

// podLogTarget returns the logs arguments for a "get" that names exactly one pod, or for pods
// picked by a label selector. "kubectl logs" has no --all-namespaces, so -A rules it out.
func podLogTarget(k kubectlCmd) ([]string, bool) {
	if k.has("-A", "--all-namespaces") {
		return nil, false
	}
	switch {
	case len(k.pos) == 2 && podTypes[bare(k.pos[0])]:
		return []string{k.pos[1]}, true
	case len(k.pos) == 1 && podTypes[bare(k.pos[0])] && k.has("-l", "--selector"):
		return nil, true
	case len(k.pos) == 1:
		if typ, name, ok := strings.Cut(bare(k.pos[0]), "/"); ok && podTypes[typ] && name != "" {
			return []string{name}, true
		}
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
