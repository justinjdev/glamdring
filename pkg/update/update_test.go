package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
