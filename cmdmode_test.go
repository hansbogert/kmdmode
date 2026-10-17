package main

import (
	"strings"
	"testing"
)

func TestApply(t *testing.T) {
	cases := []struct {
		key, in, want string // want "" means the key must not apply
	}{
		{"l", "kubectl get pods -n ns pod1", "kubectl logs -n ns pod1"},
		{"L", "kubectl get pods -n ns pod1", ""}, // keys are case-sensitive
		{"l", "kubectl -n ns get pod pod1", "kubectl logs -n ns pod1"},
		{"l", "kubectl get po pod1 --context prod -o wide", "kubectl --context prod logs pod1"},
		{"l", "kubectl get pod/pod1 -nns", "kubectl logs -nns pod1"},
		{"l", "kubectl get pods --namespace=ns -l app=api", "kubectl logs --namespace=ns -l app=api"},
		{"l", "k get pods 'pod1'", "k logs 'pod1'"},
		{"l", "KUBECONFIG=x kubectl get pods p | less", "KUBECONFIG=x kubectl logs p | less"},
		{"l", "cd /tmp && kubectl get pods p", "cd /tmp && kubectl logs p"},
		{"l", "kubectl get pods", ""},
		{"l", "kubectl get pods -A -l app=api", ""},
		{"l", "kubectl get pods a b", ""},
		{"l", "kubectl get deploy d", ""},
		{"l", "ls -l", ""},
		{"i", "kubectl get deploy d -n ns -o yaml", "kubectl describe -n ns deploy d"},
		{"i", "kubectl get pods -A -l app=api", "kubectl describe -A -l app=api pods"},
		{"z", "kubectl get pods p", ""},
		{"l", "kubectl get pods p", "kubectl logs p"},
		{"i", "kubectl get pods p", "kubectl describe pods p"},
		{"l", "kubectl get -f pods.yaml", ""},
		{"i", "kubectl get -f pods.yaml -o name", "kubectl describe -f pods.yaml"},
		{"i", "kubectl get -R -f dir deploy", "kubectl describe -R -f dir deploy"},
		{"l", "kubectl get pods p -w --show-labels --sort-by .x", "kubectl logs p"},
		{"l", "kubectl get pods p --kuberc k --proxy-url u", "kubectl logs --kuberc k --proxy-url u p"},
		// logs mode: -f is --follow here, so it must not become get's --filename.
		{"g", "kubectl logs -n ns -f p --tail 5", "kubectl get -n ns pod p"},
		{"i", "kubectl logs deploy/d", "kubectl describe deploy/d"},
		{"g", "kubectl logs -l app=api", "kubectl get -l app=api pods"},
		{"g", "kubectl logs", ""},
		{"l", "kubectl describe pod p -n ns", "kubectl logs -n ns p"},
		{"g", "kubectl describe -f x.yaml", "kubectl get -f x.yaml"},
		// delete only for named resources, never for lists, selectors, -A or files.
		{"d", "kubectl get pods p -n ns -o wide", "kubectl delete -n ns pods p"},
		{"d", "kubectl get deploy/d --context c", "kubectl --context c delete deploy/d"},
		{"d", "kubectl describe pod p", "kubectl delete pod p"},
		{"d", "kubectl logs -n ns -f p", "kubectl delete -n ns pod p"},
		{"d", "kubectl get pods", ""},
		{"d", "kubectl get pods -l app=api", ""},
		{"d", "kubectl get pods p -A", ""},
		{"d", "kubectl get -f pods.yaml", ""},
		{"d", "kubectl logs deploy/d", ""},
		{"d", "kubectl logs -l app=api", ""},
		{"g", "kubectl delete pod p --grace-period 0 -n ns", "kubectl get -n ns pod p"},
		{"i", "kubectl delete pod p", "kubectl describe pod p"},
		{"l", "kubectl delete pod p -n ns", "kubectl logs -n ns p"},
		// watch: the line itself in kview, which refreshes, so -w goes.
		{"w", "kubectl describe pod p -n ns", "kview kubectl describe pod p -n ns"},
		{"w", "kubectl get pods -n ns -w -o wide", "kview kubectl get pods -n ns -o wide"},
		{"w", "kubectl logs -f p", ""},
		// a kview line keeps a mode: W takes kview off, the inner command's actions are not offered.
		{"w", "kview kubectl describe pod p -n ns", "kubectl describe pod p -n ns"},
		{"w", "kview -c STATUS,NODE kubectl get pods -A -o wide", "kubectl get pods -A -o wide"},
		{"l", "kview kubectl get pods p", ""},
		{"o", "kview kubectl get pods -A -o wide", ""},
		// restart and scale: named workloads only, of a kind the command accepts.
		{"r", "kubectl get deploy d -n ns -o wide", "kubectl rollout restart -n ns deploy/d"},
		{"r", "kubectl get deployments.apps/d --context c", "kubectl --context c rollout restart deploy/d"},
		{"r", "kubectl describe sts a b -n ns", "kubectl rollout restart -n ns sts/a sts/b"},
		{"r", "kubectl get ds/agent -n kube-system", "kubectl rollout restart -n kube-system ds/agent"},
		{"r", "kubectl logs -n ns deploy/d -f", "kubectl rollout restart -n ns deploy/d"},
		{"r", "kubectl get pods p", ""},
		{"r", "kubectl get deploy", ""},
		{"r", "kubectl get deploy -l app=x", ""},
		{"r", "kubectl get rs r", ""},
		{"s", "kubectl get deploy d -n ns", "kubectl scale -n ns deploy/d --replicas="},
		{"s", "kubectl get rs/r", "kubectl scale rs/r --replicas="},
		{"s", "kubectl get ds d", ""},
		{"s", "kubectl get pods p", ""},
		{"s", "kubectl logs p", ""},
		// rollout and scale lines: the workloads they name get the usual verbs, minus their own.
		{"g", "kubectl rollout restart -n ns deploy/d", "kubectl get -n ns deploy/d"},
		{"i", "kubectl rollout status deploy/d --context c", "kubectl --context c describe deploy/d"},
		{"l", "kubectl rollout restart -n ns deploy/d", "kubectl logs -n ns deploy/d"},
		{"t", "kubectl rollout restart -n ns deploy/d", "kubectl rollout status -n ns deploy/d"},
		{"r", "kubectl rollout restart -n ns deploy/d", ""},
		{"s", "kubectl rollout restart -n ns deploy/d", "kubectl scale -n ns deploy/d --replicas="},
		{"t", "kubectl rollout status -n ns deploy/d", ""},
		{"r", "kubectl scale -n ns deploy/d --replicas=3", "kubectl rollout restart -n ns deploy/d"},
		{"s", "kubectl scale -n ns deploy/d --replicas=3", ""},
		{"g", "kubectl rollout restart -n ns deploy/a deploy/b", "kubectl get -n ns deploy/a deploy/b"},
		{"l", "kubectl rollout restart -n ns deploy/a deploy/b", ""},
		{"t", "kubectl get sts s -n ns", "kubectl rollout status -n ns sts/s"},
		{"t", "kubectl get rs r", ""},
		{"o", "kubectl rollout restart -n ns deploy/d", "kview kubectl get deploy -A -o wide"},
		{"o", "kubectl scale -n ns sts/s --replicas=2", "kview kubectl get sts -A -o wide"},
		// exec: one named pod, sh by default; the exec line goes up to the pod's get.
		{"x", "kubectl get -n ns pods p -o wide", "kubectl exec -n ns -it p -- sh"},
		{"x", "kubectl logs --context c -n ns p -f", "kubectl --context c exec -n ns -it p -- sh"},
		{"x", "kubectl describe pod/p", "kubectl exec -it p -- sh"},
		{"x", "kubectl get pods", ""},
		{"x", "kubectl get deploy d", ""},
		{"X", "kubectl get pods p", ""},
		// overview: the type's full list across namespaces, wide, in kview, from any kubectl line.
		{"o", "kubectl logs -n ns -f p --tail 5", "kview kubectl get pods -A -o wide"},
		{"o", "kubectl get deploy/d -o yaml --context c", "kview kubectl --context c get deploy -A -o wide"},
		{"o", "kubectl get pods -A -l app=x -w", "kview kubectl get pods -A -o wide"},
		{"o", "kubectl --context c get -n ns pods -l app=x,tier=y --field-selector a=b --sort-by .x --show-labels",
			"kview kubectl --context c get pods -A -o wide"},
		{"o", "kubectl logs -l app=x --kubeconfig k --as u -f", "kview kubectl get --kubeconfig k --as u pods -A -o wide"},
		{"o", "kubectl get -f x.yaml", "kview kubectl get pods -A -o wide"},
		{"o", "kubectl describe svc s -n ns", "kview kubectl get svc -A -o wide"},
		{"o", "kubectl exec -n ns -it p -- sh", "kview kubectl get pods -A -o wide"},
		{"o", "kubectl delete deploy d -n ns", "kview kubectl get deploy -A -o wide"},
		{"o", "kubectl -n ns", "kview kubectl get pods -A -o wide"},
		{"o", "kubectl", "kview kubectl get pods -A -o wide"},
		{"o", "kubectl() { print hi }", ""}, // a function definition, not a kubectl command
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
		{"f", "kubectl logs --follow p", "kubectl logs p"},
		{"F", "kubectl logs p", ""}, // keys are case-sensitive
		{"f", "kubectl logs --follow=false p", "kubectl logs -f p"},
		{"n", "kubectl logs p --tail 5", "kubectl logs p --tail=100"},
		{"n", "kubectl logs p --tail=100", "kubectl logs p"},
		{"f", "kubectl logs p -- x", "kubectl logs p -f -- x"},
		{"o", "kubectl get pods -o yaml", "kubectl get pods -o wide"},
		{"y", "kubectl get pods -oyaml", "kubectl get pods"},
		{"w", "kubectl get pods | less", "kubectl get pods -w | less"},
		{"e", "kubectl describe pod p", "kubectl describe pod p --show-events=false"},
		{"f", "kubectl get pods", ""},
		{"a", "kview kubectl get pods -A -o wide", "kview kubectl get pods -o wide"},
		// -A on a named resource: kubectl refuses names across namespaces, so the names and -n go.
		{"a", "kubectl get -n ns pods p -o wide", "kubectl get -o wide pods -A"},
		{"a", "kubectl get -n ns deploy/d", "kubectl get deploy -A"},
		{"a", "kubectl get -n ns pods a b --context c", "kubectl --context c get pods -A"},
		{"a", "kview kubectl get -n ns pods p", "kview kubectl get pods -A"},
		{"a", "kubectl get -n ns pods", "kubectl get -n ns pods -A"},
		{"w", "kview kubectl get pods", "kview kubectl get pods -w"},
		{"s", "kubectl get pods -A", "kubectl get pods -A --sort-by=.metadata.creationTimestamp"},
		{"s", "kubectl get pods --sort-by=.metadata.creationTimestamp", "kubectl get pods"},
		{"s", "kubectl get pods --sort-by .metadata.name", "kubectl get pods --sort-by=.metadata.creationTimestamp"},
		{"a", "kview -c STATUS,IP kubectl get pods -A", "kview -c STATUS,IP kubectl get pods"},
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

func TestExecShell(t *testing.T) {
	cases := []struct{ in, want string }{
		{"kubectl exec -n ns -it p -- sh", "kubectl exec -n ns -it p -- bash"},
		{"kubectl exec -n ns -it -c app p -- bash", "kubectl exec -n ns -it -c app p -- sh"},
		{"kubectl exec -it p -- ls -l", ""},
		{"kubectl exec -it p", ""},
	}
	for _, c := range cases {
		out, ok := run("b", true, strings.Fields(c.in))
		check(t, "b", c.in, c.want, out, ok)
	}
	for in, want := range map[string]string{
		"kubectl exec --context c -n ns -it p -- sh": "kubectl --context c get -n ns pod p",
		// naming no pod: up to the pods list where it looks
		"kubectl exec --context c -n tools -h":                   "kubectl --context c get -n tools pods",
		"kubectl exec --context c -n tools -it -l app=web": "kubectl --context c get -n tools pods",
	} {
		if out, ok := up(strings.Fields(in)); !ok || strings.Join(out, " ") != want {
			t.Errorf("exec up from %q: got %q, want %q", in, out, want)
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
			"kubectl --context c get -n ns deploy d"},
		{"kubectl get -n ns pods -o wide -w", "p 1/1 Running", "kubectl get -n ns pods p"},
		{"kubectl get pods -A --sort-by=.metadata.creationTimestamp", "ns p 1/1", "kubectl get -n ns pods p"},
		// a list reached by labels: kubectl rejects a name alongside a selector.
		{"kubectl get -n ns pods -l app=x,tier=y --field-selector status.phase=Running", "p 1/1",
			"kubectl get -n ns pods p"},
		{"kubectl get all -A", "ns pod/p 1/1 Running", "kubectl get -n ns pod/p"},
		{"kubectl get pods -A", "", ""},
		{"ls -l", "x", ""},
	}
	for _, c := range cases {
		out, ok := kubectlPick(strings.Fields(c.cmd), c.row)
		check(t, "pick", c.cmd, c.want, out, ok)
	}
}

func TestSelect(t *testing.T) {
	cases := []struct {
		cmd, row, labels, want string
	}{
		{"kubectl get pods -A -o wide", "kube-system coredns-x 1/1 Running", "k8s-app=kube-dns tier=node",
			"kubectl get -n kube-system pods -l k8s-app=kube-dns,tier=node"},
		{"kubectl get --context c -l old=1 deploy -A", "ns d 1/1", "app=d",
			"kubectl --context c get -n ns deploy -l app=d"},
		{"kubectl get -n ns pods", "p 1/1 Running", "app=x", "kubectl get -n ns pods -l app=x"},
		{"kubectl get all -A", "ns pod/p 1/1", "app=x", "kubectl get -n ns pod -l app=x"},
		{"kubectl get pods -A", "ns p", "", ""},
	}
	for _, c := range cases {
		out, ok := kubectlSelect(strings.Fields(c.cmd), c.row, strings.Fields(c.labels))
		check(t, "select", c.cmd, c.want, out, ok)
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

func TestUp(t *testing.T) {
	cases := []struct {
		in, want string // want "" means a top menu
	}{
		{"kubectl rollout restart -n ns deploy/d", "kubectl get -n ns deploy/d"},
		{"kubectl rollout status deploy/a deploy/b --context c", "kubectl --context c get deploy/a deploy/b"},
		{"kubectl scale -n ns deploy/d --replicas=3", "kubectl get -n ns deploy/d"},
		{"kubectl logs -n ns p -f --tail 5", "kubectl get -n ns pod p"},
		{"kubectl logs -n ns deploy/d", "kubectl get -n ns deploy/d"},
		{"kubectl describe -n ns pods p", "kubectl get -n ns pods p"},
		{"kubectl delete -n ns deploy d", "kubectl get -n ns deploy d"},
		{"kubectl get -n ns deploy d", ""},
		{"kubectl get pods", ""},
		{"kubectl describe pods -l app=x", ""},
		{"kview kubectl get pods -A -o wide", "kubectl get pods -A -o wide"},
		{"kview -c STATUS,NODE kubectl describe pod p", "kubectl describe pod p"},
		{"ls", ""},
	}
	for _, c := range cases {
		out, ok := up(strings.Fields(c.in))
		check(t, "up", c.in, c.want, out, ok)
	}
}

func TestKviewMenu(t *testing.T) {
	m, ok := modeFor(strings.Fields("kview kubectl get pods -A -o wide"))
	if !ok || m.name != "kview kubectl get" {
		t.Fatalf("got mode %q (ok=%v), want kview kubectl get", m.name, ok)
	}
	if len(m.actions) != 1 || m.actions[0].key != "w" {
		t.Errorf("actions: got %d, want only w", len(m.actions))
	}
	if len(m.options) == 0 {
		t.Error("no options")
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
