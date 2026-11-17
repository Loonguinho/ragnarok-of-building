# Ragnarok of Building

A build simulator for **Ragnarok Online LATAM**, with a UI modeled on the game client: draggable windows, Alt+A/Q/S hotkeys, login/char-select-style screens, and NPC dialogs.

Stack: Go (chi) · HTMX + Go templates · SQLite · data from the Divine Pride API · PT-BR / ES / EN.

## Running

```sh
go run ./cmd/server      # http://localhost:8080
go test ./...
```

(`make run` / `make test` also work if you have `make`.)

- `/` is the home screen (server-select style)
- `/uikit` is the RO UI kit with every base component. Drag windows by the title bar, toggle them with **Alt+A**, **Alt+Q**, **Alt+S**, **Alt+V**.

## Roadmap

- [x] Phase 0: scaffold + RO UI kit
- [ ] Phase 1: Divine Pride sync → SQLite (needs `DP_API_KEY`)
- [ ] Phase 2: build model, stats, HP/SP/ASPD
- [ ] Phase 3: in-game-style skill tree
- [ ] Phase 4: equipment, cards, enchants, refine
- [ ] Phase 5: damage engine + golden tests
- [ ] Phase 6: share codes, accounts, saved builds
- [ ] Phase 6b: import items from a replay (`.rrf`)
- [ ] Phase 7: compare + optimizer
- [ ] Phase 8: polish

The full design doc is in `docs/design.md`.
