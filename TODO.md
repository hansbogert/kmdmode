# TODO

## Decouple the zsh glue from the Go helper

The helper knows every command, but `kmdmode.plugin.zsh` still hard-codes meaning: `-` opens
options, `:` resources and `.` its type picker, `C` (case-sensitive) runs the context picker, `u`
shows "asking the cluster…", a line ending in `=` (s) leaves kmdmode, deletes are recognised by the
mode name `kubectl delete` and need a capital Y, and `nocontext` triggers the context-first picker.
So most features touch both sides. (The plugin now ships in the binary, `kmdmode init zsh`, and open
shells re-read it after an upgrade, so the two no longer drift apart.)

**Make the menu protocol declarative**: per key, what kind of interaction it is, not just a
label, so zsh becomes a generic interpreter with no key letters hard-coded:
- `rewrite`: most actions
- `submenu`: `-`, `:`
- `pick LIST → CMD`: fzf over a helper-provided list, choice sent back (C, `:.`)
- `edit`: leave kmdmode for typing (s's `--replicas=`)
- `confirm KEY`: delete's capital Y
- `wait MESSAGE`: u's "asking the cluster…"

zle mechanics (keymap, `zle -M`, `read -k`, precmd/line-init hooks, history recall, inline fzf)
stay zsh. More behaviour becomes unit-testable in Go.
Worth doing before adding many more keys, since each special case added now has to be ported.
