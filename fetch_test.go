package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListReposOrg(t *testing.T) {
	opts := &options{limit: 1000}
	names, err := listRepos(handlerClient(t, orgMux(t)), "acme", opts)
	if err != nil {
		t.Fatalf("listRepos: %v", err)
	}
	want := []string{"app", "empty", "bad"} // "old" is archived
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}

	opts.includeArchived = true
	names, err = listRepos(handlerClient(t, orgMux(t)), "acme", opts)
	if err != nil {
		t.Fatalf("listRepos archived: %v", err)
	}
	if len(names) != 4 {
		t.Fatalf("with archived: %v", names)
	}
}

func TestListReposUserFallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/dev/repos", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"name":"dotfiles"}]`)
	})
	names, err := listRepos(handlerClient(t, mux), "dev", &options{limit: 10})
	if err != nil {
		t.Fatalf("listRepos: %v", err)
	}
	if len(names) != 1 || names[0] != "dotfiles" {
		t.Fatalf("names = %v", names)
	}
}

func TestListReposErrors(t *testing.T) {
	// Neither org nor user endpoint exists.
	if _, err := listRepos(handlerClient(t, http.NewServeMux()), "ghost", &options{limit: 10}); err == nil ||
		!strings.Contains(err.Error(), `could not list repos for "ghost"`) {
		t.Fatalf("err = %v", err)
	}
	// Transport-level failure (not an HTTP 404).
	if _, err := listRepos(errClient(t), "acme", &options{limit: 10}); err == nil ||
		!strings.Contains(err.Error(), "network down") {
		t.Fatalf("err = %v", err)
	}
}

func TestListReposPaginationAndLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/big/repos", func(w http.ResponseWriter, r *http.Request) {
		type repo struct {
			Name string `json:"name"`
		}
		var repos []repo
		if r.URL.Query().Get("page") == "1" {
			for i := 0; i < 100; i++ {
				repos = append(repos, repo{Name: fmt.Sprintf("repo-%03d", i)})
			}
		} else {
			repos = []repo{{Name: "last"}}
		}
		_ = json.NewEncoder(w).Encode(repos)
	})

	names, err := listRepos(handlerClient(t, mux), "big", &options{limit: 1000})
	if err != nil {
		t.Fatalf("listRepos: %v", err)
	}
	if len(names) != 101 || names[100] != "last" {
		t.Fatalf("got %d names, last %q", len(names), names[len(names)-1])
	}

	names, err = listRepos(handlerClient(t, mux), "big", &options{limit: 50})
	if err != nil {
		t.Fatalf("listRepos limited: %v", err)
	}
	if len(names) != 50 {
		t.Fatalf("limit ignored: got %d names", len(names))
	}
}

func TestFetchSBOMsOrg(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "sboms")
	opts := &options{target: "acme", outDir: outDir, limit: 1000}
	var stderr bytes.Buffer

	if err := fetchSBOMs(handlerClient(t, orgMux(t)), opts, &stderr); err != nil {
		t.Fatalf("fetchSBOMs: %v", err)
	}

	out := stderr.String()
	for _, want := range []string{
		"Listing repos for acme...",
		"warning: only 2 API requests remaining this hour for 3 repos",
		"note: no dependencies detected",
		"failed (dependency graph disabled, or no access)",
		"Fetched 2/3 SBOMs",
		"Failed (1): bad",
		"Settings -> Advanced Security -> Dependency graph",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stderr missing %q in:\n%s", want, out)
		}
	}
	for _, f := range []string{"app.json", "empty.json"} {
		if _, err := os.Stat(filepath.Join(outDir, f)); err != nil {
			t.Errorf("expected %s: %v", f, err)
		}
	}
}

func TestFetchSBOMsSingleRepo(t *testing.T) {
	outDir := t.TempDir()
	opts := &options{target: "acme/app", outDir: outDir, limit: 1000}
	var stderr bytes.Buffer

	if err := fetchSBOMs(handlerClient(t, orgMux(t)), opts, &stderr); err != nil {
		t.Fatalf("fetchSBOMs: %v", err)
	}
	if !strings.Contains(stderr.String(), "Fetched 1/1 SBOMs") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestFetchSBOMsErrors(t *testing.T) {
	var stderr bytes.Buffer

	// Repo listing fails.
	if err := fetchSBOMs(errClient(t), &options{target: "acme", limit: 10}, &stderr); err == nil ||
		!strings.Contains(err.Error(), "could not list repos") {
		t.Fatalf("err = %v", err)
	}

	// Org exists but has no repos.
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/hollow/repos", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})
	if err := fetchSBOMs(handlerClient(t, mux), &options{target: "hollow", limit: 10}, &stderr); err == nil ||
		!strings.Contains(err.Error(), `no repos found for "hollow"`) {
		t.Fatalf("err = %v", err)
	}

	// Output dir path is an existing regular file.
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fetchSBOMs(handlerClient(t, orgMux(t)), &options{target: "acme/app", outDir: file, limit: 10}, &stderr); err == nil {
		t.Fatal("expected MkdirAll error")
	}

	// Output dir is not writable.
	roDir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(roDir, 0o755)
	if err := fetchSBOMs(handlerClient(t, orgMux(t)), &options{target: "acme/app", outDir: roDir, limit: 10}, &stderr); err == nil {
		t.Fatal("expected WriteFile error")
	}
}
