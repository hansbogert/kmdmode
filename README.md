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
make            # builds bin/cmdmode
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

| Mode               | Actions                                   | Options (after `-`)                                    |
|--------------------|-------------------------------------------|--------------------------------------------------------|
| `kubectl get`      | H logs (one pod, or pods `-l`), D describe | w `-w`, o `-o wide`, y `-o yaml`, a `-A`, l `--show-labels` |
| `kubectl logs`     | G get, D describe                         | f `-f`, p `-p`, t `--timestamps`, c `--all-containers`, x `--prefix`, n `--tail=100`, s `--since=1h` |
| `kubectl describe` | H logs (as for get), G get                | e `--show-events=false`                                |

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
