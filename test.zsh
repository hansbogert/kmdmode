#!/usr/bin/env zsh
# End-to-end check: type a line into a real interactive zsh, press Alt-m then a key, dump BUFFER.
emulate -L zsh
zmodload zsh/zpty
root=${0:A:h}
out=$(mktemp)
fail=0

check() {  # check LINE KEY WANT
  local line=$1 key=$2 want=$3
  : >| $out
  zpty -b t "env -i HOME=$HOME TERM=xterm zsh -f -i"
  zpty -w t "PS1='> '; source $root/cmdmode.plugin.zsh; alias kgp='kubectl get pods'"
  zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey '^T' _dump"
  zpty -w -n t "$line"$'\em'"$key"$'\x14'
  local i
  for i in {1..50}; do [[ -s $out ]] && break; sleep 0.05; done
  zpty -d t
  if [[ $(<$out) == "$want" ]]; then
    print "ok   ${(q)line} + $key"
  else
    print "FAIL ${(q)line} + $key: got ${(q)$(<$out)}, want ${(q)want}"
    fail=1
  fi
}

check 'kubectl get pods -n ns pod1 ' H 'kubectl logs -n ns pod1 '
check "kubectl get pod 'my pod' -o wide" H "kubectl logs 'my pod'"
check 'kubectl get deploy d -n ns' D 'kubectl describe -n ns deploy d'
check 'kubectl get deploy d' H 'kubectl get deploy d'
check 'kgp pod1' H 'kubectl logs pod1'
check 'kubectl get pods p' $'\e'x 'kubectl get pods px'
rm -f $out
exit $fail
