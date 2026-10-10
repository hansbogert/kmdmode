package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Pods that finished are healthy with nothing ready.
var doneStatuses = set("Completed", "Succeeded")

// faults copies a kubectl table from in to out, keeping the header (marked "[faults only]") and
// the rows that look unhealthy, like k9s's fault toggle: a STATUS other than Running or done, or
// a READY count short of its total. Colored input is matched on its text and printed as given;
// input without a READY or STATUS column (an error message, say) passes through untouched.
func faults(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if !sc.Scan() {
		return sc.Err()
	}
	header := strings.Fields(ansi.ReplaceAllString(sc.Text(), ""))
	ready, status := slices.Index(header, "READY"), slices.Index(header, "STATUS")
	if ready < 0 && status < 0 {
		fmt.Fprintln(out, sc.Text())
		for sc.Scan() {
			fmt.Fprintln(out, sc.Text())
		}
		return sc.Err()
	}
	fmt.Fprintln(out, sc.Text()+"  [faults only]")
	for sc.Scan() {
		if faulty(strings.Fields(ansi.ReplaceAllString(sc.Text(), "")), ready, status) {
			fmt.Fprintln(out, sc.Text())
		}
	}
	return sc.Err()
}

func faulty(row []string, ready, status int) bool {
	st := ""
	if status >= 0 && status < len(row) {
		st = row[status]
		if st != "Running" && !doneStatuses[st] {
			return true
		}
	}
	if ready >= 0 && ready < len(row) && !doneStatuses[st] {
		if have, want, ok := strings.Cut(row[ready], "/"); ok {
			h, err1 := strconv.Atoi(have)
			w, err2 := strconv.Atoi(want)
			return err1 == nil && err2 == nil && h < w
		}
	}
	return false
}
