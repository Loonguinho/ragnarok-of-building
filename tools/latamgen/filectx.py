"""File-level context for a latam-ro-calc job file: enums, constant arrays,
module arrow functions and class private methods, translated to Go on demand
so formulas that call them can be generated too."""
import re

from jsx import Ctx, Parser, Unsupported, as_num, ends_with_return, tokenize
from tsparse import match_brace, strip_comments


class FileCtx:
    def __init__(self, stem, src):
        self.stem = stem
        self.src = strip_comments(src)
        self.enums = {}     # Enum -> {member: value}
        self.arrays = {}    # name -> source of the array literal
        self.arrows = {}    # name -> (params, body source)
        self.methods = {}   # name -> (params [(name, type, default)], body source)
        self.gofuncs = {}   # key -> Go function name (translated)
        self.go = []        # Go function declarations
        self.choices = {}   # choice group -> ordered names
        self.translating = set()
        self._scan()

    def _scan(self):
        s = self.src
        for m in re.finditer(r'\benum\s+(\w+)\s*\{([^}]*)\}', s):
            vals, nxt = {}, 0
            for part in m.group(2).split(','):
                part = part.strip()
                if not part:
                    continue
                if '=' in part:
                    k, v = [x.strip() for x in part.split('=', 1)]
                    try:
                        nxt = float(v)
                    except ValueError:
                        continue
                else:
                    k = part
                vals[k] = nxt
                nxt += 1
            self.enums[m.group(1)] = vals
        # const Name = { A: 1, B: 2 } (as const): numeric lookup objects used like enums
        for m in re.finditer(r'^(?:export\s+)?const\s+(\w+)\s*=\s*\{([^{}]*)\}', s, re.M):
            vals = {}
            for part in m.group(2).split(','):
                mm = re.match(r'\s*(\w+)\s*:\s*(-?\d+(?:\.\d+)?)\s*$', part)
                if mm:
                    vals[mm.group(1)] = float(mm.group(2))
            if vals:
                self.enums.setdefault(m.group(1), vals)
        for m in re.finditer(r'^(?:export\s+)?const\s+(\w+)\s*(?::[^=\n]+)?=\s*\[', s, re.M):
            end = match_brace(s, m.end() - 1)
            self.arrays[m.group(1)] = s[m.end() - 1:end + 1]
        for m in re.finditer(r'^(?:export\s+)?const\s+(\w+)\s*=\s*\(([^)]*)\)\s*(?::\s*\w+\s*)?=>', s, re.M):
            params = [p.split(':')[0].strip() for p in m.group(2).split(',') if p.strip()]
            i = m.end()
            while s[i] in ' \t\n':
                i += 1
            if s[i] == '{':
                body = s[i:match_brace(s, i) + 1]
            else:
                j, depth = i, 0
                while j < len(s):
                    ch = s[j]
                    if ch in '([{':
                        depth += 1
                    elif ch in ')]}':
                        depth -= 1
                    elif ch == ';' and depth == 0:
                        break
                    j += 1
                body = s[i:j]
            self.arrows[m.group(1)] = (params, body)
        for m in re.finditer(r'^(?:\s+(?:private|protected)\s+(?:override\s+)?|function\s+)(\w+)\s*\(([^)]*)\)\s*(?::\s*[\w<>\[\]| .]+)?\s*\{', s, re.M):
            params = []
            for p in self._split_params(m.group(2)):
                name, _, rest = p.partition(':')
                typ, _, default = rest.partition('=')
                if not rest and '=' in name:
                    name, _, default = name.partition('=')
                params.append((name.strip(), typ.strip(), default.strip()))
            body = s[m.end() - 1:match_brace(s, m.end() - 1) + 1]
            self.methods[m.group(1)] = (params, body)

    @staticmethod
    def _split_params(text):
        out, depth, cur = [], 0, ''
        for ch in text:
            if ch in '(<[':
                depth += 1
            elif ch in ')>]':
                depth -= 1
            if ch == ',' and depth == 0:
                out.append(cur)
                cur = ''
            else:
                cur += ch
        if cur.strip():
            out.append(cur)
        return [p.strip() for p in out if p.strip()]

    def go_name(self, name):
        return f'gen{self.stem}_{name}'

    # ---- used by the expression translator ----
    def enum_value(self, enum, member):
        if enum in self.enums and member in self.enums[enum]:
            v = self.enums[enum][member]
            return str(int(v)) if v == int(v) else str(v)
        return None

    def array(self, name, ctx):
        src = self.arrays.get(name)
        if src is None:
            return None
        p = Parser(tokenize(src), ctx)
        p.file = self
        return p.primary()

    def call_arrow(self, name, args):
        if name not in self.arrows:
            return None
        params, body = self.arrows[name]
        if len(args) != len(params):
            raise Unsupported(f'{name}: {len(args)} args for {len(params)} params')
        key = 'arrow:' + name
        if key not in self.gofuncs:
            ctx = Ctx([])
            for p in params:
                ctx.vars[p] = 'num'
            lines = self._translate_body(body, ctx)
            go = self.go_name(name)
            sig = ', '.join(f'v_{p} float64' for p in params)
            self.go.append(f'func {go}({sig}) float64 {{\n\t' + '\n\t'.join(lines) + '\n}')
            self.gofuncs[key] = go
        return ('num', f'{self.gofuncs[key]}({", ".join(as_num(a) for a in args)})')

    def call_method(self, name, args, outer):
        if name not in self.methods:
            return None
        params, body = self.methods[name]
        key = 'method:' + name
        if key in self.translating:
            raise Unsupported(f'recursive method {name}')
        if key not in self.gofuncs:
            self.translating.add(key)
            try:
                ctx = Ctx([])
                ctx.file = self
                sig = ['in FormulaInput']
                for pname, typ, _default in params:
                    if typ == 'Input':
                        ctx.objs[pname] = 'input'
                        continue
                    if typ == 'boolean':
                        ctx.vars[pname] = 'bool'
                        sig.append(f'v_{pname} bool')
                        continue
                    kind = 'num' if typ in ('number', '') or typ in self.enums or re.fullmatch(r'[\d\s|.]+', typ) else 'str' if 'string' in typ or typ[:1].isupper() or 'typeof' in typ else None
                    if kind is None:
                        raise Unsupported(f'{name} param type {typ}')
                    ctx.vars[pname] = kind
                    sig.append(f'v_{pname} {"float64" if kind == "num" else "string"}')
                lines = self._translate_body(body, ctx)
                go = self.go_name(name)
                self.go.append(f'func {go}({", ".join(sig)}) float64 {{\n\t' + '\n\t'.join(lines) + '\n}')
                self.gofuncs[key] = go
                outer.active |= ctx.active
                outer.active_lv |= ctx.active_lv
            finally:
                self.translating.discard(key)
        call = ['in']
        j = 0  # argument cursor
        for pname, typ, default in params:
            if typ == 'Input':  # the input object: passed as `in`
                if j < len(args) and args[j][0] == 'obj':
                    j += 1
                continue
            if j < len(args):
                a = args[j]
                j += 1
                if a[0] == 'obj':
                    raise Unsupported(f'{name}: object passed as {pname}')
            elif default:
                p = Parser(tokenize(default), outer)
                p.file = self
                a = p.expr()
            else:
                raise Unsupported(f'{name}: missing arg {pname}')
            if typ == 'boolean':
                call.append(a[1] if a[0] == 'bool' else f'({as_num(a)} != 0)')
                continue
            if a[0] == 'str':
                call.append(a[1])
                lit = a[1].strip('"')
                if a[1].startswith('"'):
                    outer.active.add(lit)  # string args are skill/state names
            else:
                call.append(as_num(a))
        return ('num', f'{self.gofuncs[key]}({", ".join(call)})')

    def _translate_body(self, body, ctx):
        toks = tokenize(body)
        p = Parser(toks, ctx)
        p.file = self
        if p.at('{'):
            p.take('{')
            lines = p.block()
            p.take('}')
            if not ends_with_return(lines):
                lines.append('return 0')
            return lines
        return [f'return {as_num(p.expr())}']

    def choice(self, group, name):
        names = self.choices.setdefault(group, [])
        if name not in names:
            names.append(name)
        return names.index(name) + 1
