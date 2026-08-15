package native

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// stubDoer serves a fixed body for any request.
type stubDoer struct {
	body   []byte
	status int
}

func (s *stubDoer) Do(*http.Request) (*http.Response, error) {
	st := s.status
	if st == 0 {
		st = 200
	}
	return &http.Response{StatusCode: st, Body: io.NopCloser(bytes.NewReader(s.body))}, nil
}

func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestFetchExtractsTarGz(t *testing.T) {
	data := makeTarGz(t, map[string]string{"rue-api": "ELF...", "VERSION": "v9"})
	f := NewHTTPFetcher(&stubDoer{body: data})
	dest := filepath.Join(t.TempDir(), "releases", "v9")
	if err := f.Fetch(context.Background(), "https://x/rue-${VERSION}.tar.gz", "v9", dest, io.Discard); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "rue-api"))
	if err != nil || string(got) != "ELF..." {
		t.Fatalf("binary not extracted: %v / %q", err, got)
	}
}

func TestFetchRejectsTraversalEntry(t *testing.T) {
	data := makeTarGz(t, map[string]string{"../escape": "x"})
	f := NewHTTPFetcher(&stubDoer{body: data})
	dest := filepath.Join(t.TempDir(), "r")
	if err := f.Fetch(context.Background(), "https://x/a.tar.gz", "v1", dest, io.Discard); err == nil {
		t.Fatal("tar entry with .. must be rejected")
	}
}

func TestFetchSingleBinary(t *testing.T) {
	f := NewHTTPFetcher(&stubDoer{body: []byte("rawbinary")})
	dest := filepath.Join(t.TempDir(), "releases", "v1")
	if err := f.Fetch(context.Background(), "https://x/rue-${VERSION}", "v1", dest, io.Discard); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "rue-v1"))
	if err != nil || string(got) != "rawbinary" {
		t.Fatalf("single binary not written: %v / %q", err, got)
	}
}

func TestFetchHTTPError(t *testing.T) {
	f := NewHTTPFetcher(&stubDoer{status: 404})
	dest := filepath.Join(t.TempDir(), "r")
	if err := f.Fetch(context.Background(), "https://x/a.tar.gz", "v1", dest, io.Discard); err == nil {
		t.Fatal("non-200 must error")
	}
}
