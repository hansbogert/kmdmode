package main

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"strings"
)

// orderByName copies a kubectl table from in to out with its rows sorted by namespace, then name:
// kubectl's own order. kview fetches tables sorted by creation time, so that Alt-S can show them
// newest first at once; this gives back the usual order from the same rows. Input that is not a
// table passes through.
func orderByName(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if !sc.Scan() {
		return sc.Err()
	}
	header := sc.Text()
	names, starts := tableColumns(ansi.ReplaceAllString(strings.TrimSuffix(header, faultsMarker), ""))
	var rows []string
	for sc.Scan() {
		rows = append(rows, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return err
	}
	cell := func(row, col string) string {
		i := slices.Index(names, col)
		if i < 0 {
			return ""
		}
		end := -1
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		return strings.TrimSpace(ansi.ReplaceAllString(visibleSlice(row, starts[i], end), ""))
	}
	if slices.Contains(names, "NAME") {
		key := func(row string) string { return cell(row, "NAMESPACE") + "\x00" + cell(row, "NAME") }
		slices.SortStableFunc(rows, func(a, b string) int { return strings.Compare(key(a), key(b)) })
	}
	fmt.Fprintln(out, header)
	for _, r := range rows {
		fmt.Fprintln(out, r)
	}
	return nil
}
