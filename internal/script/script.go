// Package script evaluates the subset of rAthena item scripts that affects
// a character's status window: `bonus bX,expr;` with integer expressions,
// local variables (.@x), getrefine(), readparam(), BaseLevel/JobLevel and
// if/else blocks. Everything else (bonus2/3, autobonus, skill, unknown
// functions) is skipped and counted in Result.Skipped, never guessed.
package script

import (
	"strconv"
	"strings"
	"unicode"
)

// Context is what a script can read.
type Context struct {
	Refine   int
	Grade    int // enchant grade: 0 none, 1 D ... 4 A
	OptValue int // getrandomoptinfo(ROA_VALUE): a random option's rolled value
	BaseLv   int
	JobLv    int
	Param    map[string]int // readparam(bStr) etc.: base stats by rAthena name
	Class    int
	Weapon   string // weapon type of the right hand (Dagger, 1hSword, ...)
	EquipIDs map[int]bool
}

// Result accumulates bonuses: name (bStr, bMaxHPrate, ...) -> value.
type Result struct {
	Bonus map[string]int
	// Extra holds bonus2..bonus5 effects (damage modifiers and the like) with
	// their arguments, e.g. "bAddRace RC_DemiHuman" -> 10. They do not change
	// the status window; the damage engine and the UI read them.
	Extra   map[string]int
	Skipped int // statements the evaluator did not understand
}

// Eval runs a script and returns its bonuses.
func Eval(src string, ctx Context) Result {
	p := &parser{toks: lex(src), ctx: ctx, vars: map[string]int{}, res: Result{Bonus: map[string]int{}, Extra: map[string]int{}}}
	for !p.eof() {
		before := p.pos
		p.statement(true)
		if p.pos == before { // never loop forever on bad input
			p.pos++
		}
	}
	return p.res
}

// ---- lexer ----

type tok struct {
	kind byte // 'n' number, 'i' identifier, 's' string, 'p' punctuation
	s    string
	n    int
}

func lex(src string) []tok {
	var out []tok
	r := []rune(src)
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case c == '/' && i+1 < len(r) && r[i+1] == '/':
			for i < len(r) && r[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(r) && r[i+1] == '*':
			i += 2
			for i+1 < len(r) && !(r[i] == '*' && r[i+1] == '/') {
				i++
			}
			i += 2
		case unicode.IsDigit(c):
			j := i
			for j < len(r) && (unicode.IsDigit(r[j]) || r[j] == 'x' || (r[j] >= 'a' && r[j] <= 'f') || (r[j] >= 'A' && r[j] <= 'F')) {
				j++
			}
			n, _ := strconv.ParseInt(string(r[i:j]), 0, 64)
			out = append(out, tok{kind: 'n', s: string(r[i:j]), n: int(n)})
			i = j
		case unicode.IsLetter(c) || c == '_' || c == '.' || c == '@' || c == '$' || c == '\'':
			j := i
			for j < len(r) && (unicode.IsLetter(r[j]) || unicode.IsDigit(r[j]) || r[j] == '_' || r[j] == '.' || r[j] == '@' || r[j] == '$' || r[j] == '\'') {
				j++
			}
			out = append(out, tok{kind: 'i', s: string(r[i:j])})
			i = j
		case c == '"':
			j := i + 1
			for j < len(r) && r[j] != '"' {
				if r[j] == '\\' {
					j++
				}
				j++
			}
			end := min(j, len(r))
			out = append(out, tok{kind: 's', s: string(r[i+1 : end])})
			i = j + 1
		default:
			two := ""
			if i+1 < len(r) {
				two = string(r[i : i+2])
			}
			switch two {
			case "==", "!=", "<=", ">=", "&&", "||", "+=", "-=", "*=", "/=", "++", "--", "<<", ">>":
				out = append(out, tok{kind: 'p', s: two})
				i += 2
			default:
				out = append(out, tok{kind: 'p', s: string(c)})
				i++
			}
		}
	}
	return out
}

// ---- parser / evaluator ----

type parser struct {
	toks []tok
	pos  int
	ctx  Context
	vars map[string]int
	res  Result
	bad  bool // current expression used something unknown
}

func (p *parser) eof() bool { return p.pos >= len(p.toks) }

func (p *parser) peek() tok {
	if p.eof() {
		return tok{}
	}
	return p.toks[p.pos]
}

func (p *parser) is(s string) bool { t := p.peek(); return t.kind != 0 && t.kind != 's' && t.s == s }

func (p *parser) accept(s string) bool {
	if p.is(s) {
		p.pos++
		return true
	}
	return false
}

// skipStatement moves past the current statement (to ';' or a balanced block).
func (p *parser) skipStatement() {
	depth := 0
	for !p.eof() {
		t := p.toks[p.pos]
		p.pos++
		if t.kind == 'p' {
			switch t.s {
			case "{", "(":
				depth++
			case "}", ")":
				depth--
				if depth <= 0 && t.s == "}" {
					return
				}
			case ";":
				if depth <= 0 {
					return
				}
			}
		}
	}
}

// statement executes (or, when run=false, only parses) one statement.
func (p *parser) statement(run bool) {
	t := p.peek()
	switch {
	case p.accept(";"):
	case p.accept("{"):
		for !p.eof() && !p.is("}") {
			before := p.pos
			p.statement(run)
			if p.pos == before {
				p.pos++
			}
		}
		p.accept("}")
	case t.kind == 'i' && t.s == "if":
		p.pos++
		p.accept("(")
		p.bad = false
		cond := p.expr()
		condBad := p.bad
		p.accept(")")
		if condBad && run {
			p.res.Skipped++
		}
		p.statement(run && !condBad && cond != 0)
		if p.peek().kind == 'i' && p.peek().s == "else" {
			p.pos++
			p.statement(run && !condBad && cond == 0)
		}
	case t.kind == 'i' && t.s == "bonus" && p.pos+3 < len(p.toks) && (p.toks[p.pos+1].s == "bAtkEle" || p.toks[p.pos+1].s == "bDefEle"):
		// bonus bAtkEle,Ele_Fire; takes an element constant, recorded as an extra.
		name, ele := p.toks[p.pos+1].s, p.toks[p.pos+3]
		p.pos += 4
		ok := ele.kind == 'i' && strings.HasPrefix(ele.s, "Ele_") && p.accept(";")
		if !ok {
			p.skipStatement()
		}
		if run {
			if ok {
				p.res.Extra[name+" "+ele.s] = 1
			} else {
				p.res.Skipped++
			}
		}
	case t.kind == 'i' && t.s == "bonus":
		p.pos++
		name := p.peek()
		p.pos++
		p.accept(",")
		p.bad = false
		v := p.expr()
		ok := name.kind == 'i' && !p.bad && p.accept(";")
		if !ok {
			p.skipStatement()
		}
		if run {
			if ok && known[name.s] {
				p.res.Bonus[name.s] += v
			} else {
				p.res.Skipped++
			}
		}
	case t.kind == 'i' && (t.s == "bonus2" || t.s == "bonus3" || t.s == "bonus4" || t.s == "bonus5"):
		p.pos++
		key, v, ok := p.multiBonus(int(t.s[5] - '0'))
		if run {
			if ok {
				p.res.Extra[key] += v
			} else {
				p.res.Skipped++
			}
		}
	case t.kind == 'i' && strings.HasPrefix(t.s, ".@"):
		p.pos++
		op := p.peek().s
		if op == "=" || op == "+=" || op == "-=" || op == "*=" || op == "/=" {
			p.pos++
			p.bad = false
			v := p.expr()
			p.accept(";")
			if run && !p.bad {
				cur := p.vars[t.s]
				switch op {
				case "=":
					cur = v
				case "+=":
					cur += v
				case "-=":
					cur -= v
				case "*=":
					cur *= v
				case "/=":
					if v != 0 {
						cur /= v
					}
				}
				p.vars[t.s] = cur
			}
			return
		}
		if op == "++" || op == "--" {
			p.pos++
			p.accept(";")
			if run {
				if op == "++" {
					p.vars[t.s]++
				} else {
					p.vars[t.s]--
				}
			}
			return
		}
		p.skipStatement()
		if run {
			p.res.Skipped++
		}
	default:
		p.skipStatement()
		if run {
			p.res.Skipped++
		}
	}
}

// multiBonus parses "bName,arg,...,value;" for bonusN (n-1 arguments before
// the value). Identifier and string arguments are kept verbatim (RC_DemiHuman,
// "KN_BOWLINGBASH"); the final value must evaluate cleanly.
func (p *parser) multiBonus(n int) (string, int, bool) {
	name := p.peek()
	if name.kind != 'i' {
		p.skipStatement()
		return "", 0, false
	}
	p.pos++
	parts := []string{name.s}
	for i := 0; i < n-1; i++ {
		if !p.accept(",") {
			p.skipStatement()
			return "", 0, false
		}
		t := p.peek()
		switch t.kind {
		case 'i', 's':
			p.pos++
			// An identifier followed by an operator is an expression, not a constant.
			if t.kind == 'i' && (p.is("(") || p.is("+") || p.is("*")) {
				p.skipStatement()
				return "", 0, false
			}
			parts = append(parts, t.s)
		default:
			p.bad = false
			v := p.expr()
			if p.bad {
				p.skipStatement()
				return "", 0, false
			}
			parts = append(parts, strconv.Itoa(v))
		}
	}
	if !p.accept(",") {
		p.skipStatement()
		return "", 0, false
	}
	p.bad = false
	v := p.expr()
	if p.bad || !p.accept(";") {
		p.skipStatement()
		return "", 0, false
	}
	return strings.Join(parts, " "), v, true
}

// Expression grammar (C precedence subset): ?: || && == != < <= > >= + - * / % unary.
func (p *parser) expr() int { return p.ternary() }

func (p *parser) ternary() int {
	c := p.or()
	if p.accept("?") {
		a := p.ternary()
		p.accept(":")
		b := p.ternary()
		if c != 0 {
			return a
		}
		return b
	}
	return c
}

func (p *parser) or() int {
	v := p.and()
	for p.accept("||") {
		w := p.and()
		v = b2i(v != 0 || w != 0)
	}
	return v
}

func (p *parser) and() int {
	v := p.cmp()
	for p.accept("&&") {
		w := p.cmp()
		v = b2i(v != 0 && w != 0)
	}
	return v
}

func (p *parser) cmp() int {
	v := p.add()
	for {
		switch {
		case p.accept("=="):
			v = b2i(v == p.add())
		case p.accept("!="):
			v = b2i(v != p.add())
		case p.accept("<="):
			v = b2i(v <= p.add())
		case p.accept(">="):
			v = b2i(v >= p.add())
		case p.accept("<"):
			v = b2i(v < p.add())
		case p.accept(">"):
			v = b2i(v > p.add())
		default:
			return v
		}
	}
}

func (p *parser) add() int {
	v := p.mul()
	for {
		switch {
		case p.accept("+"):
			v += p.mul()
		case p.accept("-"):
			v -= p.mul()
		default:
			return v
		}
	}
}

func (p *parser) mul() int {
	v := p.unary()
	for {
		switch {
		case p.accept("*"):
			v *= p.unary()
		case p.accept("/"):
			if d := p.unary(); d != 0 {
				v /= d
			} else {
				p.bad = true
			}
		case p.accept("%"):
			if d := p.unary(); d != 0 {
				v %= d
			} else {
				p.bad = true
			}
		default:
			return v
		}
	}
}

func (p *parser) unary() int {
	switch {
	case p.accept("-"):
		return -p.unary()
	case p.accept("!"):
		return b2i(p.unary() == 0)
	case p.accept("("):
		v := p.expr()
		p.accept(")")
		return v
	}
	t := p.peek()
	if p.eof() {
		p.bad = true
		return 0
	}
	p.pos++
	switch t.kind {
	case 'n':
		return t.n
	case 'i':
		switch {
		case strings.HasPrefix(t.s, ".@"):
			return p.vars[t.s]
		case t.s == "BaseLevel":
			return p.ctx.BaseLv
		case t.s == "JobLevel":
			return p.ctx.JobLv
		case t.s == "Class":
			return p.ctx.Class
		case t.s == "getrefine":
			p.args()
			return p.ctx.Refine
		case t.s == "getenchantgrade":
			p.args()
			return p.ctx.Grade
		case t.s == "getrandomoptinfo":
			if a := p.rawArgs(); len(a) == 1 && a[0] == "ROA_VALUE" {
				return p.ctx.OptValue
			}
			p.bad = true
			return 0
		case strings.HasPrefix(t.s, "ENCHANTGRADE_"):
			if g, ok := map[string]int{"NONE": 0, "D": 1, "C": 2, "B": 3, "A": 4}[strings.TrimPrefix(t.s, "ENCHANTGRADE_")]; ok {
				return g
			}
		case t.s == "readparam":
			args := p.rawArgs()
			if len(args) == 1 {
				if v, ok := p.ctx.Param[args[0]]; ok {
					return v
				}
			}
			p.bad = true
			return 0
		case t.s == "isequipped":
			ids := p.args()
			for _, id := range ids {
				if !p.ctx.EquipIDs[id] {
					return 0
				}
			}
			return b2i(len(ids) > 0)
		}
		if p.is("(") { // unknown function
			p.rawArgs()
		}
		p.bad = true
		return 0
	}
	p.bad = true
	return 0
}

// args parses "(expr, expr, ...)" and returns the values.
func (p *parser) args() []int {
	var out []int
	if !p.accept("(") {
		return out
	}
	for !p.eof() && !p.accept(")") {
		out = append(out, p.expr())
		p.accept(",")
	}
	return out
}

// rawArgs returns identifier arguments without evaluating them.
func (p *parser) rawArgs() []string {
	var out []string
	if !p.accept("(") {
		return out
	}
	depth := 1
	for !p.eof() && depth > 0 {
		t := p.toks[p.pos]
		p.pos++
		switch {
		case t.kind == 'p' && t.s == "(":
			depth++
		case t.kind == 'p' && t.s == ")":
			depth--
		case t.kind == 'i' && depth == 1:
			out = append(out, t.s)
		}
	}
	return out
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// known lists the bonuses the status calculation applies.
var known = map[string]bool{
	"bStr": true, "bAgi": true, "bVit": true, "bInt": true, "bDex": true, "bLuk": true,
	"bAllStats": true, "bAgiVit": true, "bAgiDexStr": true,
	"bPow": true, "bSta": true, "bWis": true, "bSpl": true, "bCon": true, "bCrt": true, "bAllTraitStats": true,
	"bMaxHP": true, "bMaxHPrate": true, "bMaxSP": true, "bMaxSPrate": true, "bMaxAP": true, "bMaxAPrate": true,
	"bBaseAtk": true, "bAtk": true, "bAtk2": true, "bAtkRate": true, "bWeaponAtkRate": true,
	"bMatk": true, "bMatkRate": true, "bWeaponMatkRate": true,
	"bHit": true, "bHitRate": true, "bFlee": true, "bFleeRate": true, "bFlee2": true, "bFlee2Rate": true,
	"bCritical": true, "bCriticalRate": true, "bDef": true, "bDefRate": true, "bDef2": true, "bDef2Rate": true,
	"bMdef": true, "bMdefRate": true, "bMdef2": true, "bMdef2Rate": true,
	"bAspd": true, "bAspdRate": true, "bLongAtkRate": true, "bShortAtkRate": true, "bCritAtkRate": true,
	"bNonCritAtkRate": true, "bPerfectHitRate": true, "bPerfectHitAddRate": true, "bHealPower": true, "bHealPower2": true,
	"bPAtk": true, "bSMatk": true, "bRes": true, "bMRes": true, "bHPlus": true, "bCRate": true,
	"bVariableCastrate": true, "bFixedCastrate": true, "bFixedCast": true, "bDelayrate": true,
	"bHPrecovRate": true, "bSPrecovRate": true, "bUseSPrate": true, "bSpeedRate": true, "bSpeedAddRate": true,
}

// Known reports whether a bonus name is applied by the calculator.
func Known(name string) bool { return known[name] }
