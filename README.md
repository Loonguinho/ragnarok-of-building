# Ragnarok of Building

A build simulator for **Ragnarok Online LATAM**, with a UI modeled on the game client: draggable windows, Alt+A/Q/S hotkeys, login/char-select-style screens, and NPC dialogs.

Stack: Go (chi) · HTMX + Go templates · SQLite · data from the LATAM client itself (+ Divine Pride for item bonuses) · PT-BR / ES / EN.

## Running

You need the RO LATAM client installed (default `C:\Gravity\Ragnarok`). Then, once:

```powershell
go run ./cmd/fetchrathena   # downloads rAthena's tables (GPL-3.0) to data/rathena (not committed)
go run ./cmd/import         # client + rAthena -> data/gen/catalog.json.gz (~2 s)
```

and every time:

```powershell
go run ./cmd/server         # http://localhost:8080
go test ./...
```

Re-run `go run ./cmd/import` after each game patch.

**Pages**
- `/` is the home screen, styled as the RO server-select screen
- `/build` is the **build editor**, with Basic Info (Alt+V), Status (Alt+A), Equipment (Alt+Q), Skills (Alt+S), Bonuses (Alt+B) and Target/Damage (Alt+D). The build lives in the URL, so copy the link to share it.
- `/items` is the item database (PT/ES/EN search, filter by type)
- `/skills` is a standalone skill tree
- `/uikit` is the RO UI kit

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
- [~] Phase 4: equipment, cards, refine, class restrictions (enchants/combos pending)
- [x] Phase 5: physical and magic damage vs monsters with LATAM formulas for every class (275 of 308 skill formulas; see the generator report)
- [~] Phase 6: share links + last build restored (accounts/saved builds pending)
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
