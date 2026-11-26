# Web migration: progress and handoff

Status log for moving Ragnarok of Building to the browser. Read this first when picking the work up (person or Claude session). Branch: **`web`**. Last updated 2026-11-26 (step 3).

## Goal

Run **the same Gio app**, compiled to WebAssembly, as a static site on **GitHub Pages**. The old HTMX server (`legacy-web` branch) is not revived. Originally a Cloudflare Worker was to proxy Divine Pride icons and paste.rs; **dropped** (see step 4). The desktop program must keep working as it does today.

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
| 2. `cmd/web` + `web/index.html`, icons off, `make web-serve` | ✅ done, runs in Chrome (see "Verified") | `dee9a57` |
| 3. `yield()` so the optimizer doesn't freeze the page | ✅ done | this commit |
| 4a. Share links (`#b=`), no paste on web | ✅ done | this commit |
| 4b. Icons from the client `.grf` | 📝 **TODO** — decided, waiting for the client (see "TODO: icons") | |
| 5. GitHub Pages workflow + docs + README link | ✅ done — goes live once `web` is merged to `main` and Pages is switched on | this commit |

## Testing in a browser without the Chrome extension

Headless Chrome's `--virtual-time-budget` does not wait for CPU work, so screenshots come out on the loading screen. What worked: start `google-chrome --headless=new --remote-debugging-port=9333 --enable-unsafe-swiftshader --use-angle=swiftshader`, and drive it with a ~40-line Node 24 script (built-in `WebSocket`) over the DevTools protocol: `Page.navigate`, wait ~15 s, `Input.dispatchMouseEvent`/`dispatchKeyEvent`/`insertText`, `Runtime.evaluate`, `Page.captureScreenshot`. To measure responsiveness, inject a `requestAnimationFrame` counter that records the longest gap between frames. Window coordinates at 1440×900: Otimizador button (85,212), Calcular (601,219), FOR field (228,196).

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

### Step 3 — page stays responsive during searches
- `internal/view/yield.go` (no-op off the browser) and `yield_js.go`: after each ~30 ms of work, wait for one `requestAnimationFrame` (or 50 ms if the tab is hidden and frames stop). A plain `time.Sleep(1ms)` is not enough: Gio paints only on animation frames, and a 1 ms sleep usually ends before one.
- Called once per candidate in the six worker pools: `optimize.go`, `swap.go`, `best.go`, `bestcard.go`, `traits.go` (`evalAll`), `optextra.go` (`runPar`). The UI already runs all of these in goroutines, never inside a frame.
- Measured in headless Chrome: optimizer (Dano, default options) finishes in < 5 s, page paints ~40 fps meanwhile, longest gap 260 ms; with "Considerar mudar os talentos" ~47 fps, longest gap 359 ms. Results match the desktop's shape (full list, traits suggestion).

### Step 4a — share links without paste.rs
- **No Worker.** Checked 2026-11-26: `static.divine-pride.net` now answers **403 to every request**, including the desktop program's own (`User-Agent: ragnarok-of-building`) — desktop icons are broken too. Divine Pride is refusing on purpose; a Worker that poses as a browser to get around that is not something we do. paste.rs sends no CORS headers (preflight 404).
- Share on web = the page's own link: `https://<site>/#b=<code>` (`cmd/web` `shareURL`). The fragment never reaches a server; a typical code is ~180 chars.
- `App.ShareURL` (nil on desktop) adds "Link (abre no navegador)" to the Share window, default on web. `App.NoPaste` (web) hides the paste option and answers pasted paste.rs links with a clear message.
- `cmd/web` `openLinkedBuild`: opens `#b=` (and old `?b=`) at load and on `hashchange`, through `App.OpenShared` (runs on the UI goroutine), then `history.replaceState` drops the code so a reload keeps the player's edits.
- `codeInLink` (ui and view) accepts `#b=` as well as `?b=`.
- Verified in headless Chrome: replay build's code → fresh storage → open `/#b=<code>` → the replay build (FOR 120, +9 Faca, 613 dmg) shows and the address bar is back to `/`. Share window shows the Link option. **Not verified:** the copy to clipboard (headless has no clipboard).

### Review fixes (code review of the PR `edudebortoli/ragnarok-of-building` main ← `Loonguinho:web`)
- **Open errors were silent**: `App.openFile` now shows a failed read in the chat; a cancelled picker returns `ui.ErrNoFile` and stays quiet (desktop and web hosts return it).
- **Picker could stick forever** (no `cancel` event in Safari < 16.4): the web `openFile` also treats "page got focus back, no file chosen 1 s later" as a cancel; listeners are removed before release. Verified: after a `cancel` event, and after a `focus` event, the hidden input is gone and the next click opens a new picker.
- **Last change lost on tab close**: `cmd/web` `saveOnHide` calls `SaveNow` on `visibilitychange` (hidden) and `pagehide`. Verified: edit a stat, navigate away immediately, come back — the edit is there.
- **yield**: all searches waiting at once share one frame (`nextFrame` channel closed by the frame), so two concurrent searches can't steal each other's frame and all of them stop so the page can draw. Re-measured: ~47 fps, longest gap 239 ms.
- Save error no longer names `builds.json` ("Não consegui gravar os builds."; EN/ES updated); a builds backup over 16 MB gets the size message instead of a JSON error.
- Replay import verified in the browser through the real picker (CDP `Page.setInterceptFileChooserDialog` + `DOM.setFileInputFiles` with `internal/replay/testdata/synthetic.rrf`).

**Desktop is discontinued** (decision 2026-11-26): the `desktop` branch is kept as a snapshot; test and verify only the web build from now on.

## Verified (2026-11-26, headless Chrome on Linux, driven over the DevTools protocol)
- `go vet` + `go test ./...` pass; vet passes for Linux, `GOOS=windows`, and `GOOS=js GOARCH=wasm`.
- Desktop program still starts (Linux, `go run ./cmd/app`).
- Web: `app.wasm` loads, startup screen shows "Baixando os dados do jogo… 4.2 MB" then "Preparando…", the full simulator draws (~10 s on swiftshader).
- Typing a stat works (FOR → 130, damage 159 → 427).
- **Alt+D** reaches the program (opens Alvo/Dano).
- State persists across reload (`rob:data/ui.json` in localStorage; FOR 130 and the open window come back).

## Not yet tested (do these by hand in a real browser)
1. **Alt shortcuts in a real Chrome/Firefox window**: synthetic keys from DevTools never trigger browser shortcuts, so check that Alt+D doesn't jump to the address bar and Alt+E/Alt+F don't open menus (Windows/Linux).
2. **Import replay** in a real browser (headless already passed): the picker shows only `.rrf`; Cancel does nothing and a second click still works — try Safari too, it is the browser the focus fallback is for.
3. **A file over 16 MB** → "Arquivo grande demais (máx. 16 MB)."
4. **Builds**: Salvar, rename, delete; reload and they're still there.
5. **Export builds** → a `builds-ragnarok-of-building-<date>.json` download; **Import** it back.
6. **Theme and language** persist after reload (Configurações).
7. **Firefox** as well as Chrome (WebGL, file picker `cancel` event).
8. **Private window**: the app should still open (storage errors are ignored, nothing saved).
9. **Optimizer** on a slow machine/phone: the page should keep repainting (hover, scroll) while it runs; note the 30 s timeout may give partial results more often than on the desktop (one core instead of all).

## Known limits / decisions
- **Size**: `app.wasm` is 27 MB (6.8 MB gzipped) + 4.3 MB catalog ≈ 11 MB first visit if the host gzips. **Check GitHub Pages actually gzips `.wasm`**; if not, fall back to Cloudflare Pages or ship a pre-gzipped file decompressed with `DecompressionStream`.
- **One thread**: Go in wasm has no preemption, so CPU work freezes drawing unless it calls `yield()`. Catalog decode takes ~1–2 s (a status line is drawn first). Searches yield (step 3); other one-off loops (`Compare`, `CompareSets`, `BestSetEnchants`) do not yet — add `yield()` there if they feel stuck.
- **Searches use one core** in the browser (`runtime.NumCPU()` = 1), so they are slower than the desktop; the optimizer's 30 s timeout may cut more searches short.
- **Icons off** on web until the Worker exists (`a.Icons` is nil; the UI handles nil).
- **Fonts**: no Tahoma in the browser; Go fonts are used (as on Linux).
- **localStorage** is ~5 MB per site; builds are small, but a write error is shown if full.
- Memory: catalog ~64 MB live, ~115 MB peak while loading.

## Next steps (detail)

### TODO: icons from the client `.grf` (decided 2026-11-26)

**Decision:** take icons from the LATAM client's `data.grf`, not Divine Pride. Same pixels (Divine Pride's images come from the client), every LATAM-only item covered, and no dependency on a site that now refuses us (403). **Blocked on:** the client is only on the Windows PC — do this from there (or with `RO_CLIENT=<copy of the client>` in `.env`).

Already in place: `internal/grf` reads `data.grf` (`grf.Open`, `Archive.Find`, `Archive.Read`); the importer keeps each item's resource name (`it.Resource`, from `identifiedResourceName` in `internal/clientimport/import.go`); `Icons.Get` takes PNG bytes, so any source plugs in.

- [ ] **Find the paths.** `go run ./cmd/grftool <client>/data.grf find item` and `... find collection`: expect `data\texture\유저인터페이스\item\<resource>.bmp` (24×24 icon) and `...\collection\<resource>.bmp` (75×100 art). Check how `internal/grf` spells the Korean folder (EUC-KR/CP949 bytes in the GRF).
- [ ] **Skill icons.** Find where the client keeps them (`grftool find` a known skill's resource name; likely the same `item` folder) and how a skill ID maps to its file (skillinfo lub tables the importer already reads).
- [ ] **BMP → PNG.** Decode the BMP (`golang.org/x/image/bmp`), turn the magenta key (`#FF00FF`, and near-magenta) into transparency, encode PNG.
- [ ] **Import step.** `cmd/import` (or a new `cmd/iconimport`) writes the PNGs. Keep them out of `data/gen/catalog.json.gz`; decide the shipped form:
  - a folder `web/icons/<kind>/<id>.png` copied by `make web` (simple; thousands of small requests, fine over HTTP/2), or
  - one archive per kind (e.g. a zip or a packed file with an index) fetched once (fewer requests, more code).
  Check the total size before choosing.
- [ ] **Serve on web.** `view.NewWebIcons(base)`: GET `pageURL(base + rel)` via net/http (fetch), map today's Divine Pride–style paths (`ItemIcon`, `ItemArt`, `SkillIcon`, `JobIcon` in `view/imgcache.go`) to the new files. Wire `a.Icons = ui.NewIcons(...)` in `cmd/web/main.go`.
- [ ] **Licence note.** README/DESENVOLVIMENTO: the icons are game data © Gravity, like the catalog the repo already ships.
- [ ] **Later: monsters.** Their pictures are sprites (`data\sprite\몬스터\*.spr` + `.act`); draw the first frame of the idle action to PNG. More work, so after items and skills; the UI works without them meanwhile.
- [ ] **Later: job icons** (`JobIcon`): find their files in the client the same way.
- [ ] Desktop is discontinued, so its `view.IconCache` (Divine Pride) can be removed once the web uses the client icons.

### Step 5 — publish (done)
- `.github/workflows/web.yml`: on push to `main` and manual dispatch → vet (wasm) + `go test ./internal/...` → `make web` → `actions/upload-pages-artifact@v4` → `actions/deploy-pages@v4`. Runs on `main` rather than tags because the `github-pages` environment only lets the default branch deploy unless told otherwise.
- `make web` now also writes `app.wasm.gz` (6.8 MB instead of 27 MB): the page fetches it and unzips with `DecompressionStream` (skipped if the host already unzipped it); browsers without it fetch `app.wasm`. So the download is small whether or not Pages compresses `.wasm`. `index.html`, `wasm_exec.js`, the wasm and the catalog URLs carry `?v=<git describe>` (`-X main.version` for the catalog).
- **CI fix:** `ci` had been failing since `7c77e9b` (the Linux window port): `go vet ./...` on Ubuntu needs Gio's cgo libraries for `cmd/app`. `ci.yml` installs them (names checked on Ubuntu 24.04) and also vets the wasm build. Only `cmd/app` needs them; `internal/...` vets without cgo, so `web.yml` skips them.
- Verified locally: gzip path and the no-`DecompressionStream` fallback both load; served from a `/ragnarok-of-building/` subfolder (as Pages does) the app loads and a `#b=` link opens its build.
- README: "▶️ Abrir no navegador" link and browser instructions; `docs/DESENVOLVIMENTO.md`: "Web version" section.

**To go live (manual, once):**
1. Merge `web` into `main` (fast-forward: `git switch main && git merge --ff-only web && git push`).
2. GitHub → Settings → Pages → Source: **GitHub Actions**.
3. The `web` workflow runs on that push (or start it from the Actions tab). The site: https://loonguinho.github.io/ragnarok-of-building/
4. Check in a real browser: loads, Alt shortcuts, import a replay, save builds, share link, the "Not yet tested" list above.
