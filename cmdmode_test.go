package main

import (
	"strings"
	"testing"
)

func TestApply(t *testing.T) {
	cases := []struct {
		key, in, want string // want "" means the key must not apply
	}{
		{"H", "kubectl get pods -n ns pod1", "kubectl logs -n ns pod1"},
		{"H", "kubectl -n ns get pod pod1", "kubectl logs -n ns pod1"},
		{"H", "kubectl get po pod1 --context prod -o wide", "kubectl logs --context prod pod1"},
		{"H", "kubectl get pod/pod1 -nns", "kubectl logs -nns pod1"},
		{"H", "kubectl get pods --namespace=ns -l app=api", "kubectl logs --namespace=ns -l app=api"},
		{"H", "k get pods 'pod1'", "k logs 'pod1'"},
		{"H", "KUBECONFIG=x kubectl get pods p | less", "KUBECONFIG=x kubectl logs p | less"},
		{"H", "cd /tmp && kubectl get pods p", "cd /tmp && kubectl logs p"},
		{"H", "kubectl get pods", ""},
		{"H", "kubectl get pods -A -l app=api", ""},
		{"H", "kubectl get pods a b", ""},
		{"H", "kubectl get deploy d", ""},
		{"H", "ls -l", ""},
		{"D", "kubectl get deploy d -n ns -o yaml", "kubectl describe -n ns deploy d"},
		{"D", "kubectl get pods -A -l app=api", "kubectl describe -A -l app=api pods"},
		{"X", "kubectl get pods p", ""},
		{"h", "kubectl get pods p", "kubectl logs p"},
		{"d", "kubectl get pods p", "kubectl describe pods p"},
		{"H", "kubectl get -f pods.yaml", ""},
		{"D", "kubectl get -f pods.yaml -o name", "kubectl describe -f pods.yaml"},
		{"D", "kubectl get -R -f dir deploy", "kubectl describe -R -f dir deploy"},
		{"H", "kubectl get pods p -w --show-labels --sort-by .x", "kubectl logs p"},
		{"H", "kubectl get pods p --kuberc k --proxy-url u", "kubectl logs --kuberc k --proxy-url u p"},
		// logs mode: -f is --follow here, so it must not become get's --filename.
		{"G", "kubectl logs -n ns -f p --tail 5", "kubectl get -n ns pod p"},
		{"D", "kubectl logs deploy/d", "kubectl describe deploy/d"},
		{"G", "kubectl logs -l app=api", "kubectl get -l app=api pods"},
		{"G", "kubectl logs", ""},
		{"H", "kubectl describe pod p -n ns", "kubectl logs -n ns p"},
		{"G", "kubectl describe -f x.yaml", "kubectl get -f x.yaml"},
		// overview: the type's full list across namespaces, wide, in kview, from any kubectl line.
		{"O", "kubectl logs -n ns -f p --tail 5", "kview kubectl get pods -A -o wide"},
		{"o", "kubectl get deploy/d -o yaml --context c", "kview kubectl get --context c deploy -A -o wide"},
		{"O", "kubectl get pods -A -l app=x -w", "kview kubectl get -l app=x pods -A -o wide"},
		{"O", "kubectl get -f x.yaml", "kview kubectl get pods -A -o wide"},
		{"O", "kubectl describe svc s -n ns", "kview kubectl get svc -A -o wide"},
		{"O", "kubectl exec -n ns -it p -- sh", "kview kubectl get pods -A -o wide"},
		{"O", "kubectl delete deploy d -n ns", "kview kubectl get deploy -A -o wide"},
		{"O", "kubectl -n ns", "kview kubectl get pods -A -o wide"},
		{"O", "kubectl", "kview kubectl get pods -A -o wide"},
	}
	for _, c := range cases {
		out, ok := run(c.key, false, strings.Fields(c.in))
		check(t, c.key, c.in, c.want, out, ok)
	}
}

func TestToggle(t *testing.T) {
	cases := []struct {
		key, in, want string
	}{
		{"f", "kubectl logs p", "kubectl logs p -f"},
		{"f", "kubectl logs -f p", "kubectl logs p"},
		{"F", "kubectl logs --follow p", "kubectl logs p"},
		{"f", "kubectl logs --follow=false p", "kubectl logs -f p"},
		{"n", "kubectl logs p --tail 5", "kubectl logs p --tail=100"},
		{"n", "kubectl logs p --tail=100", "kubectl logs p"},
		{"f", "kubectl logs p -- x", "kubectl logs p -f -- x"},
		{"o", "kubectl get pods -o yaml", "kubectl get pods -o wide"},
		{"y", "kubectl get pods -oyaml", "kubectl get pods"},
		{"w", "kubectl get pods | less", "kubectl get pods -w | less"},
		{"e", "kubectl describe pod p", "kubectl describe pod p --show-events=false"},
		{"f", "kubectl get pods", ""},
	}
	for _, c := range cases {
		out, ok := run(c.key, true, strings.Fields(c.in))
		check(t, c.key, c.in, c.want, out, ok)
	}
}

func TestMenu(t *testing.T) {
	m, ok := modeFor(strings.Fields("kubectl logs -f p"))
	if !ok || m.name != "kubectl logs" {
		t.Fatalf("got mode %q (ok=%v), want kubectl logs", m.name, ok)
	}
	for _, o := range m.options {
		if want := o.key == "f"; o.on != want {
			t.Errorf("option %s: on=%v, want %v", o.key, o.on, want)
		}
	}
}

func TestPick(t *testing.T) {
	cases := []struct {
		cmd, row, want string
	}{
		{"kubectl get pods -A -o wide", "kube-system  coredns-x  1/1  Running  0  2d  10.0.0.1",
			"kubectl get -n kube-system pods coredns-x"},
		{"kubectl get --context c deploy -A -o wide", "ns d 1/1 1 1 5d",
			"kubectl get --context c -n ns deploy d"},
		{"kubectl get -n ns pods -o wide -w", "p 1/1 Running", "kubectl get -n ns pods p"},
		{"kubectl get all -A", "ns pod/p 1/1 Running", "kubectl get -n ns pod/p"},
		{"kubectl get pods -A", "", ""},
		{"ls -l", "x", ""},
	}
	for _, c := range cases {
		out, ok := kubectlPick(strings.Fields(c.cmd), c.row)
		check(t, "pick", c.cmd, c.want, out, ok)
	}
}

func TestFaults(t *testing.T) {
	in := strings.Join([]string{
		"\x1b[1mNAMESPACE   NAME   READY   STATUS             RESTARTS   NOMINATED NODE",
		"ns          ok     2/2     \x1b[32mRunning\x1b[0m            0          <none>",
		"ns          crash  0/1     CrashLoopBackOff   7          <none>",
		"ns          half   1/2     Running            0          <none>",
		"ns          job    0/1     Completed          0          <none>",
		"ns          wait   0/1     Pending            0          <none>",
	}, "\n")
	var out strings.Builder
	if err := faults(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n")[1:] {
		names = append(names, strings.Fields(l)[1])
	}
	if got := strings.Join(names, " "); got != "crash half wait" {
		t.Errorf("faulty rows: got %q, want %q", got, "crash half wait")
	}
	if !strings.HasSuffix(strings.Split(out.String(), "\n")[0], "[faults only]") {
		t.Errorf("header not marked: %q", strings.Split(out.String(), "\n")[0])
	}

	// Deployments have READY but no STATUS.
	out.Reset()
	faults(strings.NewReader("NAME READY UP-TO-DATE\nok 3/3 3\nshort 1/3 3\n"), &out)
	if got := out.String(); !strings.Contains(got, "short") || strings.Contains(got, "ok 3/3") {
		t.Errorf("deployments: got %q", got)
	}

	// Not a table: everything passes through, header unmarked.
	out.Reset()
	faults(strings.NewReader("error: You must be logged in\nmore\n"), &out)
	if got := out.String(); got != "error: You must be logged in\nmore\n" {
		t.Errorf("non-table: got %q", got)
	}
}

func check(t *testing.T, key, in, want string, out []string, ok bool) {
	t.Helper()
	got := strings.Join(out, " ")
	if want == "" {
		if ok {
			t.Errorf("%s on %q: want nothing, got %q", key, in, got)
		}
		return
	}
	if !ok || got != want {
		t.Errorf("%s on %q: got %q (ok=%v), want %q", key, in, got, ok, want)
	}
}
