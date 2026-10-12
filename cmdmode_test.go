package main

import (
	"strings"
	"testing"
)

func TestApply(t *testing.T) {
	cases := []struct {
		key, in, want string // want "" means the key must not apply
	}{
		{"L", "kubectl get pods -n ns pod1", "kubectl logs -n ns pod1"},
		{"L", "kubectl -n ns get pod pod1", "kubectl logs -n ns pod1"},
		{"L", "kubectl get po pod1 --context prod -o wide", "kubectl logs --context prod pod1"},
		{"L", "kubectl get pod/pod1 -nns", "kubectl logs -nns pod1"},
		{"L", "kubectl get pods --namespace=ns -l app=api", "kubectl logs --namespace=ns -l app=api"},
		{"L", "k get pods 'pod1'", "k logs 'pod1'"},
		{"L", "KUBECONFIG=x kubectl get pods p | less", "KUBECONFIG=x kubectl logs p | less"},
		{"L", "cd /tmp && kubectl get pods p", "cd /tmp && kubectl logs p"},
		{"L", "kubectl get pods", ""},
		{"L", "kubectl get pods -A -l app=api", ""},
		{"L", "kubectl get pods a b", ""},
		{"L", "kubectl get deploy d", ""},
		{"L", "ls -l", ""},
		{"I", "kubectl get deploy d -n ns -o yaml", "kubectl describe -n ns deploy d"},
		{"I", "kubectl get pods -A -l app=api", "kubectl describe -A -l app=api pods"},
		{"X", "kubectl get pods p", ""},
		{"l", "kubectl get pods p", "kubectl logs p"},
		{"i", "kubectl get pods p", "kubectl describe pods p"},
		{"L", "kubectl get -f pods.yaml", ""},
		{"I", "kubectl get -f pods.yaml -o name", "kubectl describe -f pods.yaml"},
		{"I", "kubectl get -R -f dir deploy", "kubectl describe -R -f dir deploy"},
		{"L", "kubectl get pods p -w --show-labels --sort-by .x", "kubectl logs p"},
		{"L", "kubectl get pods p --kuberc k --proxy-url u", "kubectl logs --kuberc k --proxy-url u p"},
		// logs mode: -f is --follow here, so it must not become get's --filename.
		{"G", "kubectl logs -n ns -f p --tail 5", "kubectl get -n ns pod p"},
		{"I", "kubectl logs deploy/d", "kubectl describe deploy/d"},
		{"G", "kubectl logs -l app=api", "kubectl get -l app=api pods"},
		{"G", "kubectl logs", ""},
		{"L", "kubectl describe pod p -n ns", "kubectl logs -n ns p"},
		{"G", "kubectl describe -f x.yaml", "kubectl get -f x.yaml"},
		// delete only for named resources, never for lists, selectors, -A or files.
		{"D", "kubectl get pods p -n ns -o wide", "kubectl delete -n ns pods p"},
		{"d", "kubectl get deploy/d --context c", "kubectl delete --context c deploy/d"},
		{"D", "kubectl describe pod p", "kubectl delete pod p"},
		{"D", "kubectl logs -n ns -f p", "kubectl delete -n ns pod p"},
		{"D", "kubectl get pods", ""},
		{"D", "kubectl get pods -l app=api", ""},
		{"D", "kubectl get pods p -A", ""},
		{"D", "kubectl get -f pods.yaml", ""},
		{"D", "kubectl logs deploy/d", ""},
		{"D", "kubectl logs -l app=api", ""},
		{"G", "kubectl delete pod p --grace-period 0 -n ns", "kubectl get -n ns pod p"},
		{"I", "kubectl delete pod p", "kubectl describe pod p"},
		{"L", "kubectl delete pod p -n ns", "kubectl logs -n ns p"},
		// watch: the line itself in kview, which refreshes, so -w goes.
		{"W", "kubectl describe pod p -n ns", "kview kubectl describe pod p -n ns"},
		{"w", "kubectl get pods -n ns -w -o wide", "kview kubectl get pods -n ns -o wide"},
		{"W", "kubectl logs -f p", ""},
		// a kview line keeps a mode: W takes kview off, the inner command's actions are not offered.
		{"W", "kview kubectl describe pod p -n ns", "kubectl describe pod p -n ns"},
		{"W", "kview -c STATUS,NODE kubectl get pods -A -o wide", "kubectl get pods -A -o wide"},
		{"L", "kview kubectl get pods p", ""},
		{"O", "kview kubectl get pods -A -o wide", ""},
		// restart and scale: named workloads only, of a kind the command accepts.
		{"R", "kubectl get deploy d -n ns -o wide", "kubectl rollout restart -n ns deploy/d"},
		{"r", "kubectl get deployments.apps/d --context c", "kubectl rollout restart --context c deploy/d"},
		{"R", "kubectl describe sts a b -n ns", "kubectl rollout restart -n ns sts/a sts/b"},
		{"R", "kubectl get ds/agent -n kube-system", "kubectl rollout restart -n kube-system ds/agent"},
		{"R", "kubectl logs -n ns deploy/d -f", "kubectl rollout restart -n ns deploy/d"},
		{"R", "kubectl get pods p", ""},
		{"R", "kubectl get deploy", ""},
		{"R", "kubectl get deploy -l app=x", ""},
		{"R", "kubectl get rs r", ""},
		{"S", "kubectl get deploy d -n ns", "kubectl scale -n ns deploy/d --replicas="},
		{"s", "kubectl get rs/r", "kubectl scale rs/r --replicas="},
		{"S", "kubectl get ds d", ""},
		{"S", "kubectl get pods p", ""},
		{"S", "kubectl logs p", ""},
		// rollout and scale lines: the workloads they name get the usual verbs, minus their own.
		{"G", "kubectl rollout restart -n ns deploy/d", "kubectl get -n ns deploy/d"},
		{"I", "kubectl rollout status deploy/d --context c", "kubectl describe --context c deploy/d"},
		{"L", "kubectl rollout restart -n ns deploy/d", "kubectl logs -n ns deploy/d"},
		{"T", "kubectl rollout restart -n ns deploy/d", "kubectl rollout status -n ns deploy/d"},
		{"R", "kubectl rollout restart -n ns deploy/d", ""},
		{"S", "kubectl rollout restart -n ns deploy/d", "kubectl scale -n ns deploy/d --replicas="},
		{"T", "kubectl rollout status -n ns deploy/d", ""},
		{"R", "kubectl scale -n ns deploy/d --replicas=3", "kubectl rollout restart -n ns deploy/d"},
		{"S", "kubectl scale -n ns deploy/d --replicas=3", ""},
		{"G", "kubectl rollout restart -n ns deploy/a deploy/b", "kubectl get -n ns deploy/a deploy/b"},
		{"L", "kubectl rollout restart -n ns deploy/a deploy/b", ""},
		{"T", "kubectl get sts s -n ns", "kubectl rollout status -n ns sts/s"},
		{"T", "kubectl get rs r", ""},
		{"O", "kubectl rollout restart -n ns deploy/d", "kview kubectl get deploy -A -o wide"},
		{"O", "kubectl scale -n ns sts/s --replicas=2", "kview kubectl get sts -A -o wide"},
		// overview: the type's full list across namespaces, wide, in kview, from any kubectl line.
		{"O", "kubectl logs -n ns -f p --tail 5", "kview kubectl get pods -A -o wide"},
		{"o", "kubectl get deploy/d -o yaml --context c", "kview kubectl get --context c deploy -A -o wide"},
		{"O", "kubectl get pods -A -l app=x -w", "kview kubectl get pods -A -o wide"},
		{"O", "kubectl get -n ns pods -l app=x,tier=y --field-selector a=b --sort-by .x --show-labels --context c",
			"kview kubectl get --context c pods -A -o wide"},
		{"O", "kubectl logs -l app=x --kubeconfig k --as u -f", "kview kubectl get --kubeconfig k --as u pods -A -o wide"},
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
		{"a", "kview kubectl get pods -A -o wide", "kview kubectl get pods -o wide"},
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

func TestPick(t *testing.T) {
	cases := []struct {
		cmd, row, want string
	}{
		{"kubectl get pods -A -o wide", "kube-system  coredns-x  1/1  Running  0  2d  10.0.0.1",
			"kubectl get -n kube-system pods coredns-x"},
		{"kubectl get --context c deploy -A -o wide", "ns d 1/1 1 1 5d",
			"kubectl get --context c -n ns deploy d"},
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
			"kubectl get --context c -n ns deploy -l app=d"},
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
		{"kubectl rollout status deploy/a deploy/b --context c", "kubectl get --context c deploy/a deploy/b"},
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
	if len(m.actions) != 1 || m.actions[0].key != "W" {
		t.Errorf("actions: got %d, want only W", len(m.actions))
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
