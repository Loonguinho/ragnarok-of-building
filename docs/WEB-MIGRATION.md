# Web migration: progress and handoff

Status log for moving Ragnarok of Building to the browser. Read this first when picking the work up (person or Claude session). Branch: **`web`**. Last updated 2026-11-26.

## Goal

Run **the same Gio app**, compiled to WebAssembly, as a static site on **GitHub Pages**. The old HTMX server (`legacy-web` branch) is not revived. A small **Cloudflare Worker** proxies Divine Pride icons and paste.rs, which refuse browser requests (no CORS). The desktop program must keep working as it does today.

## Branches

| Branch | What it is |
|---|---|
| `main` | Desktop program; also opens on Linux/macOS since `7c77e9b`. |
| `desktop` | Snapshot of the desktop program before the web work (`7c77e9b`). |
| `web` | This migration. Merge into `main` when the web version ships. |
| `legacy-web` | The retired HTMX server version. Not used. |

## Where things stand

| Step | Status | Commit |
|---|---|---|
| 1. Refactors with no web code (store, files as bytes, icons as bytes, `gamedata.Read`) | ✅ done | `7d51675` |
| 2. `cmd/web` + `web/index.html`, icons off, `make web-serve` | ✅ done, runs in Chrome (see "Verified") | this commit |
| 3. `yield()` so the optimizer doesn't freeze the page | ⏳ next | |
| 4. Cloudflare Worker, then icons + paste on web, then `?b=` links | ⏳ | |
| 5. GitHub Pages workflow + docs + README link | ⏳ | |

## How to run it

```bash
git switch web
make web          # builds dist/web: app.wasm, wasm_exec.js, index.html, icon.png, catalog.json.gz
make web-serve    # builds, then serves dist/web on http://localhost:8080
make vet          # vets desktop + GOOS=js GOARCH=wasm for internal/... and cmd/web
make run          # the desktop program, unchanged
```

## What was done

### Step 1 — `7d51675` "Read and write files by contents, through a store"
- `internal/store`: `Store` interface (`Read(key)`, `Write(key, b)`). `store.Dir(dir)` is the desktop version (temp file + rename, same files as before).
- `ui.App`: `StateFile`/`SavedFile` replaced by `Store`; keys are `ui.StateKey` = `data/ui.json` and `view.SavedKey` = `builds.json`. `NewApp(v, th, store)`, `UseSavedTheme(store)`.
- `App.OpenFile` now returns `(name, data []byte, err)`; new `App.SaveFile(name, data) (where, err)` for the builds export. Desktop versions in `cmd/app/files.go` (read capped at 16 MB + 1).
- `importReplayFile` / `importSavedFile` take `[]byte`.
- Icons: `Icons.Get` and `view.IconCache.Get` return PNG bytes; the disk cache is unchanged.
- `gamedata.Read(io.Reader)`; `gamedata.Load(path)` still works and calls it.

### Step 2 — web entry point
- `internal/store/browser.go` (`//go:build js`): `store.Browser()` on `localStorage`, keys `rob:<key>`; errors instead of panics when storage is blocked or full.
- `cmd/web/main.go`: same flow as `cmd/app/native.go`: startup screen → download `catalog.json.gz` (MB progress) → `gamedata.Read` → `ui.NewApp(store.Browser())`. Removes the page's `#loading` on the first frame. `DestroyEvent` saves but never exits.
- `cmd/web/browser.go`: `pageURL` (relative to the page, so a Pages subfolder works), `fetch` with progress, `openFile` (hidden `<input type=file>`, honours `cancel`, reads at most 16 MB + 1), `saveFile` (Blob + `<a download>`), `acceptOf` (`"Replays (*.rrf)|*.rrf"` → `.rrf`).
- `gamedata.Read` accepts gzipped **or** plain JSON, in case a server unzips the `.gz` on the way (`internal/gamedata/read_test.go`).
- `web/index.html`: loading screen with download bar for `app.wasm`, error message on failure, `#giowindow` host, and a capture-phase `keydown` listener that `preventDefault`s **Alt+letter** so the browser doesn't take the program's shortcuts (Gio itself doesn't prevent them).
- `tools/webserve`: tiny static server (`application/wasm`, `Cache-Control: no-store`).
- `Makefile`: `web`, `web-serve`; `vet` also vets the wasm build.

## Verified (2026-11-26, headless Chrome on Linux, driven over the DevTools protocol)
- `go vet` + `go test ./...` pass; vet passes for Linux, `GOOS=windows`, and `GOOS=js GOARCH=wasm`.
- Desktop program still starts (Linux, `go run ./cmd/app`).
- Web: `app.wasm` loads, startup screen shows "Baixando os dados do jogo… 4.2 MB" then "Preparando…", the full simulator draws (~10 s on swiftshader).
- Typing a stat works (FOR → 130, damage 159 → 427).
- **Alt+D** reaches the program (opens Alvo/Dano).
- State persists across reload (`rob:data/ui.json` in localStorage; FOR 130 and the open window come back).

## Not yet tested (do these by hand in a real browser)
1. **Alt shortcuts in a real Chrome/Firefox window**: synthetic keys from DevTools never trigger browser shortcuts, so check that Alt+D doesn't jump to the address bar and Alt+E/Alt+F don't open menus (Windows/Linux).
2. **Import replay** (button "Importar replay"): the picker shows only `.rrf`; the build opens; Cancel does nothing and a second click still works. Test file: `internal/replay/testdata/synthetic.rrf`.
3. **A file over 16 MB** → "Arquivo grande demais (máx. 16 MB)."
4. **Builds**: Salvar, rename, delete; reload and they're still there.
5. **Export builds** → a `builds-ragnarok-of-building-<date>.json` download; **Import** it back.
6. **Theme and language** persist after reload (Configurações).
7. **Firefox** as well as Chrome (WebGL, file picker `cancel` event).
8. **Private window**: the app should still open (storage errors are ignored, nothing saved).
9. **Optimizer**: it will run but **freezes the page** until done; that's step 3.
10. Desktop regression list from step 1: state, saved builds, export/import, replay import, icons, startup from the release zip.

## Known limits / decisions
- **Size**: `app.wasm` is 27 MB (6.8 MB gzipped) + 4.3 MB catalog ≈ 11 MB first visit if the host gzips. **Check GitHub Pages actually gzips `.wasm`**; if not, fall back to Cloudflare Pages or ship a pre-gzipped file decompressed with `DecompressionStream`.
- **One thread**: Go in wasm has no preemption, so CPU work freezes drawing. Catalog decode takes ~1–2 s (a status line is drawn first). The optimizer needs step 3.
- **Icons off** on web until the Worker exists (`a.Icons` is nil; the UI handles nil).
- **Fonts**: no Tahoma in the browser; Go fonts are used (as on Linux).
- **State saves after 400 ms of no changes**; closing the tab within that window loses the last change. Possible fix: save on `visibilitychange`, but `SaveNow` must run on the UI goroutine (use `App.later` or a frame).
- **localStorage** is ~5 MB per site; builds are small, but a write error is shown if full.
- Memory: catalog ~64 MB live, ~115 MB peak while loading.

## Next steps (detail)

### Step 3 — keep the page responsive
- Add `view.yield()`: `yield_js.go` does `time.Sleep(time.Millisecond)` at most every ~25 ms of work; `yield_other.go` is a no-op.
- Call it once per candidate in the worker loops: `view/optimize.go:316`, `swap.go:197`, `best.go:119`, `bestcard.go:77`, `traits.go:222`, `optextra.go:76`. (`runtime.NumCPU()` is 1 in wasm, so each pool already has one worker.)
- Test: run the optimizer in the browser; the window must keep redrawing (progress, hover).

### Step 4 — Worker, icons, paste, `?b=` links
- **First** test that Divine Pride answers a Worker (`curl` from a deployed Worker). If it's blocked too, pre-fetch icons into the site at release time instead.
- `tools/worker/`: `GET /dp/images/...` (only paths matching `dpImgPath` in `view/imgcache.go`), fetched server-side, edge-cached 30 days, `Access-Control-Allow-Origin: <pages origin>`; `POST /paste` → paste.rs (body ≤ 16 KB), `GET /paste/<id>` → raw paste. Deploy with `wrangler deploy`.
- `view.NewWebIcons(base)`: `GET base/rel` via net/http (fetch), keep the `dpImgPath` check. Wire `a.Icons` in `cmd/web/main.go`.
- `PasteURL: workerBase + "/paste"`; `pasteRawURL` (`view/paste.go`) must map `paste.rs/<id>` to `<worker>/paste/<id>` when reading.
- `?b=<code>` in the page URL opens that build (reuse the parser behind `openCode`, `win_saved.go`). Share window builds `https://<pages-url>/?b=<code>` on web.
- `workerBase`, `siteURL`, `version` via `-ldflags -X`.

### Step 5 — publish
- `.github/workflows/web.yml` on `v*` tags + manual dispatch: checkout, setup-go, `go test ./...`, `make web`, `actions/upload-pages-artifact` (`dist/web`), `actions/deploy-pages`; permissions `pages: write`, `id-token: write`.
- One-time: repo Settings → Pages → source "GitHub Actions".
- README: "▶️ Abrir no navegador" link next to the Windows download. `docs/DESENVOLVIMENTO.md`: web build, Worker, limits (single thread, data per browser, no client import).
