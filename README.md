# kmdmode

A modal keymap for zsh that turns the `kubectl` command you are typing into the next one you need,
a key at a time: from a pod's `get` to its `logs`, `describe`, `exec` or owner, from a deployment
to a `rollout restart` or `scale`, plus a k9s-style live view (`kview`) of any list, all as plain
command lines you can read, edit and run, and that land in your shell history.

```sh
go install github.com/hansbogert/kmdmode@latest
echo 'source <(kmdmode init zsh)' >> ~/.zshrc
```

See [Install](#install) for the requirements.

Type `kubectl get pods -n ns pod1`, press **Alt-k**, and the message line names the mode and its
keys:

```
kmdmode [kubectl get]  [l] logs  [i] describe  [-] options  [:] resource  [Esc] exit
```

Press **l** and the line becomes `kubectl logs -n ns pod1`; the mode follows the line, so you are
now in `[kubectl logs]`. Press **-** for that mode's flags, each a key that toggles it (`*` = on):

```
kmdmode [kubectl logs -]  [f] -f*  [p] -p  [t] --timestamps  [c] --all-containers  ...  [Esc] back
```

A line you **run from kmdmode** (Enter) comes back in kmdmode once it ends, e.g. after
`kubectl logs -f` on ctrl-c, so the next key carries on from it; Esc on that returned line clears
it. On an **empty prompt**, Alt-k brings back the most recent command (of the last 20) that has a
mode, so after `kubectl logs ...` ends you carry on from it; Esc on it goes back to the empty prompt.
With no kubectl command in recent history, it starts from the pods overview, as o gives it
(`kview kubectl get pods -A -o wide`), in kmdmode. Keys are case-sensitive and lowercase, except the ones you should not hit by accident: **C** (context), **D** (delete) and **Y** (confirming a delete).

**`:`** switches the resource type the line lists, keeping where it looks (context, `-n`/`-A`,
`-o`) and its label selector (`-l app=web` finds that app's deployments too); names and field
selectors belong to the old type: `[p] pods  [d] deployments  [s] statefulsets
[a] daemonsets  [c] configmaps  [e] secrets`, or `[.] any…`, an fzf picker below the prompt over every listable type
the line's cluster serves (`kubectl api-resources`, so built-ins and every CRD the apiextensions API
registered, qualified like `certificates.cert-manager.io`). On a `kview` line it stays watched.

**Colours.** In kmdmode the line's verb (`get`, `logs`, ...) shows in cyan, and so do the actions
in the hint that change it (`l` logs, `g` get, `d` describe, `D` delete, `x` exec, `r` restart,
`s` scale, `t` status). The hint is drawn below the line as part of zle's display
(`POSTDISPLAY`), not as a `zle -M` message, which can't be coloured; it is cleared when the line is
run or left, so it doesn't stay in the scrollback.

**Context first.** When the line names no `--context`, entering kmdmode opens the context picker
before anything else, so nothing acts on whatever context the kubeconfig happens to point at. The
current one is listed first (one Enter keeps it, now explicit on the line); Esc there leaves kmdmode
with the line untouched. Lines kmdmode hands back carry the context along, so this asks once per
flow. `KMDMODE_CONTEXT_FIRST=0` turns it off.

**x** (exec) on a line naming one pod gives `kubectl exec -it -n NS POD -- sh`, and on one naming a
workload (deployment, statefulset, daemonset, replicaset) `... -it deploy/NAME -- sh`, where kubectl
picks the first of its pods, staying in kmdmode
on the exec line: **b** there (shown as `[b] bash` or `[b] sh`) switches the shell to bash and back (sh is the default; nearly
every image has it, many lack bash), Enter runs it, and Esc goes up to the pod's get.

**C** (capital only; lowercase `c` is not it) picks a kubeconfig context in fzf below the prompt,
headed by the one the line uses now, and sets `--context NAME` on the line: replacing its own, or
added right after `kubectl`. Only the line changes, not the kubeconfig's current context, so other
terminals are unaffected. On a `kview` line it goes into the watched command.

**z** undoes the last change kmdmode made to the line (an action, a toggle, `:`, `C`, Esc up, Tab),
one at a time, staying in the menu you're in; the hint shows `[z] undo` while there is something
to undo. The history lasts for the command line.

**Tab** completes as it does outside kmdmode (through whatever your normal keymap binds Tab to,
e.g. fzf's or fzf-tab's completion) and then shows the menu for the completed line.

**Esc** goes up one menu: out of the option list (toggles kept), then from an action's menu
(`logs`, `rollout`, `scale`, `describe`, `delete`) to the get menu of the resource it acts on,
e.g. `kubectl rollout restart -n ns deploy/d` up to `kubectl get -n ns deploy/d`, and from a
`kview ...` line (after w or o) to the plain command it watches, then, at a top menu, out of
kmdmode. The hint says `[Esc] up` or `[Esc] exit`. **Enter** runs the line (in every menu, so the hint leaves it out).

## Install

Needs zsh 5.9+, kubectl and curl (and Go 1.23+ to build it). [fzf](https://github.com/junegunn/fzf)
is built in, at a version kview needs (0.59+, newer than some distributions ship), so whatever
fzf you have, if any, is left alone. [kubecolor](https://github.com/kubecolor/kubecolor) is optional
and colours kview.

```sh
go install github.com/hansbogert/kmdmode@latest     # or a binary from the releases page
```

Then in `~/.zshrc`, after your other plugins load:

```zsh
source <(kmdmode init zsh)
```

and `exec zsh`. `kmdmode init zsh` prints the zsh side from the binary, as `kubectl completion zsh`
does, so the two always match. After an upgrade, shells already open re-read it on their next
Alt-k.

Press **Alt-k** on a `kubectl` line. Set `KMDMODE_KEY` before the `source` line for another key,
e.g. `KMDMODE_KEY='^[m'` for Alt-m, and `KMDMODE_CONTEXT_FIRST=0` to skip choosing a context first.
`kview` is available as a shell function, and as `kmdmode kview` everywhere else.

## How it works

The widget splits the buffer with `${(z)BUFFER}` (quotes stay inside the words, so the line
round-trips), expands a leading alias, and asks the Go helper:

```sh
kmdmode menu -- kubectl get pods -n ns pod1       # mode, action, option ... lines
kmdmode apply l -- kubectl get pods -n ns pod1    # kubectl logs -n ns pod1
```

The helper rewrites the last simple command that has an action, so env prefixes
(`KUBECONFIG=x kubectl ...`), pipes and `&&` chains are left alone.

## Modes

| Mode               | Actions                                       | Options (after `-`)                                    |
|--------------------|-----------------------------------------------|--------------------------------------------------------|
| `kubectl get`      | l logs, d describe, D delete, r restart, s scale, f forward, w watch, o overview | w `-w`, o `-o wide`, y `-o yaml`, a `-A` (drops `-n`), l `--show-labels`, s sort by age |
| `kubectl logs`     | g get, d describe, D delete, r restart, s scale, f forward, o overview | f `-f`, p `-p`, t `--timestamps`, c `--all-containers`, x `--prefix`, n `--tail=100`, s `--since=1h` |
| `kubectl describe` | l logs, g get, D delete, r restart, s scale, f forward, w watch, o overview | e `--show-events=false`                                |
| `kubectl delete`   | l logs, g get, d describe, r restart, s scale, o overview |
| `kubectl rollout ...`, `kubectl scale ...` | g get, d describe, l logs, r restart, s scale, t status (minus its own), f forward, o overview | |
| `kubectl port-forward` | g get, d describe, p local port, o overview | |                                                        |
| any other `kubectl ...` | o overview                               |                                                        |
| `kview kubectl ...` (after w or o) | w unwatch (drops kview)    | those of the kubectl command inside, kview kept        |

**r** (`kubectl rollout restart`), **s** (`kubectl scale ... --replicas=`) and **t** (`kubectl rollout
status`, which follows a rollout until done) are offered for named workloads of a kind the command
accepts: restart and status for deployments, statefulsets and daemonsets, scale for deployments,
statefulsets and replicasets. s leaves kmdmode with the cursor after
`--replicas=`, so you type the count and Enter.

**u** (owner) goes from any single named resource to what owns it, e.g. from a pod to what runs
it, so r, s and t are in reach from the pods you browse: `kubectl get -n ns pods web-7d9f8-x2k` → u
→ `kubectl get -n ns deploy/web`. It asks the cluster on the keypress (the menu itself never
does), following ownerReferences up: pod → ReplicaSet → Deployment, pod → Job → CronJob, a Secret
→ the operator object that made it (`clickhousecluster.clickhouse.com/analytics`; kinds without a
short name are written kind.group, which kubectl resolves for any CRD). A Deployment is read off
its ReplicaSet's name (`<deployment>-<pod-template-hash>`), so a pod takes one call. Something you
created yourself, like most Certificates, has no owner, and u says so.

**f** (forward) on a line naming one pod, service or workload writes a port-forward to it of the
ports it declares, which it asks the cluster for on the keypress, like u: a service's ports, else
its containers' (a workload's from its pod template). `kubectl get -n ns svc web` → f →
`kubectl port-forward -n ns svc/web 8080:80 9090`. A port below 1024 is forwarded from 8000 above
it (80 from 8080, 443 from 8443), so it needs no root; when that local port is also one it
declares, kubectl picks a free one (`:80`). With no ports declared, the line ends after the target
and the keyboard is yours to type them. Enter runs it, Ctrl-C ends it, and the line comes back in
kmdmode, where g and d (and Esc) go back to the resource. For another local port, **p** (local
port) there empties the first port's local side and hands back the keyboard with the cursor on
it: `8080:80` → p → `|:80`, so typing `9000` gives `9000:80`.

**v** (decode) on a line naming one secret shows its values, base64-decoded: `kubectl get -n ns
secret db` → v → `kubectl get -n ns secret db -o 'go-template={{range $k, $v := .data}}{{$k}}:
{{$v | base64decode}}{{"\n"}}{{end}}'`. That is kubectl's own `base64decode`, the same as piping
each value through `base64 -d`, but it keeps the line a single kubectl command, which kmdmode and
kview still understand. v on that line goes back. Only the command lands in your history, not the
values.

Delete is a capital **D**, like the other keys not to hit by accident (C, Y). **D is offered only when the line names
specific resources** (`pods p`, `deploy/d`), never for a bare type, `-l`, `-A`, `--all` or `-f`,
where it would delete everything that matches; from `logs`, only for a plain pod name, since
`logs deploy/d` reads one pod and deleting the deployment is a bigger step. d only rewrites the line, and
Enter on a delete line asks first: it runs only on a capital **Y**; any other key, lowercase `y`
included, cancels and stays in kmdmode. l is offered for exactly one pod, pods picked by `-l`,
or one workload (`kubectl logs deploy/NAME`, one of its pods).

**o overview** is in every kubectl mode: a k9s-style live list of the line's resource type across
all namespaces, `kview kubectl get <type> -A -o wide`. The type is the one `get`/`describe`/
`delete`/... names, else pods (for `logs`, `exec`, bare `kubectl -n ns`). It is the same view from
any line: only the flags that pick the cluster and identity (`--context`, `--kubeconfig`, `--as`,
...) carry over; names, `-n`, `-l` and other selectors, sorting and output flags reset.

`kview CMD...` (`kmdmode kview` under the hood) shows CMD's output in fzf: scroll
it, type to filter (substrings, in the output's order: fzf's `--exact`, where `'term` is fuzzy
again), the first line stays pinned as the header, and CMD re-runs `$KVIEW_INTERVAL`
(default 2) seconds after each fetch. ctrl-r refreshes now, Esc leaves. kubectl output is colored
through [kubecolor](https://github.com/kubecolor/kubecolor) when it is on PATH
(`go install github.com/kubecolor/kubecolor@v0.8.0`).

When CMD fails, for instance because the session to the cluster expired, the header says why in
one line (the credential helper's own message, such as aws's "Token has expired"), over the last
output that worked, and it retries less and less often, up to once a minute. CMD runs without the
terminal as its input, so a login helper can't prompt into the view: log in again in another
shell, then ctrl-r. Likewise `u` and `:` `.`, which ask the cluster, show why they couldn't on a
line of their own above the menu.

It has two layouts. `kubectl get` output is a **table**: the cursor keeps its place across
refreshes (its position, so a pod added or removed above it shifts which row that is; matching by
name would block keys after each refresh and lose an Enter now and then), ctrl-z filters to
faults, and
Enter picks the row (below). Anything else, like `kubectl describe`, is a **document**: long lines
wrap, and Enter puts the command itself back on the prompt. A `get` with `-o yaml`, `json`,
`name` or a template is a document too. In a table of secrets, **alt-d** opens a pane beside it
with the selected secret's values, decoded, fetched only while the pane is open; for one secret,
alt-d switches the document between the secret and its decoded values (v's two views). Either way
the prompt it returns to
is already in kmdmode, so one key carries on (e.g. l for logs of the picked pod). **w** (watch) in a get or describe mode wraps the line in kview.

kview fetches outside fzf: a background loop runs CMD into a file and then tells fzf, over a
private socket, to reload from that file. While a reload runs, fzf drops cursor keys and holds
Enter, so it must never wait on kubectl. That needs fzf 0.59+, which is why kmdmode carries its own
(`kmdmode fzf`); `KMDMODE_FZF` points kview at another, which must then be 0.59+.

**ctrl-z** (as in k9s) toggles to faulty rows only, marked `[faults only]` in the header: a STATUS
other than Running/Completed/Succeeded, or a READY count short of its total (which also catches
deployments and statefulsets). The filter is `kmdmode faults`, which kview pipes the command
through while the toggle is on.

**Alt-L** picks which of the table's columns to show (Tab toggles, Enter applies at once;
NAMESPACE and NAME always stay, since picking a row reads them). It starts from what is shown,
or, the first time, everything but IP, NOMINATED NODE and READINESS GATES. `kview -c COL,...`
starts with a choice, and leaving with Esc after changing columns returns that command, updated,
in kmdmode. Columns are cut from kubectl's own table rather than `-o custom-columns`, which can't
compute READY, a STATUS like CrashLoopBackOff, RESTARTS or AGE.

**Alt-S** sorts by age, newest first, marked `[newest first]` in the header: kubectl's
`--sort-by=.metadata.creationTimestamp` (oldest first) with the rows reversed, so the newest is at
the top where the list starts. In plain kubectl output the same flag (the get option `s` under `-`)
leaves the newest at the bottom, just above the prompt. Esc after toggling returns the command
with or without the flag.

**Alt-Enter** on a row fetches its labels into a second fzf: tick any (Tab), Enter, and you are
back in kmdmode on a get of everything of that type in the row's namespace carrying them, e.g.
`kubectl get -n cert-manager pods -l app=cert-manager,app.kubernetes.io/component=controller`.
(Shift-Enter would be nicer, but most terminals, GNOME Terminal included, send it as plain Enter.)

**Enter** on a row puts a command for that resource on your next prompt, e.g. Enter on
`kube-system coredns-x ...` gives `kubectl get -n kube-system pods coredns-x`, ready for Alt-k.
kview writes the line to `$KMDMODE_PICK_FILE` and the plugin's precmd hook loads it with
`print -z`; without the plugin, kview prints the line instead.

An option toggles: on removes it, a different value (`-o yaml` when pressing o) is replaced,
absent is added before any `--`.

When an action switches subcommand, global flags (`-n`, `--context`, ...) carry over, and so do
the flags the target subcommand also has, matched by long name: `get -f` is `--filename`, which
describe has, while `logs -f` is `--follow`, which get does not. The rest is dropped. l is not
offered with `-A`, `-f` or `-k`, or for more than one pod, because `kubectl logs` cannot do those.
The flag tables in `kubectl.go` come from `kubectl <sub> --help` and `kubectl options`.

To add a mode, return it from `commandMode` in `line.go`.

## Development

```sh
make test       # Go table tests plus a zpty-driven run of the real widget
```

`make` builds `bin/kmdmode`. After changing dependencies, `make THIRD_PARTY_LICENSES` regenerates
the licenses of the modules compiled in (embedded, and shipped with releases). To try a checkout
without installing it, source the plugin from it: it then uses the checkout's `bin/`.

```zsh
source ~/src/kmdmode/kmdmode.plugin.zsh
```

Releases are built by [GoReleaser](https://goreleaser.com) from a `v*` tag
(`.github/workflows/release.yml`); CI runs `make test` on every push.

## License

Copyright 2026 Hans van den Bogert. Licensed under the [Apache License, Version 2.0](LICENSE).

The binary includes fzf and the modules it uses, under their own licenses (MIT, BSD-3-Clause):
see [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES), or `kmdmode licenses`.
