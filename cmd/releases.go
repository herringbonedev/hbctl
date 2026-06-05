package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/spf13/cobra"
)

const defaultReleasesURL = "https://api.github.com/repos/herringbonedev/Herringbone/releases"
const defaultReleaseTagURL = "https://api.github.com/repos/herringbonedev/Herringbone/releases/tags/"

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type githubRelease struct {
	Name        string               `json:"name"`
	TagName     string               `json:"tag_name"`
	HTMLURL     string               `json:"html_url"`
	TarballURL  string               `json:"tarball_url"`
	ZipballURL  string               `json:"zipball_url"`
	PublishedAt time.Time            `json:"published_at"`
	Prerelease  bool                 `json:"prerelease"`
	Draft       bool                 `json:"draft"`
	Assets      []githubReleaseAsset `json:"assets"`
}

func releasesCommand() *cobra.Command {
	var limit int
	var asJSON bool
	var includeDrafts bool
	var endpoint string
	var timeoutSeconds int

	cmd := &cobra.Command{
		Use:     "releases",
		Aliases: []string{"release"},
		Short:   "List published Herringbone releases from GitHub",
		Hidden:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.Warn("hbctl releases is deprecated. Use hbctl upgrade --list-releases instead.")
			if limit <= 0 {
				return fmt.Errorf("--limit must be greater than zero")
			}
			if timeoutSeconds <= 0 {
				return fmt.Errorf("--timeout must be greater than zero")
			}

			releases, err := fetchHerringboneReleases(endpoint, limit, includeDrafts, time.Duration(timeoutSeconds)*time.Second)
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(releases)
			}

			printReleaseList(cmd.OutOrStdout(), releases)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of releases to list")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&includeDrafts, "include-drafts", false, "Include draft releases if the GitHub API token can see them")
	cmd.Flags().StringVar(&endpoint, "url", defaultReleasesURL, "GitHub releases API URL")
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 10, "GitHub API timeout in seconds")
	return cmd
}

func printReleaseList(w io.Writer, releases []githubRelease) {
	ui.FHeader(w, "Herringbone releases")
	ui.FKeyValues(w, [][2]string{
		{"source", "github.com/herringbonedev/Herringbone"},
		{"count", strconv.Itoa(len(releases))},
	})

	if len(releases) == 0 {
		ui.FSkip(w, "No releases returned")
		return
	}

	rows := make([][]string, 0, len(releases))
	for _, release := range releases {
		kind := "release"
		if release.Prerelease {
			kind = "pre-release"
		}
		if release.Draft {
			kind = "draft"
		}

		published := "unknown"
		if !release.PublishedAt.IsZero() {
			published = release.PublishedAt.Local().Format("2006-01-02 15:04")
		}

		name := strings.TrimSpace(release.Name)
		if name == "" {
			name = release.TagName
		}

		asset := preferredReleaseAssetName(release)
		rows = append(rows, []string{release.TagName, name, kind, published, asset})
	}
	ui.FTable(w, []string{"TAG", "NAME", "TYPE", "PUBLISHED", "ASSET"}, rows)
}

func preferredReleaseAssetName(release githubRelease) string {
	asset := selectReleaseArchiveAsset(release, "")
	if asset.Name != "" {
		return asset.Name
	}
	if strings.TrimSpace(release.TarballURL) != "" {
		return "source tarball"
	}
	return "none"
}

func fetchHerringboneReleases(endpoint string, limit int, includeDrafts bool, timeout time.Duration) ([]githubRelease, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultReleasesURL
	}

	endpoint, err := releasesEndpointWithLimit(endpoint, limit)
	if err != nil {
		return nil, err
	}

	body, err := githubGet(endpoint, timeout)
	if err != nil {
		return nil, err
	}

	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub releases response: %w", err)
	}

	out := make([]githubRelease, 0, minInt(limit, len(releases)))
	for _, release := range releases {
		if release.Draft && !includeDrafts {
			continue
		}
		out = append(out, release)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func fetchHerringboneReleaseByTag(tag string, timeout time.Duration) (githubRelease, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return githubRelease{}, fmt.Errorf("release tag required")
	}

	endpoint := defaultReleaseTagURL + url.PathEscape(tag)
	body, err := githubGet(endpoint, timeout)
	if err != nil {
		return githubRelease{}, err
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return githubRelease{}, fmt.Errorf("failed to parse GitHub release response: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		release.TagName = tag
	}
	return release, nil
}

func githubGet(endpoint string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "hbctl/"+Version)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return nil, fmt.Errorf("github request failed: http %d: %s", resp.StatusCode, message)
	}
	return body, nil
}

func releasesEndpointWithLimit(endpoint string, limit int) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	perPage := minInt(limit, 100)
	if perPage <= 0 {
		perPage = 10
	}
	q := u.Query()
	if q.Get("per_page") == "" {
		q.Set("per_page", strconv.Itoa(perPage))
	}
	if q.Get("page") == "" {
		q.Set("page", "1")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
