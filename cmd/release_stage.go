package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/ui"
)

func stageReleaseFromGitHub(tag string, assetName string, timeout time.Duration) error {
	ui.Header("Herringbone release stage")
	ui.FKeyValues(os.Stdout, [][2]string{{"tag", tag}})

	release, err := fetchHerringboneReleaseByTag(tag, timeout)
	if err != nil {
		return err
	}

	asset := selectReleaseArchiveAsset(release, assetName)
	if strings.TrimSpace(assetName) != "" && asset.Name == "" {
		return fmt.Errorf("release %s does not contain asset %q", tag, assetName)
	}
	downloadURL := strings.TrimSpace(asset.BrowserDownloadURL)
	downloadName := strings.TrimSpace(asset.Name)
	if downloadURL == "" {
		downloadURL = strings.TrimSpace(release.TarballURL)
		downloadName = safeReleaseTag(tag) + ".tar.gz"
	}
	if downloadURL == "" {
		return fmt.Errorf("release %s has no downloadable tar.gz/tgz asset and no GitHub tarball URL", tag)
	}
	if downloadName == "" {
		downloadName = safeReleaseTag(tag) + ".tar.gz"
	}

	stamp := time.Now().Format("20060102-150405")
	workRoot := filepath.Join(".hbctl", "releases", safeReleaseTag(tag)+"-"+stamp)
	downloadDir := filepath.Join(workRoot, "download")
	extractDir := filepath.Join(workRoot, "extract")
	archiveDir := filepath.Join(".hbctl", "archive", safeReleaseTag(tag)+"-"+stamp)
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return err
	}

	archivePath := filepath.Join(downloadDir, downloadName)
	ui.Section("Download release")
	if asset.Name != "" {
		ui.KeyValues([][2]string{{"asset", asset.Name}, {"url", downloadURL}})
	} else {
		ui.KeyValues([][2]string{{"asset", "source tarball"}, {"url", downloadURL}})
	}
	if err := downloadReleaseArchive(downloadURL, archivePath, timeout); err != nil {
		return err
	}
	ui.Success("Downloaded %s", archivePath)

	ui.Section("Extract release")
	if err := extractReleaseArchive(archivePath, extractDir); err != nil {
		return err
	}
	payload, err := findDockerPayloadDir(extractDir)
	if err != nil {
		return err
	}
	ui.KeyValues([][2]string{{"payload", payload}})

	ui.Section("Archive current compose directory")
	if err := archiveCurrentDirectory(archiveDir); err != nil {
		return err
	}
	ui.Success("Archived current files to %s", archiveDir)

	ui.Section("Install release files")
	if err := copyReleasePayload(payload, "."); err != nil {
		return err
	}
	ui.Success("Release %s staged", tag)
	ui.Info("Secrets were preserved. Run hbctl upgrade --all to apply the staged compose/images, or use --now next time.")
	return nil
}

func selectReleaseArchiveAsset(release githubRelease, requested string) githubReleaseAsset {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		for _, asset := range release.Assets {
			if asset.Name == requested {
				return asset
			}
		}
		return githubReleaseAsset{}
	}

	preferred := []string{"docker", "compose", "herringbone"}
	for _, token := range preferred {
		for _, asset := range release.Assets {
			name := strings.ToLower(strings.TrimSpace(asset.Name))
			if isTarArchiveName(name) && strings.Contains(name, token) {
				return asset
			}
		}
	}
	for _, asset := range release.Assets {
		if isTarArchiveName(strings.ToLower(strings.TrimSpace(asset.Name))) {
			return asset
		}
	}
	for _, asset := range release.Assets {
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(asset.Name)), ".zip") {
			return asset
		}
	}
	return githubReleaseAsset{}
}

func isTarArchiveName(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".tar")
}

func downloadReleaseArchive(url string, destination string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "hbctl/"+Version)
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download release archive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("failed to download release archive: http %d: %s", resp.StatusCode, msg)
	}

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func extractReleaseArchive(archivePath string, destination string) error {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		return extractZipArchive(archivePath, destination)
	}
	return extractTarArchive(archivePath, destination)
}

func extractTarArchive(archivePath string, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var reader io.Reader = file
	if strings.HasSuffix(strings.ToLower(archivePath), ".gz") || strings.HasSuffix(strings.ToLower(archivePath), ".tgz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gz.Close()
		reader = gz
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target, err := safeJoin(destination, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Avoid restoring symlinks from release archives into the deployment dir.
			continue
		}
	}
	return nil
}

func extractZipArchive(archivePath string, destination string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, file := range zr.File {
		target, err := safeJoin(destination, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			_ = rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			_ = out.Close()
			_ = rc.Close()
			return err
		}
		if err := out.Close(); err != nil {
			_ = rc.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}
	return nil
}

func safeJoin(root string, name string) (string, error) {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(cleanRoot, name)
	cleanTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if cleanTarget != cleanRoot && !strings.HasPrefix(cleanTarget, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive contains unsafe path: %s", name)
	}
	return cleanTarget, nil
}

func findDockerPayloadDir(root string) (string, error) {
	candidates := []string{}
	if hasComposeFiles(root) {
		candidates = append(candidates, root)
	}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}
		if strings.Count(strings.TrimPrefix(path, root), string(os.PathSeparator)) > 4 {
			return filepath.SkipDir
		}
		if filepath.Base(path) == "docker" && hasComposeFiles(path) {
			candidates = append(candidates, path)
			return filepath.SkipDir
		}
		if hasComposeFiles(path) {
			candidates = append(candidates, path)
		}
		return nil
	})

	if len(candidates) == 0 {
		return "", fmt.Errorf("release archive did not contain a docker compose payload directory")
	}
	sort.Slice(candidates, func(i, j int) bool {
		if filepath.Base(candidates[i]) == "docker" && filepath.Base(candidates[j]) != "docker" {
			return true
		}
		return len(candidates[i]) < len(candidates[j])
	})
	return candidates[0], nil
}

func hasComposeFiles(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "compose*.yml"))
	if len(matches) > 0 {
		return true
	}
	matches, _ = filepath.Glob(filepath.Join(dir, "docker-compose*.yml"))
	return len(matches) > 0
}

func archiveCurrentDirectory(archiveDir string) error {
	entries, err := os.ReadDir(".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if shouldPreserveDuringReleaseStage(name) {
			ui.Skip("preserve %s", name)
			continue
		}
		if err := os.Rename(name, filepath.Join(archiveDir, name)); err != nil {
			return fmt.Errorf("failed to archive %s: %w", name, err)
		}
	}
	return nil
}

func shouldPreserveDuringReleaseStage(name string) bool {
	switch name {
	case ".", "..", ".hbctl", "secrets", "secrets.enc":
		return true
	default:
		return strings.HasPrefix(name, "secrets.")
	}
}

func copyReleasePayload(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		first := strings.Split(rel, string(os.PathSeparator))[0]
		if shouldPreserveDuringReleaseStage(first) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			mode := info.Mode().Perm()
			if mode == 0 {
				mode = 0o755
			}
			return os.MkdirAll(target, mode)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = in.Close()
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			_ = in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			_ = in.Close()
			return err
		}
		if err := out.Close(); err != nil {
			_ = in.Close()
			return err
		}
		return in.Close()
	})
}

func safeReleaseTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "release"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-")
	return replacer.Replace(tag)
}
