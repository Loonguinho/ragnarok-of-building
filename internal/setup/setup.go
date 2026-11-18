// Package setup prepares the game data: it downloads the rAthena tables
// (GPL-3.0, not redistributed) and builds the catalog from the player's own
// RO LATAM client. Used by the desktop app on first run and by cmd/import.
package setup

import (
	"context"
	"encoding/binary"
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

// LatamCalcCommit pins latam-ro-calc (MIT), whose monster list has the
// LATAM monsters rAthena has no stats for; bump deliberately.
const LatamCalcCommit = "63131d2dc67796a76aef3e570689511e350d1be8"

// LatamCalcMonsters is where its monster list is kept (not committed).
func LatamCalcMonsters(rathenaDir string) string {
	return filepath.Join(filepath.Dir(rathenaDir), "latam-ro-calc", "monster.json")
}

// FetchLatamCalc downloads latam-ro-calc's monster list to dst unless it is
// already there for the pinned commit.
func FetchLatamCalc(ctx context.Context, dst string, progress Progress) error {
	stamp := filepath.Join(filepath.Dir(dst), "COMMIT")
	have, _ := os.ReadFile(stamp)
	if _, err := os.Stat(dst); err == nil && strings.TrimSpace(string(have)) == LatamCalcCommit {
		return nil
	}
	progress("Baixando a lista de monstros do latam-ro-calc…")
	url := "https://raw.githubusercontent.com/adsonpleal/latam-ro-calc/" + LatamCalcCommit + "/src/assets/demo/data/monster.json"
	if err := download(ctx, &http.Client{Timeout: 3 * time.Minute}, url, dst); err != nil {
		return fmt.Errorf("latam-ro-calc: %w", err)
	}
	return os.WriteFile(stamp, []byte(LatamCalcCommit+"\n"), 0o644)
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
	progress("Juntando os monstros do LATAM (latam-ro-calc)…")
	mobs := LatamCalcMonsters(rathenaDir)
	if err := FetchLatamCalc(context.Background(), mobs, progress); err != nil {
		progress("Lista de monstros do latam-ro-calc não baixada: " + err.Error())
	}
	if _, _, err := clientimport.MergeLatamCalc(c, mobs); err != nil {
		progress("Lista de monstros do latam-ro-calc não lida: " + err.Error())
	}
	progress("Lendo a lista de encantamentos do cliente…")
	if _, err := clientimport.ImportEnchants(clientDir, c, db.Aegis); err != nil {
		progress("Lista de encantamentos não lida: " + err.Error())
	}
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
		if err := os.WriteFile(filepath.Join(dir, name), padWAV(b), 0o644); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// padWAV appends 300 ms of silence (after a 20 ms fade) to a 16-bit PCM
// WAV. Browsers drop the last few milliseconds of short sounds when the
// playback ends; with the padding only silence gets dropped. Anything that
// isn't plain 16-bit PCM is returned unchanged.
func padWAV(b []byte) []byte {
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return b
	}
	var channels, bits uint16
	var rate uint32
	var data []byte
	for i := 12; i+8 <= len(b); {
		id, size := string(b[i:i+4]), int(binary.LittleEndian.Uint32(b[i+4:i+8]))
		body := b[i+8 : min(len(b), i+8+size)]
		switch id {
		case "fmt ":
			if len(body) < 16 || binary.LittleEndian.Uint16(body[0:2]) != 1 {
				return b // not PCM
			}
			channels, rate, bits = binary.LittleEndian.Uint16(body[2:4]), binary.LittleEndian.Uint32(body[4:8]), binary.LittleEndian.Uint16(body[14:16])
		case "data":
			data = body
		}
		i += 8 + size + size%2
	}
	if bits != 16 || channels == 0 || rate == 0 || len(data) < 4 {
		return b
	}
	frame := int(channels) * 2
	pcm := append([]byte(nil), data[:len(data)/frame*frame]...)
	// Fade the last 20 ms to zero so the end doesn't click.
	fade := min(len(pcm)/frame, int(rate)/50)
	for f := 0; f < fade; f++ {
		gain := float64(fade-f) / float64(fade)
		at := len(pcm) - (fade-f)*frame
		for c := 0; c < int(channels); c++ {
			o := at + c*2
			v := int16(binary.LittleEndian.Uint16(pcm[o:]))
			binary.LittleEndian.PutUint16(pcm[o:], uint16(int16(float64(v)*gain)))
		}
	}
	pcm = append(pcm, make([]byte, int(rate)*3/10*frame)...) // 300 ms of silence
	out := make([]byte, 44, 44+len(pcm))
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(36+len(pcm)))
	copy(out[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 1)
	binary.LittleEndian.PutUint16(out[22:], channels)
	binary.LittleEndian.PutUint32(out[24:], rate)
	binary.LittleEndian.PutUint32(out[28:], rate*uint32(frame))
	binary.LittleEndian.PutUint16(out[32:], uint16(frame))
	binary.LittleEndian.PutUint16(out[34:], 16)
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(len(pcm)))
	return append(out, pcm...)
}
