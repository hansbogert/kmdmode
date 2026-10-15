# TODO

## Decouple the zsh glue from the Go helper

The helper knows every command, but `cmdmode.plugin.zsh` still hard-codes meaning: `-` opens
options, `:` resources and `.` its type picker, `C` (case-sensitive) runs the context picker, `U`
shows "asking the cluster…", a line ending in `=` (S) leaves cmdmode, deletes are recognised by the
mode name `kubectl delete` and need a capital Y, and `nocontext` triggers the context-first picker.
So most features touch both sides, and a shell that loaded the plugin before an update shows keys
from the new helper that its old plugin doesn't handle ("no 'C' here").

1. **Ship the glue in the binary**, as `kubectl completion zsh` does: `cmdmode init zsh` prints the
   plugin (`go:embed`), and `.zshrc` does `source <(~/src/cmdmode/bin/cmdmode init zsh)`. Add a
   version handshake: `menu` reports the glue version it expects, and a shell whose glue differs
   re-sources `cmdmode init zsh` itself. This ends the stale-plugin problem. Do this first.
2. **Make the menu protocol declarative**: per key, what kind of interaction it is, not just a
   label, so zsh becomes a generic interpreter with no key letters hard-coded:
   - `rewrite`: most actions
   - `submenu`: `-`, `:`
   - `pick LIST → CMD`: fzf over a helper-provided list, choice sent back (C, `:.`)
   - `edit`: leave cmdmode for typing (S's `--replicas=`)
   - `confirm KEY`: delete's capital Y
   - `wait MESSAGE`: U's "asking the cluster…"

   zle mechanics (keymap, `zle -M`, `read -k`, precmd/line-init hooks, history recall, inline fzf)
   stay zsh, shipped by the binary after step 1. More behaviour becomes unit-testable in Go.
   Worth doing before adding many more keys, since each special case added now has to be ported.
