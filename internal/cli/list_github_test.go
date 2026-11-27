package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cychiang/setup/internal/archive"
	"github.com/cychiang/setup/internal/catalog"
	"github.com/cychiang/setup/internal/semver"
	"github.com/cychiang/setup/internal/store"
	"github.com/cychiang/setup/internal/tree"
)

func TestListReadsTheLatestManifestFromAGitHubCatalog(t *testing.T) {
	assemble := func(name string, layer catalog.Layer, v semver.Version, desc string) []byte {
		c := archive.Assemble(catalog.Bundle{Name: name, Layer: layer, Version: v, Description: desc},
			map[string]tree.Tree{"money": {"SKILL.md": []byte("---\nname: money\ndescription: d\n---\n")}}, nil, tree.Tree{})
		var b bytes.Buffer
		if err := archive.Write(&b, c, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	latest := assemble("finance", catalog.Domain, semver.Version{Minor: 2}, "Finance.")
	base := assemble("base", catalog.Base, semver.Version{Minor: 1}, "Base.")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases":
			asset := func(name string, id int) string {
				return fmt.Sprintf(`{"name":%q,"url":"%s/assets/%d"}`, name, srv.URL, id)
			}
			fmt.Fprintf(w, `[
 {"tag_name":"finance/v0.2.0","assets":[%s,%s]},
 {"tag_name":"finance/v0.1.0","assets":[]},
 {"tag_name":"base/v0.1.0","assets":[%s,%s]},
 {"tag_name":"v9.9.9","assets":[]}]`,
				asset("finance-0.2.0.tar.gz", 1), asset("finance-0.2.0.tar.gz.sha256", 2),
				asset("base-0.1.0.tar.gz", 3), asset("base-0.1.0.tar.gz.sha256", 4))
		case "/assets/1":
			w.Write(latest)
		case "/assets/2":
			fmt.Fprintf(w, "%s  f\n", tree.SHA(latest))
		case "/assets/3":
			w.Write(base)
		case "/assets/4":
			fmt.Fprintf(w, "%s  f\n", tree.SHA(base))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	got, err := catalogBundles(&store.GitHub{Repo: "o/r", API: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "base" || got[1].Name != "finance" {
		t.Fatalf("got %+v", got)
	}
	if f := got[1]; f.LatestVersion != "0.2.0" || f.Description != "Finance." || f.Layer != catalog.Domain || len(f.Skills) != 1 || f.Skills[0] != "money" {
		t.Fatalf("finance = %+v", f)
	}
}
