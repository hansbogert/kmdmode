# cmdmode

A zsh modal keymap that rewrites the command line you are typing into a related command.

Type `kubectl get pods -n ns pod1`, press **Alt-m**, and the message line names the mode and its
keys:

```
cmdmode [kubectl get]  [l] logs  [i] describe  [-] options  [:] resource  [Esc] exit
```

Press **l** and the line becomes `kubectl logs -n ns pod1`; the mode follows the line, so you are
now in `[kubectl logs]`. Press **-** for that mode's flags, each a key that toggles it (`*` = on):

```
cmdmode [kubectl logs -]  [f] -f*  [p] -p  [t] --timestamps  [c] --all-containers  ...  [Esc] back
```

A line you **run from cmdmode** (Enter) comes back in cmdmode once it ends, e.g. after
`kubectl logs -f` on ctrl-c, so the next key carries on from it; Esc on that returned line clears
it. On an **empty prompt**, Alt-m brings back the most recent command (of the last 20) that has a
mode, so after `kubectl logs ...` ends you carry on from it; Esc on it goes back to the empty prompt.
With no kubectl command in recent history, it starts from the pods overview, as o gives it
(`kview kubectl get pods -A -o wide`), in cmdmode. Keys are case-sensitive and lowercase, except the two you should not hit by accident: **C** (context) and **Y** (confirming a delete).

**`:`** switches the resource type the line lists, keeping where it looks (context, `-n`/`-A`,
`-o`; names and selectors belong to the old type): `[p] pods  [d] deployments  [s] statefulsets
[a] daemonsets  [c] configmaps  [e] secrets`, or `[.] any…`, an fzf picker below the prompt over every listable type
the line's cluster serves (`kubectl api-resources`, so built-ins and every CRD the apiextensions API
registered, qualified like `certificates.cert-manager.io`). On a `kview` line it stays watched.

**Colours.** In cmdmode the line's verb (`get`, `logs`, ...) shows in cyan, and so do the actions
in the hint that change it (`l` logs, `g` get, `i` describe, `d` delete, `x` exec, `r` restart,
`s` scale, `t` status). The hint is drawn below the line as part of zle's display
(`POSTDISPLAY`), not as a `zle -M` message, which can't be coloured; it is cleared when the line is
run or left, so it doesn't stay in the scrollback.

**Context first.** When the line names no `--context`, entering cmdmode opens the context picker
before anything else, so nothing acts on whatever context the kubeconfig happens to point at. The
current one is listed first (one Enter keeps it, now explicit on the line); Esc there leaves cmdmode
with the line untouched. Lines cmdmode hands back carry the context along, so this asks once per
flow. `CMDMODE_CONTEXT_FIRST=0` turns it off.

**x** (exec) on a line naming one pod gives `kubectl exec -it -n NS POD -- sh`, staying in cmdmode
on the exec line: its option **b** switches the shell to bash and back (sh is the default; nearly
every image has it, many lack bash), Enter runs it, and Esc goes up to the pod's get.

**C** (capital only; lowercase `c` is not it) picks a kubeconfig context in fzf below the prompt,
headed by the one the line uses now, and sets `--context NAME` on the line: replacing its own, or
added right after `kubectl`. Only the line changes, not the kubeconfig's current context, so other
terminals are unaffected. On a `kview` line it goes into the watched command.

**Tab** completes as it does outside cmdmode (through whatever your normal keymap binds Tab to,
e.g. fzf's or fzf-tab's completion) and then shows the menu for the completed line.

**Esc** goes up one menu: out of the option list (toggles kept), then from an action's menu
(`logs`, `rollout`, `scale`, `describe`, `delete`) to the get menu of the resource it acts on,
e.g. `kubectl rollout restart -n ns deploy/d` up to `kubectl get -n ns deploy/d`, and from a
`kview ...` line (after w or o) to the plain command it watches, then, at a top menu, out of
cmdmode. The hint says `[Esc] up` or `[Esc] exit`. **Enter** runs the line (in every menu, so the hint leaves it out).

## Install

Needs zsh, Go 1.22+, curl and kubectl; [kubecolor](https://github.com/kubecolor/kubecolor) is
optional (`go install github.com/kubecolor/kubecolor@v0.8.0`) and colors kview.

```sh
git clone <this repo> ~/src/cmdmode
cd ~/src/cmdmode
make install    # builds bin/cmdmode and bin/fzf (v0.74.4, for kview only), links kview into ~/.local/bin
```

`~/.local/bin` must be on your PATH. Then in `~/.zshrc`, after your plugins load:

```zsh
source ~/src/cmdmode/cmdmode.plugin.zsh
```

and `exec zsh`. The plugin finds the helper next to itself, so any checkout path works. Set
`CMDMODE_KEY` before sourcing to use another key than Alt-m (which replaces zsh's
`copy-prev-shell-word`), and `CMDMODE_BIN` to point at the helper elsewhere. `make test` runs the
Go tests and the zsh end-to-end tests.

## How it works

The widget splits the buffer with `${(z)BUFFER}` (quotes stay inside the words, so the line
round-trips), expands a leading alias, and asks the Go helper:

```sh
cmdmode menu -- kubectl get pods -n ns pod1       # mode, action, option ... lines
cmdmode apply l -- kubectl get pods -n ns pod1    # kubectl logs -n ns pod1
```

The helper rewrites the last simple command that has an action, so env prefixes
(`KUBECONFIG=x kubectl ...`), pipes and `&&` chains are left alone.

## Modes

| Mode               | Actions                                       | Options (after `-`)                                    |
|--------------------|-----------------------------------------------|--------------------------------------------------------|
| `kubectl get`      | l logs, i describe, d delete, r restart, s scale, w watch, o overview | w `-w`, o `-o wide`, y `-o yaml`, a `-A`, l `--show-labels`, s sort by age |
| `kubectl logs`     | g get, i describe, d delete, r restart, s scale, o overview | f `-f`, p `-p`, t `--timestamps`, c `--all-containers`, x `--prefix`, n `--tail=100`, s `--since=1h` |
| `kubectl describe` | l logs, g get, d delete, r restart, s scale, w watch, o overview | e `--show-events=false`                                |
| `kubectl delete`   | l logs, g get, i describe, r restart, s scale, o overview |
| `kubectl rollout ...`, `kubectl scale ...` | g get, i describe, l logs, r restart, s scale, t status (minus its own), o overview | |                                                        |
| any other `kubectl ...` | o overview                               |                                                        |
| `kview kubectl ...` (after w or o) | w unwatch (drops kview)    | those of the kubectl command inside, kview kept        |

**r** (`kubectl rollout restart`), **s** (`kubectl scale ... --replicas=`) and **t** (`kubectl rollout
status`, which follows a rollout until done) are offered for named workloads of a kind the command
accepts: restart and status for deployments, statefulsets and daemonsets, scale for deployments,
statefulsets and replicasets. s leaves cmdmode with the cursor after
`--replicas=`, so you type the count and Enter.

**u** (owner) goes from any single named resource to what owns it, e.g. from a pod to what runs
it, so r, s and t are in reach from the pods you browse: `kubectl get -n ns pods web-7d9f8-x2k` → u
→ `kubectl get -n ns deploy/web`. It asks the cluster on the keypress (the menu itself never
does), following ownerReferences up: pod → ReplicaSet → Deployment, pod → Job → CronJob, a Secret
→ the operator object that made it (`clickhousecluster.clickhouse.com/analytics`; kinds without a
short name are written kind.group, which kubectl resolves for any CRD). A Deployment is read off
its ReplicaSet's name (`<deployment>-<pod-template-hash>`), so a pod takes one call. Something you
created yourself, like most Certificates, has no owner, and u says so.

i is describe as in "inspect", leaving d for delete. **d is offered only when the line names
specific resources** (`pods p`, `deploy/d`), never for a bare type, `-l`, `-A`, `--all` or `-f`,
where it would delete everything that matches; from `logs`, only for a plain pod name, since
`logs deploy/d` reads one pod and deleting the deployment is a bigger step. d only rewrites the line, and
Enter on a delete line asks first: it runs only on a capital **Y**; any other key, lowercase `y`
included, cancels and stays in cmdmode. l is offered for exactly one pod, or
pods picked by `-l`.

**o overview** is in every kubectl mode: a k9s-style live list of the line's resource type across
all namespaces, `kview kubectl get <type> -A -o wide`. The type is the one `get`/`describe`/
`delete`/... names, else pods (for `logs`, `exec`, bare `kubectl -n ns`). It is the same view from
any line: only the flags that pick the cluster and identity (`--context`, `--kubeconfig`, `--as`,
...) carry over; names, `-n`, `-l` and other selectors, sorting and output flags reset.

`kview CMD...` (a script, linked onto PATH by `make install`) shows CMD's output in fzf: scroll
it, type to filter, the first line stays pinned as the header, and CMD re-runs `$KVIEW_INTERVAL`
(default 2) seconds after each fetch. ctrl-r refreshes now, Esc leaves. kubectl output is colored
through [kubecolor](https://github.com/kubecolor/kubecolor) when it is on PATH
(`go install github.com/kubecolor/kubecolor@v0.8.0`).

It has two layouts. `kubectl get` output is a **table**: the cursor keeps its place across
refreshes (its position, so a pod added or removed above it shifts which row that is; matching by
name would block keys after each refresh and lose an Enter now and then), ctrl-z filters to
faults, and
Enter picks the row (below). Anything else, like `kubectl describe`, is a **document**: long lines
wrap, and Enter puts the command itself back on the prompt. Either way the prompt it returns to
is already in cmdmode, so one key carries on (e.g. l for logs of the picked pod). **w** (watch) in a get or describe mode wraps the line in kview.

kview fetches outside fzf: a background loop runs CMD into a file and then tells fzf, over a
private socket, to reload from that file. While a reload runs, fzf drops cursor keys and holds
Enter, so it must never wait on kubectl. This needs a newer fzf than distros ship (0.59+), so
`make` installs fzf v0.74.4 into `bin/` for kview alone; your fzf on PATH is untouched. Without
it, kview falls back to `watch`.

**ctrl-z** (as in k9s) toggles to faulty rows only, marked `[faults only]` in the header: a STATUS
other than Running/Completed/Succeeded, or a READY count short of its total (which also catches
deployments and statefulsets). The filter is `cmdmode faults`, which kview pipes the command
through while the toggle is on.

**Alt-L** picks which of the table's columns to show (Tab toggles, Enter applies at once;
NAMESPACE and NAME always stay, since picking a row reads them). It starts from what is shown,
or, the first time, everything but IP, NOMINATED NODE and READINESS GATES. `kview -c COL,...`
starts with a choice, and leaving with Esc after changing columns returns that command, updated,
in cmdmode. Columns are cut from kubectl's own table rather than `-o custom-columns`, which can't
compute READY, a STATUS like CrashLoopBackOff, RESTARTS or AGE.

**Alt-S** sorts by age, newest first, marked `[newest first]` in the header: kubectl's
`--sort-by=.metadata.creationTimestamp` (oldest first) with the rows reversed, so the newest is at
the top where the list starts. In plain kubectl output the same flag (the get option `s` under `-`)
leaves the newest at the bottom, just above the prompt. Esc after toggling returns the command
with or without the flag.

**Alt-Enter** on a row fetches its labels into a second fzf: tick any (Tab), Enter, and you are
back in cmdmode on a get of everything of that type in the row's namespace carrying them, e.g.
`kubectl get -n cert-manager pods -l app=cert-manager,app.kubernetes.io/component=controller`.
(Shift-Enter would be nicer, but most terminals, GNOME Terminal included, send it as plain Enter.)

**Enter** on a row puts a command for that resource on your next prompt, e.g. Enter on
`kube-system coredns-x ...` gives `kubectl get -n kube-system pods coredns-x`, ready for Alt-m.
kview writes the line to `$CMDMODE_PICK_FILE` and the plugin's precmd hook loads it with
`print -z`; without the plugin, kview prints the line instead.

An option toggles: on removes it, a different value (`-o yaml` when pressing o) is replaced,
absent is added before any `--`.

When an action switches subcommand, global flags (`-n`, `--context`, ...) carry over, and so do
the flags the target subcommand also has, matched by long name: `get -f` is `--filename`, which
describe has, while `logs -f` is `--follow`, which get does not. The rest is dropped. l is not
offered with `-A`, `-f` or `-k`, or for more than one pod, because `kubectl logs` cannot do those.
The flag tables in `kubectl.go` come from `kubectl <sub> --help` and `kubectl options`.

To add a mode, return it from `commandMode` in `line.go`.

## Test

```sh
make test       # Go table tests plus a zpty-driven run of the real widget
```
