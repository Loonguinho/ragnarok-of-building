package web

import (
	"encoding/json"
	"html/template"
	"regexp"
	"strconv"
	"strings"
)

var funcs = template.FuncMap{
	"list":   func(v ...any) []any { return v },
	"roText": roText,
	"dec":    func(n int) int { return n - 1 },
	"inc":    func(n int) int { return n + 1 },
	"upper":  strings.ToUpper,
	// gradeName: enchant grade 1..4 -> D, C, B, A.
	"gradeName": func(g int) string {
		if g < 1 || g > 4 {
			return ""
		}
		return string("DCBA"[g-1])
	},
	"json": func(v any) string {
		b, _ := json.Marshal(v)
		return string(b)
	},
	// seq lists a..b (level selectors).
	"seq": func(a, b int) []int {
		var out []int
		for i := a; i <= b; i++ {
			out = append(out, i)
		}
		return out
	},
	"pct": func(a, b int) int {
		if b <= 0 {
			return 0
		}
		return min(100, a*100/b)
	},
	"f1":   func(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) },
	"num":  fmtInt,
	"num0": func(f float64) string { return fmtInt(int64(f + 0.5)) },
}

var colorCode = regexp.MustCompile(`\^([0-9a-fA-F]{6})`)

// roText renders client text with ^RRGGBB color codes as colored spans,
// like the game's description windows. ^000000 returns to the default color.
// Input is escaped first, so only our own spans are emitted.
func roText(lines []string) template.HTML {
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteString("<br>")
		}
		esc := template.HTMLEscapeString(line)
		open := false
		last := 0
		for _, m := range colorCode.FindAllStringSubmatchIndex(esc, -1) {
			b.WriteString(esc[last:m[0]])
			if open {
				b.WriteString("</span>")
				open = false
			}
			if hex := strings.ToLower(esc[m[2]:m[3]]); hex != "000000" {
				b.WriteString(`<span style="color:#` + hex + `">`)
				open = true
			}
			last = m[1]
		}
		b.WriteString(esc[last:])
		if open {
			b.WriteString("</span>")
		}
	}
	return template.HTML(b.String())
}
