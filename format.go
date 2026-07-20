package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"strings"
)

var validFormats = []string{"tsv", "csv", "json", "html"}

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

var tableTemplate = template.Must(template.New("table").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>gh-sbom dependency table</title>
<style>
table{border-collapse:collapse;font-family:system-ui,sans-serif}
th,td{border:1px solid #ccc;padding:4px 8px;text-align:left}
th{background:#eee}
</style>
</head>
<body>
<table>
<thead><tr><th>repo</th><th>ecosystem</th><th>package</th><th>version</th></tr></thead>
<tbody>{{range .}}
<tr><td>{{.Repo}}</td><td>{{.Ecosystem}}</td><td>{{.Package}}</td><td>{{.Version}}</td></tr>{{end}}
</tbody>
</table>
</body>
</html>
`))

// exportRows converts internal rows to the exported-field view that
// text/template-based formats (json, html) need — html/template cannot
// read unexported struct fields.
func exportRows(rows []row) []jsonRow {
	out := make([]jsonRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, jsonRow{r.repo, r.ecosystem, r.pkg, r.version})
	}
	return out
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
		// Argument order matters: WriteAll fills buf before WriteFile reads it.
		return errors.Join(w.WriteAll(records), os.WriteFile(path, buf.Bytes(), 0o644))
	case "json":
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false) // keep <, >, & as literal bytes, matching tsv/csv
		enc.SetIndent("", "  ")
		// Argument order matters: Encode fills buf before WriteFile reads it.
		return errors.Join(enc.Encode(exportRows(rows)), os.WriteFile(path, buf.Bytes(), 0o644))
	case "html":
		var buf bytes.Buffer
		// Argument order matters: Execute fills buf before WriteFile reads it.
		return errors.Join(tableTemplate.Execute(&buf, exportRows(rows)), os.WriteFile(path, buf.Bytes(), 0o644))
	default:
		return invalidFormatErr(format)
	}
}
