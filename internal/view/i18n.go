package view

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// The interface is written in Portuguese; dict (i18n_dict.go) maps each PT
// text to its English and Spanish versions. Game data (items, skills,
// classes, monsters) comes translated from the player's client instead.
//
// Keys may contain fmt verbs (%s, %d, %+d, %%, %.0f): tr then matches texts
// produced by fmt.Sprintf with that format and translates them too.

// uiLangs are the interface languages, in selector order.
var uiLangs = []struct{ Code, Label, HTML string }{
	{"pt", "Português", "pt-BR"},
	{"en", "English", "en"},
	{"es", "Español", "es"},
}

func langIndex(l string) int {
	switch l {
	case "en":
		return 0
	case "es":
		return 1
	}
	return -1
}

type fmtKey struct {
	re      *regexp.Regexp
	en      string
	es      string
	literal int // length of the key without verbs
}

var (
	fmtOnce sync.Once
	fmtKeys []fmtKey
	verbRe  = regexp.MustCompile(`%[+]?(?:\.\d+)?[sdfv]|%%`)
)

// compileFmtKeys turns the keys with fmt verbs into regexps.
func compileFmtKeys() {
	for k, v := range dict {
		if !verbRe.MatchString(k) {
			continue
		}
		var b strings.Builder
		b.WriteString("^")
		last := 0
		for _, loc := range verbRe.FindAllStringIndex(k, -1) {
			b.WriteString(regexp.QuoteMeta(k[last:loc[0]]))
			if k[loc[0]:loc[1]] == "%%" {
				b.WriteString("%")
			} else {
				b.WriteString("(.+?)")
			}
			last = loc[1]
		}
		b.WriteString(regexp.QuoteMeta(k[last:]))
		b.WriteString("$")
		fmtKeys = append(fmtKeys, fmtKey{regexp.MustCompile(b.String()), v[0], v[1], len(verbRe.ReplaceAllString(k, ""))})
	}
	// Most specific (most literal text) first: "× tamanho … × elemento"
	// must win over "× elemento".
	sort.Slice(fmtKeys, func(i, j int) bool { return fmtKeys[i].literal > fmtKeys[j].literal })
}

// fill puts the captured values into a translated format, in order.
func fill(format string, vals []string) string {
	i := 0
	return verbRe.ReplaceAllStringFunc(format, func(v string) string {
		if v == "%%" {
			return "%"
		}
		if i < len(vals) {
			i++
			return vals[i-1]
		}
		return v
	})
}

// tr translates a Portuguese interface text; unknown texts stay as they are.
func tr(l, s string) string {
	i := langIndex(l)
	if i < 0 || s == "" {
		return s
	}
	if v, ok := dict[s]; ok && v[i] != "" {
		return v[i]
	}
	fmtOnce.Do(compileFmtKeys)
	for _, k := range fmtKeys {
		if m := k.re.FindStringSubmatch(s); m != nil {
			out := k.en
			if i == 1 {
				out = k.es
			}
			return fill(out, m[1:])
		}
	}
	return s
}
