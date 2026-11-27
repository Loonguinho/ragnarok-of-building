# Web migration: progress and handoff

Read this first when picking the work up (person or Claude session). Last updated 2026-11-27.

## Start here

- **Live:** https://ragnarok-of-building.github.io/rob/ — the same Gio program compiled to WebAssembly, on GitHub Pages.
- **Repo:** `ragnarok-of-building/rob` (moved from `Loonguinho/ragnarok-of-building`, then renamed; GitHub redirects the old URLs). On another clone: `git remote set-url origin https://github.com/ragnarok-of-building/rob.git`.
- **Desktop is discontinued** (decision 2026-11-26). Work and test only the web build. The `desktop` branch is a frozen snapshot; `cmd/app`, `tools/buildexe` and `view.IconCache` still exist and can be removed when convenient. Releases, tags and `release.yml` are gone (2026-11-27).
- **Deploys:** every push to `main` runs `.github/workflows/web.yml` → Pages. `ci.yml` runs on every push (vet desktop + wasm, tests). Both green.
- **Branches:** `main` = what's live; work goes in short branches merged by PR. `desktop` = snapshot of the discontinued program. `legacy-web` = the old HTMX server, unused. (`web`, where this migration was done, is merged and deleted.)
- **Run locally:** `make web-serve` → http://localhost:8080. `make vet`, `make test`.
- **Test in a browser from a script:** `tools/webtest/shot.mjs` (headless Chrome over DevTools; usage in its header). The Claude in Chrome extension is the other option when connected.

### What's left (in order)
1. ~~Merge `web` into `main`~~ — done with the icons PR (`web-icons` branch merged `web`).
2. **Check the live site in a normal browser** — see "Not yet tested" below (Alt shortcuts, file picker Cancel, Safari, Firefox, copy-to-clipboard, private window).
3. **Icons from the client's `data.grf`** — items and skills ✅ (2026-11-27, `cmd/iconimport`, `web/icons`). Left: item art (75×100), monsters, class icons; see "TODO: icons".
4. ~~Faster start-up~~ — done 2026-11-27 (PR #7): item descriptions apart, catalog downloaded along with the program. First visit ~13.8 s → ~10.4 s, repeat ~4.7 s → ~3.6 s; see "Start-up speed".
5. Optional: `yield()` in `Compare`, `CompareSets`, `BestSetEnchants` if they feel stuck in the browser.
6. Optional: root address `https://ragnarok-of-building.github.io/` by renaming the repo to `ragnarok-of-building.github.io` — before announcing, since saved builds live per address (localStorage is per origin). Same for a custom domain: `ragnarok-of-building.com` was free on 2026-11-26 but is not bought (buy → DNS records → Settings → Pages → Custom domain).
7. Optional: remove the desktop code.

## Start-up speed (measured 2026-11-26; improved 2026-11-27)

The page and the program print **`rob timing: <step> <ms>`** in the browser console (ms since the page started loading the program): program downloaded / compiled / started, game data downloaded / decoded, views ready, first simulator frame. Benchmark: `go run ./tools/webserve -maxage 600` (caches like Pages) and `tools/webtest/shot.mjs` with `{"nocache":true}` and `{"throttle":{"down":1500000,"latency":100}}`.

GitHub Pages from Brazil: ~1.2–1.9 MB/s, served from the Curitiba cache (`x-cache: HIT`), `Cache-Control: max-age=600`, **no compression** of `.wasm`.

Baseline (median of 3 runs, headless Chrome, this Linux PC):

| Time to the step | CPU only | First visit, 1.5 MB/s | Repeat visit (cached), 1.5 MB/s |
|---|---|---|---|
| Program downloaded | 0.04 s | 4.85 s | 0.04 s |
| Program compiled + started | 0.38 s | 5.15 s | 0.38 s |
| Game data downloaded | 0.58 s | 8.80 s | 0.63 s |
| Game data decoded | 2.86 s | 11.15 s | 3.02 s |
| Views ready | 3.49 s | 11.86 s | 3.71 s |
| **First simulator frame** | **3.62 s** | **11.99 s** | **3.85 s** |

Findings:
- **Item descriptions are 18 MB of the 31 MB catalog (58%)**, in pt+en+es, display-only (tooltips, item database, compare text). *Correction 2026-11-27:* skill descriptions are **not** display-only — `Skill.RatioAt` reads the damage % per level from them (`view/damage.go`, `view/skilltable.go`) — so they stay in the catalog. Catalog without descriptions: **1.33 MB gz / 9.6 MB raw** (vs 4.4 MB / 31 MB); descriptions per language ≈ 1 MB gz / 7 MB raw.
- The catalog download starts only after the program runs (sequential).
- Rejected: `gob` instead of JSON (decodes 4× faster but 6.6 MB gz vs 4.4 MB — nets ~zero); changing host (bytes are the limit, Pages already serves from Curitiba); shrinking the 27 MB wasm (Gio + runtime; `wasm-opt` maybe −10–15%, low value).

Done (2026-11-27, PR #7):
1. `tools/websplit` (run by `make web`) writes `catalog.json.gz` without the item descriptions (**1.73 MB gz / 11.7 MB raw**, was 4.2 / 28.9) and `desc-pt|en|es.json.gz` (~0.77 MB gz / ~5.8 MB raw each, `{id: lines}`, each language as `Lines.Get` resolved it, fallback included). `gamedata.TakeItemDescs` / `PutItemDescs` / `ReadItemDescs`.
2. `web/index.html` starts `window.robCatalog = fetch(catalog…)` before the program's download; `cmd/web` awaits that promise (falls back to its own fetch with an older page).
3. After the first simulator frame `cmd/web/descs.go` downloads the window's language, reads it item by item with `view.Yield` (the page keeps drawing), and the frame goroutine puts it in with `Views.PutItemDescs`, which re-runs `completeSetTexts`. A language change loads that language. Tooltips have no item text for the ~2 s until it arrives. `view/descs_test.go` checks every item in every language reads as with the whole catalog.

Result (Windows PC, headless Chrome, 1.5 MB/s, 3 runs each, interleaved, base = main before PR #7):

| First simulator frame | Before | After |
|---|---|---|
| First visit (no cache) | 13.6 / 13.8 / 14.7 s | 10.1 / 10.4 / 10.4 s |
| Repeat visit (cached) | 4.7 / 4.7 / 4.7 s | 3.3 / 3.6 / 4.1 s |

Where the time goes now (first visit, new): the catalog is ready ~0.2 s after the program (it downloads alongside), then decode ~2 s, views ~1.5 s, first frame +0.2 s; descriptions in place ~2 s after the first frame. Next candidates: `view.New` (~1.5 s: `completeSetTexts` + `NewItemIndex`) and the catalog decode (~2 s).

**Benchmark gotchas:** `tools/webtest/shot.mjs` used to leave its tab open, each one still running the program, so later runs got slower and slower; it now closes the tab and brings it to the front. Stopping a background `chrome --headless` from a shell may leave the browser and its tabs running: close it with DevTools `Browser.close`. Interleave base and new runs, and check `/json/list` is empty between them.

## Goal

Run **the same Gio app**, compiled to WebAssembly, as a static site on **GitHub Pages**. The old HTMX server (`legacy-web` branch) is not revived. A Cloudflare Worker was planned to proxy Divine Pride icons and paste.rs; **dropped** (see step 4a): no Worker, no Cloudflare account needed.

## Where things stand

| Step | Status | Commit |
|---|---|---|
| 1. Refactors with no web code (store, files as bytes, icons as bytes, `gamedata.Read`) | ✅ done | `7d51675` |
| 2. `cmd/web` + `web/index.html`, icons off, `make web-serve` | ✅ done | `dee9a57` |
| 3. `yield()` so searches don't freeze the page | ✅ done | `dfb4f05` |
| Code review fixes (picker, save on tab close, errors, yield) | ✅ done | `bc36982` |
| 4a. Share links (`#b=`), no paste on web | ✅ done | `d0d8cff` |
| 4b. Icons from the client `.grf` | ✅ items + skills; art, monsters, classes still TODO | `web-icons` branch |
| 5. GitHub Pages workflow, gzipped wasm, CI fix, docs | ✅ **live** (PR #1 merged as `f28ccc6`) | `10e7ddf` |
| Links to the live address, `tools/webtest` | ✅ merged | `66c246f` + next |
| Faster start-up (descriptions apart, catalog in parallel) | ✅ | PR #7 |

## Testing in a browser without the Chrome extension

Headless Chrome's `--virtual-time-budget` does not wait for CPU work, so screenshots come out on the loading screen. What worked: start `google-chrome --headless=new --remote-debugging-port=9333 --enable-unsafe-swiftshader --use-angle=swiftshader`, and drive it with `tools/webtest/shot.mjs` (Node 24, built-in `WebSocket`, no packages) over the DevTools protocol: `Page.navigate`, wait ~15 s, `Input.dispatchMouseEvent`/`dispatchKeyEvent`/`insertText`, `Runtime.evaluate`, `Page.captureScreenshot`. To measure responsiveness, inject a `requestAnimationFrame` counter that records the longest gap between frames. Window coordinates at 1440×900: Otimizador button (85,212), Calcular (601,219), FOR field (228,196). File pickers: `Page.setInterceptFileChooserDialog` + `DOM.setFileInputFiles` (the script's `intercept`/`choose` steps). This machine's headless Chrome rejects external HTTPS certificates, so to test what is deployed, download the files with `curl` and serve them locally under `/rob/`.

## How to run it

```bash
make web          # builds dist/web: app.wasm(.gz), wasm_exec.js, index.html, icon.png, catalog.json.gz, desc-*.json.gz, icons/
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
- **Size**: `app.wasm` is 27 MB, shipped as `app.wasm.gz` (6.8 MB; Pages does not compress `.wasm`) + catalog 1.7 MB + one language's descriptions 0.8 MB ≈ 9.3 MB first visit, plus the icons shown.
- **One thread**: Go in wasm has no preemption, so CPU work freezes drawing unless it calls `yield()`. Catalog decode takes ~1–2 s (a status line is drawn first). Searches yield (step 3); other one-off loops (`Compare`, `CompareSets`, `BestSetEnchants`) do not yet — add `yield()` there if they feel stuck.
- **Searches use one core** in the browser (`runtime.NumCPU()` = 1), so they are slower than the desktop; the optimizer's 30 s timeout may cut more searches short.
- **Character** (2026-11-27): the equipment window shows the character with its headgears/costume headgears from the client's sprites (`web/sprites`, `cmd/spriteimport`, `internal/sprite`); garments, weapons and hair colours not yet.
- **Icons**: item and skill icons from the client (`web/icons`, ~3 MB, one request per picture the window shows, plus `index.json` ~425 KB the first time an icon is needed). Item art, monsters and class icons answer "not found": the item's art box shows its small icon instead, monsters and classes show nothing.
- **Fonts**: no Tahoma in the browser; Go fonts are used (as on Linux).
- **localStorage** is ~5 MB per site; builds are small, but a write error is shown if full.
- Memory: catalog ~64 MB live, ~115 MB peak while loading.

## Next steps (detail)

### Icons from the client `.grf` (decided 2026-11-26; items and skills done 2026-11-27)

**Done:** `cmd/iconimport` (run after `cmd/import`, ~10 s) reads `data\texture\유저인터페이스\item\<resource>.bmp` for every catalog item and `<skill code, lowercase>.bmp` for every skill, keys out magenta, writes paletted PNGs named by a hash of their contents to `web/icons` (8,126 pictures for 17,707 items and 1,285 skills, 3.0 MB; cards and other shared pictures stored once) and `index.json` (ID → file). `make web` copies the folder; `internal/icons.Served` maps the window's paths (`view.ItemIcon`, `view.SkillIcon`) through the index; `cmd/web` fetches with `?v=` only on the index (the pictures never change under their names). Items still without an icon (~430) are not in the LATAM client.

**GRF scrambling:** about 30% of the item icons (3,039 of 9,967; 12,326 pictures under `data\texture` in all, including most classic items such as the Knife) are stored with the GRF format's "mixed DES" / "header DES" flags. `internal/grf` now unscrambles them (`des.go`: one keyless DES round per block plus the byte shuffle, as rAthena's grfio does); `TestClientScrambled` reads all of them when a client is set up. Before this, those entries failed with "encrypted entries are not supported". Note for the next `cmd/import`: any scrambled `.lub` it used to skip is now readable.

**Decided 2026-11-27: no item art for now.** The 75×100 art is ~58 MB as PNG (most has more than 256 colours; cropping saves little); the user chose icons only. Options if it comes back: art reduced to 56 px (~15–20 MB), or full art (~63 MB).

The original plan, with what is left at the end:

**Decision:** take icons from the LATAM client's `data.grf`, not Divine Pride. Same pixels (Divine Pride's images come from the client), every LATAM-only item covered, and no dependency on a site that now refuses us (403). **Blocked on:** the client is only on the Windows PC — do this from there (or with `RO_CLIENT=<copy of the client>` in `.env`).

Already in place: `internal/grf` reads `data.grf` (`grf.Open`, `Archive.Find`, `Archive.Read`); the importer keeps each item's resource name (`it.Resource`, from `identifiedResourceName` in `internal/clientimport/import.go`); `Icons.Get` takes PNG bytes, so any source plugs in.

- [x] **Find the paths.** `go run ./cmd/grftool <client>/data.grf find item` and `... find collection`: expect `data\texture\유저인터페이스\item\<resource>.bmp` (24×24 icon) and `...\collection\<resource>.bmp` (75×100 art). Check how `internal/grf` spells the Korean folder (EUC-KR/CP949 bytes in the GRF).
- [x] **Skill icons.** Find where the client keeps them (`grftool find` a known skill's resource name; likely the same `item` folder) and how a skill ID maps to its file (skillinfo lub tables the importer already reads).
- [x] **BMP → PNG.** Decode the BMP (`golang.org/x/image/bmp`), turn the magenta key (`#FF00FF`, and near-magenta) into transparency, encode PNG.
- [x] **Import step.** `cmd/import` (or a new `cmd/iconimport`) writes the PNGs. Keep them out of `data/gen/catalog.json.gz`; decide the shipped form:
  - a folder `web/icons/<kind>/<id>.png` copied by `make web` (simple; thousands of small requests, fine over HTTP/2), or
  - one archive per kind (e.g. a zip or a packed file with an index) fetched once (fewer requests, more code).
  Check the total size before choosing.
- [x] **Serve on web.** `view.NewWebIcons(base)`: GET `pageURL(base + rel)` via net/http (fetch), map today's Divine Pride–style paths (`ItemIcon`, `ItemArt`, `SkillIcon`, `JobIcon` in `view/imgcache.go`) to the new files. Wire `a.Icons = ui.NewIcons(...)` in `cmd/web/main.go`.
- [x] **Licence note.** (README already says the game's data and images are © Gravity.) README/DESENVOLVIMENTO: the icons are game data © Gravity, like the catalog the repo already ships.
- [ ] **Later: monsters.** Their pictures are sprites (`data\sprite\몬스터\*.spr` + `.act`); draw the first frame of the idle action to PNG. More work, so after items and skills; the UI works without them meanwhile.
- [ ] **Later: job icons** (`JobIcon`): find their files in the client the same way.
- [ ] Desktop is discontinued, so its `view.IconCache` (Divine Pride) can be removed once the web uses the client icons.

### Step 5 — publish (done)
- `.github/workflows/web.yml`: on push to `main` and manual dispatch → vet (wasm) + `go test ./internal/...` → `make web` → `actions/upload-pages-artifact@v4` → `actions/deploy-pages@v4`. Runs on `main` rather than tags because the `github-pages` environment only lets the default branch deploy unless told otherwise.
- `make web` now also writes `app.wasm.gz` (6.8 MB instead of 27 MB): the page fetches it and unzips with `DecompressionStream` (skipped if the host already unzipped it); browsers without it fetch `app.wasm`. So the download is small whether or not Pages compresses `.wasm`. `index.html`, `wasm_exec.js`, the wasm and the catalog URLs carry `?v=<git describe>` (`-X main.version` for the catalog).
- **CI fix:** `ci` had been failing since `7c77e9b` (the Linux window port): `go vet ./...` on Ubuntu needs Gio's cgo libraries for `cmd/app`. `ci.yml` installs them (names checked on Ubuntu 24.04) and also vets the wasm build. Only `cmd/app` needs them; `internal/...` vets without cgo, so `web.yml` skips them.
- Verified locally: gzip path and the no-`DecompressionStream` fallback both load; served from a `/ragnarok-of-building/` subfolder (as Pages does) the app loads and a `#b=` link opens its build.
- README: "▶️ Abrir no navegador" link and browser instructions; `docs/DESENVOLVIMENTO.md`: "Web version" section.

**Live since 2026-11-26.** The repo moved to the `ragnarok-of-building` GitHub organization (old `Loonguinho/...` links redirect); PR #1 merged `web` into `main` and the `web` workflow published it. Checked: every file serves (Pages does **not** compress `.wasm` — 26.9 MB raw — so `app.wasm.gz` matters), and the deployed files, run locally, open a `#b=` link and run the optimizer. (This machine's headless Chrome fails TLS to external sites — an untrusted-CA problem on the machine, not the site — so the live URL itself still needs a look in a normal browser.)

Domain: `ragnarok-of-building.com` was free on 2026-11-26 but is **not bought**; GitHub's "InvalidDNSError" came from trying it before buying. For the site at the root (`https://ragnarok-of-building.github.io/`), rename the repo to `ragnarok-of-building.github.io` — do it before announcing, since saved builds live per address.

**How it went live (for reference):**
1. Merge `web` into `main` (fast-forward: `git switch main && git merge --ff-only web && git push`).
2. GitHub → Settings → Pages → Source: **GitHub Actions**.
3. The `web` workflow runs on that push (or start it from the Actions tab). The site: https://ragnarok-of-building.github.io/rob/
4. Check in a real browser: loads, Alt shortcuts, import a replay, save builds, share link, the "Not yet tested" list above.
