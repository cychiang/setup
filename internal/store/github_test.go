package store_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cychiang/setup/internal/semver"
	"github.com/cychiang/setup/internal/store"
	"github.com/cychiang/setup/internal/tree"
)

func TestGitHubStore(t *testing.T) {
	payload := []byte("archive bytes")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/repos/o/r/releases":
			fmt.Fprintf(w, `[
 {"tag_name":"finance/v1.0.0","assets":[
  {"name":"finance-1.0.0.tar.gz","url":"%[1]s/assets/1"},
  {"name":"finance-1.0.0.tar.gz.sha256","url":"%[1]s/assets/2"}]},
 {"tag_name":"finance/v1.2.0","assets":[]},
 {"tag_name":"base/v3.0.0","assets":[]}]`, srv.URL)
		case "/assets/1":
			w.Write(payload)
		case "/assets/2":
			fmt.Fprintf(w, "%s  finance-1.0.0.tar.gz\n", tree.SHA(payload))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var s store.Store = &store.GitHub{Repo: "o/r", Token: "tok", API: srv.URL}
	latest, err := store.Latest(s, "finance")
	if err != nil || latest != (semver.Version{Major: 1, Minor: 2}) {
		t.Fatalf("latest = %v, %v", latest, err)
	}
	data, sum, err := s.Fetch("finance", semver.Version{Major: 1})
	if err != nil || string(data) != string(payload) || sum != tree.SHA(payload) {
		t.Fatalf("fetch = %q %s %v", data, sum, err)
	}
	if _, _, err := s.Fetch("finance", semver.Version{Major: 1, Minor: 2}); err == nil {
		t.Fatal("release without assets must fail")
	}
}
