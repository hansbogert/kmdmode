package main

import "strings"

// decodeTemplate prints each of a secret's keys with its value base64-decoded. It is kubectl's
// own base64decode, so the line stays one kubectl command, rather than a pipe into base64 -d that
// kmdmode and kview could no longer read as kubectl.
const decodeTemplate = `go-template={{range $k, $v := .data}}{{$k}}: {{$v | base64decode}}{{"\n"}}{{end}}`

var secretTypes = set("secret", "secrets")

// decoded reports whether the line is v's decoded view of a secret.
func (k kubectlCmd) decoded() bool {
	f, ok := k.find("--output")
	return ok && bare(k.value(f)) == decodeTemplate
}

// decodeAction is v on a line naming one secret: a get of its values, decoded; on that line,
// v goes back to the plain get.
func (k kubectlCmd) decodeAction() (action, bool) {
	ref, ok := singleResource(k)
	typ, _, _ := strings.Cut(ref, "/")
	if !ok || k.sub == "logs" || !secretTypes[typ] {
		return action{}, false
	}
	get := k.without("--output").rebuild("get", k.pos...)
	if k.decoded() {
		return action{"v", "encoded", func() []string { return get }}, true
	}
	return action{"v", "decode", func() []string {
		return append(get, "-o", "'"+decodeTemplate+"'")
	}}, true
}

// kviewLayout says how kview shows a command's output: "table" for a kubectl get that prints one
// (no -o, or -o wide or custom-columns), else "document". A second line "decode" offers alt-d:
// in a table of secrets, the picked row's values, decoded, beside it; in a document, the switch
// to the other of v's two views.
func kviewLayout(words []string) []string {
	k, ok := parseKubectl(words)
	if !ok {
		return []string{"document"}
	}
	out := "-"
	if f, ok := k.find("--output"); ok {
		out = bare(k.value(f))
	}
	if k.sub == "get" && (out == "-" || out == "wide" || strings.HasPrefix(out, "custom-columns")) {
		if len(k.pos) > 0 {
			if typ, _, _ := strings.Cut(bare(k.pos[0]), "/"); secretTypes[typ] {
				return []string{"table", "decode"}
			}
		}
		return []string{"table"}
	}
	if _, ok := k.decodeAction(); ok {
		return []string{"document", "decode"}
	}
	return []string{"document"}
}
