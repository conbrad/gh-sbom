package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"
)

var formatRows = []row{
	{"cli", "golang", "github.com/x/y", "v1.0.0"},
	{"web", "npm", "left,pad", "1.0"},
}

func writeAndRead(t *testing.T, format string, rows []row) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out."+format)
	if err := writeRows(path, format, rows); err != nil {
		t.Fatalf("writeRows(%s): %v", format, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestWriteRowsTSV(t *testing.T) {
	want := "repo\tecosystem\tpackage\tversion\n" +
		"cli\tgolang\tgithub.com/x/y\tv1.0.0\n" +
		"web\tnpm\tleft,pad\t1.0\n"
	if got := writeAndRead(t, "tsv", formatRows); got != want {
		t.Fatalf("tsv = %q, want %q", got, want)
	}
}

func TestWriteRowsCSV(t *testing.T) {
	// The comma-containing package name must be quoted.
	want := "repo,ecosystem,package,version\n" +
		"cli,golang,github.com/x/y,v1.0.0\n" +
		"web,npm,\"left,pad\",1.0\n"
	if got := writeAndRead(t, "csv", formatRows); got != want {
		t.Fatalf("csv = %q, want %q", got, want)
	}
}

func TestWriteRowsJSON(t *testing.T) {
	want := `[
  {
    "repo": "cli",
    "ecosystem": "golang",
    "package": "github.com/x/y",
    "version": "v1.0.0"
  },
  {
    "repo": "web",
    "ecosystem": "npm",
    "package": "left,pad",
    "version": "1.0"
  }
]
`
	if got := writeAndRead(t, "json", formatRows); got != want {
		t.Fatalf("json = %q, want %q", got, want)
	}
}

func TestWriteRowsQuoting(t *testing.T) {
	rows := []row{
		{"r", "npm", "has\ttab", "1.0"},
		{"r", "npm", `has"quote`, "2.0"},
		{"r", "npm", " leading-space", "3.0"},
	}
	// TSV: tab is the delimiter, so the tab-containing field is quoted;
	// quotes are doubled; leading whitespace forces quoting.
	wantTSV := "repo\tecosystem\tpackage\tversion\n" +
		"r\tnpm\t\"has\ttab\"\t1.0\n" +
		"r\tnpm\t\"has\"\"quote\"\t2.0\n" +
		"r\tnpm\t\" leading-space\"\t3.0\n"
	if got := writeAndRead(t, "tsv", rows); got != wantTSV {
		t.Fatalf("tsv = %q, want %q", got, wantTSV)
	}
	// CSV: an embedded tab is not special, but quotes and leading
	// whitespace still trigger quoting.
	wantCSV := "repo,ecosystem,package,version\n" +
		"r,npm,has\ttab,1.0\n" +
		"r,npm,\"has\"\"quote\",2.0\n" +
		"r,npm,\" leading-space\",3.0\n"
	if got := writeAndRead(t, "csv", rows); got != wantCSV {
		t.Fatalf("csv = %q, want %q", got, wantCSV)
	}
}

func TestWriteRowsJSONNoHTMLEscape(t *testing.T) {
	// <, >, and & must stay literal so JSON output matches tsv/csv bytes.
	got := writeAndRead(t, "json", []row{{"r", "npm", "a&b<c>d", "1.0"}})
	if !strings.Contains(got, `"a&b<c>d"`) {
		t.Fatalf("HTML-escaped JSON output: %q", got)
	}
}

func TestWriteRowsJSONEmpty(t *testing.T) {
	// Zero rows must serialize as [], never null.
	if got := writeAndRead(t, "json", nil); got != "[]\n" {
		t.Fatalf("empty json = %q, want %q", got, "[]\n")
	}
}

func TestWriteRowsInvalidFormat(t *testing.T) {
	err := writeRows(filepath.Join(t.TempDir(), "x"), "yaml", formatRows)
	if err == nil || !strings.Contains(err.Error(), `invalid format "yaml" (valid: tsv, csv, json, html, parquet)`) {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteRowsCreateError(t *testing.T) {
	for _, format := range []string{"tsv", "csv", "json", "html", "parquet"} {
		path := filepath.Join(t.TempDir(), "no", "such", "dir", "out")
		if err := writeRows(path, format, formatRows); err == nil {
			t.Fatalf("%s: expected error for unwritable path", format)
		}
	}
}

func TestIsValidFormat(t *testing.T) {
	for _, f := range validFormats {
		if !isValidFormat(f) {
			t.Errorf("isValidFormat(%q) = false", f)
		}
	}
	if isValidFormat("yaml") {
		t.Error(`isValidFormat("yaml") = true`)
	}
}

func TestWriteRowsHTML(t *testing.T) {
	want := `<!DOCTYPE html>
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
<tbody>
<tr><td>cli</td><td>golang</td><td>github.com/x/y</td><td>v1.0.0</td></tr>
<tr><td>web</td><td>npm</td><td>left,pad</td><td>1.0</td></tr>
</tbody>
</table>
</body>
</html>
`
	if got := writeAndRead(t, "html", formatRows); got != want {
		t.Fatalf("html = %q, want %q", got, want)
	}
}

func TestWriteRowsHTMLEmpty(t *testing.T) {
	want := `<!DOCTYPE html>
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
<tbody>
</tbody>
</table>
</body>
</html>
`
	if got := writeAndRead(t, "html", nil); got != want {
		t.Fatalf("empty html = %q, want %q", got, want)
	}
}

func TestWriteRowsHTMLEscaping(t *testing.T) {
	rows := []row{{"r", "npm", `<script>alert("xss")</script>`, "1.0"}}
	got := writeAndRead(t, "html", rows)
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("package name not escaped: %q", got)
	}
	if strings.Contains(got, "<script>alert") {
		t.Fatalf("raw unescaped markup leaked into output: %q", got)
	}
}

// parquetTestRow decodes the format's public schema independently of
// whatever struct writeRows uses internally to produce it.
type parquetTestRow struct {
	Repo      string `parquet:"repo"`
	Ecosystem string `parquet:"ecosystem"`
	Package   string `parquet:"package"`
	Version   string `parquet:"version"`
}

func readParquet(t *testing.T, path string) []parquetTestRow {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r := parquet.NewGenericReader[parquetTestRow](f)
	defer r.Close()
	got := make([]parquetTestRow, r.NumRows())
	n, err := r.Read(got)
	if err != nil && err != io.EOF {
		t.Fatalf("read parquet: %v", err)
	}
	return got[:n]
}

func TestWriteRowsParquet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.parquet")
	if err := writeRows(path, "parquet", formatRows); err != nil {
		t.Fatalf("writeRows(parquet): %v", err)
	}

	want := []parquetTestRow{
		{"cli", "golang", "github.com/x/y", "v1.0.0"},
		{"web", "npm", "left,pad", "1.0"},
	}
	if got := readParquet(t, path); !reflect.DeepEqual(got, want) {
		t.Fatalf("parquet rows = %+v, want %+v", got, want)
	}
}

func TestWriteRowsParquetEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.parquet")
	if err := writeRows(path, "parquet", nil); err != nil {
		t.Fatalf("writeRows(parquet empty): %v", err)
	}
	if got := readParquet(t, path); len(got) != 0 {
		t.Fatalf("empty parquet rows = %+v, want none", got)
	}
}
