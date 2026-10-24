#!/usr/bin/env zsh
# End-to-end check: type a line into a real interactive zsh, press Alt-m and then KEYS, and
# compare the resulting BUFFER (dumped by ^T) or, after Enter, what the stub kubectl ran.
emulate -L zsh
zmodload zsh/zpty
root=${0:A:h}
out=$(mktemp)
fail=0

# The terminal output without colour codes: the hint colours its verbs, which splits the text.
plain() { print -r -- "$1" | sed 's/\x1b\[[0-9;]*m//g' }

check() {  # check LINE KEYS WANT [TEXT the hint (or a message) must show]
  local line=$1 keys=$2 want=$3 shows=$4
  : >| $out; : >| $out.hint
  zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm zsh -f -i"
  zpty -w t "PS1='> '; source $root/kmdmode.plugin.zsh; alias kgp='kubectl get pods'"
  # The hint is POSTDISPLAY, which zle redraws incrementally, so dump it rather than read it off
  # the terminal.
  zpty -w t "_dump() { print -rn -- \$POSTDISPLAY >| $out.hint; [[ -n \$BUFFER ]] && print -rn -- \$BUFFER >| $out }"
  zpty -w t "zle -N _dump; bindkey '^T' _dump; bindkey -M kmdmode '^T' _dump"
  zpty -w t "kubectl() { print -rn -- \"ran: kubectl \$*\" >| $out }"
  zpty -w -n t "$line"$'\ek'"$keys"$'\x14'
  local i tty chunk
  for i in {1..50}; do [[ -s $out ]] && break; sleep 0.05; done
  while zpty -rt t chunk; do tty+=$chunk; done
  tty=$(plain $tty)
  zpty -d t
  local name="${(q)line} + ${(q)keys}"
  if [[ $(<$out) != "$want" ]]; then
    print "FAIL $name: got ${(q)$(<$out)}, want ${(q)want}"; fail=1
  elif [[ $tty == *(bad option|command not found|_kmdmode)* ]]; then
    print "FAIL $name: zsh error on the terminal: ${(q)tty}"; fail=1
  elif [[ -n $shows && $tty != *"$shows"* && $(<$out.hint) != *"$shows"* ]]; then
    print "FAIL $name: never showed ${(q)shows}; hint was ${(q)$(<$out.hint)}"; fail=1
  else
    print "ok   $name"
  fi
}

check 'kubectl get pods -n ns pod1 ' l 'kubectl logs -n ns pod1 ' 'kmdmode [kubectl logs]  [g] get'
check "kubectl get pod 'my pod' -o wide" l "kubectl logs 'my pod'"
check 'kubectl get deploy d -n ns' d 'kubectl describe -n ns deploy d' 'kmdmode [kubectl describe]'
check 'kubectl get deploy d -n ns' D 'kubectl delete -n ns deploy d' 'kmdmode [kubectl delete]'
check 'kubectl get svc s' l 'kubectl get svc s' "no 'l' here"
check 'kubectl get -n ns deploy web' l 'kubectl logs -n ns deploy/web' 'kmdmode [kubectl logs]'
check 'kubectl get pods p' L 'kubectl get pods p' "no 'L' here"  # keys are case-sensitive
check 'kubectl get -n ns pods p' x 'kubectl exec -n ns -it p -- sh' 'kmdmode [kubectl exec]'
check 'kubectl get -f /us' $'\t' 'kubectl get -f /usr/' 'kmdmode [kubectl get]'  # Tab completes, still in kmdmode
check 'kubectl get -n ns pods p' 'x' 'kubectl exec -n ns -it p -- sh' 'kmdmode [kubectl exec]  [b] bash'
check 'kubectl get -n ns pods p' 'xb' 'kubectl exec -n ns -it p -- bash' 'kmdmode [kubectl exec]  [b] sh'
check 'kubectl get -n ns deploy web' x 'kubectl exec -n ns -it deploy/web -- sh' 'kmdmode [kubectl exec]  [b] bash'
check 'kubectl get -n ns deploy web' $'x\e' 'kubectl get -n ns deploy/web'  # up from exec to the deployment
check 'kgp pod1' l 'kubectl logs pod1'
check 'kubectl get pods p' $'\e'x 'kubectl get pods px'
check 'kubectl get pods p' 'l-ft' 'kubectl logs p -f --timestamps' '[f] -f*'
check "kubectl get pods p" "l-f" "kubectl logs p -f" "[f] -f*  [p]"
check 'kubectl logs -f p' '-' 'kubectl logs -f p' 'kmdmode [kubectl logs -]'
check 'kubectl logs -f p' '-f'$'\e''g' 'kubectl get pod p'
check 'kubectl -n ns ' o 'kview kubectl get pods -A -o wide ' 'kmdmode [kubectl]  [o] overview'
check 'kubectl -n ns ' o 'kview kubectl get pods -A -o wide ' 'kmdmode [kview kubectl get]  [w] unwatch'
check 'kubectl -n ns ' ow 'kubectl get pods -A -o wide '
check 'kubectl -n ns ' $'o\e' 'kubectl get pods -A -o wide' '[Esc] up'  # watching is a menu below
check 'kubectl get -n ns pods p -o wide' ':' 'kubectl get -n ns pods p -o wide' 'kmdmode [kubectl get :]  [p] pods'
check 'kubectl get -n ns pods p -o wide' ':c' 'kubectl get -n ns -o wide configmaps'
check 'kubectl get -n ns pods p' $':\e' 'kubectl get -n ns pods p'  # Esc leaves the resource list
check 'kubectl get -n ns pods p -o wide' '-a' 'kubectl get -o wide pods -A'  # -A drops names and -n
check 'kubectl get deploy d -n ns' r 'kubectl rollout restart -n ns deploy/d' 'kmdmode [kubectl rollout]'
check 'kubectl get deploy d -n ns ' s3 'kubectl scale -n ns deploy/d --replicas=3'  # S leaves kmdmode
check 'kubectl get deploy d -n ns' $'r\e' 'kubectl get -n ns deploy/d' '[Esc] up'
check 'kubectl get deploy d -n ns' $'r\ed' 'kubectl describe -n ns deploy/d'
check 'kubectl get deploy d -n ns' $'s\ek\e' 'kubectl get -n ns deploy/d'  # up from S, which left kmdmode
check 'kubectl get pods p' $'l-f\e\e' 'kubectl get pod p'  # up out of options, then up from logs
check 'kubectl get pods p' $'l-f\e' 'kubectl logs p -f'  # Esc keeps the toggle: up, not undo
check 'kubectl get pods p' $'l-f\r' 'kubectl logs p -f'  # ran, and came back in kmdmode
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
  zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm PATH=$fake:$root:$PATH zsh -f -i"
  zpty -w t "stty rows 20 cols 100; PS1='> '; source $root/kmdmode.plugin.zsh"
  zpty -w t "_dump() { [[ -n \$BUFFER ]] && print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey '^T' _dump; bindkey -M kmdmode '^T' _dump"
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
zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm PATH=$fake:$root:$PATH zsh -f -i"
zpty -w t "stty rows 20 cols 100; PS1='> '; source $root/kmdmode.plugin.zsh"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M kmdmode '^T' _dump"
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
  print -r -- "stty rows 12 cols 80; export PATH=$fake:\$PATH; unset KMDMODE_PICK_FILE
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
zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm zsh -f -i"
zpty -w t "PS1='> '; source $root/kmdmode.plugin.zsh; kubectl() { :; }"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M kmdmode '^T' _dump"
zpty -w t "kubectl logs -n ns p --tail 5"
zpty -w t "true"
drain 10
zpty -w -n t $'\ek'g$'\x14'
for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
zpty -d t
if [[ $(<$out) == 'kubectl get -n ns pod p' ]]; then
  print "ok   empty prompt + Alt-m recalls the logs line: G gives ${(q)$(<$out)}"
else
  print "FAIL empty prompt + Alt-m recall: got ${(q)$(<$out)}"; fail=1
fi

# With no kubectl command in history, Alt-m on an empty prompt starts from the pods overview.
: >| $out
TTY=
zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm zsh -f -i"
zpty -w t "PS1='> '; source $root/kmdmode.plugin.zsh"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M kmdmode '^T' _dump"
drain 6
zpty -w -n t $'\ek'
local screen chunk
repeat 10 { while zpty -rt t chunk; do screen+=$chunk; done; sleep 0.05 }
screen=$(plain $screen)
zpty -w -n t $'\x14'
for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
zpty -d t
if [[ $(<$out) == 'kview kubectl get pods -A -o wide' && $screen == *'kmdmode [kview kubectl get]'* ]]; then
  print "ok   empty prompt, no kubectl history, Alt-m: ${(q)$(<$out)} in kmdmode"
else
  print "FAIL empty prompt, no kubectl history: got ${(q)$(<$out)}"; fail=1
fi

# The same recall after a rollout restart: its line has a real menu, so T follows the rollout.
: >| $out
zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm zsh -f -i"
zpty -w t "PS1='> '; source $root/kmdmode.plugin.zsh; kubectl() { :; }"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M kmdmode '^T' _dump"
zpty -w t "kubectl rollout restart -n ns deploy/d"
drain 10
zpty -w -n t $'\ek't$'\x14'
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
  zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm zsh -f -i"
  zpty -w t "PS1='> '; source $root/kmdmode.plugin.zsh; kubectl() { :; }"
  zpty -w t "_dump() { print -rn -- \"[\$BUFFER]\" >| $out }; zle -N _dump; bindkey '^T' _dump; bindkey -M kmdmode '^T' _dump"
  zpty -w t "kubectl rollout restart -n ns deploy/d"
  drain 10
  zpty -w -n t $'\ek'$keys
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

# A line run from kmdmode comes back in kmdmode: G then acts on it, and Esc goes up from the
# logs menu to the pod's get menu.
for keys want in g 'kubectl get -n ns pod p' $'\e' 'kubectl get -n ns pod p'; do
  : >| $out
  zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm zsh -f -i"
  rm -f $out.ran
  zpty -w t "PS1='> '; source $root/kmdmode.plugin.zsh; kubectl() { print -rn -- \"\$*\" >| $out.ran }"
  zpty -w t "_dump() { print -rn -- \"[\$BUFFER]\" >| $out }; zle -N _dump"
  zpty -w t "bindkey '^T' _dump; bindkey -M kmdmode '^T' _dump"
  drain 10
  zpty -w -n t "kubectl logs -n ns p"$'\ek\r'
  drain 10
  zpty -w -n t "$keys"$'\x14'
  for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
  zpty -d t
  if [[ $(<$out) == "[$want]" && $(<$out.ran) == 'logs -n ns p' ]]; then
    print "ok   run from kmdmode, back in kmdmode, ${(q)keys}: ${(q)$(<$out)}"
  else
    print "FAIL run from kmdmode, back in kmdmode, ${(q)keys}: got ${(q)$(<$out)}, want [$want]; ran ${(q)$(<$out.ran)}"; fail=1
  fi
done

# Alt-L: untick READY in the column picker, the table drops it, and Esc returns in kmdmode on
# the view's command updated to the chosen columns. A view started with -c shows only those.
cat >| $fake/kubectl <<'SH'
#!/bin/sh
echo "NAMESPACE     NAME        READY   STATUS             RESTARTS   AGE   IP         NODE     NOMINATED NODE   READINESS GATES"
echo "kube-system   coredns-x   1/1     Running            0          2d    10.0.0.1   node-a   <none>           <none>"
SH
: >| $out
zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm PATH=$fake:$root:$PATH zsh -f -i"
zpty -w t "stty rows 20 cols 140; PS1='> '; source $root/kmdmode.plugin.zsh"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M kmdmode '^T' _dump"
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
  zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm PATH=$fake:$root:$PATH zsh -f -i"
  zpty -w t "stty rows 20 cols 140; PS1='> '; source $root/kmdmode.plugin.zsh"
  zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M kmdmode '^T' _dump"
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
zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm PATH=$fake:$PATH zsh -f -i"
zpty -w t "PS1='> '; source $root/kmdmode.plugin.zsh"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M kmdmode '^T' _dump"
drain 4
zpty -w -n t "kubectl get -n ns pods web-7d9f8-x2k"$'\ek'
drain 4
zpty -w -n t u
drain 10
zpty -w -n t r$'\x14'
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
zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm PATH=$fake:$PATH zsh -f -i"
zpty -w t "stty rows 20 cols 100; PS1='> '; source $root/kmdmode.plugin.zsh"
zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M kmdmode '^T' _dump"
drain 4
zpty -w -n t "kubectl get -n ns pods"$'\ek'
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

# Esc in that picker goes back to the main menu, not to the ":" list it was opened from.
: >| $out; : >| $out.hint
zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm PATH=$fake:$PATH zsh -f -i"
zpty -w t "stty rows 20 cols 100; PS1='> '; source $root/kmdmode.plugin.zsh"
zpty -w t "_dump() { print -rn -- \$POSTDISPLAY >| $out.hint; print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M kmdmode '^T' _dump"
drain 4
zpty -w -n t "kubectl get -n ns pods"$'\ek'
drain 4
zpty -w -n t ':.'
drain 20
zpty -w -n t $'\e'
drain 12
zpty -w -n t $'\x14'
for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
zpty -d t
if [[ $(<$out) == 'kubectl get -n ns pods' && $(<$out.hint) == *'kmdmode [kubectl get]'* ]]; then
  print "ok   :. picker, Esc: back in the main menu"
else
  print "FAIL :. picker, Esc: line ${(q)$(<$out)}, hint ${(q)$(<$out.hint)}"; fail=1
fi

# C (capital only) picks a kubeconfig context in fzf and sets --context on the line; c does not.
cat >| $fake/kubectl <<'SH'
#!/bin/sh
case "$*" in
  *current-context*) echo mgmt ;;
  *get-contexts*) printf 'staging\nprod\nmgmt\n' ;;
  *) echo "unexpected: $*" >&2; exit 1 ;;
esac
SH
for keys want in C 'kubectl --context prod get -n ns pods' c 'kubectl get -n ns pods'; do
  : >| $out
  zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm PATH=$fake:$PATH zsh -f -i"
  zpty -w t "stty rows 20 cols 100; PS1='> '; source $root/kmdmode.plugin.zsh"
  zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump; bindkey -M kmdmode '^T' _dump"
  drain 4
  zpty -w -n t "kubectl get -n ns pods"$'\ek'
  drain 4
  zpty -w -n t $keys
  drain 20
  if [[ $keys == C ]]; then
    zpty -w -n t prod
    drain 10
    zpty -w -n t $'\r'
    drain 10
  fi
  zpty -w -n t $'\x14'
  for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
  zpty -d t
  if [[ $(<$out) == "$want" ]]; then
    print "ok   context key ${keys}: ${(q)$(<$out)}"
  else
    print "FAIL context key ${keys}: got ${(q)$(<$out)}, want ${(q)want}"; fail=1
  fi
done

# Context first: a line naming no context opens the picker on entry, the current one listed
# first (Enter keeps it); Esc there leaves kmdmode with the line untouched. The fake kubeconfig's
# current context, mgmt, comes last from get-contexts.
for keys want in $'\r' 'kubectl --context mgmt get -n ns pods' $'\e' 'kubectl get -n ns pods'; do
  : >| $out
  zpty -b t "env -i HOME=$HOME TERM=xterm PATH=$fake:$PATH zsh -f -i"
  zpty -w t "stty rows 20 cols 100; PS1='> '; source $root/kmdmode.plugin.zsh"
  zpty -w t "_dump() { print -rn -- \$BUFFER >| $out }; zle -N _dump"
  zpty -w t "bindkey '^T' _dump; bindkey -M kmdmode '^T' _dump"
  drain 4
  zpty -w -n t "kubectl get -n ns pods"$'\ek'
  drain 20
  zpty -w -n t $keys
  drain 12
  zpty -w -n t $'\x14'
  for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
  zpty -d t
  if [[ $(<$out) == "$want" ]]; then
    print "ok   context first, ${(q)keys}: ${(q)$(<$out)}"
  else
    print "FAIL context first, ${(q)keys}: got ${(q)$(<$out)}, want ${(q)want}"; fail=1
  fi
done

# Enter on a delete line asks first: capital Y runs it, y (or anything else) cancels and stays.
for keys ran in Y 'delete -n ns pods p' y ''; do
  rm -f $out.ran
  zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm zsh -f -i"
  zpty -w t "PS1='> '; source $root/kmdmode.plugin.zsh; kubectl() { print -rn -- \"\$*\" >| $out.ran }"
  drain 4
  zpty -w -n t "kubectl get -n ns pods p"$'\ek'
  drain 4
  local screen= asked= chunk
  zpty -w -n t $'D\r'
  repeat 6 { while zpty -rt t chunk; do asked+=$chunk; done; sleep 0.05 }
  zpty -w -n t $keys
  repeat 10 { while zpty -rt t chunk; do screen+=$chunk; done; sleep 0.05 }
  asked=$(plain $asked) screen=$(plain $screen)
  zpty -d t
  got=; [[ -e $out.ran ]] && got=$(<$out.ran)
  if [[ $asked == *"press Y (capital)"* && $got == "$ran" && ( -n $ran || $screen == *"not deleted"* ) ]]; then
    print "ok   delete confirm ${keys}: ran ${(q)got}"
  else
    print "FAIL delete confirm ${keys}: ran ${(q)got}, want ${(q)ran}"; fail=1
  fi
done

# The release path: `source <(kmdmode init zsh)` gives a working kmdmode and a kview function, and a
# shell holding an older glue version re-reads the plugin from the binary on Alt-k.
for case in fresh stale; do
  : >| $out
  zpty -b t "env -i KMDMODE_CONTEXT_FIRST=0 HOME=$HOME TERM=xterm zsh -f -i"
  zpty -w t "PS1='> '; source <($root/bin/kmdmode init zsh)"
  [[ $case == stale ]] && zpty -w t "_kmdmode_glue=0ld"
  # Dump from the normal keymap: the reload re-creates the kmdmode one, dropping a test binding.
  zpty -w t "_dump() { print -rn -- \"\$BUFFER|\$_kmdmode_glue|\$(whence -w kview)\" >| $out }; zle -N _dump; bindkey '^T' _dump"
  drain 4
  zpty -w -n t "kubectl get -n ns pods p"$'\ekl'
  drain 6
  zpty -w -n t $'\e'
  drain 12
  zpty -w -n t $'\e'
  drain 12
  zpty -w -n t $'\x14'
  for i in {1..40}; do [[ -s $out ]] && break; drain 1; done
  zpty -d t
  want="kubectl get -n ns pod p|$($root/bin/kmdmode glue)|kview: function"
  if [[ $(<$out) == "$want" ]]; then
    print "ok   init zsh, $case shell: ${(q)$(<$out)}"
  else
    print "FAIL init zsh, $case shell: got ${(q)$(<$out)}, want ${(q)want}"; fail=1
  fi
done

rm -rf $out $out.ran $out.hint $kv $fake
exit $fail
