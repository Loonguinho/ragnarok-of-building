# Development

Stack: Go · native window drawn with [Gio](https://gioui.org) (no browser engine) · data from the LATAM client itself (+ Divine Pride for item icons) · PT-BR / ES / EN.

The old web version (HTMX pages, the hosted server and its Docker image) was retired in November 2026; its last state is the `legacy-web` branch.

## Game data

The game data is read from, in order: a `data.grf` and `System` folder copied next to the executable; the `catalog.json.gz` there; the installed client; the `catalog.json.gz` attached to the program's release, downloaded once. Without a catalog it finds the RO LATAM client (or asks for its folder), downloads rAthena's tables and latam-ro-calc's monster list once, and builds the catalog from that client; after each game patch it rebuilds it by itself.

## Publishing a version

To publish a version, push a tag: `git tag v0.1.0` then `git push origin v0.1.0`. The `release` workflow tests, builds `cmd/app` for Windows and attaches the zip to a new release. GitHub has no game client, so the catalog the zip ships is the committed `data/gen/catalog.json.gz`: after a game patch, run `go run ./cmd/import` and commit it before tagging. To build it locally: `go run ./tools/buildexe` (or `go build -ldflags -H=windowsgui -o RagnarokOfBuilding.exe ./cmd/app`).

## Running from source

You need the RO LATAM client installed (default `C:\Gravity\Ragnarok`; set `RO_CLIENT=<folder>` in `.env` to read a copy of it instead, so the installed game is never touched). Then, once:

```powershell
go run ./cmd/fetchrathena   # downloads rAthena's tables (GPL-3.0) to data/rathena (not committed)
go run ./cmd/import         # client + rAthena -> data/gen/catalog.json.gz (~3 s)
```

and every time:

```powershell
go run ./cmd/app            # the program's window
go test ./...
```

Re-run `go run ./cmd/import` after each game patch.

**Linux (and macOS).** The same window runs there. The committed `data/gen/catalog.json.gz` is used, so no game client is needed: `go run ./cmd/app` from the repository. Gio needs cgo and the desktop libraries (Debian/Ubuntu: `gcc pkg-config libwayland-dev libx11-dev libx11-xcb-dev libxkbcommon-x11-dev libgles2-mesa-dev libegl1-mesa-dev libffi-dev libxcursor-dev libvulkan-dev`). File and folder pickers use `zenity` or `kdialog` (osascript on macOS). Under `go run` the program works in the current folder instead of next to the temporary executable.

**Languages.** The settings menu (the gear in the Menu window) has a PT / EN / ES picker, kept in `data/ui.json`. Game data (items, skills, classes, monsters) comes in each language from the client; the interface texts are in `internal/view/i18n_dict.go`, keyed by the Portuguese text.

**Code.** `internal/calc` is the formula engine; `internal/view` computes what each window shows (build, equipment, buffs, damage, skill tree, optimizer, replay recap) as plain structs; `internal/edit` holds the editing rules (linked headgear slots, exclusive buffs, skill tree points); `internal/ui` draws the windows with Gio; `cmd/app` is the program (startup, import, window).

**Windows:** Menu (gear: floating windows Alt+W, reset layout, open all, language), Basic Info (Alt+V, class list Alt+J), Status (Alt+A), Equipment (Alt+Q), Skills (Alt+S), Bonuses (Alt+B), Target/Damage (Alt+D), Buffs (Alt+U), Optimizer (Alt+O), Item database (Alt+E), Saved builds (Alt+C), Share (Alt+L), Shortcuts (Alt+H), and the replay damage recap after importing a `.rrf`. Saved builds live in `builds.json` next to the program.


### Regenerating the LATAM formulas

```powershell
git clone --depth 1 https://github.com/adsonpleal/latam-ro-calc tmp/latam-ro-calc
python -I tools/latamgen/latamgen.py      # writes internal/calc/latam_gen.go + tools/latamgen/REPORT.md
python -I tools/latamgen/buffgen.py       # each class's own buffs -> internal/calc/latam_buffs_gen.go + tools/latamgen/BUFFS.md
python -I tools/latamgen/tablegen.py      # LATAM class tables (HP/SP, job bonus, ASPD) -> internal/clientimport/latam_tables_gen.go
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
- [x] Phase 4: equipment, cards, enchants, refine, enchant grades (D–A: refine bonus and the items' own grade bonuses), random options (the client's option texts + rAthena scripts, also read from replays), class restrictions, item combos (rAthena + LATAM-only items read from the client descriptions)
- [x] Phase 5: physical and magic damage vs monsters with LATAM formulas for every class, checked against bROWiki; buffs and consumables
- [x] Phase 6: share links, last build restored, saved builds (Builds window)
- [x] Phase 6b: import a character from a replay (`.rrf`)
- [~] Phase 7: item comparison in the equipment picker (damage change per result, stat changes in the tooltip) and card/enchant optimizer done; stat and gear optimizer pending
- [ ] Phase 8: polish

The full design doc is in `design.md`.

## Credits

- **[latam-ro-calc](https://github.com/adsonpleal/latam-ro-calc)** by Adson Leal (MIT): the LATAM monster list (`src/assets/demo/data/monster.json`, downloaded at import at a pinned commit into `data/latam-ro-calc/`, not committed: stats checked against LATAM, including monsters rAthena has no stats for, such as Betelgeuse) and the damage reductions it measured; the LATAM skill formulas (all classes, including the 2022 rebalance LATAM uses since the 28/10/2026 client) are translated to Go by `tools/latamgen` into `internal/calc/latam_gen.go`; the physical/magic pipeline adjustments in `damage.go`/`magic.go` follow its measurements against real LATAM recordings. Coverage: `tools/latamgen/REPORT.md`.
- **[bROWiki](https://browiki.org)**: skill pages are read through its MediaWiki API (1 request/second, cached in `data/browiki/`, not committed) to check every formula against the per-level tables of the current LATAM server. Where a table was rewritten for the 28/10/2026 client and disagrees, or a skill has no latam-ro-calc formula, `internal/calc/latam_wiki.go` follows the wiki. Report: `tools/browiki/COMPARE.md`.
- **[rrfparser](https://github.com/adsonpleal/rrfparser)** by Adson Leal (MIT): format reference for the `.rrf` replay reader in `internal/replay` (independent Go implementation).
- **[rAthena](https://github.com/rathena/rathena)** (GPL-3.0): job tables, items, monsters and renewal formulas. Its data files are downloaded locally by `cmd/fetchrathena` and not redistributed.
- **[Divine Pride](https://www.divine-pride.net)** (optional): stats of the remaining monsters that neither rAthena nor latam-ro-calc has, fetched one ID at a time when chosen as a target, only if you set your own `DP_API_KEY` in `.env`; cached in `data/cache/dp/` (not committed).
- Game names, descriptions and skill trees are read from the user's own RO LATAM client. Ragnarok Online is © Gravity Co., Ltd.
