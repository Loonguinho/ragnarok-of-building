"""The 2022 rebalance layer latam-ro-calc applies for LATAM's "current"
formulas (skills/rebalance-2026.ts: resolveRebalancedSkill and
rebalancedSkillCatalog with REBALANCE_2022_PATCHES + REBALANCE_2022_REFERENCE).

Patched skills take hit counts and the magic flag from the reference, then the
patch fields; patch formulas use a small DSL (lv, pow, spl, con, scale, linear,
race, climax, active, learned, state) that the translator understands when the
parser's file has dsl=True."""
import json
import os
import re

from tsparse import match_brace, strip_comments, skip_string


from filectx import FileCtx  # noqa: E402


class RebalanceCtx(FileCtx):
    """FileCtx over skills/rebalance-2026.ts, with the patch DSL enabled."""
    dsl = True

    def __init__(self, root):
        path = os.path.join(root, 'src', 'app', 'skills', 'rebalance-2026.ts')
        super().__init__('Rebalance', open(path, encoding='utf-8').read())
        # The DSL helpers are handled by the translator itself.
        for name in ('lv', 'pow', 'spl', 'con', 'learned', 'active', 'state', 'scale', 'race', 'climax', 'linear', 'part'):
            self.arrows.pop(name, None)
            self.methods.pop(name, None)


def split_top(body):
    """Top-level comma-separated items of an object/array body."""
    items, depth, cur, i = [], 0, '', 0
    while i < len(body):
        ch = body[i]
        if ch in '"\'`':
            j = skip_string(body, i)
            cur += body[i:j + 1]
            i = j + 1
            continue
        if ch in '([{':
            depth += 1
        elif ch in ')]}':
            depth -= 1
        if ch == ',' and depth == 0:
            items.append(cur.strip())
            cur = ''
        else:
            cur += ch
        i += 1
    if cur.strip():
        items.append(cur.strip())
    return items


def object_fields(src):
    """{ ...Base['X'], key: value } -> (spread bases, {key: value source})."""
    src = src.strip()
    assert src.startswith('{'), src[:40]
    spreads, fields = [], {}
    for item in split_top(src[1:-1]):
        if item.startswith('...'):
            spreads.append(item[3:].strip())
            continue
        m = re.match(r"([A-Za-z_]\w*|'[^']+')\s*:\s*(.*)$", item, re.S)
        if m:
            fields[m.group(1).strip("'")] = m.group(2).strip()
    return spreads, fields


class Rebalance:
    def __init__(self, root):
        skills = os.path.join(root, 'src', 'app', 'skills')
        src = strip_comments(open(os.path.join(skills, 'rebalance-2026.ts'), encoding='utf-8').read())
        self.patches = {}
        raw = {}
        for name in ('REBALANCE_2026_PATCHES', 'REBALANCE_2022_PATCHES'):
            m = re.search(name + r'\s*:\s*Record<[^>]+>\s*=\s*\{', src)
            body = src[m.end() - 1:match_brace(src, m.end() - 1) + 1]
            raw[name] = split_top(body[1:-1])
        self._resolved = {}
        p26 = self._resolve_table(raw['REBALANCE_2026_PATCHES'], {})
        self.patches = self._resolve_table(raw['REBALANCE_2022_PATCHES'], {'REBALANCE_2026_PATCHES': p26})

        ref = open(os.path.join(skills, 'rebalance-2022-reference.ts'), encoding='utf-8').read()
        self.groups = []  # (classes, {skill: data})
        for gm in re.finditer(r'\{\s*classes:\s*\[([^\]]*)\][^{]*skills:\s*\{', ref):
            classes = re.findall(r'ClassName\.(\w+)', gm.group(1))
            body = ref[gm.end() - 1:match_brace(ref, gm.end() - 1) + 1]
            data = {}
            for sm in re.finditer(r"'([^']+)'\s*:\s*(\{[^{}]*\})", body):
                data[sm.group(1)] = json.loads(sm.group(2))
            self.groups.append((classes, data))
        self.reference = {}
        for _classes, data in self.groups:
            for k, v in data.items():
                self.reference.setdefault(k, v)

    def _resolve_table(self, items, tables):
        out = {}
        for item in items:
            if item.startswith('...'):
                out.update(tables[item[3:].strip()])
                continue
            m = re.match(r"'([^']+)'\s*:\s*(.*)$", item, re.S)
            if not m:
                continue
            name, value = m.group(1), m.group(2).strip()
            ref = re.fullmatch(r"(\w+)\['([^']+)'\]", value)
            if ref:
                base = tables.get(ref.group(1), out if ref.group(1) == 'REBALANCE_2022_PATCHES' else {})
                if ref.group(2) in base:
                    out[name] = dict(base[ref.group(2)])
                continue
            if not value.startswith('{'):
                continue
            spreads, fields = object_fields(value)
            merged = {}
            for sp in spreads:
                ref = re.fullmatch(r"(\w+)\['([^']+)'\]", sp)
                if ref and ref.group(2) in tables.get(ref.group(1), {}):
                    merged.update(tables[ref.group(1)][ref.group(2)])
            merged.update(fields)
            out[name] = merged
        return out

    def apply(self, name, fields):
        """Patched copy of a class skill's fields, or None when not patched.
        Patched fields are marked so they are translated with the DSL."""
        ref, patch = self.reference.get(name), self.patches.get(name)
        if not ref or not patch:
            return None
        f = dict(fields)
        f['__dsl__'] = set()
        f['isMatk'] = 'true' if ref.get('isMatk') else 'false'
        f['totalHit'] = str(ref.get('totalHit', 1))
        for k, v in patch.items():
            f[k] = v
            f['__dsl__'].add(k)
        if patch.get('canCri', '').strip() not in ('', 'false'):
            f['criDmgPercentage'] = '0.5'
        return f

    def extra_skills(self, stem, existing):
        """Reference skills for this class missing from its file (pushed)."""
        out = []
        for classes, data in self.groups:
            if stem not in classes:
                continue
            for name, ref in data.items():
                if name in existing:
                    continue
                patch = self.patches.get(name)
                if not patch or 'formula' not in patch:
                    continue
                el = ref.get('element')
                f = {'name': f"'{name}'", 'formula': patch['formula'], '__dsl__': {'formula'},
                     'isMatk': 'true' if ref.get('isMatk') else 'false',
                     'isMelee': 'true' if ref.get('isMelee') else 'false',
                     'totalHit': str(ref.get('totalHit', 1))}
                if el:
                    f['element'] = 'ElementType.' + el[0].upper() + el[1:]
                for k, v in patch.items():
                    if k != 'formula':
                        f[k] = v
                        f['__dsl__'].add(k)
                out.append((name, f))
        return out


TRAITS = {'pow': 'pow', 'spl': 'spl', 'con': 'con'}


def expand_linear(src):
    """linear(base, perLevel, 'trait', coef) -> an equivalent arrow function."""
    m = re.fullmatch(r"linear\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*'(pow|spl|con)'\s*,\s*([\d.]+)\s*\)", src.strip())
    if not m:
        return None
    b, p, t, c = m.groups()
    return f'i => scale(i, {b} + lv(i) * {p} + {t}(i) * {c})'
