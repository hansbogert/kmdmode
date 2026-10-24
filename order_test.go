package main

import (
	"strings"
	"testing"
)

func TestOrderByName(t *testing.T) {
	// As kview fetches it: oldest first.
	table := strings.Join([]string{
		"NAMESPACE   NAME    READY   AGE",
		"ns-b        mid     1/1     3h",
		"ns-a        \x1b[32mold\x1b[0m     1/1     10d",
		"ns-a        new     1/1     5m",
	}, "\n") + "\n"
	var out strings.Builder
	if err := orderByName(strings.NewReader(table), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var got []string
	for _, l := range lines[1:] {
		got = append(got, strings.Fields(ansi.ReplaceAllString(l, ""))[1])
	}
	if strings.Join(got, " ") != "new old mid" {
		t.Errorf("got %q, want by namespace then name", got)
	}
	if lines[0] != "NAMESPACE   NAME    READY   AGE" || !strings.Contains(lines[2], "\x1b[32mold") {
		t.Errorf("header or colours not kept: %q", lines)
	}

	out.Reset()
	orderByName(strings.NewReader("error: no route\n"), &out)
	if out.String() != "error: no route\n" {
		t.Errorf("non-table: got %q", out.String())
	}
}
