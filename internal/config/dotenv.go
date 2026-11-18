// Package config loads settings from the environment and an optional .env file.
package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv reads KEY=VALUE lines from path and sets any variable that is not
// already set in the environment. A missing file is not an error.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(strings.TrimPrefix(k, "export "))
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, v)
		}
	}
	return sc.Err()
}

// DefaultClientDir is where the RO LATAM installer puts the client.
const DefaultClientDir = `C:\Gravity\Ragnarok`

// ClientDir is the RO client folder the importer and the tools read: RO_CLIENT
// (from the environment or the nearest .env, looking up from the current
// folder so tests in subpackages find it), else DefaultClientDir. Point it
// at a copy of the client to keep the installed game untouched.
func ClientDir() string {
	if v := os.Getenv("RO_CLIENT"); v != "" {
		return v
	}
	dir, err := os.Getwd()
	for err == nil {
		if _, statErr := os.Stat(dir + string(os.PathSeparator) + ".env"); statErr == nil {
			_ = LoadDotEnv(dir + string(os.PathSeparator) + ".env")
			break
		}
		parent := dir[:max(strings.LastIndexAny(dir, `/\`), 0)]
		if parent == "" || parent == dir {
			break
		}
		dir = parent
	}
	if v := os.Getenv("RO_CLIENT"); v != "" {
		return v
	}
	return DefaultClientDir
}
