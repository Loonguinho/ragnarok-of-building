// buildexe builds the Windows program as released: RagnarokOfBuilding.exe
// with its icon, version details and manifest (Windows resources), as a
// windowed app (no console), plus the zip with LEIAME and LICENSE.
//
//	go run ./tools/buildexe                          # version from git describe
//	go run ./tools/buildexe -version v0.1.0-beta.3   # the release workflow passes the tag
//
// Output: dist/RagnarokOfBuilding/ and dist/RagnarokOfBuilding-<version>-windows.zip.
// Works from any OS (pure Go: cross-compiles with CGO_ENABLED=0).
package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

const (
	exeName = "RagnarokOfBuilding.exe"
	sysoOut = "cmd/app/rsrc_windows_amd64.syso" // linked into cmd/app by go build (Windows only)
	iconPNG = "./cmd/app/icon.png" // drawn by tools/icon
)

func main() {
	ver := flag.String("version", "", "version tag (default: git describe --tags, else dev)")
	repo := flag.String("repo", envOr("GITHUB_REPOSITORY", "Loonguinho/ragnarok-of-building"), "GitHub repository whose release holds the ready catalog")
	out := flag.String("out", "dist", "output folder")
	flag.Parse()
	if *ver == "" {
		*ver = gitVersion()
	}
	if err := run(*ver, *repo, *out); err != nil {
		fmt.Fprintln(os.Stderr, "buildexe:", err)
		os.Exit(1)
	}
}

func run(ver, repo, out string) error {
	if err := writeResources(ver, sysoOut); err != nil {
		return fmt.Errorf("resources: %w", err)
	}
	dir := filepath.Join(out, "RagnarokOfBuilding")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	exe := filepath.Join(dir, exeName)
	ldflags := "-s -w -H windowsgui -X main.version=" + ver + " -X main.repo=" + repo
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", exe, "./cmd/app")
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}
	for _, f := range []string{"docs/LEIAME.txt", "LICENSE"} {
		if err := copyFile(f, filepath.Join(dir, filepath.Base(f))); err != nil {
			return err
		}
	}
	zipPath := filepath.Join(out, "RagnarokOfBuilding-"+ver+"-windows.zip")
	if err := zipDir(dir, zipPath); err != nil {
		return err
	}
	fmt.Println("ok:", exe)
	fmt.Println("ok:", zipPath)
	return nil
}

// writeResources writes the .syso with the icon (#1: the window's IconId),
// the version details shown in the file's Properties, and the manifest.
func writeResources(ver, dst string) error {
	f, err := os.Open(iconPNG)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	icon, err := winres.NewIconFromResizedImage(img, []int{256, 64, 48, 32, 24, 16})
	if err != nil {
		return err
	}
	rs := &winres.ResourceSet{}
	if err := rs.SetIcon(winres.ID(1), icon); err != nil {
		return err
	}
	rs.SetManifest(winres.AppManifest{
		DPIAwareness:        winres.DPIPerMonitorV2, // sharp window on scaled screens
		UseCommonControlsV6: true,                   // modern dialogs (folder picker, message boxes)
		LongPathAware:       true,
	})
	var vi version.Info
	vi.SetFileVersion(numericVersion(ver))
	vi.SetProductVersion(ver)
	for k, v := range map[string]string{
		version.ProductName:      "Ragnarok of Building",
		version.FileDescription:  "Ragnarok of Building — simulador de builds do RO LATAM",
		version.CompanyName:      "Ragnarok of Building",
		version.LegalCopyright:   "MIT License. Ragnarok Online é marca da Gravity; projeto não oficial.",
		version.OriginalFilename: exeName,
		version.InternalName:     "RagnarokOfBuilding",
		version.Comments:         "https://github.com/Loonguinho/ragnarok-of-building",
	} {
		if err := vi.Set(version.LangDefault, k, v); err != nil {
			return err
		}
	}
	rs.SetVersionInfo(vi)
	var buf bytes.Buffer
	if err := rs.WriteObject(&buf, winres.ArchAMD64); err != nil {
		return err
	}
	return os.WriteFile(dst, buf.Bytes(), 0o644)
}

var reDigits = regexp.MustCompile(`\d+`)

// numericVersion: "v0.1.0-beta.3" -> "0.1.0.3" (Windows wants four numbers).
func numericVersion(v string) string {
	n := reDigits.FindAllString(v, 4)
	for len(n) < 4 {
		n = append(n, "0")
	}
	return strings.Join(n, ".")
}

func gitVersion() string {
	b, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output()
	if err != nil || len(bytes.TrimSpace(b)) == 0 {
		return "dev"
	}
	return string(bytes.TrimSpace(b))
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

// zipDir zips dir (as a top folder of the same name) into dst.
func zipDir(dir, dst string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	base := filepath.Dir(dir)
	err = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(base, p)
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name, h.Method = filepath.ToSlash(rel), zip.Deflate
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		src, err := os.Open(p)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(w, src)
		return err
	})
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
