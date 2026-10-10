# cmdmode

A zsh modal keymap that rewrites the command line you are typing into a related command.

Type `kubectl get pods -n ns pod1`, press **Alt-m**, and the message line shows the actions for
that line (`[H] logs  [D] describe  [Esc] cancel`). Press **H** and the line becomes
`kubectl logs -n ns pod1`, ready to edit or run.

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

## Actions

| Line                                   | Key | Result                                          |
|----------------------------------------|-----|-------------------------------------------------|
| `kubectl get pod[s]/po NAME`, `pod/NAME` | H | `kubectl logs NAME`                             |
| `kubectl get pods -l SEL`              | H   | `kubectl logs -l SEL`                           |
| `kubectl get ...`                      | D   | `kubectl describe ...`                          |

Connection flags (`-n`, `--context`, `--kubeconfig`, ...) carry over; `get`-only flags such as
`-o` are dropped. Add a rule by returning another `action` from a function like `kubectlActions`
and listing it in `commandActions`.

## Test

```sh
make test       # Go table tests plus a zpty-driven run of the real widget
```
