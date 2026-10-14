# cmdmode: press $CMDMODE_KEY (default Alt-m) to enter a modal keymap that rewrites the current
# command line, e.g. "kubectl get pods -n ns pod1" then L gives "kubectl logs -n ns pod1".
# The message line names the mode ("kubectl get"); "-" lists that mode's flags to toggle, ":"
# switches the resource type it lists.
# The rewrite rules live in the Go helper; this file only owns the keymap and the buffer.

typeset -g _cmdmode_bin=${CMDMODE_BIN:-${0:A:h}/bin/cmdmode}
[[ -x $_cmdmode_bin ]] || _cmdmode_bin=cmdmode
typeset -g _cmdmode_prev_keymap=main
typeset -g _cmdmode_sub=  # the submenu shown: "options" ("-"), "resources" (":"), or none

# Fills $words from the buffer, expanding a leading alias so "kgp pod1" reads as kubectl.
_cmdmode_words() {
  words=(${(z)BUFFER})
  (( $#words )) && (( $+aliases[$words[1]] )) && words=(${(z)aliases[$words[1]]} "${(@)words[2,-1]}")
}

_cmdmode_leave() {
  zle -K $_cmdmode_prev_keymap
  _cmdmode_sub=
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
    # One message only: clearing it (as _cmdmode_leave does) and then setting it again in the
    # same keypress shifts the prompt up a row on screen.
    zle -K $_cmdmode_prev_keymap
    _cmdmode_sub=
    [[ $2 == quiet ]] && zle -M '' || zle -M "cmdmode: no actions for this line"
    return 1
  fi
  local name msg line has_options= has_resources= has_up=
  for line in $lines; do
    f=("${(@ps:\t:)line}")
    case $f[1] in
      mode) name=$f[2] ;;
      up) has_up=1 ;;
      action) [[ -z $_cmdmode_sub ]] && msg+="  [$f[2]] $f[3]" ;;
      option)
        has_options=1
        [[ $_cmdmode_sub == options ]] && msg+="  [$f[2]] $f[3]${${f[4]:#off}:+*}" ;;
      resource)
        has_resources=1
        [[ $_cmdmode_sub == resources ]] && msg+="  [$f[2]] $f[3]" ;;
    esac
  done
  local esc=exit tail
  [[ -n $has_up ]] && esc=up
  case $_cmdmode_sub in
    options) msg="${1}cmdmode [$name -]${msg}" tail="  [Esc] up" ;;
    resources) msg="${1}cmdmode [$name :]${msg}" tail="  [Esc] up" ;;
    *) msg="${1}cmdmode [$name]${msg}"
       tail="${has_options:+  [-] options}${has_resources:+  [:] resource}  [Esc] $esc" ;;
  esac
  # When too wide, cut the list rather than the keys that always apply.
  (( $#msg + $#tail < COLUMNS )) || msg="${msg[1,COLUMNS-$#tail-2]}…"
  zle -M "$msg$tail"
}

# On an empty prompt, Alt-m starts from the most recent command in history that has a mode, so
# after "kubectl logs ..." ends you can carry on from it. Looks back 20 commands at most; with
# none, it starts from the pods overview, as O gives it. At a top menu, Esc on the recalled line,
# untouched, goes back to the empty prompt.
_cmdmode_recall() {
  local n
  local -a words
  for n in ${${(Onk)history}[1,20]}; do
    BUFFER=$history[$n]
    _cmdmode_words
    if [[ -n $("$_cmdmode_bin" menu -- "${words[@]}") ]]; then
      CURSOR=$#BUFFER
      _cmdmode_returned=$BUFFER  # so Esc on it, untouched, goes back to the empty prompt
      return
    fi
  done
  BUFFER=$("$_cmdmode_bin" apply O -- kubectl)
  CURSOR=$#BUFFER
}

_cmdmode_enter() {
  [[ -z ${BUFFER//[[:space:]]/} ]] && _cmdmode_recall
  _cmdmode_prev_keymap=$KEYMAP
  _cmdmode_sub=
  zle -K cmdmode
  _cmdmode_render
}

_cmdmode_dispatch() {
  if [[ -z $_cmdmode_sub && $KEYS == [-:] ]]; then
    [[ $KEYS == - ]] && _cmdmode_sub=options || _cmdmode_sub=resources
    _cmdmode_render
    return
  fi
  local -a words
  _cmdmode_words
  local out typ
  if [[ $_cmdmode_sub == resources ]]; then
    typ=$KEYS
    if [[ $KEYS == . ]]; then
      _cmdmode_pick_resource || { _cmdmode_render; return }
      typ=$REPLY
    fi
    if out=$("$_cmdmode_bin" retype "$typ" -- "${words[@]}"); then
      _cmdmode_sub=
      BUFFER=$out
      CURSOR=$#BUFFER
      _cmdmode_render "" quiet
    else
      _cmdmode_render "no '$KEYS' here; "
    fi
    return
  fi
  if [[ -z $_cmdmode_sub && $KEYS == [uU] ]]; then  # U asks the cluster: say so while it does
    zle -M "cmdmode: asking the cluster for the owner…"
    zle -R
  fi
  if out=$("$_cmdmode_bin" ${${_cmdmode_sub:+toggle}:-apply} "$KEYS" -- "${words[@]}"); then
    if [[ $out == *= ]]; then
      # A line ending in "=" (S's --replicas=) waits for a value: hand the keyboard back.
      BUFFER=$out
      CURSOR=$#BUFFER
      _cmdmode_leave
      return
    fi
    # Keep a trailing space so you can carry on typing arguments.
    [[ $BUFFER == *[[:space:]] ]] && out+=' '
    BUFFER=$out
    CURSOR=$#BUFFER
    _cmdmode_render "" quiet
  elif [[ -z $_cmdmode_sub && $KEYS == [uU] ]]; then
    _cmdmode_render "no owner found; "
  else
    _cmdmode_render "no '$KEYS' here; "
  fi
}

# ":" then "." offers every resource type the line's cluster serves (built-ins and the CRDs the
# apiextensions API registered) in fzf, below the prompt; sets REPLY to the chosen one. Called
# directly, not in $(...): it redraws through zle, which a subshell can't.
_cmdmode_pick_resource() {
  local fzf=${_cmdmode_bin:h}/fzf types typ
  [[ -x $fzf ]] || fzf=fzf
  zle -M "cmdmode: asking the cluster for its resource types…"
  zle -R
  types=$("$_cmdmode_bin" api-resources -- "${words[@]}") || return 1
  typ=$(print -r -- $types | $fzf --height=40% --layout=reverse --prompt="resource > " \
    --header="enter: list it  esc: cancel")
  zle reset-prompt
  REPLY=$typ
  [[ -n $typ ]]
}

# Esc goes up one menu: out of the option or resource list (toggles kept), then from an action's
# menu (logs, rollout, scale, describe, delete) to the get menu of the resource it acts on, then,
# at a top menu, out of cmdmode.
_cmdmode_escape() {
  local -a words
  local parent
  _cmdmode_words
  if [[ -n $_cmdmode_sub ]]; then
    _cmdmode_sub=
    _cmdmode_render
  elif parent=$("$_cmdmode_bin" up -- "${words[@]}"); then
    BUFFER=$parent
    CURSOR=$#BUFFER
    _cmdmode_render
  else
    # On a prompt cmdmode came back to, Esc means done with it: the untouched line goes too.
    [[ -n $_cmdmode_returned && $BUFFER == "$_cmdmode_returned" ]] && BUFFER=
    _cmdmode_leave
  fi
}

# A line run from cmdmode comes back in cmdmode once it ends (after logs -f, on ctrl-c), so you
# carry on from it.
_cmdmode_run() {
  _cmdmode_rerun=$BUFFER
  _cmdmode_leave
  zle accept-line
}

# kview writes the command for a picked row here. The next prompt starts with it, or else with
# the line run from cmdmode, already in cmdmode, since acting on it is why you came back.
export CMDMODE_PICK_FILE=${XDG_RUNTIME_DIR:-${TMPDIR:-/tmp}}/cmdmode-pick.$$
typeset -g _cmdmode_picked= _cmdmode_rerun= _cmdmode_returned=

_cmdmode_precmd() {
  if [[ -s $CMDMODE_PICK_FILE ]]; then
    print -z -- "$(<$CMDMODE_PICK_FILE) "
    rm -f -- $CMDMODE_PICK_FILE
    _cmdmode_picked=1
  elif [[ -n $_cmdmode_rerun ]]; then
    print -z -- "$_cmdmode_rerun"
    _cmdmode_picked=1
  fi
  _cmdmode_rerun=
}

_cmdmode_line_init() {
  _cmdmode_returned=
  [[ -n $_cmdmode_picked ]] || return
  _cmdmode_picked=
  _cmdmode_returned=$BUFFER
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
