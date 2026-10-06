package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveChecksumAndContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "formatfast.tar.gz")
	want := []byte("binary fixture")
	hash, err := writeArchive(path, want)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if hash != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatal("checksum does not match archive")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	header, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if header.Name != "formatfast" || header.Mode != 0755 {
		t.Fatalf("unexpected tar header: %+v", header)
	}
	got, err := io.ReadAll(tr)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("payload: %q %v", got, err)
	}
	if _, err = tr.Next(); err != io.EOF {
		t.Fatalf("extra archive entry: %v", err)
	}
	again, err := writeArchive(path, want)
	if err != nil || again != hash {
		t.Fatal("packaging is not deterministic", err)
	}
}

func TestVersionValidation(t *testing.T) {
	for _, s := range []string{"", "0.3.0-dev", "../1.2.3", "01.2.3", "1.2", "1.2.3;echo bad"} {
		if err := buildRelease(s, filepath.Join(t.TempDir(), "out")); err == nil {
			t.Fatalf("accepted version %q", s)
		}
	}
}

func TestBuildEnvironment(t *testing.T) {
	t.Setenv("GOOS", "windows")
	t.Setenv("GOARCH", "386")
	t.Setenv("CGO_ENABLED", "1")
	t.Setenv("GOFLAGS", "-race")
	values := map[string]string{}
	for _, entry := range buildEnvironment("linux", "arm64") {
		k, v, _ := strings.Cut(entry, "=")
		if _, ok := values[k]; ok {
			t.Fatalf("duplicate env var %s", k)
		}
		values[k] = v
	}
	if values["GOOS"] != "linux" || values["GOARCH"] != "arm64" || values["CGO_ENABLED"] != "0" || values["GOFLAGS"] != "" {
		t.Fatal("build environment not isolated")
	}
}
