# Ragnarok of Building: Design Doc

## Context
The repo is empty (no commits). The goal is a build simulator for **Ragnarok Online LATAM** that improves on https://simulador.latam-tools.com.br. That site is an Angular SPA that loads ~2 MB of JS plus static JSON (items-core, monsters, classes, hpsp, i18n names) and runs everything on the client.

"Better" means:
1. More accurate damage, from a tested formula engine.
2. A faster, lighter, mobile-friendly UX.
3. Build compare and optimize.
4. Save and share builds.
5. **The whole app looks like the RO client.** Every screen uses in-game-style windows, not just the skill tree.

Stack (user's choice): **Go backend, HTMX + Go templates frontend, SQLite, Divine Pride API as the data source, PT-BR/ES/EN UI.**

---

## 1. Architecture

```
Browser (HTMX + Alpine-sized vanilla JS for the skill tree, CSS)
   │  hx-get/hx-post → HTML fragments
Go HTTP server (net/http + chi)
   ├─ handlers/  (pages + fragments)
   ├─ calc/      (pure damage/stat engine: no I/O, heavily tested)
   ├─ build/     (build model, validation, encode/decode share codes)
   ├─ gamedata/  (in-memory read-only catalog loaded from SQLite at boot)
   ├─ store/     (SQLite: users, builds, synced game data)
   ├─ i18n/      (pt-BR, es, en message files)
   └─ cmd/sync   (Divine Pride importer → SQLite)
```

- **The server computes everything.** Each input change makes HTMX post the build state and swap in the stats/damage panel. The calc engine is pure Go, so it is fast (µs per build) and testable, and it can never drift from a separate client copy.
- **Game data stays in memory.** At startup it is loaded from SQLite into maps (items, monsters, skills, jobs). It is read-only, so concurrent requests need no locks.
- **Divine Pride is never called per request.** `cmd/sync` pulls the data into SQLite offline, which respects rate limits and the API key.

## 2. Repo layout
```
cmd/server/main.go        cmd/sync/main.go
internal/calc/            stats.go aspd.go hpsp.go damage_physical.go damage_magic.go modifiers.go *_test.go
internal/build/           build.go validate.go sharecode.go
internal/gamedata/        catalog.go item.go monster.go skill.go job.go bonus.go
internal/divinepride/     client.go (rate-limited, cached) mapping.go
internal/store/           sqlite.go migrations/*.sql builds.go users.go
internal/web/             router.go handlers_*.go middleware.go
web/templates/            layout.html, pages/*.html, partials/*.html
web/static/               css/app.css, js/htmx.min.js, js/skilltree.js, img/
i18n/                     pt-BR.json es.json en.json
data/overrides/           latam-*.yaml (manual corrections and LATAM availability)
testdata/golden/          in-game verified damage cases (yaml)
Makefile  go.mod  README.md
```
Assets are embedded via `embed.FS`, so the app deploys as a single binary plus the SQLite file.

## 3. Data layer (revised 2026-11-17)
We never bulk-download Divine Pride: its usage rules forbid mirroring the database or enumerating IDs in bulk.

| Data | Source |
|---|---|
| Item catalog, PT/ES/EN names and descriptions (with `^RRGGBB` color codes), slots | **The LATAM game client**: `System/iteminfo_new.lub` plus the `english/` and `spanish/` copies, about 18k items each, read by `internal/lua51` |
| Skill trees, skill info and descriptions, icons | **The client's `data.grf`** (GRF 0x300), read by a GRF reader (next step) |
| Item bonuses (`aegisScript`, `itemSets`), weapon/armor stats, allowed jobs | **Divine Pride on demand**, one ID at a time with `x-server: LATAM`, cached forever, refreshed daily via `latestupdates` |
| Monsters (stats, element resistances, LATAM spawns) | Divine Pride on demand, cached |
| Job stat bonuses, HP/SP tables, gaps | rAthena (open source) |
| LATAM corrections | `data/overrides/*.yaml`, applied last |

- `bonus.go` defines a typed bonus DSL (`Stat`, `ATKPercent`, `RaceDmg{race,pct}`, `SkillDmg{skill,pct}`, conditionals by refine/combo), parsed from `aegisScript`.
- Client art (sprites, icons) is used locally only. The public site uses Divine Pride image URLs.
- Prior art: `adsonpleal/latam-ro-calc` (MIT) reads the same client files and `.rrf` replays. It's a legal reference for the formats, with credit given.

## 4. Domain model (`internal/build`)
```go
type Build struct {
  Job        JobID; BaseLv, JobLv int
  Stats      [6]int        // STR AGI VIT INT DEX LUK
  Traits     [6]int        // POW STA WIS SPL CON CRT (4th jobs)
  Skills     map[SkillID]int
  Equip      map[Slot]EquipPiece // item, refine, grade, cards[4], enchants[4]
  Buffs      []BuffID; Consumables []ItemID
  Target     MonsterID or custom
  ActiveSkill SkillID; SkillLv int
}
```
- **Validation:** stat points come from the base-level table, and skill points come from job level per tier. Prerequisites are enforced, and item slot and job restrictions are checked.
- **Share code:** a compact binary encoding, then base64url (e.g. `/b/Xk3...`), so links work without an account. Saved builds get a short ID.

## 5. Calc engine (`internal/calc`): accuracy first
- Implements Renewal formulas: status ATK/MATK, weapon ATK with size and refine, equip ATK, the order of multipliers (race, size, element, class, monster-specific, skill), DEF/MDEF reduction (hard/soft), crit, ASPD, HP/SP from the class tables, cast time (variable/fixed), plus 4th-job P.ATK/S.MATK/RES/MRES.
- The engine is a set of pure functions, `Compute(build, catalog) Result`, that return a breakdown: every modifier is listed so users can see *why* a number comes out as it does. This alone beats most simulators.
- **Accuracy loop:** `testdata/golden/*.yaml` holds real in-game measurements (build, target, expected min/max). They run in a table-driven `go test`, and CI fails on any regression.

## 6. UI / UX: modeled on the Ragnarok Online client UI
The whole app should look and feel like the RO client, not like a web form. It is recreated with our own CSS and SVG (no extracted Gravity client sprites), and item, skill and monster icons come from Divine Pride image URLs.

**RO window system** (`web/static/css/ro-ui.css`, `js/rowindow.js`):
- Every panel is an RO-style window: a light-blue gradient title bar with a small title, a close/minimize button at the top right, a beveled light-gray body, thin dark borders, compact 11–12px text, and small RO-style buttons and scrollbars.
- On desktop, windows are **draggable and toggleable** like in the game, with their positions saved in localStorage. The hotkeys match the client: **Alt+A** Status, **Alt+Q** Equipment, **Alt+S** Skills, **Alt+E** Inventory/item search, **Alt+V** Basic Info, plus a custom window for Target/Damage.
- A bottom-left **Basic Info** window shows the character name, job, Base/Job level bars, HP/SP bars, and buttons that open the other windows, like the client's main menu.
- The background is a dimmed, blurred field-map-style scene, so the windows "float over the game".

**Windows:**
- **Status (Alt+A):** the classic six-stat grid with `+` buttons and a "required points" column. Remaining status points appear at the bottom, and derived stats (ATK, MATK, HIT, FLEE, CRIT, ASPD, DEF, MDEF) sit on the right, just as in the client. A second page holds the 4th-job traits (POW/STA/WIS/SPL/CON/CRT).
- **Equipment (Alt+Q):** the character sprite in the middle, with slots on the left and right in the client's arrangement (head top/mid/low, armor, weapon, shield, garment, shoes, accessories) and a "Costume / Shadow" tab. Clicking a slot opens the item window, where you can add cards, enchants, refine (+0 to +20) and grade.
- **Skill tree (Alt+S):** an in-game grid with job-tier tabs, skill icons, and level shown as `x/max`. Left-click adds a point (shift adds max), right-click removes one. Prerequisites auto-fill when you add a skill and cascade-remove when you remove one, and points left per tier are shown at the bottom. Hovering shows the localized description in an RO-style tooltip. It is built in `skilltree.js` (~200 lines of vanilla JS over server-rendered markup), which posts changes with HTMX.
- **Item info:** an RO-style item description window (icon, name with refine prefix, slot count `[4]`, description text, weight, required level).
- **Damage / Target:** a monster picker shown as RO monster info, and a results breakdown styled like the in-game chat or info window.
- **Mobile:** windows turn into a single stacked column of full-width RO-styled panels with a bottom tab bar (Status / Equip / Skills / Damage). Dragging is disabled.

**Every other screen is also in-game style.** No page uses a normal web layout:
- **Landing / home:** looks like the RO login and server-select screen. "Novo Build", "Meus Builds" and "Carregar código" are presented as the client's server list and buttons.
- **Login / register:** the RO login window (ID / password fields and the classic OK / Cancel buttons).
- **My builds:** the RO **character select** screen. Each saved build is a character slot showing its sprite, job, level and stats preview, with "Criar" / "Apagar" / "Entrar" buttons.
- **New build / job choice:** a character-creation-style window with a job list in a tree (Novice → 1st → 2nd → 3rd → 4th).
- **Compare:** up to 4 Status windows side by side, with diffs highlighted in green/red like stat changes in the client.
- **Optimizer:** looks like an NPC dialog or shop window, listing candidate items with their damage gain as the "price" column.
- **Shared build page (`/b/{code}`):** the same window desktop, read-only, with a "Copiar build" button.
- **Language / settings:** the RO Options window (Alt+O).
- **Confirmations and errors:** RO-style NPC dialog boxes with "Próximo" / "Fechar" / "OK" buttons, and RO-style system messages in a chat window at the bottom-left for toasts like "Build salvo!".
- **Typography:** a small pixel-like sans font similar to the client (a free font such as "Press Start"-lite or "Pixelify Sans", or plain Tahoma/Verdana 11px), with no modern web chrome.
- **Item picker (inside the inventory/item window):** a search box with debounced `hx-get` fuzzy search across all three languages, filtered by slot and job. Cards and enchants are filtered by the slot they fit.
- **Compare:** pin up to 4 builds side by side, with diffs highlighted.
- **Optimizer (v2):** for one slot, rank every valid item or card by damage gain against the chosen target. This is a brute force over candidates, which stays cheap because the engine is fast.
- **i18n:** the language comes from a cookie or `Accept-Language`, and both templates and game names are localized.
- **Phase 0 includes a static "RO UI kit" page** (window, button, tab, slot, tooltip, bar) so the look is settled before features are built on it.

## 6b. Replay import (`.rrf`): import the player's real items
**Goal:** the user uploads a replay recorded by the RO LATAM client (the `Replay/*.rrf` folder). We extract every item the character had (equipped items, inventory, and storage or cart if opened during the recording) with refine, cards, enchants/random options and grade, then load them into the build.

**How:** a replay is a recording of the client↔server packet stream, saved in a container format. When a character logs into a map, the server sends the full item lists, so the data is in the first seconds of any replay:
- `ZC_EQUIPMENT_ITEMLIST*` (equipped and equippable items: itemID, slot location, refine, cards[4], random options[5], grade, bind flags)
- `ZC_NORMAL_ITEMLIST*` (stackable inventory)
- `ZC_STORE_*ITEMLIST*` and `ZC_CART_*ITEMLIST*` (storage and cart, if opened)
- Status packets (`ZC_STATUS`, `ZC_PAR_CHANGE`, `ZC_LONGPAR_CHANGE`) give base and job level, job and allocated stats. This lets us also prefill **stats and job**, plus skills from `ZC_SKILLINFO_LIST`.

**Package `internal/replay`:**
- `container.go`: parses the `.rrf` header and chunk/frame layout. Some chunks are zlib-compressed and/or obfuscated. This is the reverse-engineering part: start from public community notes and existing open-source RRF parsers, then confirm against real LATAM samples.
- `packets.go`: a packet decoder table keyed by packet ID, with lengths for the LATAM `packetver` (the client date is taken from the replay header). Only the packets listed above need full decoding, and everything else is skipped by length.
- `extract.go`: turns the decoded packets into `[]OwnedItem{ItemID, Refine, Cards[4], Options[], Grade, Location, Source(equip/inv/storage/cart)}` plus `CharSnapshot{Job, BaseLv, JobLv, Stats, Skills}`.
- Table-driven tests run on sample replays in `testdata/replays/` (anonymized).

**UX (in-game style):**
- On the Inventory window (Alt+E), an "Importar Replay" button opens an RO dialog with a file upload or drag-and-drop.
- The imported items fill the **Inventory window** with Equip/Item/Etc tabs, exactly like the client. Items flagged as equipped are auto-placed on the Equipment window. Drag from the inventory to a slot (or double-click) to equip, as in-game.
- An NPC-style dialog asks: "Importar também atributos, nível e habilidades? [Sim] [Não]".
- When logged in, the imported inventory is saved as the user's **"Meu Inventário"**. The optimizer can then be limited to "items I own" ("what's my best setup with what I have?"), which is a big differentiator.

**Privacy and safety:** a replay also contains chat, other players and positions. The file is parsed in memory and never stored, and only the extracted item list (and optional char snapshot) is kept. Uploads are capped (e.g. 50 MB), parsing has a time limit, and malformed input is rejected cleanly, because the parser treats the file as untrusted (bounds-checked reads and fuzz tests via `go test -fuzz`).

**Risks:**
- The container format may be undocumented or change between client updates. Mitigation: handle the version from the header, keep the decoder table data-driven, and fall back to a "paste item list" import.
- Packet IDs and lengths for LATAM must be confirmed with real samples. **We need 2–3 sample `.rrf` files from the user** (log in, open the inventory and storage, then stop recording).

## 7. Accounts & persistence
- SQLite tables: `users`, `sessions`, `builds(id, user_id, name, job, code, created, updated, public)`.
- **Auth:** start with optional magic-link or simple email/password (bcrypt) and cookie sessions. Builds also work anonymously through share codes, and a "My builds" list needs login.

## 8. Phased roadmap
| Phase | Deliverable |
|---|---|
| 0 | Scaffold: go mod, chi, templates, embed, Makefile, CI (go test, vet, golangci-lint), README |
| 1 | Divine Pride client and sync for items, monsters and skills in 3 locales into SQLite, plus the in-memory catalog |
| 2 | Build model, stat allocation, HP/SP/ASPD calc, stats page working end to end with HTMX |
| 3 | Skill tree UI (in-game style), prerequisites, point limits |
| 4 | Equipment doll, item/card/enchant/refine picker, the bonus DSL, and the first batch of item-bonus overrides |
| 5 | Damage engine (physical, magic, crit, skills) with breakdown, target picker, golden tests |
| 6 | Share codes, accounts, saved builds |
| 6b | Replay import: research the `.rrf` container with samples, decode the packets, fill the inventory window, equip by drag-and-drop, save "Meu Inventário" (research can start in parallel from Phase 1) |
| 7 | Compare view, then the slot optimizer (including "only items I own") |
| 8 | Polish: mobile, performance, more bonus coverage, buffs/consumables |

## 9. Risks
- **Divine Pride API terms and rate limits.** Keep the key in env `DP_API_KEY`, sync offline, and cache the raw responses.
- **Structured bonuses are missing** from Divine Pride, so the override YAML is the main ongoing data work. Start with the meta items.
- **The LATAM server code and data parity** with Divine Pride need confirming in Phase 1.

## Verification
- `go test ./...`, with the calc golden tests as the main gate.
- `go run ./cmd/sync -limit 200` fills SQLite, and spot-checking item and monster names in all three locales confirms the import.
- `go run ./cmd/server`, then open localhost:8080. Make a build (e.g. a 4th-job class with skills and gear), check that the stats and damage update live, copy the share link into a new browser, and confirm the build reloads unchanged.
- Upload a sample `.rrf` and confirm that every equipped item (refine, cards, enchants) matches what the client showed and that the inventory window lists the items. Fuzz the parser with `go test -fuzz=FuzzReplay ./internal/replay`.
- Compare a handful of builds against in-game numbers and against the reference simulator.
