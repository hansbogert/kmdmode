package main

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

// Kept whatever is chosen: picking a row reads NAMESPACE and NAME from its leading columns.
var keyColumns = set("NAMESPACE", "NAME")

// Left out of the picker's starting choice when nothing is chosen yet: rarely wanted, and wide.
var offByDefault = set("IP", "NOMINATED NODE", "READINESS GATES")

const faultsMarker = "  [faults only]"

// tableColumns finds a kubectl table's columns from its header, without colors: kubectl aligns
// each column at the same character offset in every row, starting at 0 and wherever a word
// follows two or more spaces (one space joins words, as in "NOMINATED NODE").
func tableColumns(header string) (names []string, starts []int) {
	runes := []rune(header)
	for i := 0; i < len(runes); i++ {
		if runes[i] == ' ' || i > 0 && !(i >= 2 && runes[i-1] == ' ' && runes[i-2] == ' ') {
			continue
		}
		j := i
		for j < len(runes) && !(runes[j] == ' ' && (j+1 == len(runes) || runes[j+1] == ' ')) {
			j++
		}
		names, starts = append(names, string(runes[i:j])), append(starts, i)
		i = j
	}
	return names, starts
}

// visibleSlice cuts line between visible character offsets from and to (to < 0: the end), keeping
// the color codes that fall inside so kubecolor's colors survive.
func visibleSlice(line string, from, to int) string {
	var b strings.Builder
	n := 0
	for i := 0; i < len(line); {
		if line[i] == '\x1b' {
			if m := ansi.FindStringIndex(line[i:]); m != nil && m[0] == 0 {
				if n >= from && (to < 0 || n < to) {
					b.WriteString(line[i : i+m[1]])
				}
				i += m[1]
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		if n >= from && (to < 0 || n < to) {
			b.WriteRune(r)
		}
		n++
		i += size
	}
	return b.String()
}

// columns copies a kubectl table from in to out with only the wanted columns (plus NAMESPACE and
// NAME), each cut at its offsets so the alignment holds. Input that is not a table passes through.
func columns(in io.Reader, out io.Writer, want []string) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if !sc.Scan() {
		return sc.Err()
	}
	header, marker := strings.CutSuffix(sc.Text(), faultsMarker)
	names, starts := tableColumns(ansi.ReplaceAllString(header, ""))
	if !slices.Contains(names, "NAME") {
		fmt.Fprintln(out, sc.Text())
		for sc.Scan() {
			fmt.Fprintln(out, sc.Text())
		}
		return sc.Err()
	}
	keep := set(want...)
	cut := func(line string) string {
		var b strings.Builder
		for i, name := range names {
			if keep[name] || keyColumns[name] {
				end := -1
				if i+1 < len(starts) {
					end = starts[i+1]
				}
				b.WriteString(visibleSlice(line, starts[i], end))
			}
		}
		return strings.TrimRight(b.String(), " ") + "\x1b[0m"
	}
	h := cut(header)
	if marker {
		h += faultsMarker
	}
	fmt.Fprintln(out, h)
	for sc.Scan() {
		fmt.Fprintln(out, cut(sc.Text()))
	}
	return sc.Err()
}

// listColumns prints the table's columns as "NAME<TAB>on|off" for the picker: on when in current,
// or, with nothing chosen yet, unless offByDefault. NAMESPACE and NAME are always kept, so they
// are not offered.
func listColumns(in io.Reader, out io.Writer, current []string) error {
	header, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	header, _ = strings.CutSuffix(strings.TrimRight(header, "\n"), faultsMarker)
	names, _ := tableColumns(ansi.ReplaceAllString(header, ""))
	for _, name := range names {
		if keyColumns[name] {
			continue
		}
		on := !offByDefault[name]
		if len(current) > 0 {
			on = slices.Contains(current, name)
		}
		fmt.Fprintf(out, "%s\t%s\n", name, map[bool]string{true: "on", false: "off"}[on])
	}
	return nil
}
