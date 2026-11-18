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

// Label is the option's text in a language with the value filled in
// ("MaxHP +100"); with v = 0 the placeholder is dropped ("MaxHP +").
func (o *RandomOpt) Label(lang string, v int) string {
	name := o.Name.Get(lang)
	if name == "" {
		name = o.Var
	}
	val := ""
	if v != 0 {
		val = strconv.Itoa(v)
	}
	name = strings.ReplaceAll(name, "%%", "%")
	return strings.TrimSpace(strings.Replace(name, "%d", val, 1))
}
