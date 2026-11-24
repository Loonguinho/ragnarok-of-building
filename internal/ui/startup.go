package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

// StartupStatus is what the startup screen shows while the game data gets
// ready.
type StartupStatus struct {
	Lines   []string
	Ask     bool   // waiting for the client folder
	AskNote string // why
	Error   string // what stopped it
}

// Startup is the window shown before the simulator: the progress lines, and
// the client folder question when the game is not found.
type Startup struct {
	Th        *Theme
	Status    func() StartupStatus
	Pick      func() (string, error) // folder picker
	UseClient func(dir string) error
	// Invalidate asks for a frame (the folder dialog answers from its own
	// goroutine).
	Invalidate func()

	picked  chan string // the open folder dialog's answer
	pick    widget.Clickable
	ok      widget.Clickable
	dir     Input
	list    widget.List
	message string
}

// Frame draws the startup window centered on the backdrop.
func (s *Startup) Frame(gtx C) D {
	th := s.Th
	st := s.Status()
	// The folder dialog runs on its own: waiting for it inside a frame
	// hangs the window (see App.openFile).
	if s.pick.Clicked(gtx) && s.Pick != nil && s.picked == nil {
		ch := make(chan string, 1)
		s.picked = ch
		go func() {
			dir, err := s.Pick()
			if err != nil {
				dir = ""
			}
			ch <- dir
			if s.Invalidate != nil {
				s.Invalidate()
			}
		}()
	}
	if s.picked != nil {
		select {
		case dir := <-s.picked:
			s.picked = nil
			if dir != "" {
				s.dir.Ed.SetText(dir)
				s.use(dir)
			}
		default:
		}
	}
	if s.ok.Clicked(gtx) {
		s.use(s.dir.Text())
	}
	size := gtx.Constraints.Max
	backdrop(gtx, size)
	w := min(gtx.Dp(460), size.X-gtx.Dp(32))
	win := &Window{Title: "Ragnarok of Building", Width: 460}
	win.Body = func(gtx C) D {
		kids := []layout.Widget{
			func(gtx C) D {
				s.list.Axis = layout.Vertical
				s.list.ScrollToEnd = true
				gtx.Constraints.Max.Y = gtx.Dp(220)
				gtx.Constraints.Min.Y = gtx.Dp(120)
				return Box{Bg: ColPanel, Line: ColListBorder, Pad: Inset(4, 6, 4, 6)}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return s.list.Layout(gtx, len(st.Lines), func(gtx C, i int) D {
						return T(th, st.Lines[i]).Wrapping().Layout(gtx)
					})
				})
			},
		}
		if st.Error != "" {
			kids = append(kids, T(th, "! "+st.Error).C(ColDown).Wrapping().W())
		}
		if st.Ask {
			kids = append(kids,
				T(th, st.AskNote).Wrapping().W(),
				func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							d, _, sub := s.dir.Layout(gtx, th, 0)
							if sub {
								s.use(s.dir.Text())
							}
							return d
						}),
						layout.Rigid(Gap(4, 0)),
						layout.Rigid(func(gtx C) D { return Button{Label: "Procurar…"}.Layout(gtx, th, &s.pick) }),
						layout.Rigid(Gap(4, 0)),
						layout.Rigid(func(gtx C) D { return Button{Label: "OK", Bold: true}.Layout(gtx, th, &s.ok) }),
					)
				},
			)
			if s.message != "" {
				kids = append(kids, T(th, s.message).C(ColDown).Wrapping().W())
			}
		}
		if len(st.Lines) == 0 && st.Error == "" && !st.Ask {
			kids = append(kids, T(th, "Abrindo…").W())
		}
		return Col(6, kids...)(gtx)
	}
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(w, size.Y)}
	d := NewDesk()
	wd := win.layout(g, th, d, false)
	call := m.Stop()
	off := op.Offset(image.Pt((size.X-wd.Size.X)/2, max(gtx.Dp(16), (size.Y-wd.Size.Y)/3))).Push(gtx.Ops)
	call.Add(gtx.Ops)
	off.Pop()
	return D{Size: size}
}

func (s *Startup) use(dir string) {
	if s.UseClient == nil {
		return
	}
	if err := s.UseClient(dir); err != nil {
		s.message = err.Error()
	} else {
		s.message = ""
	}
}
