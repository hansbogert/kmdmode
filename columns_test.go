package main

import (
	"strings"
	"testing"
)

const wideHeader = "NAMESPACE     NAME        READY   STATUS             RESTARTS   AGE   IP         NODE     NOMINATED NODE   READINESS GATES"

func TestTableColumns(t *testing.T) {
	names, starts := tableColumns(wideHeader)
	want := "NAMESPACE|NAME|READY|STATUS|RESTARTS|AGE|IP|NODE|NOMINATED NODE|READINESS GATES"
	if got := strings.Join(names, "|"); got != want {
		t.Fatalf("names: got %q, want %q", got, want)
	}
	if starts[1] != strings.Index(wideHeader, "NAME ") || starts[8] != strings.Index(wideHeader, "NOMINATED") {
		t.Errorf("starts: got %v", starts)
	}
}

func TestColumns(t *testing.T) {
	row := "kube-system   coredns-x   1/1     CrashLoopBackOff   7          2d    10.0.0.1   node-a   <none>           <none>"
	colored := strings.Replace(row, "CrashLoopBackOff", "\x1b[31mCrashLoopBackOff\x1b[0m", 1)
	var out strings.Builder
	in := wideHeader + faultsMarker + "\n" + colored + "\n"
	if err := columns(strings.NewReader(in), &out, []string{"STATUS", "NODE"}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	plain := func(s string) string { return strings.Join(strings.Fields(ansi.ReplaceAllString(s, "")), " ") }
	if got := plain(lines[0]); got != "NAMESPACE NAME STATUS NODE [faults only]" {
		t.Errorf("header: got %q", got)
	}
	if got := plain(lines[1]); got != "kube-system coredns-x CrashLoopBackOff node-a" {
		t.Errorf("row: got %q", got)
	}
	if !strings.Contains(lines[1], "\x1b[31mCrashLoopBackOff") {
		t.Errorf("row lost its color: %q", lines[1])
	}
	// Columns stay aligned: STATUS starts at the same offset in header and row.
	if h, r := strings.Index(ansi.ReplaceAllString(lines[0], ""), "STATUS"), strings.Index(ansi.ReplaceAllString(lines[1], ""), "Crash"); h != r {
		t.Errorf("misaligned: STATUS at %d, value at %d", h, r)
	}

	out.Reset()
	columns(strings.NewReader("error: You must be logged in\n"), &out, []string{"NODE"})
	if got := out.String(); got != "error: You must be logged in\n" {
		t.Errorf("non-table: got %q", got)
	}
}

func TestListColumns(t *testing.T) {
	var out strings.Builder
	listColumns(strings.NewReader(wideHeader+"\nrow\n"), &out, nil)
	want := "READY on|STATUS on|RESTARTS on|AGE on|IP off|NODE on|NOMINATED NODE off|READINESS GATES off"
	if got := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(out.String()), "\t", " "), "\n", "|"); got != want {
		t.Errorf("defaults: got %q, want %q", got, want)
	}
	out.Reset()
	listColumns(strings.NewReader(wideHeader+"\n"), &out, []string{"IP"})
	if got := out.String(); !strings.Contains(got, "IP\ton") || !strings.Contains(got, "READY\toff") {
		t.Errorf("current: got %q", got)
	}
}
