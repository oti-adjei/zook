package native

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/oti-adjei/zook/internal/health"
)

// HTTPFetcher downloads artifacts over HTTP and extracts .tar.gz archives.
type HTTPFetcher struct {
	doer health.Doer
}

// NewHTTPFetcher returns a fetcher backed by doer (an *http.Client satisfies it).
func NewHTTPFetcher(doer health.Doer) *HTTPFetcher { return &HTTPFetcher{doer: doer} }

// Fetch downloads the version's artifact into destDir.
func (f *HTTPFetcher) Fetch(ctx context.Context, urlTemplate, version, destDir string, out io.Writer) error {
	url := expandVersion(urlTemplate, version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "fetching %s\n", url)
	resp, err := f.doer.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		return extractTarGz(resp.Body, destDir)
	}
	// single binary: name it after the URL's basename
	name := path.Base(url)
	dst := filepath.Join(destDir, name)
	out2, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out2.Close()
	_, err = io.Copy(out2, resp.Body)
	return err
}

func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		// reject path traversal.
		// filepath.Clean normalises the entry before we validate it; keep this order.
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("unsafe tar entry %q", hdr.Name)
		}
		target := filepath.Join(destDir, clean)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			w, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			w.Close()
		}
	}
}
