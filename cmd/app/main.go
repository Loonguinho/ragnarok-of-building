// app is Ragnarok of Building as one program for players: double-click it
// and the simulator opens in its own window (no console, no browser).
//
// The release zip ships the game catalog (catalog.json.gz next to the
// executable), used as is. Without it, on first run (and after each game
// patch) it builds the catalog from the player's own RO LATAM client,
// downloading rAthena's tables (GPL-3.0) and latam-ro-calc's monster list
// (MIT) once. The window shows that progress, and asks for the client
// folder when it is not found.
//
//	go run ./tools/buildexe            # the .exe with its icon, as released
//	RagnarokOfBuilding.exe             # find the client, import if needed, open the window
//	RagnarokOfBuilding.exe -import     # rebuild the catalog now
//	RagnarokOfBuilding.exe -client D:\RO
//
// The client can also be a copy: a data.grf and the System folder copied
// next to the executable are read instead of the installed game. A ready
// catalog.json.gz placed there is used as is.
//
// Its files (.env, data\, builds.json, RagnarokOfBuilding.log) live next to
// the executable.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ragnarok-of-building/internal/config"
	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/setup"
	"ragnarok-of-building/internal/view"
)

const (
	appTitle    = "Ragnarok of Building"
	catalogPath = `data/gen/catalog.json.gz`
	rathenaDir  = `data/rathena`
	// sharedCatalog is a ready catalog someone passed along: dropped next
	// to the executable, it is used as is (no client needed).
	sharedCatalog = `catalog.json.gz`
	logFile       = `RagnarokOfBuilding.log`
)

// runNative opens the program's window (native.go) and never returns.
var runNative func(st *startup, o options, log *slog.Logger)

// isGoRun says whether exe was built by `go run` (in a go-build folder
// under the temporary directory).
func isGoRun(exe string) bool {
	return strings.Contains(filepath.ToSlash(exe), "/go-build")
}

type options struct {
	reimport bool
	client   string
}

func main() {
	if err := run(); err != nil {
		fatal(err.Error())
		os.Exit(1)
	}
}

func run() error {
	var o options
	flag.BoolVar(&o.reimport, "import", false, "rebuild the catalog from the client now")
	flag.StringVar(&o.client, "client", "", "RO LATAM client folder (default: RO_CLIENT in .env, else the usual install folders)")
	flag.Parse()

	// Work next to the executable, wherever it was started from; under
	// `go run` the executable sits in a temporary build folder, so stay in
	// the folder it was run from (the repository).
	if exe, err := os.Executable(); err == nil && !isGoRun(exe) {
		if err := os.Chdir(filepath.Dir(exe)); err != nil {
			return err
		}
	}
	if !singleInstance() {
		focusExisting()
		return nil
	}
	log := openLog()
	_ = config.LoadDotEnv(".env")
	runNative(newStartup(log, pickFolder), o, log)
	return nil
}

// openLog writes the program's messages to RagnarokOfBuilding.log (there
// is no console), starting it over at each run.
func openLog() *slog.Logger {
	var w io.Writer = io.Discard
	if f, err := os.Create(logFile); err == nil {
		w = f
	}
	if runtime.GOOS != "windows" {
		w = io.MultiWriter(w, os.Stdout)
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

// prepare finds the game data and returns the simulator's views. Where the
// game data comes from, first match wins: -client; a data.grf (+ System)
// copied into this folder; a ready catalog.json.gz here; the client in
// RO_CLIENT or the usual install folders; the catalog attached to this
// version's GitHub release (downloaded once into this folder).
func prepare(st *startup, o options, log *slog.Logger) (*view.Views, error) {
	catalog := catalogPath
	client := o.client
	here, _ := os.Getwd()
	switch {
	case client != "":
	case setup.IsClient(here):
		st.say("Usando o data.grf desta pasta.")
		if err := checkSystem(here); err != nil {
			return nil, err
		}
		client = here
	case !o.reimport && exists(sharedCatalog):
		st.say("Usando os dados do jogo do arquivo " + sharedCatalog + " desta pasta.")
		catalog = sharedCatalog
	default:
		client = setup.FindClient()
	}
	why := needsImport(client, o.reimport)
	if catalog == catalogPath && why != "" && client == "" && !o.reimport {
		// No game here: the catalog published with this version.
		if err := downloadCatalog(st, catalogURL(), sharedCatalog); err == nil {
			catalog, why = sharedCatalog, ""
		} else {
			st.say("Não deu para baixar: " + err.Error())
		}
	}
	if catalog == catalogPath && why != "" {
		st.say(why)
		if client == "" || !setup.IsClient(client) {
			client = st.askClient(`Não achei o cliente do RO LATAM nas pastas de sempre (ex.: C:\Gravity\Ragnarok). Escolha a pasta do jogo (a que tem o data.grf).`)
		}
		if err := importCatalog(st, client); err != nil {
			// The client could not be read (no internet for rAthena, a
			// broken copy...): the catalog published with this version.
			st.say("Não deu para ler o jogo: " + err.Error())
			if exists(sharedCatalog) {
				catalog = sharedCatalog
			} else if derr := downloadCatalog(st, catalogURL(), sharedCatalog); derr == nil {
				catalog = sharedCatalog
			} else {
				return nil, err
			}
			st.say("Usando os dados do jogo publicados com esta versão.")
		}
	}

	st.say("Carregando os dados do jogo…")
	cat, err := gamedata.Load(catalog)
	if err != nil {
		return nil, fmt.Errorf("catálogo do jogo: %w (abra de novo com -import)", err)
	}
	return view.New(view.Options{Log: log, Catalog: cat, PasteURL: os.Getenv("PASTE_URL")})
}

// needsImport says why the catalog must be (re)built, or "" when it is
// current: missing, asked for, or older than the client files (a patch).
func needsImport(client string, forced bool) string {
	st, err := os.Stat(catalogPath)
	switch {
	case forced:
		return "Reimportando os dados do jogo…"
	case err != nil:
		return "Primeira vez: vou ler os dados do seu cliente do RO (leva uns segundos)."
	case client == "":
		return ""
	}
	for _, f := range []string{"data.grf", filepath.Join("System", "iteminfo_new.lub")} {
		if c, err := os.Stat(filepath.Join(client, f)); err == nil && c.ModTime().After(st.ModTime()) {
			return "O jogo foi atualizado desde a última vez: relendo os dados do cliente."
		}
	}
	return ""
}

// Set at release time (-ldflags -X): the version tag and the GitHub
// repository whose release holds the ready catalog.
var (
	version = ""
	repo    = "Loonguinho/ragnarok-of-building"
)

// catalogURL is the catalog.json.gz attached to this version's release
// (or to the latest release, for builds without a version).
func catalogURL() string {
	if version != "" {
		return "https://github.com/" + repo + "/releases/download/" + version + "/" + sharedCatalog
	}
	return "https://github.com/" + repo + "/releases/latest/download/" + sharedCatalog
}

// downloadCatalog fetches the catalog at u into dst, keeping it only if it
// loads.
func downloadCatalog(st *startup, u, dst string) error {
	st.say("Baixando os dados do jogo da página do programa no GitHub…")
	c := &http.Client{Timeout: 5 * time.Minute}
	res, err := c.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d (a versão não tem o %s anexado)", res.StatusCode, sharedCatalog)
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, io.LimitReader(res.Body, 256<<20))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		_, err = gamedata.Load(tmp) // keep it only if it reads
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// checkSystem reports the System files the importer needs from a client
// folder (item names and descriptions in each language), so a data.grf
// copied without them gets a clear message.
func checkSystem(dir string) error {
	var missing []string
	for _, f := range []string{`System\iteminfo_new.lub`, `System\english\iteminfo_new.lub`, `System\spanish\iteminfo_new.lub`} {
		if !exists(filepath.Join(dir, f)) {
			missing = append(missing, f)
		}
	}
	if missing == nil {
		return nil
	}
	return fmt.Errorf("falta(m) %s nesta pasta.\n"+
		"Copie a pasta System do jogo (ex.: C:\\Gravity\\Ragnarok\\System) para cá, ao lado do data.grf", strings.Join(missing, ", "))
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func importCatalog(st *startup, client string) error {
	st.say("Cliente do RO: " + client)
	ctx := context.Background()
	if err := setup.FetchRathena(ctx, rathenaDir, st.say); err != nil {
		return fmt.Errorf("baixar os dados do rAthena (precisa de internet na primeira vez): %w", err)
	}
	start := time.Now()
	c, err := setup.BuildCatalog(client, rathenaDir, catalogPath, st.say)
	if err != nil {
		return err
	}
	st.say(fmt.Sprintf("%d itens, %d habilidades, %d classes (%s)", len(c.Items), len(c.Skills), len(c.Jobs), time.Since(start).Round(time.Second)))
	return nil
}

// remember appends KEY=value to .env (next to the executable).
func remember(k, v string) {
	f, err := os.OpenFile(".env", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s=%s\n", k, v)
}
