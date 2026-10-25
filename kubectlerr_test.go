package main

import "testing"

func TestKubectlError(t *testing.T) {
	klog := `E1025 08:03:09.594820 1791569 memcache.go:381] "Couldn't get current server API group list" err="..."` + "\n"
	sso := "\nError when retrieving token from sso: Token has expired and refresh failed\n"
	for _, c := range []struct{ name, stderr, want string }{
		{"expired session", sso + klog + sso + klog +
			"Unable to connect to the server: getting credentials: exec: executable /usr/local/bin/aws failed with exit code 255\n",
			"no valid session: aws: Error when retrieving token from sso: Token has expired and refresh failed"},
		{"silent helper", klog + "Unable to connect to the server: getting credentials: exec: executable tsh failed with exit code 1\n",
			"no valid session: tsh failed to get credentials"},
		{"rejected token", klog + "error: You must be logged in to the server (Unauthorized)\n",
			"You must be logged in to the server (Unauthorized)"},
		{"not found", `Error from server (NotFound): pods "p" not found` + "\n", `Error from server (NotFound): pods "p" not found`},
		{"nothing said", "", "kubectl failed without saying why"},
	} {
		if got := kubectlError(c.stderr); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
