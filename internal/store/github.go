package store

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cychiang/setup/internal/archive"
	"github.com/cychiang/setup/internal/semver"
	"github.com/cychiang/setup/internal/tree"
)

// GitHub reads release assets over HTTPS. Releases are tagged <bundle>/v<version>.
type GitHub struct {
	Repo  string // owner/repo
	Token string
	API   string
	HTTP  *http.Client

	cached []release
}

type release struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Download string `json:"browser_download_url"`
	} `json:"assets"`
}

func (g *GitHub) client() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (g *GitHub) get(url, accept string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	resp, err := g.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, fmt.Errorf("GitHub API rate limit reached (%s); set GITHUB_TOKEN or GH_TOKEN, for example GH_TOKEN=$(gh auth token)", resp.Status)
		}
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return body, nil
}

func (g *GitHub) source() {}

// releases is fetched once per store: anonymous API calls are limited to 60 an hour per IP.
func (g *GitHub) releases() ([]release, error) {
	if g.cached != nil {
		return g.cached, nil
	}
	var all []release
	for page := 1; ; page++ {
		body, err := g.get(fmt.Sprintf("%s/repos/%s/releases?per_page=100&page=%d", g.API, g.Repo, page), "application/vnd.github+json")
		if err != nil {
			return nil, err
		}
		var rs []release
		if err := json.Unmarshal(body, &rs); err != nil {
			return nil, err
		}
		all = append(all, rs...)
		if len(rs) < 100 {
			if all == nil {
				all = []release{}
			}
			g.cached = all
			return all, nil
		}
	}
}

func (g *GitHub) Name() string { return "github:" + g.Repo }

func (g *GitHub) Bundles() ([]string, error) {
	rs, err := g.releases()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, r := range rs {
		name, ver, ok := strings.Cut(r.Tag, "/v")
		if !ok {
			continue
		}
		if _, err := semver.Parse(ver); err == nil {
			names = append(names, name)
		}
	}
	return uniqueSorted(names), nil
}

func (g *GitHub) Versions(bundle string) ([]semver.Version, error) {
	rs, err := g.releases()
	if err != nil {
		return nil, err
	}
	var out []semver.Version
	for _, r := range rs {
		ver, ok := strings.CutPrefix(r.Tag, bundle+"/v")
		if !ok {
			continue
		}
		if v, err := semver.Parse(ver); err == nil {
			out = append(out, v)
		}
	}
	return out, nil
}

func (g *GitHub) Fetch(bundle string, v semver.Version) ([]byte, string, error) {
	rs, err := g.releases()
	if err != nil {
		return nil, "", err
	}
	tag := fmt.Sprintf("%s/v%s", bundle, v)
	file := archive.FileName(bundle, v)
	for _, r := range rs {
		if r.Tag != tag {
			continue
		}
		urls := map[string]string{}
		for _, a := range r.Assets {
			// Without a token, download URLs avoid the API rate limit. A token may be for a private repo, which only the API serves.
			if g.Token == "" && a.Download != "" {
				urls[a.Name] = a.Download
			} else {
				urls[a.Name] = a.URL
			}
		}
		if urls[file] == "" || urls[file+".sha256"] == "" {
			return nil, "", fmt.Errorf("release %s lacks %s or its .sha256", tag, file)
		}
		sum, err := g.get(urls[file+".sha256"], "application/octet-stream")
		if err != nil {
			return nil, "", err
		}
		want, err := parseChecksum(sum, file)
		if err != nil {
			return nil, "", err
		}
		data, err := g.get(urls[file], "application/octet-stream")
		if err != nil {
			return nil, "", err
		}
		if got := tree.SHA(data); got != want {
			return nil, "", fmt.Errorf("%s: checksum mismatch (want %s, got %s)", file, want, got)
		}
		return data, want, nil
	}
	return nil, "", fmt.Errorf("release %s not found in %s", tag, g.Repo)
}
