// SPDX-License-Identifier: GPL-3.0-or-later

package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// JSON prints v as indented JSON to w (or os.Stdout when w is nil).
func JSON(w io.Writer, v any) error {
	if w == nil {
		w = os.Stdout
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Table writes a header + rows to w as an aligned tab-separated table.
// rows is a slice of slices; each inner slice's length should match len(header).
func Table(w io.Writer, header []string, rows [][]string) {
	if w == nil {
		w = os.Stdout
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if len(header) > 0 {
		fmt.Fprintln(tw, strings.Join(header, "\t"))
	}
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	tw.Flush()
}
