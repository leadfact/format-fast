// Release packages binaries and a matching Homebrew formula locally.
// It never creates tags, commits, pushes or publishes a release.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"
)

const repository = "https://github.com/leadfact/format-fast"

var stableVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type asset struct{ Name, URL, SHA256 string }
type release struct {
	Version                                string
	MacARM, MacIntel, LinuxARM, LinuxIntel asset
}

func main() {
	version := flag.String("version", "", "stable version, e.g. 0.3.0 or v0.3.0")
	out := flag.String("out", "dist", "output directory")
	flag.Parse()
	if err := buildRelease(strings.TrimPrefix(*version, "v"), *out); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func buildRelease(version, out string) error {
	if !stableVersion.MatchString(version) {
		return fmt.Errorf("expected stable version X.Y.Z, got %q", version)
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(out, ".build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	r := release{Version: version}
	targets := []struct {
		os, arch string
		asset    *asset
	}{
		{"darwin", "arm64", &r.MacARM}, {"darwin", "amd64", &r.MacIntel},
		{"linux", "arm64", &r.LinuxARM}, {"linux", "amd64", &r.LinuxIntel},
	}
	var checksums strings.Builder
	for _, target := range targets {
		fmt.Printf("Building %s/%s\n", target.os, target.arch)
		binary := filepath.Join(staging, "formatfast")
		cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags",
			"-s -w -X github.com/leadfact/format-fast/internal/cli.Version="+version,
			"-o", binary, "./cmd/formatfast")
		cmd.Env = buildEnvironment(target.os, target.arch)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err = cmd.Run(); err != nil {
			return err
		}
		data, err := os.ReadFile(binary)
		if err != nil {
			return err
		}
		name := fmt.Sprintf("formatfast_%s_%s_%s.tar.gz", version, target.os, target.arch)
		hash, err := writeArchive(filepath.Join(out, name), data)
		if err != nil {
			return err
		}
		*target.asset = asset{Name: name, URL: repository + "/releases/download/v" + version + "/" + name, SHA256: hash}
		fmt.Fprintf(&checksums, "%s  %s\n", hash, name)
	}
	var formula bytes.Buffer
	if err = template.Must(template.New("formula").Parse(formulaTemplate)).Execute(&formula, r); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(out, "formatfast.rb"), formula.Bytes(), 0644); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(out, "checksums.txt"), []byte(checksums.String()), 0644); err != nil {
		return err
	}
	fmt.Printf("Prepared %s (local files only)\n", out)
	return nil
}

func buildEnvironment(goos, goarch string) []string {
	var env []string
	for _, s := range os.Environ() {
		name, _, _ := strings.Cut(s, "=")
		switch name {
		case "GOOS", "GOARCH", "CGO_ENABLED", "GOAMD64", "GOARM64", "GOFLAGS":
			continue
		}
		env = append(env, s)
	}
	return append(env, "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0", "GOAMD64=v1", "GOARM64=v8.0", "GOFLAGS=")
}

func writeArchive(path string, binary []byte) (string, error) {
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(f, hash))
	tw := tar.NewWriter(gz)
	// Fixed timestamps and metadata make packaging deterministic for a binary.
	err = tw.WriteHeader(&tar.Header{Name: "formatfast", Mode: 0755, Size: int64(len(binary)), ModTime: time.Unix(0, 0)})
	if err == nil {
		_, err = tw.Write(binary)
	}
	if closeErr := tw.Close(); err == nil {
		err = closeErr
	}
	if closeErr := gz.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

const formulaTemplate = `# Generated from the release archives; do not edit checksums manually.
class Formatfast < Formula
  desc "Fast, lossless JSON formatter and multiline log viewer"
  homepage "https://github.com/leadfact/format-fast"
  version "{{.Version}}"

  on_macos do
    on_arm do
      url "{{.MacARM.URL}}"
      sha256 "{{.MacARM.SHA256}}"
    end
    on_intel do
      url "{{.MacIntel.URL}}"
      sha256 "{{.MacIntel.SHA256}}"
    end
  end

  on_linux do
    on_arm do
      url "{{.LinuxARM.URL}}"
      sha256 "{{.LinuxARM.SHA256}}"
    end
    on_intel do
      url "{{.LinuxIntel.URL}}"
      sha256 "{{.LinuxIntel.SHA256}}"
    end
  end

  def install
    bin.install "formatfast"
  end

  test do
    assert_match "formatfast #{version}", shell_output("#{bin}/formatfast --version")
    assert_equal "{\"amount\":1.2300,\"id\":9007199254740993}\n",
                 shell_output("#{bin}/formatfast --compact '{\"amount\":1.2300,\"id\":9007199254740993}'")
  end
end
`
