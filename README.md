# Ragnarok of Building

A build simulator for **Ragnarok Online LATAM**, with a UI modeled on the game client: draggable windows, Alt+A/Q/S hotkeys, login/char-select-style screens, and NPC dialogs.

Stack: Go (chi) · HTMX + Go templates · SQLite · data from the LATAM client itself (+ Divine Pride for item bonuses) · PT-BR / ES / EN.

## Running

You need the RO LATAM client installed (default `C:\Gravity\Ragnarok`) to generate the game data:

```powershell
go run ./cmd/import                 # reads the client -> data/gen/catalog.json.gz (~1.5 s)
go run ./cmd/server                 # http://localhost:8080
go test ./...
```

- `/` is the home screen (server-select style)
- `/skills` is the skill tree window, using real client data. Pick a job, click to add a point (Shift+click for max), right-click to remove one. Prerequisites are filled in automatically.
- `/uikit` is the RO UI kit

### Development (hot reload)

```powershell
$env:DEV = "1"; go run ./cmd/server   # templates/CSS/JS: edit and refresh the browser
```

To also reload on **Go** changes, install [Air](https://github.com/air-verse/air) once with `go install github.com/air-verse/air@latest`, then just run `air` (it already sets `DEV=1`).

### Data tools

```powershell
go run ./cmd/grftool C:/Gravity/Ragnarok/data.grf find skilltree       # list files in the GRF
go run ./cmd/lubdump C:/Gravity/Ragnarok/System/iteminfo_new.lub tbl 4001
go run ./cmd/dpcheck                                                   # test the Divine Pride key (.env)
```

## Roadmap

- [x] Phase 0: scaffold + RO UI kit
- [x] Phase 1: client data import (items, skills, trees, jobs in PT/ES/EN)
- [ ] Phase 2: build model, stats, HP/SP/ASPD
- [~] Phase 3: in-game-style skill tree (real client data; point limits pending)
- [ ] Phase 4: equipment, cards, enchants, refine
- [ ] Phase 5: damage engine + golden tests
- [ ] Phase 6: share codes, accounts, saved builds
- [ ] Phase 6b: import items from a replay (`.rrf`)
- [ ] Phase 7: compare + optimizer
- [ ] Phase 8: polish

The full design doc is in `docs/design.md`.
