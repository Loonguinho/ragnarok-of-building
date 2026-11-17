"""A tiny translator for the TypeScript subset used by latam-ro-calc skill
formulas: arithmetic, comparisons, ternaries, const/let, if/else, return,
array literals with indexing, Math.*, and a fixed set of input accessors and
helpers. Everything else raises Unsupported, so a formula is either
translated faithfully or reported, never guessed.

Values are typed during translation: 'num' (Go float64), 'bool', 'str'.
"""
import re

BACKSLASH = chr(92)


class Unsupported(Exception):
    pass


# Helpers of latam-ro-calc's rebalance patches (skills/rebalance-2026.ts).
DSL = {'lv': 'float64(in.Lv)', 'pow': 'in.T(POW)', 'spl': 'in.T(SPL)', 'con': 'in.T(CON)',
       'climax': '', 'scale': '', 'race': '', 'learned': '', 'active': '', 'state': ''}


TOKEN = re.compile(r"""
    (?P<ws>\s+)
  | (?P<num>\d+(?:\.\d+)?(?:e[+-]?\d+)?|\.\d+)
  | (?P<str>'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*")
  | (?P<id>[A-Za-z_$][A-Za-z0-9_$]*)
  | (?P<op>===|!==|\*\*|=>|\?\?|\?\.|&&|\|\||<=|>=|==|!=|\+=|-=|\*=|/=|\.\.\.|[-+*/%<>!?:.,()\[\]{};=])
""", re.X)


def tokenize(src):
    out, i = [], 0
    while i < len(src):
        m = TOKEN.match(src, i)
        if not m:
            raise Unsupported(f'token at {src[i:i+20]!r}')
        i = m.end()
        kind = m.lastgroup
        if kind == 'ws':
            continue
        text = m.group(kind)
        if kind == 'str':
            text = text[1:-1]
        out.append((kind, text))
    out.append(('eof', ''))
    return out


# Their weapon type names -> rAthena SubType.
WEAPONS = {
    'dagger': 'Dagger', 'sword': '1hSword', 'twohandSword': '2hSword', 'spear': '1hSpear',
    'twohandSpear': '2hSpear', 'axe': '1hAxe', 'twohandAxe': '2hAxe', 'mace': 'Mace',
    'twohandMace': '2hMace', 'rod': 'Staff', 'twohandRod': '2hStaff', 'staff': 'Staff',
    'twohandStaff': '2hStaff', 'bow': 'Bow', 'knuckle': 'Knuckle', 'instrument': 'Musical',
    'whip': 'Whip', 'book': 'Book', 'katar': 'Katar', 'pistol': 'Revolver', 'rifle': 'Rifle',
    'gatlingGun': 'Gatling', 'shotgun': 'Shotgun', 'grenadeLauncher': 'Grenade', 'huuma': 'Huuma',
}

STATUS = {  # status.<name> -> Go
    'totalPow': 'in.T(POW)', 'totalSta': 'in.T(STA)', 'totalWis': 'in.T(WIS)', 'totalSpl': 'in.T(SPL)',
    'totalCon': 'in.T(CON)', 'totalCrt': 'in.T(CRT)',
    'totalStr': 'in.S(STR)', 'totalAgi': 'in.S(AGI)', 'totalVit': 'in.S(VIT)', 'totalInt': 'in.S(INT)',
    'totalDex': 'in.S(DEX)', 'totalLuk': 'in.S(LUK)',
    'totalHit': 'float64(in.HIT)', 'totalFlee': 'float64(in.FLEE)', 'totalCri': 'float64(in.CRIT)',
    'totalAspd': 'float64(in.ASPD)', 'totalMaxHp': 'float64(in.MaxHP)', 'totalMaxSp': 'float64(in.MaxSP)',
}
MODEL = {'level': 'float64(in.BaseLv)', 'jobLevel': 'float64(in.JobLv)'}
WEAPON_DATA = {'weight': 'float64(in.WeaponWt)/10', 'baseWeaponLevel': 'float64(in.WeaponLv)',
               'refine': 'float64(in.WeaponRefine)', 'baseWeaponAtk': 'float64(in.WeaponATK)'}
INPUT = {'skillLevel': ('num', 'float64(in.Lv)'), 'maxHp': ('num', 'float64(in.MaxHP)'),
         'maxSp': ('num', 'float64(in.MaxSP)'), 'currentHp': ('num', 'float64(in.MaxHP)'),
         'currentSp': ('num', 'float64(in.MaxSP)'), 'stack': ('num', 'float64(in.Stack)')}


class Ctx:
    """Translation context: local variable types and the helper calls seen."""

    def __init__(self, params=None):
        self.file = None     # FileCtx of the job file being translated
        self.params = {}     # parameter name -> (kind, Go expression)
        self.vars = {}       # name -> kind
        self.objs = {}       # name -> object root ('input', 'status', 'model', 'monster', 'weapon', 'weapon.data')
        self.active = set()  # names used with isSkillActive (toggles)
        self.active_lv = set()
        self.learn = set()
        for p in params or []:
            self.bind_param(p)

    def bind_param(self, name):
        if name in INPUT:
            self.params[name] = INPUT[name]
        elif name in ('input', 'status', 'model', 'monster', 'weapon'):
            self.objs[name] = name
        elif name in ('i', 'params'):
            self.objs[name] = 'input'
        elif name == 'weaponType':
            self.params[name] = ('str', 'in.WeaponName()')
        else:
            raise Unsupported(f'param {name}')


def go_local(name):
    return 'v_' + name


class Parser:
    def __init__(self, toks, ctx):
        self.t, self.i, self.ctx = toks, 0, ctx
        self.file = None

    def peek(self, k=0):
        return self.t[self.i + k]

    def take(self, text=None):
        tok = self.t[self.i]
        if text is not None and tok[1] != text:
            raise Unsupported(f'expected {text!r}, got {tok[1]!r}')
        self.i += 1
        return tok

    def at(self, text):
        return self.t[self.i][1] == text and self.t[self.i][0] in ('op', 'id')

    # ---------- statements ----------
    def block(self):
        """Statements until '}' or eof. Returns list of Go lines."""
        out = []
        while not self.at('}') and self.peek()[0] != 'eof':
            out.extend(self.statement())
        return out

    def statement(self):
        tok = self.peek()
        if tok[1] in ('const', 'let', 'var'):
            return self.decl()
        if tok[1] == 'return':
            self.take()
            v = self.expr()
            self.opt(';')
            return [f'return {as_num(v)}']
        if tok[1] == 'if':
            return self.if_stmt()
        if tok[1] == 'switch':
            return self.switch_stmt()
        if tok[1] == 'break':
            raise Unsupported('break outside switch')
        if tok[1] == '{':
            self.take('{')
            body = self.block()
            self.take('}')
            return ['{'] + body + ['}']
        if tok[1] == ';':
            self.take()
            return []
        if tok[0] == 'id' and self.peek(1)[1] in ('=', '+=', '-=', '*=', '/='):
            name = self.take()[1]
            op = self.take()[1]
            if name not in self.ctx.vars:
                raise Unsupported(f'assign to {name}')
            v = self.expr()
            self.opt(';')
            kind = self.ctx.vars[name]
            if op == '=':
                return [f'{go_local(name)} = {conv(v, kind)}']
            return [f'{go_local(name)} {op} {as_num(v)}']
        raise Unsupported(f'statement {tok[1]!r}')

    def switch_stmt(self):
        """switch (x) { case a: case b: ...; return/break; default: ... }"""
        self.take('switch')
        self.take('(')
        subject = as_num(self.expr())
        self.take(')')
        self.take('{')
        out = [f'switch {subject} {{']
        while not self.at('}'):
            labels, is_default = [], False
            while self.at('case') or self.at('default'):
                if self.take()[1] == 'default':
                    is_default = True
                else:
                    labels.append(as_num(self.expr()))
                self.take(':')
            body, ended = [], False
            while not (self.at('case') or self.at('default') or self.at('}')):
                if self.at('break'):
                    self.take('break')
                    self.opt(';')
                    ended = True
                    continue
                stmt = self.statement()
                body.extend(stmt)
                if stmt and stmt[-1].startswith('return '):
                    ended = True
            if not ended and body:
                raise Unsupported('switch fallthrough')
            if is_default:
                out.append('default:')
            else:
                out.append(f'case {", ".join(labels)}:')
            out.extend(body)
        self.take('}')
        out.append('}')
        return out

    def opt(self, text):
        if self.at(text):
            self.take()

    def if_stmt(self):
        self.take('if')
        self.take('(')
        cond = self.expr()
        self.take(')')
        then = self.stmt_block()
        out = [f'if {as_bool(cond)} {{'] + then
        if self.at('else'):
            self.take('else')
            if self.at('if'):
                rest = self.if_stmt()
                out.append('} else ' + rest[0])
                out.extend(rest[1:])
                return out
            out.append('} else {')
            out.extend(self.stmt_block())
        out.append('}')
        return out

    def stmt_block(self):
        if self.at('{'):
            self.take('{')
            body = self.block()
            self.take('}')
            return body
        return self.statement()

    def decl(self):
        self.take()
        out = []
        while True:
            if self.at('{'):  # destructuring
                self.take('{')
                props = []
                while not self.at('}'):
                    key = self.take()[1]
                    alias = key
                    default = None
                    if self.at(':'):
                        self.take(':')
                        alias = self.take()[1]
                    if self.at('='):
                        self.take('=')
                        default = self.expr()
                    props.append((key, alias, default))
                    self.opt(',')
                self.take('}')
                self.take('=')
                src = self.expr()
                if src[0] != 'obj':
                    raise Unsupported('destructure non-object')
                for key, alias, _default in props:
                    v = member(src, key, self.ctx)
                    out.extend(self.bind(alias, v))
            else:
                name = self.take()[1]
                if self.at(':'):  # type annotation
                    self.take(':')
                    self.type_annotation()
                if self.at('='):
                    self.take('=')
                    v = self.expr()
                else:
                    v = ('num', '0')
                out.extend(self.bind(name, v))
            if self.at(','):
                self.take(',')
                continue
            break
        self.opt(';')
        return out

    def type_annotation(self):
        depth = 0
        while True:
            tok = self.peek()
            if tok[0] == 'eof':
                raise Unsupported('unterminated type annotation')
            if tok[1] in ('=', ';', ',', ')', '=>') and depth == 0:
                return
            if tok[1] in ('[', '<', '('):
                depth += 1
            if tok[1] in (']', '>', ')'):
                depth -= 1
            self.take()

    def bind(self, name, v):
        if v[0] == 'obj':
            self.ctx.objs[name] = v[1]
            return []
        if v[0] == 'arr':
            raise Unsupported('array variable')
        self.ctx.vars[name] = v[0]
        go = go_local(name)
        expr = f'float64({v[1]})' if v[0] == 'num' else v[1]  # JS numbers are floats
        return [f'{go} := {expr}', f'_ = {go}']

    # ---------- expressions (precedence climbing) ----------
    def expr(self):
        return self.ternary()

    def ternary(self):
        c = self.nullish()
        if self.at('?'):
            self.take('?')
            a = self.ternary()
            self.take(':')
            b = self.ternary()
            if a[0] == 'str' or b[0] == 'str':
                return ('str', f'ternS({as_bool(c)}, {a[1]}, {b[1]})')
            if a[0] == 'bool' and b[0] == 'bool':
                return ('bool', f'ternB({as_bool(c)}, {a[1]}, {b[1]})')
            return ('num', f'tern({as_bool(c)}, {as_num(a)}, {as_num(b)})')
        return c

    def nullish(self):
        v = self.logic_or()
        while self.at('??'):
            self.take()
            self.logic_or()  # left side is always defined in our inputs
        return v

    def logic_or(self):
        v = self.logic_and()
        while self.at('||'):
            self.take()
            w = self.logic_and()
            if v[0] == 'bool' and w[0] == 'bool':
                v = ('bool', f'({v[1]} || {w[1]})')
            else:
                v = ('num', f'orNum({as_num(v)}, {as_num(w)})')
        return v

    def logic_and(self):
        v = self.equality()
        while self.at('&&'):
            self.take()
            w = self.equality()
            if v[0] == 'bool' and w[0] == 'bool':
                v = ('bool', f'({v[1]} && {w[1]})')
            else:
                v = ('num', f'andNum({as_num(v)}, {as_num(w)})')
        return v

    def equality(self):
        v = self.relational()
        while self.peek()[1] in ('===', '!==', '==', '!='):
            op = '==' if self.take()[1] in ('===', '==') else '!='
            w = self.relational()
            if v[0] == 'str' or w[0] == 'str':
                neg = '' if op == '==' else '!'
                v = ('bool', f'{neg}strEq({v[1]}, {w[1]})')
            elif v[0] == 'bool' and w[0] == 'bool':
                v = ('bool', f'({v[1]} {op} {w[1]})')
            else:
                v = ('bool', f'({as_num(v)} {op} {as_num(w)})')
        return v

    def relational(self):
        v = self.additive()
        while self.peek()[1] in ('<', '>', '<=', '>='):
            op = self.take()[1]
            w = self.additive()
            v = ('bool', f'({as_num(v)} {op} {as_num(w)})')
        return v

    def additive(self):
        v = self.multiplicative()
        while self.peek()[1] in ('+', '-'):
            op = self.take()[1]
            w = self.multiplicative()
            if v[0] == 'str' or w[0] == 'str':
                raise Unsupported('string concatenation')
            v = ('num', f'({as_num(v)} {op} {as_num(w)})')
        return v

    def multiplicative(self):
        v = self.power()
        while self.peek()[1] in ('*', '/', '%'):
            op = self.take()[1]
            w = self.power()
            if op == '%':
                v = ('num', f'math.Mod({as_num(v)}, {as_num(w)})')
            else:
                v = ('num', f'({as_num(v)} {op} {as_num(w)})')
        return v

    def power(self):
        v = self.unary()
        if self.at('**'):
            self.take()
            w = self.power()
            return ('num', f'math.Pow({as_num(v)}, {as_num(w)})')
        return v

    def unary(self):
        if self.at('!'):
            self.take()
            v = self.unary()
            return ('bool', f'!{as_bool(v)}')
        if self.at('-'):
            self.take()
            v = self.unary()
            return ('num', f'(-{as_num(v)})')
        if self.at('+'):
            self.take()
            return ('num', as_num(self.unary()))
        return self.postfix()

    def postfix(self):
        v = self.primary()
        while True:
            if self.at('.') or self.at('?.'):
                self.take()
                name = self.take()[1]
                if self.at('('):
                    args = self.args()
                    if v[0] == 'arr' and name == 'includes':
                        items = ', '.join(as_num(x) for x in v[1])
                        v = ('bool', f'containsF([]float64{{{items}}}, {as_num(args[0])})')
                        continue
                    v = method(v, name, args, self.ctx)
                else:
                    v = member(v, name, self.ctx)
            elif self.at('['):
                self.take('[')
                idx = self.expr()
                self.take(']')
                if v[0] != 'arr':
                    raise Unsupported('index non-array')
                v = ('num', f'idx([]float64{{{", ".join(as_num(x) for x in v[1])}}}, {as_num(idx)})')
            elif self.at('('):
                raise Unsupported('call of value')
            else:
                return v

    def dsl_call(self, name):
        args = self.args()
        rest = [a for a in args if a[0] != 'obj']  # drop the `i` input argument
        if name in ('lv', 'pow', 'spl', 'con'):
            return ('num', DSL[name])
        if name == 'climax':
            self.ctx.active_lv.add('Climax')
            return ('num', 'float64(in.ActiveLv("Climax"))')
        if name == 'scale':
            return ('num', f'({as_num(rest[0])} * float64(in.BaseLv) / 100)')
        if name == 'race':
            return ('bool', f'in.MonIs("race", {", ".join(a[1] for a in rest)})')
        skill = rest[0][1] if rest and rest[0][0] == 'str' else None
        if skill is None:
            raise Unsupported(f'{name} without a skill name')
        if name == 'learned':
            return ('num', f'float64(in.LearnName({skill}))')
        if name == 'active':
            self.ctx.active.add(skill.strip('"'))
            return ('bool', f'in.Active({skill})')
        if name == 'state':
            self.ctx.active_lv.add(skill.strip('"'))
            return ('num', f'float64(in.ActiveLv({skill}))')
        raise Unsupported(f'dsl {name}')

    def args(self):
        self.take('(')
        out = []
        while not self.at(')'):
            out.append(self.expr())
            self.opt(',')
        self.take(')')
        return out

    def primary(self):
        kind, text = self.take()
        if kind == 'num':
            return ('num', fmt_num(text))
        if kind == 'str':
            return ('str', go_str(text))
        if text == '(':
            v = self.expr()
            self.take(')')
            return v
        if text == '[':
            items = []
            while not self.at(']'):
                items.append(self.expr())
                self.opt(',')
            self.take(']')
            return ('arr', items)
        if text == 'true':
            return ('bool', 'true')
        if text == 'false':
            return ('bool', 'false')
        if text in ('undefined', 'null'):
            return ('num', '0')
        if text == 'Math':
            self.take('.')
            fn = self.take()[1]
            args = self.args()
            go = {'floor': 'math.Floor', 'ceil': 'math.Ceil', 'round': 'jsRound', 'trunc': 'math.Trunc',
                  'abs': 'math.Abs', 'sqrt': 'math.Sqrt', 'min': 'fmin', 'max': 'fmax', 'pow': 'math.Pow'}.get(fn)
            if not go:
                raise Unsupported(f'Math.{fn}')
            return ('num', f'{go}({", ".join(as_num(a) for a in args)})')
        if text == 'floor' or text == 'round':  # imported helpers
            args = self.args()
            return ('num', f'{"math.Floor" if text == "floor" else "jsRound"}({", ".join(as_num(a) for a in args)})')
        if text == 'this':
            self.take('.')
            name = self.take()[1]
            if not self.at('('):
                raise Unsupported(f'this.{name}')
            args = self.args()
            return helper(name, args, self.ctx, self.file or self.ctx.file)
        if text == 'ElementType':
            self.take('.')
            return ('str', go_str(self.take()[1]))
        if text == 'CelestialSpace':
            self.take('.')
            name = self.take()[1]
            spaces = {'Sunrise': 1, 'Noon': 2, 'Sunset': 3, 'Moonrise': 4, 'Midnight': 5, 'Moonset': 6, 'Unity': 7}
            if name not in spaces:
                raise Unsupported(f'CelestialSpace.{name}')
            return ('num', str(spaces[name]))
        f = self.file or (self.ctx.file if self.ctx else None)
        if f is not None and getattr(f, 'dsl', False) and self.at('(') and text in DSL:
            return self.dsl_call(text)
        if f is not None and text in f.enums and self.at('.'):
            self.take('.')
            member = self.take()[1]
            v = f.enum_value(text, member)
            if v is None:
                raise Unsupported(f'{text}.{member}')
            return ('num', v)
        if text in self.ctx.params:
            return self.ctx.params[text]
        if text in self.ctx.vars:
            return (self.ctx.vars[text], go_local(text))
        if text in self.ctx.objs:
            return ('obj', self.ctx.objs[text])
        if text in INPUT:
            return INPUT[text]
        if f is not None and text in f.arrays:
            return f.array(text, self.ctx)
        if f is not None and text in f.arrows and self.at('('):
            return f.call_arrow(text, self.args())
        if f is not None and text in f.methods and self.at('('):
            v = f.call_method(text, self.args(), self.ctx)
            if v is not None:
                return v
        raise Unsupported(f'identifier {text}')


def fmt_num(text):
    return text if ('.' in text or 'e' in text) else text + '.0' if False else text


def go_str(s):
    return '"' + s.replace('"', BACKSLASH + '"') + '"'


def as_num(v):
    if v[0] == 'num':
        return v[1]
    if v[0] == 'bool':
        return f'b2f({v[1]})'
    raise Unsupported(f'{v[0]} used as number')


def as_bool(v):
    if v[0] == 'bool':
        return v[1]
    if v[0] == 'num':
        return f'({v[1]} != 0)'
    if v[0] == 'str':
        return f'({v[1]} != "")'
    raise Unsupported(f'{v[0]} used as condition')


def conv(v, kind):
    return as_num(v) if kind == 'num' else as_bool(v) if kind == 'bool' else v[1]


def member(obj, name, ctx):
    if obj[0] != 'obj':
        raise Unsupported(f'.{name} of {obj[0]}')
    root = obj[1]
    if root == 'input':
        if name in INPUT:
            return INPUT[name]
        if name in ('model', 'status', 'monster', 'weapon'):
            return ('obj', name)
        raise Unsupported(f'input.{name}')
    if root == 'status':
        if name in STATUS:
            return ('num', STATUS[name])
        raise Unsupported(f'status.{name}')
    if root == 'model':
        if name in MODEL:
            return ('num', MODEL[name])
        raise Unsupported(f'model.{name}')
    if root == 'weapon':
        if name == 'data':
            return ('obj', 'weapon.data')
        raise Unsupported(f'weapon.{name}')
    if root == 'weapon.data':
        if name in WEAPON_DATA:
            return ('num', WEAPON_DATA[name])
        if name == 'typeName':
            return ('str', 'in.WeaponName()')
        raise Unsupported(f'weapon.data.{name}')
    if root == 'monster':
        if name in ('race', 'element', 'size', 'type'):
            return ('str', f'in.Mon("{name}")')
        if name == 'data':
            return ('obj', 'monster.data')
        raise Unsupported(f'monster.{name}')
    if root == 'monster.data':
        if name in ('hp', 'level', 'def', 'mdef', 'res', 'mres'):
            return ('num', f'in.MonNum("{name}")')
        raise Unsupported(f'monster.data.{name}')
    raise Unsupported(f'{root}.{name}')


def method(obj, name, args, ctx):
    if obj[0] == 'obj' and obj[1] == 'monster':
        if name in ('isRace', 'isElement', 'isSize', 'isType'):
            return ('bool', f'in.MonIs("{name[2:].lower()}", {", ".join(a[1] for a in args)})')
    if obj[0] == 'obj' and obj[1] == 'weapon' and name == 'isSubType':
        sub = {'Rifle': 'Rifle', 'Gatling Gun': 'Gatling', 'Shotgun': 'Shotgun', 'Grenade Launcher': 'Grenade',
               'Revolver': 'Revolver', 'Pistol': 'Revolver'}
        names = []
        for a in args:
            t = a[1].strip('"')
            if a[0] != 'str' or t not in sub:
                raise Unsupported(f'weapon subtype {t}')
            names.append(go_str(sub[t]))
        return ('bool', f'in.WeaponIs({", ".join(names)})')
    if obj[0] == 'obj' and obj[1] == 'weapon' and name == 'isType':
        names = []
        for a in args:
            if a[0] != 'str':
                raise Unsupported('weapon.isType arg')
            t = a[1].strip('"')
            if t not in WEAPONS:
                raise Unsupported(f'weapon type {t}')
            names.append(go_str(WEAPONS[t]))
        return ('bool', f'in.WeaponIs({", ".join(names)})')
    if obj[0] == 'str' and name in ('includes', 'startsWith'):
        raise Unsupported(f'string.{name}')
    raise Unsupported(f'method {name}')


def helper(name, args, ctx, file=None):
    def skill_arg():
        if len(args) < 1 or args[0][0] != 'str':
            raise Unsupported(f'{name} arg')
        return args[0][1]
    if name == 'learnLv':
        a = skill_arg()
        ctx.learn.add(a.strip('"'))
        return ('num', f'float64(in.LearnName({a}))')
    if name == 'isSkillActive':
        a = skill_arg()
        ctx.active.add(a.strip('"'))
        return ('bool', f'in.Active({a})')
    if name == 'activeSkillLv':
        a = skill_arg()
        ctx.active_lv.add(a.strip('"'))
        return ('num', f'float64(in.ActiveLv({a}))')
    if name == 'isCelestialSpace':
        return ('bool', f'in.CelestialSpace(int({as_num(args[0])}))')
    if name in ('getCurrentSP', 'getCurrentSp'):
        return ('num', 'float64(in.MaxSP)')
    if name in ('getCurrentHP', 'getCurrentHp'):
        return ('num', 'float64(in.MaxHP)')
    if name == 'isSpirit' and file is not None:
        a = skill_arg().strip('"')
        return ('bool', f'in.Choice("spirit", {file.choice("spirit", a)})')
    if file is not None:
        v = file.call_method(name, args, ctx)
        if v is not None:
            return v
    raise Unsupported(f'helper this.{name}')


def translate_function(src, file=None, weapon_first=False):
    """Translate an arrow function `(params) => expr|{body}` into a Go body
    returning float64. Returns (go_lines, ctx). With weapon_first, the first
    parameter is the weapon type name (isMelee callbacks)."""
    toks = tokenize(src)
    p = Parser(toks, None)
    params = parse_params(p)
    if weapon_first and params:
        ctx = Ctx(params[1:])
        ctx.params[params[0]] = ('str', 'in.WeaponName()')
    else:
        ctx = Ctx(params)
    ctx.file = file
    p.ctx = ctx
    p.file = file
    p.take('=>')
    if p.at('{'):
        p.take('{')
        lines = p.block()
        p.take('}')
        if not ends_with_return(lines):
            lines.append('return 0')
    else:
        v = p.expr()
        lines = [f'return {as_num(v)}']
    return lines, ctx


def translate_bool_function(src):
    toks = tokenize(src)
    p = Parser(toks, None)
    params = parse_params(p)
    ctx = Ctx(params)
    p.ctx = ctx
    p.take('=>')
    if p.at('{'):
        p.take('{')
        lines = p.block()
        p.take('}')
        lines = [l.replace('return ', 'return ', 1) for l in lines]
        if not ends_with_return(lines):
            lines.append('return 0')
    else:
        v = p.expr()
        lines = [f'return {as_num(v)}']
    return lines, ctx


def parse_params(p):
    """`(input: T)`, `({ skillLevel, monster })`, `()`, `(weaponType)`, `x`."""
    names = []
    if p.at('('):
        p.take('(')
        if p.at('{'):
            p.take('{')
            while not p.at('}'):
                key = p.take()[1]
                if p.at(':'):
                    p.take(':')
                    p.take()
                names.append(key)
                p.opt(',')
            p.take('}')
            if p.at(':'):
                p.take(':')
                p.type_annotation()
        while not p.at(')'):
            name = p.take()[1]
            if p.at(':'):
                p.take(':')
                p.type_annotation()
            names.append(name)
            p.opt(',')
        p.take(')')
        if p.at(':'):  # return type
            p.take(':')
            p.take()
    else:
        names.append(p.take()[1])
    return [n for n in names if n != '_']


def ends_with_return(lines):
    """True when the body always returns (last statement is a return or an
    if/else chain whose branches all end in return)."""
    if not lines:
        return False
    if lines[-1].startswith('return '):
        return True
    if lines[-1] != '}':
        return False
    # A trailing `switch` with a default (every branch ends in return; the
    # translator rejects fallthrough) is terminating in Go.
    depth = 0
    for k in range(len(lines) - 1, -1, -1):
        line = lines[k]
        if line.startswith('}'):
            depth += 1
        if line.endswith('{'):
            depth -= 1
            if depth == 0:
                if line.startswith('switch '):
                    return 'default:' in lines[k:]
                break
    # Walk back the trailing if/else chain at depth 0.
    depth = 0
    branches_end_in_return = []
    has_else = False
    prev = None
    for line in lines:
        if depth == 0 and line.startswith('if '):
            branches_end_in_return = []
            has_else = False
        opens = line.endswith('{')
        closes = line.startswith('}')
        if closes:
            depth -= 1
            if depth == 0:
                branches_end_in_return.append(prev is not None and prev.startswith('return '))
                if 'else {' in line:
                    has_else = True
        if opens:
            depth += 1
        prev = line
    return has_else and all(branches_end_in_return)
