package binary

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://github.com/peggco/pegg/releases/download"
	maxDownload    = 200 << 20
)

type Progress struct {
	Stage   string `json:"stage"`
	Percent int    `json:"percent"`
	Detail  string `json:"detail,omitempty"`
}

type Downloader struct {
	BaseURL string
	Dir     string
	GOOS    string
	GOARCH  string
	HTTP    *http.Client
}

func NewDownloader() *Downloader {
	dir := ManagedDir()
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "pegg-desktop")
	}
	return &Downloader{
		BaseURL: DefaultBaseURL,
		Dir:     dir,
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
		HTTP:    &http.Client{Timeout: 10 * time.Minute},
	}
}

func (d *Downloader) assetName() (string, string) {
	ext := "tar.gz"
	if d.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("pegg_%s_%s.%s", d.GOOS, d.GOARCH, ext), ext
}

func (d *Downloader) Install(ctx context.Context, version string, progress func(Progress)) (Status, error) {
	report := func(stage string, pct int, detail string) {
		if progress != nil {
			progress(Progress{Stage: stage, Percent: pct, Detail: detail})
		}
	}
	asset, ext := d.assetName()
	report("Preparing", 2, asset)

	if err := os.MkdirAll(d.Dir, 0o755); err != nil {
		return Status{}, err
	}
	tmpDir, err := os.MkdirTemp(d.Dir, ".install-*")
	if err != nil {
		return Status{}, err
	}
	defer os.RemoveAll(tmpDir)

	archive := filepath.Join(tmpDir, asset)
	report("Downloading", 8, asset)
	if err := d.download(ctx, d.assetURL(version, asset), archive, func(pct int) {
		report("Downloading", 8+pct*62/100, asset)
	}); err != nil {
		return Status{}, fmt.Errorf("download %s: %w", asset, err)
	}

	if os.Getenv("PEGG_SKIP_CHECKSUM") == "" {
		report("Verifying", 74, "checksums.txt")
		sums, err := d.fetchChecksums(ctx, version)
		if err != nil {
			return Status{}, fmt.Errorf("download checksums: %w", err)
		}
		want, ok := sums[asset]
		if !ok {
			return Status{}, fmt.Errorf("checksums.txt has no entry for %s", asset)
		}
		got, err := fileSHA256(archive)
		if err != nil {
			return Status{}, err
		}
		if !strings.EqualFold(got, want) {
			return Status{}, fmt.Errorf("checksum mismatch for %s", asset)
		}
	}

	report("Extracting", 82, asset)
	binPath := filepath.Join(tmpDir, Name())
	if strings.HasSuffix(ext, "zip") {
		err = extractZip(archive, Name(), binPath)
	} else {
		err = extractTarGz(archive, Name(), binPath)
	}
	if err != nil {
		return Status{}, fmt.Errorf("extract %s: %w", asset, err)
	}
	if d.GOOS != "windows" {
		if err := os.Chmod(binPath, 0o755); err != nil {
			return Status{}, err
		}
	}

	final := filepath.Join(d.Dir, Name())
	report("Installing", 92, final)
	_ = os.Remove(final)
	if err := os.Rename(binPath, final); err != nil {
		if err := copyFile(binPath, final); err != nil {
			return Status{}, err
		}
	}
	if d.GOOS != "windows" {
		_ = os.Chmod(final, 0o755)
	}
	report("Checking", 96, final)
	st := Probe(ctx, final)
	if !st.Found {
		return st, fmt.Errorf("installed binary failed version check: %s", st.Error)
	}
	st.Source = "managed"
	st.Managed = true
	report("Ready", 100, st.Version)
	return st, nil
}

func (d *Downloader) assetURL(version, asset string) string {
	base := strings.TrimRight(d.BaseURL, "/")
	if version == "" || version == "latest" || version == "match-extension" {
		if strings.HasSuffix(base, "/download") {
			return strings.TrimSuffix(base, "/download") + "/latest/download/" + asset
		}
		return base + "/latest/" + asset
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return base + "/" + version + "/" + asset
}

func (d *Downloader) download(ctx context.Context, url, dest string, progress func(int)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "pegg-desktop")
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()
	total := resp.ContentLength
	var read int64
	buf := make([]byte, 64<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			read += int64(n)
			if read > maxDownload {
				return errors.New("download exceeds size limit")
			}
			if _, werr := file.Write(buf[:n]); werr != nil {
				return werr
			}
			if progress != nil && total > 0 {
				progress(int(read * 100 / total))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return nil
}

func (d *Downloader) fetchChecksums(ctx context.Context, version string) (map[string]string, error) {
	url := d.assetURL(version, "checksums.txt")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "pegg-desktop")
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	sums := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sums[strings.TrimPrefix(fields[1], "*")] = fields[0]
	}
	return sums, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractTarGz(archive, want, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("archive did not contain %s", want)
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != want {
			continue
		}
		return writeLimited(dest, tr, 0o755)
	}
}

func extractZip(archive, want, dest string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || filepath.Base(f.Name) != want {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeLimited(dest, rc, 0o755)
		rc.Close()
		return err
	}
	return fmt.Errorf("archive did not contain %s", want)
}

func writeLimited(dest string, r io.Reader, mode os.FileMode) error {
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(r, maxDownload))
	if err != nil {
		return err
	}
	if n >= maxDownload {
		return errors.New("archive member exceeds size limit")
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
