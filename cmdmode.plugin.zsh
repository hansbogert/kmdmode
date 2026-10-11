# cmdmode: press $CMDMODE_KEY (default Alt-m) to enter a modal keymap that rewrites the current
# command line, e.g. "kubectl get pods -n ns pod1" then H gives "kubectl logs -n ns pod1".
# The message line names the mode ("kubectl get"); "-" lists that mode's flags to toggle.
# The rewrite rules live in the Go helper; this file only owns the keymap and the buffer.

typeset -g _cmdmode_bin=${CMDMODE_BIN:-${0:A:h}/bin/cmdmode}
[[ -x $_cmdmode_bin ]] || _cmdmode_bin=cmdmode
typeset -g _cmdmode_prev_keymap=main
typeset -g _cmdmode_options=  # set while the "-" option list is shown

# Fills $words from the buffer, expanding a leading alias so "kgp pod1" reads as kubectl.
_cmdmode_words() {
  words=(${(z)BUFFER})
  (( $#words )) && (( $+aliases[$words[1]] )) && words=(${(z)aliases[$words[1]]} "${(@)words[2,-1]}")
}

_cmdmode_leave() {
  zle -K $_cmdmode_prev_keymap
  _cmdmode_options=
  zle -M ''
}

# Shows the current mode's keys, prefixed by an optional note in $1. Kept to one terminal line:
# a message wider than the terminal wraps behind zle's back, which throws its later redraws off by
# a row (garbling the prompt) and scrolls the screen. Leaves when the line has no mode, saying so
# unless $2 is "quiet" (after an action like W, whose kview line has no mode by design).
_cmdmode_render() {
  local -a words lines f
  _cmdmode_words
  lines=(${(f)"$("$_cmdmode_bin" menu -- "${words[@]}")"})
  if (( ! $#lines )); then
    _cmdmode_leave
    [[ $2 == quiet ]] || zle -M "cmdmode: no actions for this line"
    return 1
  fi
  local name msg line
  for line in $lines; do
    f=("${(@ps:\t:)line}")
    case $f[1] in
      mode) name=$f[2] ;;
      action) [[ -z $_cmdmode_options ]] && msg+="  [$f[2]] $f[3]" ;;
      option) [[ -n $_cmdmode_options ]] && msg+="  [$f[2]] $f[3]${${f[4]:#off}:+*}" ;;
    esac
  done
  if [[ -n $_cmdmode_options ]]; then
    msg="${1}cmdmode [$name -]${msg}  [Esc] back"
  else
    msg="${1}cmdmode [$name]${msg}  [-] options  [Esc] exit  [Enter] run"
  fi
  (( $#msg < COLUMNS )) || msg="${msg[1,COLUMNS-2]}…"
  zle -M "$msg"
}

# On an empty prompt, Alt-m starts from the most recent command in history that has a mode, so
# after "kubectl logs ..." ends you can carry on from it. Looks back 20 commands at most.
_cmdmode_recall() {
  local n
  local -a words
  for n in ${${(Onk)history}[1,20]}; do
    BUFFER=$history[$n]
    _cmdmode_words
    if [[ -n $("$_cmdmode_bin" menu -- "${words[@]}") ]]; then
      CURSOR=$#BUFFER
      return
    fi
  done
  BUFFER=
}

_cmdmode_enter() {
  [[ -z ${BUFFER//[[:space:]]/} ]] && _cmdmode_recall
  _cmdmode_prev_keymap=$KEYMAP
  _cmdmode_options=
  zle -K cmdmode
  _cmdmode_render
}

_cmdmode_dispatch() {
  if [[ -z $_cmdmode_options && $KEYS == - ]]; then
    _cmdmode_options=1
    _cmdmode_render
    return
  fi
  local -a words
  _cmdmode_words
  local out
  if out=$("$_cmdmode_bin" ${${_cmdmode_options:+toggle}:-apply} "$KEYS" -- "${words[@]}"); then
    # Keep a trailing space so you can carry on typing arguments.
    [[ $BUFFER == *[[:space:]] ]] && out+=' '
    BUFFER=$out
    CURSOR=$#BUFFER
    _cmdmode_render "" quiet
  else
    _cmdmode_render "no '$KEYS' here; "
  fi
}

_cmdmode_escape() {
  if [[ -n $_cmdmode_options ]]; then
    _cmdmode_options=
    _cmdmode_render
  else
    _cmdmode_leave
  fi
}

_cmdmode_run() {
  _cmdmode_leave
  zle accept-line
}

# kview writes the command for a picked row here; the next prompt starts with it, already in
# cmdmode, since acting on it is why you came back.
export CMDMODE_PICK_FILE=${XDG_RUNTIME_DIR:-${TMPDIR:-/tmp}}/cmdmode-pick.$$
typeset -g _cmdmode_picked=

_cmdmode_precmd() {
  [[ -s $CMDMODE_PICK_FILE ]] || return
  print -z -- "$(<$CMDMODE_PICK_FILE) "
  rm -f -- $CMDMODE_PICK_FILE
  _cmdmode_picked=1
}

_cmdmode_line_init() {
  [[ -n $_cmdmode_picked ]] || return
  _cmdmode_picked=
  zle cmdmode-enter
}

autoload -Uz add-zsh-hook add-zle-hook-widget
add-zsh-hook precmd _cmdmode_precmd
add-zle-hook-widget line-init _cmdmode_line_init

zle -N cmdmode-enter _cmdmode_enter
zle -N cmdmode-dispatch _cmdmode_dispatch
zle -N cmdmode-escape _cmdmode_escape
zle -N cmdmode-leave _cmdmode_leave
zle -N cmdmode-run _cmdmode_run

bindkey -N cmdmode
bindkey -M cmdmode -R ' '-'~' cmdmode-dispatch
bindkey -M cmdmode '^[' cmdmode-escape '^M' cmdmode-run '^J' cmdmode-run '^G' cmdmode-leave
# Arrow keys are bound explicitly so their escape sequences leave whole, not as Esc + "[A".
local _k
for _k in '^[[A' '^[[B' '^[[C' '^[[D' '^[OA' '^[OB' '^[OC' '^[OD'; do
  bindkey -M cmdmode $_k cmdmode-leave
done
unset _k
bindkey ${CMDMODE_KEY:-'^[m'} cmdmode-enter
