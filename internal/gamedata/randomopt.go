package gamedata

import (
	"strconv"
	"strings"
)

// RandomOpt is a random option ("bônus aleatório"): the client's text for it
// (a format with %d for the value) and the rAthena script that applies it.
type RandomOpt struct {
	ID     int    `json:"id"`
	Var    string `json:"var,omitempty"` // VAR_MAXHPAMOUNT...
	Name   Text   `json:"name,omitempty"`
	Script string `json:"script"`
}

// OptRange is a random option a position can roll, with its value range.
type OptRange struct {
	ID  int `json:"id"`
	Min int `json:"min,omitempty"`
	Max int `json:"max,omitempty"`
}

// ItemOpts says which random options an item can get. Positions lists, in
// order, the options each position can roll (from rAthena's groups). When
// only a count is known (latam-ro-calc), Any is that count and any option
// goes.
type ItemOpts struct {
	Positions [][]OptRange `json:"pos,omitempty"`
	Any       int          `json:"any,omitempty"`
}

// Slots is how many random options the item takes.
func (o *ItemOpts) Slots() int {
	if o == nil {
		return 0
	}
	return max(len(o.Positions), o.Any)
}

// At lists the options position i can roll; nil means any option.
func (o *ItemOpts) At(i int) []OptRange {
	if o == nil || i >= len(o.Positions) {
		return nil
	}
	return o.Positions[i]
}

// Label is the option's text in a language with the value filled in
// ("MaxHP +100"); with v = 0 the value shows as N ("MaxHP +N").
func (o *RandomOpt) Label(lang string, v int) string {
	name := o.Name.Get(lang)
	if name == "" {
		name = o.Var
	}
	val := "N"
	if v != 0 {
		val = strconv.Itoa(v)
	}
	name = strings.ReplaceAll(name, "%%", "%")
	return strings.TrimSpace(strings.Replace(name, "%d", val, 1))
}
