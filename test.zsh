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

check 'kubectl get pods -n ns pod1 ' L 'kubectl logs -n ns pod1 ' 'cmdmode [kubectl logs]  [G] get'
check "kubectl get pod 'my pod' -o wide" l "kubectl logs 'my pod'"
check 'kubectl get deploy d -n ns' I 'kubectl describe -n ns deploy d' 'cmdmode [kubectl get]  [I] describe'
check 'kubectl get deploy d' L 'kubectl get deploy d' "no 'L' here"
check 'kgp pod1' l 'kubectl logs pod1'
check 'kubectl get pods p' $'\e'x 'kubectl get pods px'
check 'kubectl get pods p' 'l-ft' 'kubectl logs p -f --timestamps' '[f] -f*'
check "kubectl get pods p" "l-f" "kubectl logs p -f" "[f] -f*  [p]"
check 'kubectl logs -f p' '-f'$'\e''g' 'kubectl get pod p' 'cmdmode [kubectl logs -]'
check 'kubectl -n ns ' O 'kview kubectl get pods -A -o wide ' 'cmdmode [kubectl]  [O] overview'
check 'kubectl -n ns ' O 'kview kubectl get pods -A -o wide ' 'cmdmode [kview kubectl get]  [W] unwatch'
check 'kubectl -n ns ' OW 'kubectl get pods -A -o wide '
check 'kubectl -n ns ' $'O\e' 'kubectl get pods -A -o wide' '[Esc] up'  # watching is a menu below
check 'kubectl get -n ns pods p -o wide' ':c' 'kubectl get -n ns -o wide configmaps' 'cmdmode [kubectl get :]  [p] pods'
check 'kubectl get -n ns pods p' $':\e' 'kubectl get -n ns pods p'  # Esc leaves the resource list
check 'kubectl get -n ns pods p -o wide' '-a' 'kubectl get -o wide pods -A'  # -A drops names and -n
check 'kubectl get deploy d -n ns' R 'kubectl rollout restart -n ns deploy/d' 'cmdmode [kubectl rollout]'
check 'kubectl get deploy d -n ns ' S3 'kubectl scale -n ns deploy/d --replicas=3'  # S leaves cmdmode
check 'kubectl get deploy d -n ns' $'R\e' 'kubectl get -n ns deploy/d' '[Esc] up'
check 'kubectl get deploy d -n ns' $'R\eI' 'kubectl describe -n ns deploy/d'
check 'kubectl get deploy d -n ns' $'S\em\e' 'kubectl get -n ns deploy/d'  # up from S, which left cmdmode
check 'kubectl get pods p' $'L-f\e\e' 'kubectl get pod p'  # up out of options, then up from logs
check 'kubectl get pods p' $'L-f\e' 'kubectl logs p -f'  # Esc keeps the toggle: up, not undo
check 'kubectl get pods p' $'l-f\r' 'kubectl logs p -f'  # ran, and came back in cmdmode
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
# nobody reads fills up and blocks it. Answers a cursor-position query (fzf --height asks) as a
# terminal would.
drain() {
  local chunk
  repeat $1 {
    while zpty -rt t chunk; do
      [[ $chunk == *$'\e[6n'* ]] && zpty -w -n t $'\e[5;1R'
    done
    sleep 0.05
  }
}

check_pick() {  # check_pick KEYS WANT [KEY pressed on the returned prompt]
  : >| $out
  zpty -b t "env -i HOME=$HOME TERM=xterm PATH=$fake:$root:$PATH zsh -f -i"
  zpty -w t "stty rows 20 cols 100; PS1='> '; source $root/cmdmode.plugin.zsh"
  zpty -w t "_dump() { [[ -n \$BUFFER ]] && print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey '^T' _dump; bindkey -M cmdmode '^T' _dump"
  zpty -w t "KVIEW_INTERVAL=0.3 kview kubectl get pods -A -o wide"
  drain 30
  [[ -n $1 ]] && { zpty -w -n t $1; drain 20 }
  zpty -w -n t $'\r'
  drain 20
  [[ -n $3 ]] && { zpty -w -n t $3; drain 10 }
  zpty -w -n t $'\x14'
  for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
  zpty -d t
  if [[ $(<$out) == "$2" ]]; then
    print "ok   kview ${(q)1} Enter ${3:+then $3}: next prompt is ${(q)$(<$out)}"
  else
    print "FAIL kview ${(q)1} Enter: next prompt is ${(q)$(<$out)}, want ${(q)2}"; fail=1
  fi
}
# Alt-Enter on a row: tick two of its labels, come back with a get by those labels.
print -r -- $'#!/bin/sh\ncase "$*" in *go-template*) printf "k8s-app=kube-dns\\ntier=node\\nzone=a\\n"; exit;; esac' >| $fake/labels
sed -i '1r '$fake/labels $fake/kubectl
: >| $out
zpty -b t "env -i HOME=$HOME TERM=xterm PATH=$fake:$root:$PATH zsh -f -i"
zpty -w t "stty rows 20 cols 100; PS1='> '; source $root/cmdmode.plugin.zsh"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M cmdmode '^T' _dump"
zpty -w t "KVIEW_INTERVAL=0.3 kview kubectl get pods -A -o wide"
drain 30
zpty -w -n t $'\e\r'
drain 20
zpty -w -n t $'\t\t\r'
drain 20
zpty -w -n t $'\x14'
for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
zpty -d t
if [[ $(<$out) == 'kubectl get -n kube-system pods -l k8s-app=kube-dns,tier=node ' ]]; then
  print "ok   kview Alt-Enter, two labels ticked: ${(q)$(<$out)}"
else
  print "FAIL kview Alt-Enter labels: got ${(q)$(<$out)}"; fail=1
fi

check_pick '' 'kubectl get -n kube-system pods coredns-x '
check_pick $'\x1a' 'kubectl get -n kube-system pods bad-y '
check_pick '' 'kubectl logs -n kube-system coredns-x ' l
# kview keeps the cursor on its row across refreshes, though READY flips each time, and Enter on
# a describe document gives the describe line back.
cat >| $fake/kubectl <<'SH'
#!/bin/sh
if [ "$1" = describe ]; then
  echo "Name: p"; i=1; while [ $i -le 40 ]; do echo "Field-$i: value"; i=$((i+1)); done; exit
fi
r=$(( $(date +%s) % 2 ))
echo "NAME READY STATUS"; i=1; while [ $i -le 40 ]; do echo "row-$i $r/1 Running"; i=$((i+1)); done
SH
rm -f $fake/kubecolor
for c want in 'get pods' 'kubectl get pods row-6' 'describe pod p' 'kubectl describe pod p'; do
  print -r -- "stty rows 12 cols 80; export PATH=$fake:\$PATH; unset CMDMODE_PICK_FILE
KVIEW_INTERVAL=0.3 $root/kview kubectl $c; print exit=\$?" >| $kv
  screen=$( (sleep 1; printf '\016\016\016\016\016'; sleep 2.5; printf '\r'; sleep 1.5) |
    timeout 12 script -qfc "zsh -f $kv" /dev/null)
  got=$(print -r -- $screen | grep -aoE 'kubectl (get pods row-[0-9]+|describe pod p)' | tail -1)
  if [[ $got == "$want" && $screen == *exit=0* ]]; then
    print "ok   kview $c: 5 down, refreshes, Enter gives ${(q)got}"
  else
    print "FAIL kview $c: Enter gave ${(q)got}, want ${(q)want}"; fail=1
    print -r -- $screen | sed "s/\x1b\[[0-9;?]*[a-zA-Z]//g" | tr "\r" "\n" | grep -av "^\s*$" | tail -4 | cut -c1-100
  fi
done
# A describe document refreshes like a table does.
print -r -- $'#!/bin/sh\necho "Name: p"; echo "Tick: $(date +%s%N)"' >| $fake/kubectl
print -r -- "stty rows 15 cols 80; export PATH=$fake:\$PATH
KVIEW_INTERVAL=0.5 $root/kview kubectl describe pod p" >| $kv
ticks=$( (sleep 3; printf '\e'; sleep 1) | timeout 10 script -qfc "zsh -f $kv" /dev/null |
  grep -aoE 'Tick: [0-9]+' | sort -u | wc -l)
if (( ticks >= 3 )); then
  print "ok   kview describe refreshes: $ticks versions seen"
else
  print "FAIL kview describe refreshes: $ticks versions seen"; fail=1
fi
# Alt-m on an empty prompt recalls the last command with a mode, skipping ones without.
: >| $out
zpty -b t "env -i HOME=$HOME TERM=xterm zsh -f -i"
zpty -w t "PS1='> '; source $root/cmdmode.plugin.zsh; kubectl() { :; }"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M cmdmode '^T' _dump"
zpty -w t "kubectl logs -n ns p --tail 5"
zpty -w t "true"
drain 10
zpty -w -n t $'\em'g$'\x14'
for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
zpty -d t
if [[ $(<$out) == 'kubectl get -n ns pod p' ]]; then
  print "ok   empty prompt + Alt-m recalls the logs line: G gives ${(q)$(<$out)}"
else
  print "FAIL empty prompt + Alt-m recall: got ${(q)$(<$out)}"; fail=1
fi

# The same recall after a rollout restart: its line has a real menu, so T follows the rollout.
: >| $out
zpty -b t "env -i HOME=$HOME TERM=xterm zsh -f -i"
zpty -w t "PS1='> '; source $root/cmdmode.plugin.zsh; kubectl() { :; }"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M cmdmode '^T' _dump"
zpty -w t "kubectl rollout restart -n ns deploy/d"
drain 10
zpty -w -n t $'\em't$'\x14'
for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
zpty -d t
if [[ $(<$out) == 'kubectl rollout status -n ns deploy/d' ]]; then
  print "ok   empty prompt + Alt-m after a rollout restart: T gives ${(q)$(<$out)}"
else
  print "FAIL Alt-m after rollout restart: got ${(q)$(<$out)}"; fail=1
fi

# Esc on a recalled rollout line goes up to its workload's get menu; there, at the top, Esc exits.
for keys want in $'\e' '[kubectl get -n ns deploy/d]' $'\e\e' '[kubectl get -n ns deploy/d]'; do
  : >| $out
  zpty -b t "env -i HOME=$HOME TERM=xterm zsh -f -i"
  zpty -w t "PS1='> '; source $root/cmdmode.plugin.zsh; kubectl() { :; }"
  zpty -w t "_dump() { print -rn -- \"[\$BUFFER]\" >| $out }; zle -N _dump; bindkey '^T' _dump; bindkey -M cmdmode '^T' _dump"
  zpty -w t "kubectl rollout restart -n ns deploy/d"
  drain 10
  zpty -w -n t $'\em'$keys
  drain 12
  zpty -w -n t $'\x14'
  for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
  zpty -d t
  if [[ $(<$out) == "$want" ]]; then
    print "ok   recalled rollout line, ${(q)keys}: ${(q)$(<$out)}"
  else
    print "FAIL recalled rollout line, ${(q)keys}: got ${(q)$(<$out)}, want $want"; fail=1
  fi
done

# A line run from cmdmode comes back in cmdmode: G then acts on it, and Esc goes up from the
# logs menu to the pod's get menu.
for keys want in g 'kubectl get -n ns pod p' $'\e' 'kubectl get -n ns pod p'; do
  : >| $out
  zpty -b t "env -i HOME=$HOME TERM=xterm zsh -f -i"
  rm -f $out.ran
  zpty -w t "PS1='> '; source $root/cmdmode.plugin.zsh; kubectl() { print -rn -- \"\$*\" >| $out.ran }"
  zpty -w t "_dump() { print -rn -- \"[\$BUFFER]\" >| $out }; zle -N _dump"
  zpty -w t "bindkey '^T' _dump; bindkey -M cmdmode '^T' _dump"
  drain 10
  zpty -w -n t "kubectl logs -n ns p"$'\em\r'
  drain 10
  zpty -w -n t "$keys"$'\x14'
  for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
  zpty -d t
  if [[ $(<$out) == "[$want]" && $(<$out.ran) == 'logs -n ns p' ]]; then
    print "ok   run from cmdmode, back in cmdmode, ${(q)keys}: ${(q)$(<$out)}"
  else
    print "FAIL run from cmdmode, back in cmdmode, ${(q)keys}: got ${(q)$(<$out)}, want [$want]; ran ${(q)$(<$out.ran)}"; fail=1
  fi
done

# Alt-L: untick READY in the column picker, the table drops it, and Esc returns in cmdmode on
# the view's command updated to the chosen columns. A view started with -c shows only those.
cat >| $fake/kubectl <<'SH'
#!/bin/sh
echo "NAMESPACE     NAME        READY   STATUS             RESTARTS   AGE   IP         NODE     NOMINATED NODE   READINESS GATES"
echo "kube-system   coredns-x   1/1     Running            0          2d    10.0.0.1   node-a   <none>           <none>"
SH
: >| $out
zpty -b t "env -i HOME=$HOME TERM=xterm PATH=$fake:$root:$PATH zsh -f -i"
zpty -w t "stty rows 20 cols 140; PS1='> '; source $root/cmdmode.plugin.zsh"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M cmdmode '^T' _dump"
zpty -w t "KVIEW_INTERVAL=0.5 kview kubectl get pods -A -o wide"
drain 30
zpty -w -n t $'\el'
drain 30
zpty -w -n t $'\t\r'
drain 30
zpty -w -n t $'\e'
drain 20
zpty -w -n t $'\x14'
for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
zpty -d t
if [[ $(<$out) == 'kview -c STATUS,RESTARTS,AGE,NODE kubectl get pods -A -o wide ' ]]; then
  print "ok   kview Alt-L, READY unticked, Esc: ${(q)$(<$out)}"
else
  print "FAIL kview Alt-L columns: got ${(q)$(<$out)}"; fail=1
fi
print -r -- "stty rows 10 cols 140; export PATH=$fake:\$PATH
$root/kview -c 'STATUS,NOMINATED NODE' kubectl get pods -A -o wide" >| $kv
screen=$( (sleep 1.5; printf '\e'; sleep 1) | timeout 10 script -qfc "zsh -f $kv" /dev/null |
  sed 's/\x1b\[[0-9;?]*[a-zA-Z]//g' | tr -s ' ')
if [[ $screen == *'NAMESPACE NAME STATUS NOMINATED NODE'* && $screen != *READY* ]]; then
  print "ok   kview -c shows only those columns"
else
  print "FAIL kview -c: ${(q)screen[1,300]}"; fail=1
fi

# Alt-S sorts by age, newest first: the fake lists oldest first only when given the sort, so the
# top row after Alt-S is the newest. Esc returns the command with the sort.
cat >| $fake/kubectl <<'SH'
#!/bin/sh
echo "NAMESPACE   NAME    READY   STATUS"
case "$*" in
  *creationTimestamp*) printf 'ns   old-a   1/1   Running\nns   mid-b   1/1   Running\nns   new-c   1/1   Running\n' ;;
  *) printf 'ns   mid-b   1/1   Running\nns   new-c   1/1   Running\nns   old-a   1/1   Running\n' ;;
esac
SH
for keys want in $'\es\r' 'kubectl get -n ns pods new-c ' $'\es\e' 'kview kubectl get pods -A --sort-by=.metadata.creationTimestamp '; do
  : >| $out
  zpty -b t "env -i HOME=$HOME TERM=xterm PATH=$fake:$root:$PATH zsh -f -i"
  zpty -w t "stty rows 20 cols 140; PS1='> '; source $root/cmdmode.plugin.zsh"
  zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M cmdmode '^T' _dump"
  zpty -w t "KVIEW_INTERVAL=0.5 kview kubectl get pods -A"
  drain 30
  zpty -w -n t ${keys[1,2]}
  drain 30
  zpty -w -n t ${keys[3,-1]}
  drain 20
  zpty -w -n t $'\x14'
  for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
  zpty -d t
  if [[ $(<$out) == "$want" ]]; then
    print "ok   kview Alt-S then ${(q)keys[3,-1]}: ${(q)$(<$out)}"
  else
    print "FAIL kview Alt-S then ${(q)keys[3,-1]}: got ${(q)$(<$out)}, want ${(q)want}"; fail=1
  fi
done

# U: from a pod line to its owner (one lookup, against a fake cluster), and then R restarts it.
cat >| $fake/kubectl <<'SH'
#!/bin/sh
case "$*" in
  *web-7d9f8-x2k*ownerReferences*) echo "ReplicaSet|apps/v1|web-7d9f8|7d9f8" ;;
  *) echo "unexpected: $*" >&2; exit 1 ;;
esac
SH
: >| $out
zpty -b t "env -i HOME=$HOME TERM=xterm PATH=$fake:$PATH zsh -f -i"
zpty -w t "PS1='> '; source $root/cmdmode.plugin.zsh"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M cmdmode '^T' _dump"
drain 4
zpty -w -n t "kubectl get -n ns pods web-7d9f8-x2k"$'\em'
drain 4
zpty -w -n t U
drain 10
zpty -w -n t R$'\x14'
for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
zpty -d t
if [[ $(<$out) == 'kubectl rollout restart -n ns deploy/web' ]]; then
  print "ok   pod line, U then R: ${(q)$(<$out)}"
else
  print "FAIL pod line, U then R: got ${(q)$(<$out)}"; fail=1
fi

# ":" then "." lists the cluster's resource types (a fake cluster here) in fzf below the prompt;
# typing "cert" and Enter switches the line to that CRD.
cat >| $fake/kubectl <<'SH'
#!/bin/sh
case "$*" in
  *api-resources*) printf 'pods\nconfigmaps\ncertificates.cert-manager.io\nissuers.cert-manager.io\n' ;;
  *) echo "unexpected: $*" >&2; exit 1 ;;
esac
SH
: >| $out
zpty -b t "env -i HOME=$HOME TERM=xterm PATH=$fake:$PATH zsh -f -i"
zpty -w t "stty rows 20 cols 100; PS1='> '; source $root/cmdmode.plugin.zsh"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M cmdmode '^T' _dump"
drain 4
zpty -w -n t "kubectl get -n ns pods"$'\em'
drain 4
zpty -w -n t ':.'
drain 20
zpty -w -n t cert
drain 10  # let fzf filter before Enter, as a person typing would
zpty -w -n t $'\r'
drain 10
zpty -w -n t $'\x14'
for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
zpty -d t
if [[ $(<$out) == 'kubectl get -n ns certificates.cert-manager.io' ]]; then
  print "ok   :. picker, cert + Enter: ${(q)$(<$out)}"
else
  print "FAIL :. picker: got ${(q)$(<$out)}"; fail=1
fi

rm -rf $out $out.ran $kv $fake
exit $fail
