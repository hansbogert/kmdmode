package main

import (
	"regexp"
	"slices"
	"strings"
)

// klog's own lines ("E1025 08:03:09.594820 1791569 memcache.go:381] ..."): kubectl repeats them
// for every discovery attempt, so they bury the one line that says what is wrong.
var klogLine = regexp.MustCompile(`^[IWEF]\d{4} \d\d:\d\d:\d\d`)

var credentialHelper = regexp.MustCompile(`getting credentials: exec: executable (\S+) failed`)

// kubectlError turns what a failed kubectl wrote on stderr into one line: kubectl's own last
// word on it, or, when the credential helper (aws, tsh, kubelogin, ...) failed, so that there is
// no valid session, what that helper said about why.
func kubectlError(stderr string) string {
	var lines []string
	for _, l := range strings.Split(stderr, "\n") {
		l = strings.TrimSpace(ansi.ReplaceAllString(l, ""))
		if l != "" && !klogLine.MatchString(l) && !slices.Contains(lines, l) {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return "kubectl failed without saying why"
	}
	last := lines[len(lines)-1]
	m := credentialHelper.FindStringSubmatch(last)
	if m == nil {
		return strings.TrimPrefix(last, "error: ")
	}
	helper := m[1][strings.LastIndex(m[1], "/")+1:]
	// The helper's own lines come before kubectl's.
	if len(lines) > 1 {
		return "no valid session: " + helper + ": " + lines[len(lines)-2]
	}
	return "no valid session: " + helper + " failed to get credentials"
}
