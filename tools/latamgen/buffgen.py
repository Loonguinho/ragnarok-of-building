"""Generate internal/calc/latam_buffs_gen.go: each class's own buffs (the
active skill lists of latam-ro-calc's job files) with the bonuses they give.

latam-ro-calc (Adson Leal, MIT License) lists, per class, the self-buffs a
player turns on (Encantar Lâmina, Postura de Ataque...) and the bonuses of
each level, from bROWiki and LATAM measurements. Only bonuses the simulator
computes are translated (stats, ATK/MATK, P.ATK/S.MATK, crit, ASPD, damage
modifiers...); the rest is listed in tools/latamgen/BUFFS.md instead of
being guessed. The buffs share their key with the formula states
(stateKey(name)), so one toggle drives both.

Usage (from the repo root, with a clone in tmp/latam-ro-calc):
    python -I tools/latamgen/buffgen.py [path/to/latam-ro-calc]
"""
import glob, os, re, sys

sys.path.insert(0, os.path.dirname(__file__))
from latamgen import JOB_ALIASES, SKILL_ALIASES, load_catalog, norm  # noqa: E402
from tsparse import array_items, top_fields, match_brace  # noqa: E402

HERE = os.path.dirname(__file__)
ROOT = os.path.abspath(os.path.join(HERE, '..', '..'))
SRC = sys.argv[1] if len(sys.argv) > 1 else os.path.join(ROOT, 'tmp', 'latam-ro-calc')
JOBS = os.path.join(SRC, 'src', 'app', 'jobs')
OUT = os.path.join(ROOT, 'internal', 'calc', 'latam_buffs_gen.go')
REPORT = os.path.join(HERE, 'BUFFS.md')

ELES = {'neutral': 'Neutral', 'water': 'Water', 'earth': 'Earth', 'fire': 'Fire', 'wind': 'Wind', 'poison': 'Poison',
        'holy': 'Holy', 'dark': 'Dark', 'ghost': 'Ghost', 'undead': 'Undead', 'all': 'All'}
RACES = {'all': 'All', 'formless': 'Formless', 'undead': 'Undead', 'brute': 'Brute', 'plant': 'Plant', 'insect': 'Insect',
         'fish': 'Fish', 'demon': 'Demon', 'demihuman': 'DemiHuman', 'human': 'DemiHuman', 'angel': 'Angel', 'dragon': 'Dragon'}
SIMPLE = {
    'str': 'bStr', 'agi': 'bAgi', 'vit': 'bVit', 'int': 'bInt', 'dex': 'bDex', 'luk': 'bLuk', 'allStatus': 'bAllStats',
    'pow': 'bPow', 'sta': 'bSta', 'wis': 'bWis', 'spl': 'bSpl', 'con': 'bCon', 'crt': 'bCrt',
    'hit': 'bHit', 'flee': 'bFlee', 'cri': 'bCritical', 'criDmg': 'bCritAtkRate', 'cRate': 'bCRate',
    'aspd': 'bAspd', 'aspdPercent': 'bAspdRate', 'atk': 'bAtk', 'x_atk': 'bAtk', 'matk': 'bMatk',
    'pAtk': 'bPAtk', 'sMatk': 'bSMatk', 'def': 'bDef', 'mdef': 'bMdef', 'res': 'bRes', 'mres': 'bMRes',
    'hp': 'bMaxHP', 'sp': 'bMaxSP', 'hpPercent': 'bMaxHPrate', 'spPercent': 'bMaxSPrate',
    'atkPercent': 'bAtkRate', 'matkPercent': 'bMatkRate', 'range': 'bLongAtkRate', 'melee': 'bShortAtkRate',
}


def script_for(key, v, skill_code):
    """rAthena script for one latam-ro-calc bonus key, or None."""
    if key in SIMPLE:
        return f'bonus {SIMPLE[key]},{v};'
    m = re.fullmatch(r'(p|m)_(my_)?element_(\w+)', key)
    if m and m.group(3) in ELES:
        e = ELES[m.group(3)]
        if m.group(2):  # the attacker's own element (magic)
            return f'bonus2 bMagicAtkEle,Ele_{e},{v};' if m.group(1) == 'm' else None
        return f'bonus2 {"bAddEle" if m.group(1) == "p" else "bMagicAddEle"},Ele_{e},{v};'
    m = re.fullmatch(r'(p|m)_race_(\w+)', key)
    if m and m.group(2) in RACES:
        return f'bonus2 {"bAddRace" if m.group(1) == "p" else "bMagicAddRace"},RC_{RACES[m.group(2)]},{v};'
    if key == 'p_size_all':
        return f'bonus2 bAddSize,Size_All,{v};'
    if key == 'p_pene_class_all':
        return f'bonus2 bIgnoreDefClassRate,Class_All,{v};'
    if key in ('p_pene_race_all', 'm_pene_race_all'):
        return f'bonus2 {"bIgnoreDefRaceRate" if key[0] == "p" else "bIgnoreMdefRaceRate"},RC_All,{v};'
    code = skill_code(key)
    if code:  # "+N% damage" of one skill
        return f'bonus2 bSkillAtk,"{code}",{v};'
    return None


NUM = re.compile(r'[-\d\s+*/().]+')


def ev(expr, var=None, val=None):
    if var:
        expr = re.sub(rf'\b{var}\b', str(val), expr)
    expr = expr.strip()
    if not NUM.fullmatch(expr):
        return None
    try:
        return int(eval(expr, {'__builtins__': {}}))  # digits and + - * / ( ) only
    except Exception:
        return None


def bonus_pairs(body):
    """'atk: 8 * 2, "Rampage Blast": 30' -> [(key, expr)]."""
    out = []
    for kv in re.finditer(r"""(['"]?)([A-Za-z_][\w ]*?|[^'":,]+?)\1\s*:\s*([^,]+)""", body):
        out.append((kv.group(2).strip(), kv.group(3).strip()))
    return out


def options(dd):
    """[(value, label, [(key, expr)], var, varval)] for a dropdown source."""
    out = []
    i = dd.find('[')
    if i < 0:
        return out
    body = dd[i + 1:match_brace(dd, i)]
    for m in re.finditer(r'\.\.\.Array\.from\(\{\s*length:\s*(\d+)\s*\},\s*\(\s*_\s*,\s*(\w+)\s*\)\s*=>\s*\(\{', body):
        start = m.end() - 1
        obj = body[start:match_brace(body, start) + 1]
        f = top_fields(obj)
        b = re.search(r'bonus:\s*\{([^{}]*)\}', obj)
        for k in range(int(m.group(1))):
            v = ev(f.get('value') or '', m.group(2), k)
            if v is None:
                continue
            out.append((v, f'Nv. {v}', bonus_pairs(b.group(1)) if b else [], m.group(2), k))
    for item in array_items('[' + body + ']', 0):
        f = top_fields(item)
        if (f.get('isUse') or '').strip() == 'false':
            continue
        v = ev(f.get('value') or '')
        if v is None or v == 0:
            continue
        lab = (f.get('label') or '').strip('\'"`')
        b = re.search(r'bonus:\s*\{([^{}]*)\}', item)
        out.append((v, lab or str(v), bonus_pairs(b.group(1)) if b else [], None, None))
    return out


def go_str(s):
    return '"' + s.replace('\\', '\\\\').replace('"', '\\"') + '"'


def main():
    cat = load_catalog()
    jobs, skills = cat['jobs'], cat['skills']
    job_by_name = {}
    for j in jobs.values():
        job_by_name.setdefault(norm(j['name'].get('en')), []).append(j['code'])
    by_en = {}
    for s in skills.values():
        by_en.setdefault(norm(s['name'].get('en')), s['code'])

    def skill_code(name):
        return SKILL_ALIASES.get(name) or by_en.get(norm(name))

    gen, report, total, kept = [], [], 0, 0
    for path in sorted(glob.glob(os.path.join(JOBS, '*.ts'))):
        stem = os.path.basename(path)[:-3]
        if stem.endswith('.spec') or stem.startswith('_') or '.' in stem:
            continue
        src = open(path, encoding='utf-8').read()
        codes = JOB_ALIASES.get(stem) or job_by_name.get(norm(re.sub(r'(?<!^)([A-Z])', r' \1', stem)))
        if not codes:
            continue
        defs = []
        for m in re.finditer(r'ActiveSkillModel\[\]\s*=\s*\[', src):
            for e in array_items(src, m.end() - 1):
                f = top_fields(e)
                name = (f.get('name') or '').strip('\'"`')
                if not name:
                    continue
                opts = options(f.get('dropdown') or '')
                if not any(o[2] for o in opts):
                    continue  # no bonus: a formula state only
                total += 1
                scripts, missing = {}, set()
                for v, lab, pairs, var, val in opts:
                    parts = []
                    for k, expr in pairs:
                        n = ev(expr, var, val)
                        sc = script_for(k, n, skill_code) if n is not None else None
                        if sc is None:
                            missing.add(k)
                        elif n != 0:
                            parts.append(sc)
                    if parts:
                        scripts[v] = ' '.join(parts)
                code = skill_code(name) or ''
                label = (f.get('label') or '').strip('\'"`') or name
                if missing:
                    report.append(f'- {stem} · **{label}** ({code or name}): not computed: {", ".join(sorted(missing))}')
                if not scripts:
                    continue
                kept += 1
                toggle = len(opts) == 1
                choices = [('0', '—')] + [(str(v), lab) for v, lab, *_ in opts]
                ch = ', '.join(f'{{{go_str(a)}, {go_str(b)}}}' for a, b in choices)
                sc = ', '.join(f'{k}: {go_str(s)}' for k, s in sorted(scripts.items()))
                defs.append(f'\t\t{{Name: {go_str(name)}, Skill: {go_str(code)}, Label: {go_str(label)}, '
                            f'Toggle: {str(toggle).lower()}, Choices: [][2]string{{{ch}}}, Scripts: map[int]string{{{sc}}}}},')
        if defs:
            for jc in codes:
                gen.append(f'\tregClassBuffs({go_str(jc)}, []ClassBuff{{\n' + '\n'.join(defs) + '\n\t})')
    with open(OUT, 'w', encoding='utf-8', newline='\n') as fh:
        fh.write('// Code generated by tools/latamgen/buffgen.py from latam-ro-calc (MIT); DO NOT EDIT.\n\n')
        fh.write('package calc\n\nfunc init() {\n' + '\n'.join(gen) + '\n}\n')
    with open(REPORT, 'w', encoding='utf-8', newline='\n') as fh:
        fh.write('# Class buffs (buffgen)\n\n')
        fh.write(f'Buffs with bonuses: {total} · with at least one computed bonus: {kept}\n\n')
        fh.write('Bonuses the simulator does not compute (kept out):\n\n' + '\n'.join(report) + '\n')
    print(f'class buffs: {kept} of {total} -> {OUT}')


if __name__ == '__main__':
    main()
