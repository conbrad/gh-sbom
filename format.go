package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var validFormats = []string{"tsv", "csv", "json"}

func isValidFormat(f string) bool {
	for _, v := range validFormats {
		if f == v {
			return true
		}
	}
	return false
}

func invalidFormatErr(f string) error {
	return fmt.Errorf("invalid format %q (valid: %s)", f, strings.Join(validFormats, ", "))
}

type jsonRow struct {
	Repo      string `json:"repo"`
	Ecosystem string `json:"ecosystem"`
	Package   string `json:"package"`
	Version   string `json:"version"`
}

// writeRows serializes the combined dependency table to path. Columns are
// always repo, ecosystem, package, version.
func writeRows(path, format string, rows []row) error {
	switch format {
	case "tsv", "csv":
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		if format == "tsv" {
			w.Comma = '\t'
		}
		records := make([][]string, 0, len(rows)+1)
		records = append(records, []string{"repo", "ecosystem", "package", "version"})
		for _, r := range rows {
			records = append(records, []string{r.repo, r.ecosystem, r.pkg, r.version})
		}
		_ = w.WriteAll(records) // writing to a bytes.Buffer cannot fail
		return os.WriteFile(path, buf.Bytes(), 0o644)
	case "json":
		out := make([]jsonRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, jsonRow{r.repo, r.ecosystem, r.pkg, r.version})
		}
		data, _ := json.MarshalIndent(out, "", "  ") // marshaling plain strings cannot fail
		return os.WriteFile(path, append(data, '\n'), 0o644)
	default:
		return invalidFormatErr(format)
	}
}
