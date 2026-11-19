// app is Ragnarok of Building as one program for players: double-click it
// and it opens the simulator in the browser.
//
// On first run (and after each game patch) it builds the game catalog from
// the player's own RO LATAM client, downloading rAthena's tables (GPL-3.0)
// and latam-ro-calc's monster list (MIT) once. No game data ships with the
// program: everything is read from the client already installed.
//
//	go build -o RagnarokOfBuilding.exe ./cmd/app
//	RagnarokOfBuilding.exe            # find the client, import if needed, open the browser
//	RagnarokOfBuilding.exe -import    # rebuild the catalog now
//	RagnarokOfBuilding.exe -client D:\RO -port 8090
//
// The client can also be a copy: a data.grf and the System folder copied
// next to the executable are read instead of the installed game. A ready
// catalog.json.gz placed there is used as is.
//
// Its files (.env, data\) live next to the executable.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ragnarok-of-building/internal/config"
	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/setup"
	"ragnarok-of-building/internal/web"
)

const (
	catalogPath = `data/gen/catalog.json.gz`
	rathenaDir  = `data/rathena`
	// sharedCatalog is a ready catalog someone passed along: dropped next
	// to the executable, it is used as is (no client needed).
	sharedCatalog = `catalog.json.gz`
)

func main() {
	if err := run(); err != nil {
		fmt.Println()
		fmt.Println("ERRO:", err)
		pause()
		os.Exit(1)
	}
}

func run() error {
	reimport := flag.Bool("import", false, "rebuild the catalog from the client now")
	clientFlag := flag.String("client", "", "RO LATAM client folder (default: RO_CLIENT in .env, else the usual install folders)")
	port := flag.Int("port", 8080, "first port to try (the next free one up to +20 is used)")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser")
	flag.Parse()

	// Work next to the executable, wherever it was started from.
	if exe, err := os.Executable(); err == nil {
		if err := os.Chdir(filepath.Dir(exe)); err != nil {
			return err
		}
	}
	_ = config.LoadDotEnv(".env")
	fmt.Println("Ragnarok of Building — simulador de builds do RO LATAM")
	fmt.Println()

	// Where the game data comes from, first match wins: -client; a data.grf
	// (+ System) copied into this folder; a ready catalog.json.gz here; the
	// client in RO_CLIENT or the usual install folders; the catalog attached
	// to this version's GitHub release (downloaded once into this folder).
	catalog := catalogPath
	client := *clientFlag
	here, _ := os.Getwd()
	switch {
	case client != "":
	case setup.IsClient(here):
		fmt.Println("Usando o data.grf desta pasta.")
		if err := checkSystem(here); err != nil {
			return err
		}
		client = here
	case !*reimport && exists(sharedCatalog):
		fmt.Println("Usando os dados do jogo do arquivo", sharedCatalog, "desta pasta.")
		catalog = sharedCatalog
	default:
		client = setup.FindClient()
	}
	why := needsImport(client, *reimport)
	if catalog == catalogPath && why != "" && client == "" && !*reimport {
		// No game here: the catalog published with this version.
		if err := downloadCatalog(catalogURL(), sharedCatalog); err == nil {
			catalog, why = sharedCatalog, ""
		} else {
			fmt.Println("  Não deu para baixar:", err)
		}
	}
	if catalog == catalogPath && why != "" {
		fmt.Println(why)
		if client == "" || !setup.IsClient(client) {
			client = askClient()
		}
		if err := importCatalog(client); err != nil {
			// The client could not be read (no internet for rAthena, a
			// broken copy...): the catalog published with this version.
			fmt.Println("  Não deu para ler o jogo:", err)
			if exists(sharedCatalog) {
				catalog = sharedCatalog
			} else if derr := downloadCatalog(catalogURL(), sharedCatalog); derr == nil {
				catalog = sharedCatalog
			} else {
				return err
			}
			fmt.Println("Usando os dados do jogo publicados com esta versão.")
		}
	}

	cat, err := gamedata.Load(catalog)
	if err != nil {
		return fmt.Errorf("catálogo do jogo: %w (rode de novo com -import)", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	srv, err := web.New(web.Options{Log: log, Catalog: cat, NoRateLimit: true,
		SoundDir: filepath.Join(filepath.Dir(catalogPath), "sounds"), PasteURL: os.Getenv("PASTE_URL"),
		// Saved builds in a file next to the executable (not in the browser).
		SavedFile: "builds.json"})
	if err != nil {
		return err
	}
	ln, err := listen(*port)
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/build"
	hs := &http.Server{Handler: srv.Routes(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("ERRO:", err)
			os.Exit(1)
		}
	}()

	fmt.Println()
	fmt.Println("Pronto! O simulador está aberto em:", url)
	fmt.Println("Deixe esta janela aberta enquanto usa. Para sair, feche-a (ou Ctrl+C).")
	if !*noBrowser {
		openBrowser(url)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return hs.Shutdown(ctx)
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
func downloadCatalog(u, dst string) error {
	fmt.Println("Baixando os dados do jogo da página do programa no GitHub…")
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

// askClient asks for the client folder until one has a data.grf, and
// remembers it in .env.
func askClient() string {
	fmt.Println()
	fmt.Println(`Não achei o cliente do RO LATAM nas pastas de sempre (ex.: C:\Gravity\Ragnarok).`)
	fmt.Println("Ou feche esta janela, copie o data.grf e a pasta System do jogo para a pasta")
	fmt.Println("do RagnarokOfBuilding.exe e abra de novo.")
	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Cole aqui a pasta do jogo (a que tem o data.grf) e aperte Enter: ")
		if !in.Scan() {
			os.Exit(1)
		}
		dir := strings.Trim(strings.TrimSpace(in.Text()), `"`)
		if setup.IsClient(dir) {
			remember("RO_CLIENT", dir)
			return dir
		}
		fmt.Println("Essa pasta não tem o data.grf. Tente de novo.")
	}
}

func importCatalog(client string) error {
	fmt.Println("Cliente do RO:", client)
	progress := func(m string) { fmt.Println("  " + m) }
	ctx := context.Background()
	if err := setup.FetchRathena(ctx, rathenaDir, progress); err != nil {
		return fmt.Errorf("baixar os dados do rAthena (precisa de internet na primeira vez): %w", err)
	}
	start := time.Now()
	c, err := setup.BuildCatalog(client, rathenaDir, catalogPath, progress)
	if err != nil {
		return err
	}
	fmt.Printf("  %d itens, %d habilidades, %d classes (%s)\n", len(c.Items), len(c.Skills), len(c.Jobs), time.Since(start).Round(time.Second))
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

// listen takes the first free local port from port to port+20 (local only:
// the simulator is not exposed to the network).
func listen(port int) (net.Listener, error) {
	var err error
	for p := port; p <= port+20; p++ {
		var ln net.Listener
		if ln, err = net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p)); err == nil {
			return ln, nil
		}
	}
	return nil, fmt.Errorf("nenhuma porta livre entre %d e %d: %w", port, port+20, err)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Println("Abra no navegador:", url)
	}
}

// pause keeps the console open after an error (double-clicked programs
// close their window when they exit).
func pause() {
	fmt.Print("\nAperte Enter para fechar.")
	bufio.NewReader(os.Stdin).ReadString('\n')
}
