package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"sync"

	"gioui.org/io/key"
	"gioui.org/layout"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/view"
)

// App is the build editor: its windows are computed by view.Views and drawn
// with Gio.
type App struct {
	Th    *Theme
	Desk  *Desk
	V     *view.Views
	Icons *Icons
	Lang  string

	// Invalidate asks the window for a new frame (set by the host).
	Invalidate func()
	// Saved is the saved-builds file (builds.json next to the program).
	SavedFile string
	// StateFile keeps the window layout, language and last build.
	StateFile string
	// OpenFile asks the system for a file to open (replay import).
	OpenFile func(title, filter string) (string, error)

	mu    sync.Mutex
	b     calc.Build     // the build being edited (normalized by the last compute)
	view  view.BuildView // its windows
	ready bool
	rev   int

	chat        []chatLine
	needSave    bool
	modal       *Modal
	savedID     string // the saved build being edited ("" = not saved)
	pendingCopy string // a paste link to copy on the next frame

	// Windows.
	wMenu, wBasic, wStatus, wEquip, wSkills, wBonus, wDamage, wBuffs, wOptimize *Window
	wSaved, wShare, wHelp, wSlot, wRdmg, wItems                                 *Window

	menu   menuState
	basic  basicState
	status statusState
	equip  equipState
	slot   slotState
	damage damageState
	buffs  buffsState
	skills skillsState
	saved  savedState
	share  shareState
	replay replayState
	opt    optimizeState
	items  itemsState

	showReplayDamage bool // a replay with damage was just imported: open its window
}

type chatLine struct {
	S     string
	Error bool
}

// uiState is what data/ui.json keeps between runs.
type uiState struct {
	Lang     string                 `json:"lang"`
	Floating bool                   `json:"floating"`
	Build    string                 `json:"build"`           // share code of the last build
	Saved    string                 `json:"saved,omitempty"` // its saved build's id
	Windows  map[string]windowState `json:"windows"`
}

type windowState struct {
	Open   bool `json:"open"`
	X      int  `json:"x"`
	Y      int  `json:"y"`
	Placed bool `json:"placed"`
}

// NewApp builds the app over the views; it opens the last build (or the
// default one).
func NewApp(v *view.Views, th *Theme, stateFile string) *App {
	a := &App{Th: th, Desk: NewDesk(), V: v, Lang: "pt", StateFile: stateFile}
	a.makeWindows()
	b := v.DefaultBuild()
	if st, ok := a.loadState(); ok {
		if st.Lang != "" {
			a.Lang = st.Lang
		}
		a.Desk.Floating = st.Floating
		if d, ok := view.DecodeBuild(st.Build); ok {
			b = d
		}
		a.savedID = st.Saved
		for id, ws := range st.Windows {
			if w := a.Desk.Find(id); w != nil {
				w.Open, w.Pos, w.Placed = ws.Open, image.Pt(ws.X, ws.Y), ws.Placed
			}
		}
	}
	a.SetBuild(b)
	return a
}

func (a *App) makeWindows() {
	d := a.Desk
	a.wMenu = d.Add(&Window{ID: "win-menu", Title: "Menu", Col: 0, Open: true, NoShowAll: true})
	a.wBasic = d.Add(&Window{ID: "win-basic", Title: "Informações Básicas", Hotkey: "V", Col: 0, Open: true})
	a.wStatus = d.Add(&Window{ID: "win-status", Title: "Atributos", Hotkey: "A", Col: 0, Open: true})
	a.wEquip = d.Add(&Window{ID: "win-equip", Title: "Equipamentos", Hotkey: "Q", Col: 1, Open: true})
	a.wSlot = d.Add(&Window{ID: "win-slot", Title: "Equipamento", Col: 1, NoShowAll: true})
	a.wBonus = d.Add(&Window{ID: "win-bonus", Title: "Bônus do Equipamento", Hotkey: "B", Col: 1, Open: true})
	a.wDamage = d.Add(&Window{ID: "win-damage", Title: "Alvo / Dano", Hotkey: "D", Col: 2, Open: true})
	a.wBuffs = d.Add(&Window{ID: "win-buffs", Title: "Buffs e Consumíveis", Hotkey: "U", Col: 1})
	a.wBuffs.Body = a.buffsBody
	a.wSkills = d.Add(&Window{ID: "win-skills", Title: "Habilidades", Hotkey: "S", Col: 2})
	a.wSkills.Body = a.skillsBody
	a.wSkills.Footer = a.skillsFooter
	a.wSaved = d.Add(&Window{ID: "win-saved", Title: "Builds salvos", Hotkey: "C", Col: 0})
	a.wShare = d.Add(&Window{ID: "win-share", Title: "Compartilhar Build", Hotkey: "L", Col: 0})
	a.wHelp = d.Add(&Window{ID: "win-help", Title: "Atalhos e comandos", Hotkey: "H", Col: 0, NoShowAll: true})
	a.wSaved.Body = a.savedBody
	a.wShare.Body = a.shareBody
	a.wHelp.Body = a.helpBody
	d.Below = a.chatBox
	a.wRdmg = d.Add(&Window{ID: "win-rdmg", Title: "Resumo de dano do replay", Col: 2, NoShowAll: true})
	a.wRdmg.Body = a.replayBody
	a.wOptimize = d.Add(&Window{ID: "win-optimize", Title: "Otimizador", Hotkey: "O", Col: 2})
	a.wOptimize.Body = a.optimizeBody
	a.wItems = d.Add(&Window{ID: "win-items", Title: "Banco de Itens", Hotkey: "E", Col: 2})
	a.wItems.Body = a.itemsBody
	a.wBonus.Body = a.bonusBody
	a.wDamage.Body = a.damageBody
	a.wEquip.Body = a.equipBody
	a.wSlot.Body = a.slotBody
	a.wMenu.Body = a.menuBody
	a.wMenu.TitleExtra = a.gearButton
	a.wBasic.Body = a.basicBody
	a.wStatus.Body = a.statusBody
}

// T translates an interface text.
func (a *App) T(s string) string { return view.Tr(a.Lang, s) }

// Build returns the build being edited.
func (a *App) Build() calc.Build {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.b
}

// SetBuild replaces the build and recomputes every window in the
// background (a newer change discards an older result).
func (a *App) SetBuild(b calc.Build) {
	a.mu.Lock()
	a.rev++
	rev := a.rev
	a.b = b
	lang := a.Lang
	a.mu.Unlock()
	go func() {
		v := a.V.Build(b, lang)
		a.mu.Lock()
		if rev == a.rev {
			a.view, a.b, a.ready = v, v.Build, true
			for _, n := range v.Notes {
				a.chat = append(a.chat, chatLine{S: n})
			}
		}
		a.needSave = true
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
}

// Edit applies fn to a copy of the build and recomputes.
func (a *App) Edit(fn func(b *calc.Build)) {
	b := a.Build()
	fn(&b)
	a.SetBuild(b)
}

// Say adds a line to the chat log.
func (a *App) Say(s string, isErr bool) {
	a.mu.Lock()
	a.chat = append(a.chat, chatLine{S: s, Error: isErr})
	if len(a.chat) > 200 {
		a.chat = a.chat[len(a.chat)-200:]
	}
	a.mu.Unlock()
}

// SetLang changes the interface language.
func (a *App) SetLang(l string) {
	a.Lang = l
	a.SetBuild(a.Build())
}

// Frame draws one frame.
func (a *App) Frame(gtx C) D {
	a.mu.Lock()
	ready := a.ready
	a.mu.Unlock()
	a.Desk.Hotkeys(gtx)
	a.shortcuts(gtx)
	a.updateTitles()
	if !ready {
		backdrop(gtx, gtx.Constraints.Max)
		return layout.Center.Layout(gtx, T(a.Th, a.T("Calculando…")).C(ColWhite).W())
	}
	if a.showReplayDamage {
		a.showReplayDamage = false
		if !a.wRdmg.Open {
			a.Desk.Toggle(a.wRdmg)
		}
		a.Desk.Raise(a.wRdmg)
	}
	d := a.Desk.Layout(gtx, a.Th)
	a.layoutModal(gtx)
	a.mu.Lock()
	save := a.needSave
	a.needSave = false
	a.mu.Unlock()
	if a.Desk.Changed() || save {
		st := a.snapshot()
		go a.writeState(st)
	}
	return d
}

// shortcuts are the global keys (Alt+W floating windows...).
func (a *App) shortcuts(gtx C) {
	for {
		ev, ok := gtx.Event(key.Filter{Name: "W", Required: key.ModAlt}, key.Filter{Name: "S", Required: key.ModShortcut})
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch e.Name {
		case "W":
			a.Desk.Floating = !a.Desk.Floating
			a.Desk.changed = true
		case "S":
			a.saveBuild(false)
		}
	}
}

func (a *App) updateTitles() {
	a.wMenu.Title = a.T("Menu")
	a.wBasic.Title = a.T("Informações Básicas")
	a.wStatus.Title = a.T("Atributos")
	a.wEquip.Title = a.T("Equipamentos")
	a.wBonus.Title = a.T("Bônus do Equipamento")
	a.wDamage.Title = a.T("Alvo / Dano")
	a.wBuffs.Title = a.T("Buffs e Consumíveis")
	a.wSkills.Title = a.T("Habilidades")
	a.wSaved.Title = a.T("Builds salvos")
	if n := len(a.loadSaved()); n > 0 {
		a.wSaved.Title += fmt.Sprintf(" (%d)", n)
	}
	a.wShare.Title = a.T("Compartilhar Build")
	a.wHelp.Title = a.T("Atalhos e comandos")
	a.wRdmg.Title = a.T("Resumo de dano do replay")
	a.wOptimize.Title = a.T("Otimizador")
	a.wItems.Title = a.T("Banco de Itens")
	a.wSlot.Title = a.slotTitle()
}

// view returns the last computed windows.
func (a *App) View() view.BuildView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.view
}

func (a *App) loadState() (uiState, bool) {
	var st uiState
	b, err := os.ReadFile(a.StateFile)
	if err != nil || json.Unmarshal(b, &st) != nil {
		return st, false
	}
	return st, true
}

var saveMu sync.Mutex

// snapshot is the state to keep (taken on the UI goroutine).
func (a *App) snapshot() uiState {
	st := uiState{Lang: a.Lang, Floating: a.Desk.Floating, Build: view.EncodeBuild(a.Build()), Saved: a.savedID, Windows: map[string]windowState{}}
	for _, w := range a.Desk.Windows {
		st.Windows[w.ID] = windowState{Open: w.Open, X: w.Pos.X, Y: w.Pos.Y, Placed: w.Placed}
	}
	return st
}

func (a *App) writeState(st uiState) {
	if a.StateFile == "" {
		return
	}
	b, _ := json.MarshalIndent(st, "", " ")
	saveMu.Lock()
	defer saveMu.Unlock()
	tmp := a.StateFile + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, a.StateFile)
	}
}
