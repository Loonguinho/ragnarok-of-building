// Package setup prepares the game data: it downloads the rAthena tables
// (GPL-3.0, not redistributed) and builds the catalog from the player's own
// RO LATAM client. Used by the desktop app on first run and by cmd/import.
package setup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ragnarok-of-building/internal/clientimport"
	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/grf"
	"ragnarok-of-building/internal/rathena"
)

// RathenaCommit is pinned for reproducible imports; bump deliberately.
const RathenaCommit = "dd073ca4720b90e4eaf90fd5788eb7676f1b9146"

// RathenaFiles are the tables the importer reads.
var RathenaFiles = []string{
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

// Progress receives human-readable steps (PT).
type Progress func(msg string)

// FetchRathena downloads the rAthena tables into dir, skipping files that
// are already there for the pinned commit.
func FetchRathena(ctx context.Context, dir string, progress Progress) error {
	have, _ := os.ReadFile(filepath.Join(dir, "COMMIT"))
	sameCommit := strings.TrimSpace(string(have)) == RathenaCommit
	client := &http.Client{Timeout: 3 * time.Minute}
	for i, f := range RathenaFiles {
		dst := filepath.Join(dir, filepath.FromSlash(f))
		if _, err := os.Stat(dst); err == nil && sameCommit {
			continue
		}
		progress(fmt.Sprintf("Baixando dados do rAthena (%d/%d): %s", i+1, len(RathenaFiles), filepath.Base(f)))
		url := "https://raw.githubusercontent.com/rathena/rathena/" + RathenaCommit + "/" + f
		if err := download(ctx, client, url, dst); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	return os.WriteFile(filepath.Join(dir, "COMMIT"), []byte(RathenaCommit+"\n"), 0o644)
}

func download(ctx context.Context, c *http.Client, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
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

// BuildCatalog reads the client and the rAthena tables and writes the
// catalog to out.
func BuildCatalog(clientDir, rathenaDir, out string, progress Progress) (*gamedata.Catalog, error) {
	progress("Lendo o cliente do RO (itens, habilidades, classes)…")
	c, err := clientimport.Import(clientDir)
	if err != nil {
		return nil, fmt.Errorf("cliente do RO: %w", err)
	}
	progress("Juntando atributos, equipamentos e monstros do rAthena…")
	db, err := rathena.Load(rathenaDir)
	if err != nil {
		return nil, fmt.Errorf("rAthena: %w", err)
	}
	clientimport.MergeRathena(c, db)
	progress("Lendo os bônus das descrições dos itens exclusivos do LATAM…")
	clientimport.DescBonuses(c)
	c.ApplyLatamLimits()
	progress("Copiando os sons da interface…")
	if _, err := ExtractSounds(clientDir, filepath.Join(filepath.Dir(out), "sounds")); err != nil {
		progress("Sons da interface não encontrados: " + err.Error())
	}
	progress("Salvando o catálogo…")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return nil, err
	}
	if err := c.Save(out); err != nil {
		return nil, err
	}
	return c, nil
}

// IsClient reports whether dir looks like an RO client folder.
func IsClient(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "data.grf"))
	return err == nil && !st.IsDir()
}

// FindClient looks for the RO LATAM client in the usual install folders.
func FindClient() string {
	var cands []string
	for _, d := range "CDEFGH" {
		root := string(d) + `:\`
		cands = append(cands,
			root+`Gravity\Ragnarok`,
			root+`Gravity\Ragnarok Online`,
			root+`Program Files (x86)\Gravity\Ragnarok`,
			root+`Program Files\Gravity\Ragnarok`,
			root+`Ragnarok`,
			root+`Games\Ragnarok`,
		)
	}
	for _, c := range cands {
		if IsClient(c) {
			return c
		}
	}
	return ""
}

// UISounds are the client sounds the interface plays, by the name the site
// serves them under. They are read from the player's own client, never
// committed or redistributed with the code.
var UISounds = map[string]string{
	"click.wav": `data\wav\버튼소리.wav`, // the classic menu button click
}

// ExtractSounds copies the UI sounds from the client's data.grf into dir.
func ExtractSounds(clientDir, dir string) (int, error) {
	a, err := grf.Open(filepath.Join(clientDir, "data.grf"))
	if err != nil {
		return 0, err
	}
	defer a.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	n := 0
	for name, path := range UISounds {
		b, err := a.Read(path)
		if err != nil {
			continue // a client without the file: the site just stays silent
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
