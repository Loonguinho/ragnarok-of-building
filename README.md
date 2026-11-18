# Ragnarok of Building

A build simulator for **Ragnarok Online LATAM**, with a UI modeled on the game client: draggable windows, Alt hotkeys, a character-select home screen and NPC dialogs.

Stack: Go (chi) · HTMX + Go templates · data from the LATAM client itself (+ Divine Pride for item bonuses) · PT-BR / ES / EN.

## Online

The site is one Go binary plus the game catalog, packaged with Docker (about 170 MB of RAM, so free 512 MB plans fit). The catalog is generated from your own client and copied into the image: it is never committed to git.

```powershell
go run ./cmd/fetchrathena; go run ./cmd/import   # once, and after each game patch
docker build -t ragnarok-of-building .
docker run -p 8080:8080 ragnarok-of-building    # http://localhost:8080
```

Hosts pass the port in `PORT`. Saved builds live in each visitor's browser (localStorage) and builds are shared by link, so the server keeps no state and any free container host works. Per-IP rate limits protect the calculation and replay upload endpoints, and the pages send a Content-Security-Policy.

## Running from source

You need the RO LATAM client installed (default `C:\Gravity\Ragnarok`). Then, once:

```powershell
go run ./cmd/fetchrathena   # downloads rAthena's tables (GPL-3.0) to data/rathena (not committed)
go run ./cmd/import         # client + rAthena -> data/gen/catalog.json.gz (~3 s)
```

and every time:

```powershell
go run ./cmd/server         # http://localhost:8080
go test ./...
```

Re-run `go run ./cmd/import` after each game patch.

**Languages.** The home screen has a Português / English / Español picker (remembered in a cookie; first visits follow the browser's language). Game data (items, skills, classes, monsters) comes in each language from the client; the interface texts are in `internal/web/i18n_dict.go`, keyed by the Portuguese text. Mark new texts with `{{t "…"}}` in templates or `T("…")` in scripts: `go test ./internal/web` fails while one lacks its EN/ES version.

**Sound.** Buttons play the classic RO menu click. `go run ./cmd/import` copies it from your own client (`data\wav\버튼소리.wav` -> `data/gen/sounds/click.wav`, not committed); without it the site stays silent. The home screen has a toggle to mute it.

**Pages**
- `/` is the home screen: builds saved in this browser as character slots, plus the menu
- `/build` is the **build editor**, with Basic Info (Alt+V), Status (Alt+A), Equipment (Alt+Q), Skills (Alt+S), Bonuses (Alt+B), Target/Damage (Alt+D) and Buffs (Alt+U). Ctrl+S saves the build in the browser; **Copiar link** shares it.
- `/new` starts a build from a replay (`.rrf`) or a blank class
- `/items` is the item database (PT/ES/EN search, filter by type)
- `/skills` is a standalone skill tree
- `/uikit` is the RO UI kit (development)

### Development (hot reload)

```powershell
$env:DEV = "1"; go run ./cmd/server   # templates/CSS/JS: edit and refresh the browser
```

To also reload on **Go** changes, install [Air](https://github.com/air-verse/air) once with `go install github.com/air-verse/air@latest`, then just run `air` (it already sets `DEV=1`).

### Regenerating the LATAM formulas

```powershell
git clone --depth 1 https://github.com/adsonpleal/latam-ro-calc tmp/latam-ro-calc
python -I tools/latamgen/latamgen.py      # writes internal/calc/latam_gen.go + tools/latamgen/REPORT.md
go test ./internal/calc
```

### Checking formulas against bROWiki

```powershell
python -I tools/browiki/fetch.py                                   # fetches missing pages only
$env:DUMP_RATIOS="tmp/ours.json"; go test -run TestDumpRatios ./internal/calc
python -I tools/browiki/compare.py tmp/ours.json                   # writes tools/browiki/COMPARE.md
```

### Data tools

```powershell
go run ./cmd/grftool C:/Gravity/Ragnarok/data.grf find skilltree       # list files in the GRF
go run ./cmd/lubdump C:/Gravity/Ragnarok/System/iteminfo_new.lub tbl 4001
go run ./cmd/dpcheck                                                   # test the Divine Pride key (.env)
go run ./cmd/scriptcov                                                 # how much of the item scripts the calculator applies
```

## Roadmap

- [x] Phase 0: scaffold + RO UI kit
- [x] Phase 1: client data import (items, skills, trees, jobs in PT/ES/EN)
- [x] Phase 2: build model, stats, HP/SP/ASPD, traits (rAthena renewal formulas)
- [x] Phase 3: in-game-style skill tree with skill point limits per class step
- [x] Phase 4: equipment, cards, enchants, refine, class restrictions, item combos (rAthena + LATAM-only items read from the client descriptions)
- [x] Phase 5: physical and magic damage vs monsters with LATAM formulas for every class, checked against bROWiki; buffs and consumables
- [x] Phase 6: share links, last build restored, saved builds (browser storage, character select home), Docker image for hosting
- [x] Phase 6b: import a character from a replay (`.rrf`)
- [ ] Phase 7: compare + optimizer
- [ ] Phase 8: polish

The full design doc is in `docs/design.md`.

## Credits

- **[latam-ro-calc](https://github.com/adsonpleal/latam-ro-calc)** by Adson Leal (MIT): the LATAM skill formulas (all classes, including the 2022 rebalance LATAM uses since the 28/10/2026 client) are translated to Go by `tools/latamgen` into `internal/calc/latam_gen.go`; the physical/magic pipeline adjustments in `damage.go`/`magic.go` follow its measurements against real LATAM recordings. Coverage: `tools/latamgen/REPORT.md`.
- **[bROWiki](https://browiki.org)**: skill pages are read through its MediaWiki API (1 request/second, cached in `data/browiki/`, not committed) to check every formula against the per-level tables of the current LATAM server. Where a table was rewritten for the 28/10/2026 client and disagrees, or a skill has no latam-ro-calc formula, `internal/calc/latam_wiki.go` follows the wiki. Report: `tools/browiki/COMPARE.md`.
- **[rrfparser](https://github.com/adsonpleal/rrfparser)** by Adson Leal (MIT): format reference for the `.rrf` replay reader in `internal/replay` (independent Go implementation).
- **[rAthena](https://github.com/rathena/rathena)** (GPL-3.0): job tables, items, monsters and renewal formulas. Its data files are downloaded locally by `cmd/fetchrathena` and not redistributed.
- Game names, descriptions and skill trees are read from the user's own RO LATAM client. Ragnarok Online is © Gravity Co., Ltd.
