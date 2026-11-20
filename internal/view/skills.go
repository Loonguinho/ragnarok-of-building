package view

type skillCell struct {
	Empty bool
	ID    int
	Name  string
	MaxLv int
	Desc  []string
}

type skillTab struct {
	JobID int
	Name  string
	Rows  [][]skillCell
}

type jobOption struct {
	ID       int
	Name     string
	Selected bool
}

// skillJS is the per-skill data the client-side tree logic needs.
type skillJS struct {
	Max   int      `json:"max"`
	Needs [][2]int `json:"needs,omitempty"`
	Group int      `json:"g"`              // index into the skill point groups
	Free  bool     `json:"free,omitempty"` // quest/soul skills cost no points
}
