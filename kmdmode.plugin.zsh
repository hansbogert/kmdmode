# kmdmode: press $KMDMODE_KEY (default Alt-k) to enter a modal keymap that rewrites the current
# command line, e.g. "kubectl get pods -n ns pod1" then l gives "kubectl logs -n ns pod1".
# The hint below the line names the mode ("kubectl get"); "-" lists that mode's flags to toggle,
# ":" switches the resource type it lists. The verb, and the actions that change it, show in cyan.
# The rewrite rules live in the Go helper; this file only owns the keymap and the buffer.

# `kmdmode init zsh` presets _kmdmode_bin (the binary that printed this) and _kmdmode_glue (its
# version of this code); sourced straight from a checkout, it uses the checkout's bin/.
typeset -g _kmdmode_bin=${KMDMODE_BIN:-${_kmdmode_bin:-${0:A:h}/bin/kmdmode}}
typeset -g _kmdmode_glue=${_kmdmode_glue:-}
[[ -x $_kmdmode_bin ]] || _kmdmode_bin=kmdmode

# kview runs from the binary, so it is never older than the binary.
kview() { "$_kmdmode_bin" kview "$@" }
typeset -g _kmdmode_prev_keymap=main
typeset -ga _kmdmode_undo      # the line before each change kmdmode made to it, for z
typeset -g _kmdmode_sub=  # the submenu shown: "options" ("-"), "resources" (":"), or none
typeset -gi _kmdmode_shift=0  # words a leading alias added (see _kmdmode_words)

# Fills $words from the buffer, expanding a leading alias so "kgp pod1" reads as kubectl;
# _kmdmode_shift is how many words that added, to map a word back to the buffer.
_kmdmode_words() {
  words=(${(z)BUFFER})
  _kmdmode_shift=0
  if (( $#words )) && (( $+aliases[$words[1]] )); then
    local -a expansion=(${(z)aliases[$words[1]]})
    _kmdmode_shift=$(( $#expansion - 1 ))
    words=($expansion "${(@)words[2,-1]}")
  fi
}

# Draws $1 below the command line (POSTDISPLAY) rather than as a zle -M message: zle knows its
# width, and it can be coloured. The other arguments are "start end" character ranges of $1 to
# show in cyan. region_highlight counts POSTDISPLAY on from the end of BUFFER ("P" would mean
# counting from PREDISPLAY, not into POSTDISPLAY); the +1 is the newline.
_kmdmode_show() {
  local text=$1 r base=$(( $#BUFFER + 1 ))
  shift
  _kmdmode_clear
  POSTDISPLAY=$'\n'$text
  for r in "$@"; do
    region_highlight+=("$(( base + ${r% *} )) $(( base + ${r#* } )) fg=cyan,memo=kmdmode")
  done
}

# Removes the hint and the colours; also on leaving the line, so neither stays in the scrollback.
_kmdmode_clear() {
  POSTDISPLAY=
  region_highlight=(${region_highlight:#*memo=kmdmode*})
}

# A short note while something takes a moment, drawn where the hint is.
_kmdmode_note() {
  _kmdmode_show "kmdmode: $1"
  zle -R
}

# Colours the buffer's word number $1 (0-based, in $words, as the helper counts) cyan: the verb.
# A verb inside an expanded alias ("kgp") is not in the buffer, so it stays uncoloured.
_kmdmode_color_verb() {
  local -a tokens=(${(z)BUFFER})
  local n=$(( $1 + 1 - _kmdmode_shift )) i pos=0 tok
  (( n >= 1 && n <= $#tokens && (_kmdmode_shift == 0 || n > 1) )) || return
  for (( i = 1; i <= n; i++ )); do
    tok=$tokens[i]
    while [[ ${BUFFER[pos+1]} == [[:space:]] ]]; do (( pos++ )); done
    [[ ${BUFFER[pos+1,pos+$#tok]} == "$tok" ]] || return
    (( i < n )) && (( pos += $#tok ))
  done
  region_highlight+=("$pos $(( pos + $#tok )) fg=cyan,memo=kmdmode")
}

# Every change kmdmode makes to the line goes through here, so z can take it back.
_kmdmode_change() {
  _kmdmode_undo+=("$BUFFER")
  BUFFER=$1
  CURSOR=$#BUFFER
}

_kmdmode_leave() {
  zle -K $_kmdmode_prev_keymap
  _kmdmode_sub=
  _kmdmode_clear
}

# Shows the current mode's keys, prefixed by an optional note in $1. Kept to one terminal line:
# a message wider than the terminal wraps behind zle's back, which throws its later redraws off by
# a row (garbling the prompt) and scrolls the screen. Leaves when the line has no mode, saying so
# unless $2 is "quiet" (after an action like W, whose kview line has no mode by design).
_kmdmode_render() {
  local -a words lines f
  _kmdmode_words
  lines=(${(f)"$("$_kmdmode_bin" menu -- "${words[@]}")"})
  if (( ! $#lines )); then
    # One message only: clearing it and then setting it again in the same keypress shifts the
    # prompt up a row on screen.
    _kmdmode_leave
    [[ $2 == quiet ]] || zle -M "kmdmode: no actions for this line"
    return 1
  fi
  local name msg line verb= has_options= has_resources= has_up=
  local -a cyan
  for line in $lines; do
    f=("${(@ps:\t:)line}")
    case $f[1] in
      mode) name=$f[2] ;;
      up) has_up=1 ;;
      verb) verb=$f[2] ;;
      action)
        [[ -z $_kmdmode_sub ]] || continue
        [[ $f[4] == verb ]] && cyan+=("$(( $#msg + 2 )) $(( $#msg + 5 + $#f[2] + $#f[3] ))")
        msg+="  [$f[2]] $f[3]" ;;
      option)
        has_options=1
        [[ $_kmdmode_sub == options ]] && msg+="  [$f[2]] $f[3]${${f[4]:#off}:+*}" ;;
      resource)
        has_resources=1
        [[ $_kmdmode_sub == resources ]] && msg+="  [$f[2]] $f[3]" ;;
    esac
  done
  # A prefix ending in a newline (why the cluster could not answer) gets a line of its own above
  # the menu, cut to the width; a short one ("no 'l' here; ") goes in front of it.
  local note= esc=exit tail head="${1}kmdmode [$name"
  if [[ $1 == *$'\n' ]]; then
    note=$1 head="kmdmode [$name"
    (( $#note <= COLUMNS )) || note="${note[1,COLUMNS-2]}…"$'\n'
  fi
  local -a ranges
  [[ -n $has_up ]] && esc=up
  # The mode's verb is the last word of its name, unless that is kubectl itself (no verb).
  [[ $name == *\ * && ${name##* } != kubectl ]] && ranges=("$(( $#head - ${#${name##* }} )) $#head")
  local undo=
  (( $#_kmdmode_undo )) && undo="  [z] undo"
  case $_kmdmode_sub in
    options) head+=" -]" tail="$undo  [Esc] up" ;;
    resources) head+=" :]" tail="$undo  [Esc] up" ;;
    *) head+="]"
       tail="${has_options:+  [-] options}${has_resources:+  [:] resource}$undo  [Esc] $esc" ;;
  esac
  local r
  local -a kept
  for r in $cyan; do  # counted within the list; shift them past the head
    ranges+=("$(( ${r% *} + $#head )) $(( ${r#* } + $#head ))")
  done
  msg=$head$msg
  # When too wide, cut the list rather than the keys that always apply; colours stop at the cut.
  if (( $#msg + $#tail >= COLUMNS )); then
    local cut=$(( COLUMNS - $#tail - 2 ))
    msg="${msg[1,cut]}…"
    for r in $ranges; do
      (( ${r% *} < cut )) && kept+=("${r% *} $(( ${r#* } < cut ? ${r#* } : cut ))")
    done
    ranges=($kept)
  fi
  kept=()
  for r in $ranges; do kept+=("$(( ${r% *} + $#note )) $(( ${r#* } + $#note ))"); done
  _kmdmode_show "$note$msg$tail" $kept
  [[ -n $verb ]] && _kmdmode_color_verb $verb
}

# On an empty prompt, Alt-m starts from the most recent command in history that has a mode, so
# after "kubectl logs ..." ends you can carry on from it. Looks back 20 commands at most; with
# none, it starts from the pods overview, as O gives it. At a top menu, Esc on the recalled line,
# untouched, goes back to the empty prompt.
_kmdmode_recall() {
  local n
  local -a words
  for n in ${${(Onk)history}[1,20]}; do
    BUFFER=$history[$n]
    _kmdmode_words
    if [[ -n $("$_kmdmode_bin" menu -- "${words[@]}") ]]; then
      CURSOR=$#BUFFER
      _kmdmode_returned=$BUFFER  # so Esc on it, untouched, goes back to the empty prompt
      return
    fi
  done
  BUFFER=$("$_kmdmode_bin" apply o -- kubectl)
  CURSOR=$#BUFFER
}

# Unless the line names a context, entering kmdmode asks for one first: acting on whatever context
# the kubeconfig happens to point at is too easy a mistake against production. The current one is
# listed first, so keeping it is one Enter; Esc there leaves the line alone. KMDMODE_CONTEXT_FIRST=0
# turns this off.
_kmdmode_enter() {
  # After an upgrade, a shell loaded before it re-reads this code from the binary first.
  local glue
  if [[ -n $_kmdmode_glue ]] && glue=$("$_kmdmode_bin" glue 2>/dev/null) &&
      [[ -n $glue && $glue != $_kmdmode_glue ]]; then
    eval "$("$_kmdmode_bin" init zsh)"
    _kmdmode_enter "$@"
    return
  fi
  [[ -z ${BUFFER//[[:space:]]/} ]] && _kmdmode_recall
  if [[ ${KMDMODE_CONTEXT_FIRST:-1} != 0 ]]; then
    local -a words
    _kmdmode_words
    if [[ ${(f)"$("$_kmdmode_bin" menu -- "${words[@]}")"} == *nocontext* ]]; then
      local out
      if ! _kmdmode_pick_context; then
        _kmdmode_clear
        zle -M "kmdmode: pick a context to start; Esc cancelled"
        return
      fi
      out=$("$_kmdmode_bin" context "$REPLY" -- "${words[@]}") && BUFFER=$out && CURSOR=$#BUFFER
    fi
  fi
  _kmdmode_prev_keymap=$KEYMAP
  _kmdmode_sub=
  zle -K kmdmode
  _kmdmode_render
}

_kmdmode_dispatch() {
  if [[ $KEYS == z ]]; then  # undo the last change kmdmode made to the line; stays in the menu
    if (( $#_kmdmode_undo )); then
      BUFFER=$_kmdmode_undo[-1]
      CURSOR=$#BUFFER
      shift -p _kmdmode_undo
      _kmdmode_render "" quiet
    else
      _kmdmode_render "nothing to undo; "
    fi
    return
  fi
  if [[ -z $_kmdmode_sub && $KEYS == C ]]; then  # case-sensitive: lowercase c is not context
    local -a words
    _kmdmode_words
    local out
    _kmdmode_pick_context || { _kmdmode_render; return }
    if out=$("$_kmdmode_bin" context "$REPLY" -- "${words[@]}"); then
      _kmdmode_change $out
    fi
    _kmdmode_render "" quiet
    return
  fi
  if [[ -z $_kmdmode_sub && $KEYS == [-:] ]]; then
    [[ $KEYS == - ]] && _kmdmode_sub=options || _kmdmode_sub=resources
    _kmdmode_render
    return
  fi
  local -a words
  _kmdmode_words
  local out typ
  if [[ $_kmdmode_sub == resources ]]; then
    typ=$KEYS
    if [[ $KEYS == . ]]; then
      # Cancelling the picker goes back to the main menu: the ":" list was only the way into it.
      _kmdmode_pick_resource || { _kmdmode_sub=; _kmdmode_render ${REPLY:+"$REPLY"$'\n'}; return }
      typ=$REPLY
    fi
    if out=$("$_kmdmode_bin" retype "$typ" -- "${words[@]}"); then
      _kmdmode_sub=
      _kmdmode_change $out
      _kmdmode_render "" quiet
    else
      _kmdmode_render "no '$KEYS' here; "
    fi
    return
  fi
  if [[ -z $_kmdmode_sub && $KEYS == [uf] ]]; then  # u and f ask the cluster: say so meanwhile
    local what=owner
    [[ $KEYS == f ]] && what=ports
    _kmdmode_note "asking the cluster for the $what…"
  fi
  if out=$("$_kmdmode_bin" ${${_kmdmode_sub:+toggle}:-apply} "$KEYS" -- "${words[@]}"); then
    if [[ $out == *$'\n'cursor$'\t'<-> ]]; then
      # The action puts the cursor mid-line for typing there (p, before ":80"): hand it back.
      _kmdmode_change ${out%%$'\n'*}
      CURSOR=${out##*$'\t'}
      _kmdmode_leave
      zle -M "kmdmode: type the local port, then Enter"
      return
    fi
    if [[ $out == *[=\ ] ]]; then
      # A line ending in "=" (s's --replicas=) or a space (f without declared ports) waits for
      # a value: hand the keyboard back.
      _kmdmode_change $out
      _kmdmode_leave
      [[ $out == *\  ]] && zle -M "kmdmode: no ports declared; type them, e.g. 8080:80"
      return
    fi
    # Keep a trailing space so you can carry on typing arguments.
    [[ $BUFFER == *[[:space:]] ]] && out+=' '
    _kmdmode_change $out
    _kmdmode_render "" quiet
  elif [[ -z $_kmdmode_sub && $KEYS == [uf] && -n $out ]]; then
    _kmdmode_render "$out"$'\n'  # why the cluster could not tell
  elif [[ -z $_kmdmode_sub && $KEYS == u ]]; then
    _kmdmode_render "no owner found; "
  else
    _kmdmode_render "no '$KEYS' here; "
  fi
}

# ":" then "." offers every resource type the line's cluster serves (built-ins and the CRDs the
# apiextensions API registered) in fzf, below the prompt; sets REPLY to the chosen one, or on
# failure to why the cluster could not list them. Called directly, not in $(...): it redraws
# through zle, which a subshell can't.
_kmdmode_pick_resource() {
  local types typ
  local -a fzf=("$_kmdmode_bin" fzf)  # the fzf built into kmdmode
  _kmdmode_note "asking the cluster for its resource types…"
  REPLY=
  types=$("$_kmdmode_bin" api-resources -- "${words[@]}") || { REPLY=$types; return 1 }
  _kmdmode_clear  # fzf draws where the hint was
  zle -R
  typ=$(print -r -- $types | $fzf --height=40% --layout=reverse --prompt="resource > " \
    --header="enter: list it  esc: cancel")
  zle reset-prompt
  REPLY=$typ
  [[ -n $typ ]]
}

# C offers the kubeconfig's contexts in fzf below the prompt, headed by the one the line uses now;
# sets REPLY to the chosen one. Only the line changes, not the kubeconfig's current context.
_kmdmode_pick_context() {
  local ctx
  local -a lines fzf=("$_kmdmode_bin" fzf)  # the fzf built into kmdmode
  lines=(${(f)"$("$_kmdmode_bin" contexts -- "${words[@]}")"}) || return 1
  _kmdmode_clear  # fzf draws where the hint was
  zle -R
  # The one in use first, so keeping it is a single Enter.
  ctx=$(print -rl -- ${lines[1]} ${${lines[2,-1]}:#${lines[1]}} |
    $fzf --height=40% --layout=reverse --prompt="context > " \
    --header="now: ${lines[1]:-?}  enter: use it  esc: cancel")
  zle reset-prompt
  REPLY=$ctx
  [[ -n $ctx ]]
}

# Tab completes as it does outside kmdmode, through whatever the keymap kmdmode came from binds
# it to (fzf's or fzf-tab's completion here), then shows the menu for the completed line.
_kmdmode_complete() {
  local widget=${${(z)"$(bindkey -M $_kmdmode_prev_keymap '^I')"}[2]}
  _kmdmode_clear  # completion lists and pickers draw where the hint was
  local before=$BUFFER
  zle ${widget:-expand-or-complete}
  [[ $BUFFER != "$before" ]] && _kmdmode_undo+=("$before")
  _kmdmode_render "" quiet
}

# Esc goes up one menu: out of the option or resource list (toggles kept), then from an action's
# menu (logs, rollout, scale, describe, delete) to the get menu of the resource it acts on, then,
# at a top menu, out of kmdmode.
_kmdmode_escape() {
  # An arrow key (or Home, End, ...) is Esc plus the rest of its sequence ("[C", "OC", "[1;5C")
  # within a few milliseconds: it leaves kmdmode with the line and cursor as they are, to edit.
  # Read here rather than bound: a binding longer than Esc makes zle wait KEYTIMEOUT after Esc.
  local c seq
  if read -s -t 0.03 -k 1 c; then
    if [[ $c == [\[O] ]]; then
      seq=$c
      while (( $#seq < 2 )) || [[ $seq[-1] != [A-Za-z~] ]]; do
        read -s -t 0.03 -k 1 c || break
        seq+=$c
      done
      _kmdmode_leave
      return
    fi
    zle -U -- "$c"  # a key typed right after Esc still counts, after it
  fi
  local -a words
  local parent
  _kmdmode_words
  if [[ -n $_kmdmode_sub ]]; then
    _kmdmode_sub=
    _kmdmode_render
  elif parent=$("$_kmdmode_bin" up -- "${words[@]}"); then
    _kmdmode_change $parent
    _kmdmode_render
  else
    # On a prompt kmdmode came back to, Esc means done with it: the untouched line goes too.
    [[ -n $_kmdmode_returned && $BUFFER == "$_kmdmode_returned" ]] && BUFFER=
    _kmdmode_leave
  fi
}

# A line run from kmdmode comes back in kmdmode once it ends (after logs -f, on ctrl-c), so you
# carry on from it. A delete runs only after a capital Y: D then Enter is too quick a slip.
_kmdmode_run() {
  local -a words
  _kmdmode_words
  local -a lines=(${(f)"$("$_kmdmode_bin" menu -- "${words[@]}")"})
  if [[ $lines[1] == $'mode\tkubectl delete' ]]; then
    local key
    _kmdmode_note "delete? press Y (capital) to run it, any other key cancels"
    read -k 1 key
    if [[ $key != Y ]]; then
      _kmdmode_render "not deleted; "
      return
    fi
  fi
  _kmdmode_rerun=$BUFFER
  _kmdmode_leave
  zle accept-line
}

# kview writes the command for a picked row here. The next prompt starts with it, or else with
# the line run from kmdmode, already in kmdmode, since acting on it is why you came back.
export KMDMODE_PICK_FILE=${XDG_RUNTIME_DIR:-${TMPDIR:-/tmp}}/kmdmode-pick.$$
typeset -g _kmdmode_picked= _kmdmode_rerun= _kmdmode_returned=

_kmdmode_precmd() {
  if [[ -s $KMDMODE_PICK_FILE ]]; then
    print -z -- "$(<$KMDMODE_PICK_FILE) "
    rm -f -- $KMDMODE_PICK_FILE
    _kmdmode_picked=1
  elif [[ -n $_kmdmode_rerun ]]; then
    print -z -- "$_kmdmode_rerun"
    _kmdmode_picked=1
  fi
  _kmdmode_rerun=
}

_kmdmode_line_finish() {
  _kmdmode_clear
}

_kmdmode_line_init() {
  _kmdmode_undo=()
  _kmdmode_returned=
  [[ -n $_kmdmode_picked ]] || return
  _kmdmode_picked=
  _kmdmode_returned=$BUFFER
  zle kmdmode-enter
}

autoload -Uz add-zsh-hook add-zle-hook-widget
add-zsh-hook precmd _kmdmode_precmd
add-zle-hook-widget line-init _kmdmode_line_init
add-zle-hook-widget line-finish _kmdmode_line_finish

zle -N kmdmode-enter _kmdmode_enter
zle -N kmdmode-dispatch _kmdmode_dispatch
zle -N kmdmode-escape _kmdmode_escape
zle -N kmdmode-leave _kmdmode_leave
zle -N kmdmode-run _kmdmode_run
zle -N kmdmode-complete _kmdmode_complete

bindkey -N kmdmode
bindkey -M kmdmode -R ' '-'~' kmdmode-dispatch
bindkey -M kmdmode '^[' kmdmode-escape '^M' kmdmode-run '^J' kmdmode-run '^G' kmdmode-leave
bindkey -M kmdmode '^I' kmdmode-complete
# No bindings longer than Esc start with Esc here (the arrow keys' sequences): with one, zle waits
# KEYTIMEOUT (0.4s by default) after every Esc for the rest. kmdmode-escape tells an arrow key
# from Esc itself instead.
bindkey ${KMDMODE_KEY:-'^[k'} kmdmode-enter
