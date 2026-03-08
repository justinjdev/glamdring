package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCheckLatest_NewerVersionAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ghRelease{
			TagName: "v0.3.0",
			Assets: []ghAsset{
				{Name: "glamdring_0.3.0_linux_amd64.tar.gz", URL: "https://example.com/glamdring_0.3.0_linux_amd64.tar.gz"},
				{Name: "checksums.txt", URL: "https://example.com/checksums.txt"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rel, err := checkLatest("v0.2.0", "linux", "amd64", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel == nil {
		t.Fatal("expected a release, got nil")
	}
	if rel.Version != "v0.3.0" {
		t.Errorf("expected version v0.3.0, got %s", rel.Version)
	}
	if rel.AssetURL != "https://example.com/glamdring_0.3.0_linux_amd64.tar.gz" {
		t.Errorf("unexpected asset URL: %s", rel.AssetURL)
	}
	if rel.ChecksumURL != "https://example.com/checksums.txt" {
		t.Errorf("unexpected checksum URL: %s", rel.ChecksumURL)
	}
}

func TestCheckLatest_AlreadyUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ghRelease{
			TagName: "v0.2.0",
			Assets: []ghAsset{
				{Name: "glamdring_0.2.0_linux_amd64.tar.gz", URL: "https://example.com/glamdring_0.2.0_linux_amd64.tar.gz"},
				{Name: "checksums.txt", URL: "https://example.com/checksums.txt"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rel, err := checkLatest("v0.2.0", "linux", "amd64", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != nil {
		t.Fatalf("expected nil (up to date), got %+v", rel)
	}
}

func TestCheckLatest_DevVersionSkipped(t *testing.T) {
	rel, err := checkLatest("dev", "linux", "amd64", "http://should-not-be-called")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != nil {
		t.Fatalf("expected nil for dev version, got %+v", rel)
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v0.3.0", "v0.2.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"v0.1.0", "v0.2.0", false},
		{"v1.0.0", "v0.9.9", true},
		{"v0.2.1", "v0.2.0", true},
		{"v0.2.0", "v0.2.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.latest+"_vs_"+tt.current, func(t *testing.T) {
			got := isNewer(tt.latest, tt.current)
			if got != tt.want {
				t.Errorf("isNewer(%s, %s) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}

func makeTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Size:     int64(len(content)),
		Mode:     0o755,
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestDownloadAndReplace(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho updated")
	tarball := makeTarGz(t, "glamdring", binaryContent)
	hash := sha256.Sum256(tarball)
	checksumLine := fmt.Sprintf("%x  glamdring_0.3.0_darwin_arm64.tar.gz\n", hash)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/glamdring_0.3.0_darwin_arm64.tar.gz":
			w.Write(tarball)
		case "/checksums.txt":
			w.Write([]byte(checksumLine))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tmpFile, err := os.CreateTemp("", "glamdring-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Write([]byte("old binary"))
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	err = Download(&Release{
		Version:     "v0.3.0",
		AssetURL:    srv.URL + "/glamdring_0.3.0_darwin_arm64.tar.gz",
		ChecksumURL: srv.URL + "/checksums.txt",
	}, tmpFile.Name())
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	got, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, binaryContent) {
		t.Errorf("binary content mismatch: got %q, want %q", got, binaryContent)
	}
}

func TestDownloadChecksumMismatch(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho updated")
	tarball := makeTarGz(t, "glamdring", binaryContent)
	badChecksum := strings.Repeat("0", 64) + "  glamdring_0.3.0_darwin_arm64.tar.gz\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/glamdring_0.3.0_darwin_arm64.tar.gz":
			w.Write(tarball)
		case "/checksums.txt":
			w.Write([]byte(badChecksum))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tmpFile, err := os.CreateTemp("", "glamdring-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Write([]byte("old binary"))
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	err = Download(&Release{
		Version:     "v0.3.0",
		AssetURL:    srv.URL + "/glamdring_0.3.0_darwin_arm64.tar.gz",
		ChecksumURL: srv.URL + "/checksums.txt",
	}, tmpFile.Name())
	if err == nil {
		t.Fatal("expected error for checksum mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("expected error containing 'checksum', got: %v", err)
	}
}
