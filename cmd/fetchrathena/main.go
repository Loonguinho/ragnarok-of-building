// fetchrathena downloads the rAthena (GPL-3.0) renewal database files the
// importer uses into data/rathena/. They are not committed to this repo.
//
//	go run ./cmd/fetchrathena
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Pinned for reproducible imports; bump deliberately.
const defaultCommit = "dd073ca4720b90e4eaf90fd5788eb7676f1b9146"

var files = []string{
	"db/re/item_db_equip.yml",
	"db/re/item_db_etc.yml",
	"db/re/job_stats.yml",
	"db/re/job_basepoints.yml",
	"db/re/job_aspd.yml",
	"db/re/statpoint.yml",
	"db/re/refine.yml",
	"db/re/size_fix.yml",
	"db/re/job_exp.yml",
	"db/re/mob_db.yml",
	"db/re/attr_fix.yml",
	"db/re/item_combos.yml",
	"db/re/item_db_usable.yml",
	"src/common/mmo.hpp", // e_job enum: job name -> ID
}

func main() {
	commit := flag.String("commit", defaultCommit, "rAthena commit")
	out := flag.String("out", "data/rathena", "output folder")
	flag.Parse()

	client := &http.Client{Timeout: 2 * time.Minute}
	for _, f := range files {
		url := "https://raw.githubusercontent.com/rathena/rathena/" + *commit + "/" + f
		dst := filepath.Join(*out, filepath.FromSlash(f))
		if err := download(client, url, dst); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", f, err)
			continue
		}
		st, _ := os.Stat(dst)
		fmt.Printf("%9d  %s\n", st.Size(), f)
	}
	os.WriteFile(filepath.Join(*out, "COMMIT"), []byte(*commit+"\n"), 0o644)
}

func download(c *http.Client, url, dst string) error {
	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 64<<20)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
