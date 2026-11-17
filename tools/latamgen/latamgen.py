"""Generate internal/calc/latam_gen.go from latam-ro-calc's job files.

latam-ro-calc (Adson Leal, MIT License, https://github.com/adsonpleal/latam-ro-calc)
holds LATAM skill formulas measured against real recordings. This tool
translates each formula into Go (see jsx.py) and maps skill/class names to the
RO LATAM client's codes using our catalog. Anything it cannot translate is
listed in the report instead of being guessed.

Usage (from the repo root, with a clone in tmp/latam-ro-calc):
    python -I tools/latamgen/latamgen.py [path/to/latam-ro-calc]
"""
import difflib, glob, gzip, json, os, re, sys

sys.path.insert(0, os.path.dirname(__file__))
from jsx import Unsupported, WEAPONS, go_str, translate_function  # noqa: E402
from filectx import FileCtx  # noqa: E402
from rebalance import Rebalance, RebalanceCtx, expand_linear  # noqa: E402

HERE = os.path.dirname(__file__)
TAB = chr(9)
ROOT = os.path.abspath(os.path.join(HERE, '..', '..'))
SRC = sys.argv[1] if len(sys.argv) > 1 else os.path.join(ROOT, 'tmp', 'latam-ro-calc')
JOBS = os.path.join(SRC, 'src', 'app', 'jobs')
OUT = os.path.join(ROOT, 'internal', 'calc', 'latam_gen.go')
REPORT = os.path.join(ROOT, 'tools', 'latamgen', 'REPORT.md')

# Class files whose name doesn't match the client's English job name.
JOB_ALIASES = {
    'Doram': ['JT_DO_SUMMONER'], 'Taekwondo': ['JT_TAEKWON'], 'SoulLinker': ['JT_LINKER'],
    'StarGladiator': ['JT_STAR'], 'SuperNovice': ['JT_SUPERNOVICE', 'JT_SUPERNOVICE2'],
    'Kagerou': ['JT_KAGEROU'], 'Oboro': ['JT_OBORO'], 'Mage': ['JT_MAGICIAN'],
    'HighWizard': ['JT_WIZARD_H'], 'HighPriest': ['JT_PRIEST_H'], 'Whitesmith': ['JT_BLACKSMITH_H'],
    'Creator': ['JT_ALCHEMIST_H'], 'Paladin': ['JT_CRUSADER_H'], 'Champion': ['JT_MONK_H'],
    'Professor': ['JT_SAGE_H'], 'Stalker': ['JT_ROGUE_H'], 'AssassinCross': ['JT_ASSASSIN_H'],
    'Sniper': ['JT_HUNTER_H'], 'LordKnight': ['JT_KNIGHT_H'], 'Clown': ['JT_BARD_H'], 'Gypsy': ['JT_DANCER_H'],
    'Minstrel': ['JT_MINSTREL', 'JT_MINSTREL_H'], 'Wanderer': ['JT_WANDERER', 'JT_WANDERER_H'],
    'Genetic': ['JT_GENETIC', 'JT_GENETIC_H'], 'ArchBishop': ['JT_ARCHBISHOP', 'JT_ARCHBISHOP_H'],
    'GuillotineCross': ['JT_GUILLOTINE_CROSS', 'JT_GUILLOTINE_CROSS_H'],  # client: "Glt. Cross"
    'Shiranui': ['JT_SHIRANUI'],  # no name in the client
    'Trouvere': ['JT_TROUVERE'],  # client: "Diva"
}
# Skill names that differ between latam-ro-calc and the client's English names.
SKILL_ALIASES = {
    'Hack and Slasher': 'DK_HACKANDSLASHER',
    'Fatal Shadow Claw': 'SHC_FATAL_SHADOW_CROW',
    'Unlucky Rush': 'ABC_UNLUCKY_RUSH', 'Rain of Crystal': 'AG_RAIN_OF_CRYSTAL', 'Framen': 'CD_FRAMEN',
    'Palm Strike': 'CH_PALMSTRIKE', 'Tiger Fist': 'CH_TIGERFIST', 'Rapid Shower': 'GS_RAPIDSHOWER',
    'Dust': 'GS_DUST', 'Spread Shot': 'GS_SPREADATTACK', 'Third Punish': 'IQ_THIRD_PUNISH',
    'Head Crush': 'LK_HEADCRUSH', 'Shield Chain': 'PA_SHIELDCHAIN', 'Bomb Cluster': 'RA_CLUSTERBOMB',
    'Dragon Breath - WATER': 'RK_DRAGONBREATH_WATER', 'Infiltration': 'SS_SHIMIRU',
    'Mandala of the Beasts': 'SOA_CIRCLE_OF_DIRECTIONS_AND_ELEMENTALS', 'Fatal Manace': 'SC_FATALMENACE',
    'Asura Strike': 'MO_EXTREMITYFIST', 'Lion Howling': 'SR_HOWLINGOFLION', 'Hell Gate': 'SR_GATEOFHELL',
}


def state_defs(src, fctx):
    """name -> (label, choices) from Active/Passive skill lists; choices
    empty means an on/off toggle."""
    from tsparse import array_items, top_fields
    out = {}
    for m in re.finditer(r'(?:Active|Passive)SkillModel\[\]\s*=\s*\[', src):
        for e in array_items(src, m.end() - 1):
            f = top_fields(e)
            name = (f.get('name') or '').strip('\'"`')
            if not name or name in out:
                continue
            label = (f.get('label') or '').strip('\'"`') or name
            dd = (f.get('dropdown') or '').strip()
            choices = []
            g = re.match(r'genSkillList(?:WithLabel)?\(\s*(\d+)', dd)
            if g:
                choices = [('0', '—')] + [(str(i), str(i)) for i in range(1, int(g.group(1)) + 1)]
            elif dd.startswith('['):
                for item in array_items(dd, 0):
                    it = top_fields(item)
                    v = (it.get('value') or '').strip()
                    lab = (it.get('label') or '').strip('\'"`')
                    if re.fullmatch(r'-?\d+(\.\d+)?', v):
                        num = v
                    elif v in ('true', 'false'):
                        num = '1' if v == 'true' else '0'
                    elif '.' in v:
                        num = fctx.enum_value(*v.split('.', 1)) if fctx else None
                        if num is None:
                            choices = []
                            break
                    else:
                        choices = []
                        break
                    choices.append((num, lab or num))
                if choices and all(c[0] in ('0', '1') for c in choices) and len(choices) <= 2:
                    choices = []  # plain on/off
            out[name] = (label, choices)
    return out


def norm(s):
    return re.sub(r'[^a-z0-9]', '', (s or '').lower())


def load_catalog():
    with gzip.open(os.path.join(ROOT, 'data', 'gen', 'catalog.json.gz'), 'rt', encoding='utf-8') as f:
        return json.load(f)


def main():
    from tsparse import atk_lists, top_fields  # noqa: E402
    cat = load_catalog()
    jobs, skills = cat['jobs'], cat['skills']
    by_code = {j['code']: j for j in jobs.values()}
    job_by_name = {}
    for j in jobs.values():
        job_by_name.setdefault(norm(j['name'].get('en')), []).append(j['code'])

    def chain(job):
        out, seen = [], set()
        while job and job['id'] not in seen:
            seen.add(job['id'])
            out.append(job)
            job = jobs.get(str(job['parent']))
        return out

    def tree_skills(codes):
        ids = set()
        for code in codes:
            for j in chain(by_code[code]):
                ids.update(j['tree'].values())
        return [skills[str(i)] for i in ids if str(i) in skills]

    gen, report, states, helpers, choices = [], [], {}, [], {}
    rb = Rebalance(SRC)
    all_defs = {}
    rctx = RebalanceCtx(SRC)
    patched_count = 0
    stats = {'ok': 0, 'partial': 0, 'skipped': 0}
    for path in sorted(glob.glob(os.path.join(JOBS, '*.ts'))):
        stem = os.path.basename(path)[:-3]
        if stem.endswith('.spec') or stem.startswith('_') or '.' in stem:
            continue
        src = open(path, encoding='utf-8').read()
        entries = atk_lists(src)
        if not entries:
            continue
        codes = JOB_ALIASES.get(stem) or job_by_name.get(norm(re.sub(r'(?<!^)([A-Z])', r' \1', stem)))
        if not codes:
            report.append(f'- **{stem}**: class not found in the client catalog ({len(entries)} skills skipped)')
            stats['skipped'] += len(entries)
            continue
        candidates = tree_skills(codes)
        file_states = {'toggle': set(), 'level': set()}
        fctx = FileCtx(stem, src)
        for k, v in state_defs(src, fctx).items():
            all_defs.setdefault(k, v)
        todo = []
        for e in entries:
            f = top_fields(e)
            name = (f.get('name') or '').strip('\'"`')
            if not name or 'formula' not in f:
                continue
            patched = rb.apply(name, f)
            if patched is not None:
                patched_count += 1
                f = patched
            todo.append((name, f))
        for name, f in rb.extra_skills(stem, {n for n, _ in todo}):
            patched_count += 1
            todo.append((name, f))
        for name, f in todo:
            code = SKILL_ALIASES.get(name)
            if not code:
                exact = [s for s in candidates if norm(s['name'].get('en')) == norm(name)]
                if not exact:
                    exact = [s for s in skills.values() if norm(s['name'].get('en')) == norm(name)]
                if not exact:
                    names = {norm(s['name'].get('en')): s for s in candidates}
                    close = difflib.get_close_matches(norm(name), list(names), n=1, cutoff=0.88)
                    exact = [names[close[0]]] if close else []
                if not exact:
                    report.append(f'- {stem} · **{name}**: skill not found in the client')
                    stats['skipped'] += 1
                    continue
                code = exact[0]['code']
            try:
                lit, ctx_notes = build_literal(code, name, f, fctx, rctx)
            except (Unsupported, IndexError, KeyError, ValueError) as ex:
                report.append(f'- {stem} · **{name}** ({code}): not translated: {type(ex).__name__} {ex}')
                stats['skipped'] += 1
                continue
            for kind, vals in ctx_notes['states'].items():
                file_states[kind].update(vals)
            if ctx_notes['partial']:
                stats['partial'] += 1
                report.append(f'- {stem} · {name} ({code}): partial: {ctx_notes["partial"]}')
            else:
                stats['ok'] += 1
            for jc in codes:
                gen.append(f'\tregJob({go_str(jc)}, {lit})')
        helpers.extend(fctx.go)
        fctx.go = []
        for group, names in fctx.choices.items():
            for jc in codes:
                choices.setdefault(jc, {})[group] = names
        if file_states['toggle'] or file_states['level']:
            for jc in codes:
                states.setdefault(jc, {'toggle': set(), 'level': set()})
                states[jc]['toggle'] |= file_states['toggle']
                states[jc]['level'] |= file_states['level']

    helpers.extend(rctx.go)
    write_go(gen, states, skills, helpers, choices, all_defs)
    with open(REPORT, 'w', encoding='utf-8') as fh:
        fh.write('# latamgen report\n\n')
        fh.write(f'Translated: {stats["ok"]} · partial: {stats["partial"]} · not translated: {stats["skipped"]}\n\n')
        fh.write('\n'.join(report) + '\n')
    print(f'translated {stats["ok"]}, partial {stats["partial"]}, skipped {stats["skipped"]} '
          f'({patched_count} with the 2022 rebalance) -> {OUT}')


def fn_body(src, kind='num', file=None, weapon_first=False):
    """Go func literal from a TS arrow function."""
    lines, ctx = translate_function(src, file, weapon_first)
    body = '\n\t\t\t'.join(lines)
    if kind == 'num':
        return f'func(in FormulaInput) float64 {{\n\t\t\t{body}\n\t\t}}', ctx
    if kind == 'int':
        return f'func(in FormulaInput) int {{ return int((func() float64 {{\n\t\t\t{body}\n\t\t}})()) }}', ctx
    return f'func(in FormulaInput) bool {{ return (func() float64 {{\n\t\t\t{body}\n\t\t}})() != 0 }}', ctx


def collect(ctx, notes):
    notes['states']['toggle'].update(ctx.active)
    notes['states']['level'].update(ctx.active_lv)


def build_literal(code, name, f, fctx=None, rctx=None):
    notes = {'states': {'toggle': set(), 'level': set()}, 'partial': ''}
    parts = [f'Code: {go_str(code)}', f'Name: {go_str(name)}']
    dsl = f.get('__dsl__', set())
    ctx_for = lambda key: rctx if key in dsl else fctx  # noqa: E731
    formula = f['formula']
    if 'formula' in dsl and re.fullmatch(r'[A-Za-z_]\w*', formula.strip()):
        formula = f'i => {formula.strip()}(i)'  # a helper passed as the formula
    if 'formula' in dsl and formula.strip().startswith('linear('):
        formula = expand_linear(formula)
        if formula is None:
            raise Unsupported('linear() form')
    if dsl:
        parts.append('Rebalanced: true')
    ratio, ctx = fn_body(formula, file=ctx_for('formula'))
    collect(ctx, notes)
    parts.append(f'Ratio: {ratio}')
    if f.get('isMatk', '').strip() == 'true':
        parts.append('Magic: true')
    el = f.get('element', '').strip()
    if el.startswith('ElementType.'):
        parts.append(f'Element: {go_str(el.split(".", 1)[1])}')
    elif el:
        raise Unsupported(f'element {el}')
    if 'getElement' in f:
        notes['partial'] = 'element chosen by the skill at runtime (uses the weapon element here)'
    th = f.get('totalHit', '').strip()
    if th:
        if re.fullmatch(r'\d+', th):
            if th != '1':
                parts.append(f'Hits: constHits({th})')
        else:
            body, ctx = fn_body(th, 'int', ctx_for('totalHit'))
            collect(ctx, notes)
            parts.append(f'Hits: {body}')
    cc = f.get('canCri', '').strip()
    if cc == 'true':
        parts.append('CanCrit: always')
    elif cc and cc != 'false':
        body, ctx = fn_body(cc, 'bool', ctx_for('canCri'))
        collect(ctx, notes)
        parts.append(f'CanCrit: {body}')
    for key, field in (('criDmgPercentage', 'CritDmg'), ('baseCriPercentage', 'CritRate')):
        if f.get(key):
            parts.append(f'{field}: {float(f[key])}')
    for key, field in (('forceCri', 'ForceCrit'), ('isIgnoreDef', 'IgnoreDef'), ('isIgnoreSDef', 'IgnoreSoftDef'),
                       ('isIgnoreRes', 'IgnoreRes')):
        if f.get(key, '').strip() == 'true':
            parts.append(f'{field}: true')
    mel = f.get('isMelee', '').strip()
    if mel == 'true':
        parts.append('Melee: meleeAlways')
    elif mel == 'false':
        parts.append('Melee: meleeNever')
    elif mel:
        body, ctx = fn_body(mel, 'bool', ctx_for('isMelee'), weapon_first=True)
        parts.append(f'Melee: {body}')
    vf = f.get('verifyItemFn', '')
    if vf:
        req = re.findall(r"'([A-Za-z]+)'", vf)
        aura = re.search(r"requiresAura\('([^']+)'\)", vf)
        if aura:
            parts.append(f'RequiresState: {go_str(aura.group(1))}')
            notes['states']['toggle'].add(aura.group(1))
        weps = [WEAPONS[r] for r in req if r in WEAPONS]
        if weps:
            parts.append('Weapons: []string{' + ', '.join(go_str(w) for w in weps) + '}')
    if f.get('maxStack'):
        parts.append(f'Stack: [2]int{{{int(f.get("minStack", "1") or 1)}, {int(f["maxStack"])}}}')
    extra = [k for k in ('part2', 'finalDmgFormula', 'flatDamage', 'customFormula', 'damageRepeat', 'currentHpFn', 'currentSpFn') if k in f]
    if extra:
        notes['partial'] = (notes['partial'] + '; ' if notes['partial'] else '') + 'not simulated: ' + ', '.join(extra)
        parts.append(f'Partial: {go_str(", ".join(extra))}')
    return '&SkillFormula{\n\t\t' + ',\n\t\t'.join(parts) + ',\n\t}', notes


def write_go(gen, states, skills, helpers, choices, defs):
    en_to_pt = {norm(s['name'].get('en')): s['name'].get('pt') or s['name'].get('en') for s in skills.values()}
    lines = [
        '// Code generated by tools/latamgen from latam-ro-calc; DO NOT EDIT.',
        '//',
        '// Skill formulas measured against LATAM recordings by latam-ro-calc',
        '// (Copyright (c) 2026 Adson Leal, MIT License,',
        '// https://github.com/adsonpleal/latam-ro-calc), translated to Go.',
        '',
        'package calc',
        '',
        'import "math"',
        '',
        'var _ = math.Floor',
        '',
        'func init() {',
    ]
    lines.extend(gen)
    for jc in sorted(states):
        opts = []
        for name in sorted(states[jc]['toggle'] | states[jc]['level']):
            label, ch = defs.get(name, (None, None))
            if label is None or label == name:
                label = en_to_pt.get(norm(name), name.strip('_').replace('_', ' '))
            if ch:
                cs = ', '.join('{' + go_str(v) + ', ' + go_str(l) + '}' for v, l in ch)
                opts.append(f'{{Key: stateKey({go_str(name)}), Label: {go_str(label)}, Choices: [][2]string{{{cs}}}}}')
            elif name in states[jc]['level']:
                opts.append(f'{{Key: stateKey({go_str(name)}), Label: {go_str(label)}, Choices: levelChoices(10)}}')
            else:
                opts.append(f'{{Key: stateKey({go_str(name)}), Label: {go_str(label)}}}')
        lines.append(f'\tregStates({go_str(jc)}, []StateOption{{\n\t\t' + ',\n\t\t'.join(opts) + ',\n\t})')
    for jc in sorted(choices):
        for group, names in choices[jc].items():
            opts = ', '.join('{' + go_str(str(i + 1)) + ', ' + go_str(n) + '}' for i, n in enumerate(names))
            label = {'spirit': 'Espírito'}.get(group, group)
            lines.append(TAB + 'regStates(' + go_str(jc) + ', []StateOption{{Key: "c_' + group + '", Label: '
                         + go_str(label) + ', Choices: [][2]string{{"0", "—"}, ' + opts + '}}})')
    lines.append('}')
    lines.extend([''] + helpers)
    with open(OUT, 'w', encoding='utf-8') as fh:
        fh.write('\n'.join(lines) + '\n')


if __name__ == '__main__':
    main()
