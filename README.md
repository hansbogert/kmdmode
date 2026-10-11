# cmdmode

A zsh modal keymap that rewrites the command line you are typing into a related command.

Type `kubectl get pods -n ns pod1`, press **Alt-m**, and the message line names the mode and its
keys:

```
cmdmode [kubectl get]  [H] logs  [D] describe  [-] options  [Esc] exit  [Enter] run
```

Press **H** and the line becomes `kubectl logs -n ns pod1`; the mode follows the line, so you are
now in `[kubectl logs]`. Press **-** for that mode's flags, each a key that toggles it (`*` = on):

```
cmdmode [kubectl logs -]  [f] -f*  [p] -p  [t] --timestamps  [c] --all-containers  ...  [Esc] back
```

Keys are case-insensitive. **Esc** goes back from the options, then leaves the mode; **Enter**
runs the line.

## Install

```sh
make install    # builds bin/cmdmode and links kview into ~/.local/bin
```

Then in `~/.zshrc`, after the plugins load:

```zsh
source ~/src/cmdmode/cmdmode.plugin.zsh
```

Set `CMDMODE_KEY` before sourcing to use another key than Alt-m (which replaces zsh's
`copy-prev-shell-word`), and `CMDMODE_BIN` to point at the helper elsewhere.

## How it works

The widget splits the buffer with `${(z)BUFFER}` (quotes stay inside the words, so the line
round-trips), expands a leading alias, and asks the Go helper:

```sh
cmdmode hints -- kubectl get pods -n ns pod1      # H<TAB>logs, D<TAB>describe
cmdmode apply H -- kubectl get pods -n ns pod1    # kubectl logs -n ns pod1
```

The helper rewrites the last simple command that has an action, so env prefixes
(`KUBECONFIG=x kubectl ...`), pipes and `&&` chains are left alone.

## Modes

| Mode               | Actions                                       | Options (after `-`)                                    |
|--------------------|-----------------------------------------------|--------------------------------------------------------|
| `kubectl get`      | H logs, I describe, D delete, W watch, O overview | w `-w`, o `-o wide`, y `-o yaml`, a `-A`, l `--show-labels` |
| `kubectl logs`     | G get, I describe, D delete, O overview       | f `-f`, p `-p`, t `--timestamps`, c `--all-containers`, x `--prefix`, n `--tail=100`, s `--since=1h` |
| `kubectl describe` | H logs, G get, D delete, W watch, O overview  | e `--show-events=false`                                |
| `kubectl delete`   | H logs, G get, I describe, O overview         |                                                        |
| any other `kubectl ...` | O overview                               |                                                        |
| `kview kubectl ...` (after W or O) | W unwatch (drops kview)    | those of the kubectl command inside, kview kept        |

I is describe as in "inspect", leaving D for delete. **D is offered only when the line names
specific resources** (`pods p`, `deploy/d`), never for a bare type, `-l`, `-A`, `--all` or `-f`,
where it would delete everything that matches; from `logs`, only for a plain pod name, since
`logs deploy/d` reads one pod and deleting the deployment is a bigger step. Like every action, D
only rewrites the line: nothing runs until you press Enter. H is offered for exactly one pod, or
pods picked by `-l`.

**O overview** is in every kubectl mode: a k9s-style live list of the line's resource type across
all namespaces, `kview kubectl get <type> -A -o wide`. The type is the one `get`/`describe`/
`delete`/... names, else pods (for `logs`, `exec`, bare `kubectl -n ns`); names and `-n` are
dropped, context and `-l` carry over, and an existing `-o`/`-w` is replaced.

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
is already in cmdmode, so one key carries on (e.g. H for logs of the picked pod). **W** (watch) in a get or describe mode wraps the line in kview.

kview fetches outside fzf: a background loop runs CMD into a file and then tells fzf, over a
private socket, to reload from that file. While a reload runs, fzf drops cursor keys and holds
Enter, so it must never wait on kubectl. This needs a newer fzf than distros ship (0.59+), so
`make` installs fzf v0.74.4 into `bin/` for kview alone; your fzf on PATH is untouched. Without
it, kview falls back to `watch`.

**ctrl-z** (as in k9s) toggles to faulty rows only, marked `[faults only]` in the header: a STATUS
other than Running/Completed/Succeeded, or a READY count short of its total (which also catches
deployments and statefulsets). The filter is `cmdmode faults`, which kview pipes the command
through while the toggle is on.

**Enter** on a row puts a command for that resource on your next prompt, e.g. Enter on
`kube-system coredns-x ...` gives `kubectl get -n kube-system pods coredns-x`, ready for Alt-m.
kview writes the line to `$CMDMODE_PICK_FILE` and the plugin's precmd hook loads it with
`print -z`; without the plugin, kview prints the line instead.

An option toggles: on removes it, a different value (`-o yaml` when pressing o) is replaced,
absent is added before any `--`.

When an action switches subcommand, global flags (`-n`, `--context`, ...) carry over, and so do
the flags the target subcommand also has, matched by long name: `get -f` is `--filename`, which
describe has, while `logs -f` is `--follow`, which get does not. The rest is dropped. H is not
offered with `-A`, `-f` or `-k`, or for more than one pod, because `kubectl logs` cannot do those.
The flag tables in `kubectl.go` come from `kubectl <sub> --help` and `kubectl options`.

To add a mode, return it from `commandMode` in `line.go`.

## Test

```sh
make test       # Go table tests plus a zpty-driven run of the real widget
```
