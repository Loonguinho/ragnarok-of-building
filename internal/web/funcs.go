package web

import (
	"html/template"
	"regexp"
	"strings"
)

var funcs = template.FuncMap{
	"list":   func(v ...any) []any { return v },
	"roText": roText,
	"dec":    func(n int) int { return n - 1 },
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
