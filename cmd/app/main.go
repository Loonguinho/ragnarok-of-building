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
// A catalog.json.gz placed next to the executable (one someone already
// built) is used instead, so no client is needed.
//
// Its files (.env, data\) live next to the executable.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
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

	catalog := catalogPath
	client := *clientFlag
	if _, err := os.Stat(sharedCatalog); err == nil && !*reimport && client == "" {
		fmt.Println("Usando os dados do jogo do arquivo", sharedCatalog, "desta pasta.")
		catalog = sharedCatalog
	} else if client == "" {
		client = setup.FindClient()
	}
	if why := needsImport(client, *reimport); catalog == catalogPath && why != "" {
		fmt.Println(why)
		if client == "" || !setup.IsClient(client) {
			client = askClient()
		}
		if err := importCatalog(client); err != nil {
			return err
		}
	}

	cat, err := gamedata.Load(catalog)
	if err != nil {
		return fmt.Errorf("catálogo do jogo: %w (rode de novo com -import)", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	srv, err := web.New(web.Options{Log: log, Catalog: cat, NoRateLimit: true,
		SoundDir: filepath.Join(filepath.Dir(catalogPath), "sounds"), PasteURL: os.Getenv("PASTE_URL")})
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

// askClient asks for the client folder until one has a data.grf, and
// remembers it in .env.
func askClient() string {
	fmt.Println()
	fmt.Println(`Não achei o cliente do RO LATAM nas pastas de sempre (ex.: C:\Gravity\Ragnarok).`)
	fmt.Println("Se alguém te passou o arquivo " + sharedCatalog + ", feche esta janela, coloque o arquivo")
	fmt.Println("na mesma pasta do RagnarokOfBuilding.exe e abra de novo.")
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
