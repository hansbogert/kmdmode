# cmdmode: press $CMDMODE_KEY (default Alt-m) to enter a modal keymap that rewrites the current
# command line, e.g. "kubectl get pods -n ns pod1" then H gives "kubectl logs -n ns pod1".
# The rewrite rules live in the Go helper; this file only owns the keymap and the buffer.

typeset -g _cmdmode_bin=${CMDMODE_BIN:-${0:A:h}/bin/cmdmode}
[[ -x $_cmdmode_bin ]] || _cmdmode_bin=cmdmode
typeset -g _cmdmode_prev_keymap=main

# Fills $words from the buffer, expanding a leading alias so "kgp pod1" reads as kubectl.
_cmdmode_words() {
  words=(${(z)BUFFER})
  (( $#words )) && (( $+aliases[$words[1]] )) && words=(${(z)aliases[$words[1]]} "${(@)words[2,-1]}")
}

_cmdmode_enter() {
  local -a words
  _cmdmode_words
  local hints line msg
  hints=$("$_cmdmode_bin" hints -- "${words[@]}") || return 1
  if [[ -z $hints ]]; then
    zle -M "cmdmode: no actions for this line"
    return 1
  fi
  for line in ${(f)hints}; do
    msg+="[${line%%$'\t'*}] ${line#*$'\t'}  "
  done
  _cmdmode_prev_keymap=$KEYMAP
  zle -K cmdmode
  zle -M "cmdmode: ${msg}[Esc] cancel"
}

_cmdmode_dispatch() {
  local -a words
  _cmdmode_words
  local out
  zle -K $_cmdmode_prev_keymap
  if out=$("$_cmdmode_bin" apply "$KEYS" -- "${words[@]}"); then
    # Keep a trailing space so you can carry on typing arguments.
    [[ $BUFFER == *[[:space:]] ]] && out+=' '
    BUFFER=$out
    CURSOR=$#BUFFER
    zle -M ''
  else
    zle -M "cmdmode: no action for '$KEYS'"
  fi
}

_cmdmode_cancel() {
  zle -K $_cmdmode_prev_keymap
  zle -M ''
}

zle -N cmdmode-enter _cmdmode_enter
zle -N cmdmode-dispatch _cmdmode_dispatch
zle -N cmdmode-cancel _cmdmode_cancel

bindkey -N cmdmode
bindkey -M cmdmode -R ' '-'~' cmdmode-dispatch
# Arrow keys are bound explicitly so their escape sequences cancel whole, not as Esc + "[A".
local _k
for _k in '^[' '^G' '^M' '^[[A' '^[[B' '^[[C' '^[[D' '^[OA' '^[OB' '^[OC' '^[OD'; do
  bindkey -M cmdmode $_k cmdmode-cancel
done
unset _k
bindkey ${CMDMODE_KEY:-'^[m'} cmdmode-enter
