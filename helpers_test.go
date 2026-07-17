package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

func init() {
	sleepBetween = 0
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestClient(t *testing.T, rt http.RoundTripper) *api.RESTClient {
	t.Helper()
	client, err := api.NewRESTClient(api.ClientOptions{
		AuthToken: "test-token",
		Host:      "github.com",
		Transport: rt,
	})
	if err != nil {
		t.Fatalf("NewRESTClient: %v", err)
	}
	return client
}

func handlerClient(t *testing.T, h http.Handler) *api.RESTClient {
	t.Helper()
	return newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		resp := rec.Result()
		resp.Request = req // go-gh's error handling reads resp.Request.URL
		return resp, nil
	}))
}

func errClient(t *testing.T) *api.RESTClient {
	t.Helper()
	return newTestClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network down")
	}))
}

const goodSBOM = `{"sbom":{"packages":[
	{"SPDXID":"SPDXRef-root","name":"com.github.acme/app"},
	{"SPDXID":"SPDXRef-a","name":"lodash","versionInfo":"4.17.21",
	 "externalRefs":[{"referenceType":"purl","referenceLocator":"pkg:npm/lodash@4.17.21"}]},
	{"SPDXID":"SPDXRef-b","name":"left-pad"},
	{"SPDXID":"SPDXRef-c","name":""}],
	"relationships":[{"relationshipType":"DEPENDS_ON","relatedSpdxElement":"SPDXRef-a"},
	{"relationshipType":"DESCRIBES","relatedSpdxElement":"SPDXRef-root"}]}}`

const emptySBOM = `{"sbom":{"packages":[{"SPDXID":"SPDXRef-root","name":"com.github.acme/empty"}],
	"relationships":[{"relationshipType":"DESCRIBES","relatedSpdxElement":"SPDXRef-root"}]}}`

func orgMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/acme/repos", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"name":"app"},{"name":"empty"},{"name":"bad"},{"name":"old","archived":true}]`)
	})
	mux.HandleFunc("/rate_limit", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"resources":{"core":{"remaining":2}}}`)
	})
	mux.HandleFunc("/repos/acme/app/dependency-graph/sbom", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, goodSBOM)
	})
	mux.HandleFunc("/repos/acme/empty/dependency-graph/sbom", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, emptySBOM)
	})
	// acme/bad has no handler, so its SBOM fetch 404s.
	return mux
}

func writeSBOMDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
