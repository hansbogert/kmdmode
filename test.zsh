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
check 'kubectl get deploy d -n ns' I 'kubectl describe -n ns deploy d' 'cmdmode [kubectl get]  [I] describe  [D] delete'
check 'kubectl get deploy d' H 'kubectl get deploy d' "no 'H' here"
check 'kgp pod1' h 'kubectl logs pod1'
check 'kubectl get pods p' $'\e'x 'kubectl get pods px'
check 'kubectl get pods p' 'h-ft' 'kubectl logs p -f --timestamps' '[f] -f*'
check "kubectl get pods p" "h-f" "kubectl logs p -f" "[f] -f*  [p]"
check 'kubectl logs -f p' '-f'$'\e''g' 'kubectl get pod p' 'cmdmode [kubectl logs -]'
check 'kubectl -n ns ' O 'kview kubectl get pods -A -o wide ' 'cmdmode [kubectl]  [O] overview'
check 'kubectl get pods p' $'h-f\r' 'ran: kubectl logs p -f'
# kview: under a real tty, the header stays pinned and the rows refresh.
kv=$(mktemp)
print -r -- "stty rows 20 cols 100
KVIEW_INTERVAL=0.3 $root/kview zsh -c 'print NAME STATUS; print pod Running \$RANDOM'; print exit=\$?" >| $kv
screen=$( (sleep 2; printf '\e'; sleep 1) | timeout 10 script -qfc "zsh -f $kv" /dev/null | sed 's/\x1b\[[0-9;?]*[a-zA-Z]//g')
rows=$(print -r -- $screen | grep -oE 'Running [0-9]+' | sort -u | wc -l)
if [[ $screen == *'NAME STATUS'* && $screen == *exit=0* ]] && (( rows >= 3 )); then
  print "ok   kview: header pinned, $rows refreshes, Esc exits 0"
else
  print "FAIL kview: rows=$rows, screen: ${(q)screen[1,400]}"; fail=1
fi
# kview Enter: the picked row's namespace and name land on the next prompt. Fake kubectl and
# kubecolor print a colored table, so this also checks the colors are stripped from the pick;
# after ctrl-z only the crashing pod is listed, so Enter picks that one.
fake=$(mktemp -d)
print -r -- $'#!/bin/sh\nprintf "NAMESPACE NAME READY STATUS\\n\\033[36mkube-system\\033[0m coredns-x 1/1 \\033[32mRunning\\033[0m\\nkube-system bad-y 0/1 CrashLoopBackOff\\n"' >| $fake/kubectl
cp $fake/kubectl $fake/kubecolor
chmod +x $fake/kubectl $fake/kubecolor

# Waits $1 twentieths of a second while reading the pty: fzf redraws constantly, and a pty
# nobody reads fills up and blocks it.
drain() {
  local chunk
  repeat $1 { while zpty -rt t chunk; do :; done; sleep 0.05 }
}

check_pick() {  # check_pick KEYS WANT
  : >| $out
  zpty -b t "env -i HOME=$HOME TERM=xterm PATH=$fake:$root:$PATH zsh -f -i"
  zpty -w t "stty rows 20 cols 100; PS1='> '; source $root/cmdmode.plugin.zsh"
  zpty -w t "_dump() { [[ -n \$BUFFER ]] && print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey '^T' _dump"
  zpty -w t "KVIEW_INTERVAL=0.3 kview kubectl get pods -A -o wide"
  drain 30
  [[ -n $1 ]] && { zpty -w -n t $1; drain 20 }
  zpty -w -n t $'\r'
  drain 20
  zpty -w -n t $'\x14'
  for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
  zpty -d t
  if [[ $(<$out) == "$2" ]]; then
    print "ok   kview ${(q)1} Enter: next prompt is ${(q)$(<$out)}"
  else
    print "FAIL kview ${(q)1} Enter: next prompt is ${(q)$(<$out)}, want ${(q)2}"; fail=1
  fi
}
check_pick '' 'kubectl get -n kube-system pods coredns-x '
check_pick $'\x1a' 'kubectl get -n kube-system pods bad-y '
rm -rf $out $kv $fake
exit $fail
