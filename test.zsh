#!/usr/bin/env zsh
# End-to-end check: type a line into a real interactive zsh, press Alt-m and then KEYS, and
# compare the resulting BUFFER (dumped by ^T) or, after Enter, what the stub kubectl ran.
emulate -L zsh
zmodload zsh/zpty
root=${0:A:h}
out=$(mktemp)
fail=0

check() {  # check LINE KEYS WANT [TEXT the terminal must show]
  local line=$1 keys=$2 want=$3 shows=$4
  : >| $out
  zpty -b t "env -i HOME=$HOME TERM=xterm zsh -f -i"
  zpty -w t "PS1='> '; source $root/cmdmode.plugin.zsh; alias kgp='kubectl get pods'"
  zpty -w t "_dump() { [[ -n \$BUFFER ]] && print -rn -- \$BUFFER >| $out }; zle -N _dump"
  zpty -w t "bindkey '^T' _dump; bindkey -M cmdmode '^T' _dump"
  zpty -w t "kubectl() { print -rn -- \"ran: kubectl \$*\" >| $out }"
  zpty -w -n t "$line"$'\em'"$keys"$'\x14'
  local i tty chunk
  for i in {1..50}; do [[ -s $out ]] && break; sleep 0.05; done
  while zpty -rt t chunk; do tty+=$chunk; done
  zpty -d t
  local name="${(q)line} + ${(q)keys}"
  if [[ $(<$out) != "$want" ]]; then
    print "FAIL $name: got ${(q)$(<$out)}, want ${(q)want}"; fail=1
  elif [[ $tty == *(bad option|command not found|_cmdmode)* ]]; then
    print "FAIL $name: zsh error on the terminal: ${(q)tty}"; fail=1
  elif [[ -n $shows && $tty != *"$shows"* ]]; then
    print "FAIL $name: terminal never showed ${(q)shows}"; fail=1
  else
    print "ok   $name"
  fi
}

check 'kubectl get pods -n ns pod1 ' H 'kubectl logs -n ns pod1 ' 'cmdmode [kubectl logs]  [G] get'
check "kubectl get pod 'my pod' -o wide" h "kubectl logs 'my pod'"
check 'kubectl get deploy d -n ns' D 'kubectl describe -n ns deploy d' 'cmdmode [kubectl get]  [D] describe'
check 'kubectl get deploy d' H 'kubectl get deploy d' "no 'H' here"
check 'kgp pod1' h 'kubectl logs pod1'
check 'kubectl get pods p' $'\e'x 'kubectl get pods px'
check 'kubectl get pods p' 'h-ft' 'kubectl logs p -f --timestamps' '[f] -f*'
check "kubectl get pods p" "h-f" "kubectl logs p -f" "[f] -f*  [p]"
check 'kubectl logs -f p' '-f'$'\e''g' 'kubectl get pod p' 'cmdmode [kubectl logs -]'
check 'kubectl get pods p' $'h-f\r' 'ran: kubectl logs p -f'
rm -f $out
exit $fail
