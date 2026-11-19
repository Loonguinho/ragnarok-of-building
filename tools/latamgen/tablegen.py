"""Generate internal/clientimport/latam_tables_gen.go: the LATAM (current
rebalance) class tables of latam-ro-calc (MIT), which the import puts over
rAthena's:

  - base HP/SP per class and level (src/assets/demo/data/hp_sp_table.json);
  - stat and trait bonuses per job level (jobBonusTable / traitBonusTable
    of each class file);
  - base ASPD per class and weapon (src/app/jobs/_aspd-table.ts, from the
    bROWiki class pages).

Only what latam-ro-calc lists is replaced; the rest keeps rAthena's values.

Usage (from the repo root, with a clone in tmp/latam-ro-calc):
    python -I tools/latamgen/tablegen.py [path/to/latam-ro-calc]
"""
import gzip, json, os, re, sys

HERE = os.path.dirname(__file__)
ROOT = os.path.abspath(os.path.join(HERE, '..', '..'))
SRC = sys.argv[1] if len(sys.argv) > 1 else os.path.join(ROOT, 'tmp', 'latam-ro-calc')
OUT = os.path.join(ROOT, 'internal', 'clientimport', 'latam_tables_gen.go')

# latam-ro-calc class names that are not the client's job codes (without JT_).
ALIAS = {
    'Mage': 'MAGICIAN', 'Taekwon': 'TAEKWON', 'Taekwondo': 'TAEKWON', 'Doram': 'DO_SUMMONER', 'Summoner': 'DO_SUMMONER',
    'StarGladiator': 'STAR', 'SoulLinker': 'LINKER', 'SuperNovice': 'SUPERNOVICE', 'SuperNoviceE': 'SUPERNOVICE2',
    'Champion': 'MONK_H', 'Clown': 'BARD_H', 'Gypsy': 'DANCER_H', 'Creator': 'ALCHEMIST_H', 'Paladin': 'CRUSADER_H',
    'Professor': 'SAGE_H', 'Scholar': 'SAGE_H', 'Stalker': 'ROGUE_H', 'AssassinCross': 'ASSASSIN_H', 'Sniper': 'HUNTER_H',
    'LordKnight': 'KNIGHT_H', 'HighWizard': 'WIZARD_H', 'HighPriest': 'PRIEST_H', 'Whitesmith': 'BLACKSMITH_H',
    'Troubadour': 'TROUBADOUR', 'Trouvere': 'TROUVERE',
}
WEAPON = {'Dagger': 'Dagger', 'Sword': '1hSword', 'Two-Handed Sword': '2hSword', 'Spear': '1hSpear', 'Two-Handed Spear': '2hSpear',
          'Axe': '1hAxe', 'Two-Handed Axe': '2hAxe', 'Mace': 'Mace', 'Two-Handed Mace': '2hMace', 'Rod': 'Staff',
          'Two-Handed Rod': '2hStaff', 'Bow': 'Bow', 'Fistweapon': 'Knuckle', 'Instrument': 'Musical', 'Whip': 'Whip',
          'Book': 'Book', 'Katar': 'Katar', 'Huuma': 'Huuma', 'Pistol': 'Revolver', 'Rifle': 'Rifle', 'Gatling Gun': 'Gatling',
          'Shotgun': 'Shotgun', 'Grenade Launcher': 'Grenade'}


def load_codes():
    with gzip.open(os.path.join(ROOT, 'data', 'gen', 'catalog.json.gz'), 'rt', encoding='utf-8') as f:
        return {j['code'] for j in json.load(f)['jobs'].values()}


def snake(name):
    return re.sub(r'(?<!^)([A-Z])', r'_\1', name).upper()


def codes_for(name, codes, trans_split):
    """Job codes a latam-ro-calc class name stands for. trans_split: the name
    is the non-transcendent row (its _T row is separate), as in the HP table;
    otherwise (class files, ASPD table) it covers both versions."""
    if re.search(r'(\d|T2)$', name) or name.startswith('Baby') or name in ('Dance', 'SuperBaby', 'SuperBabyE'):
        return []  # mounted sprites and baby classes: not simulated
    trans = False
    base = name
    if name.endswith('_T'):
        base, trans = name[:-2], True
    elif name.endswith('High') and name != 'High':
        base, trans = name[:-4], True
    stem = ALIAS.get(base)
    if stem is None:
        flat = base.upper()
        stem = next((c[3:] for c in sorted(codes) if c[3:].replace('_', '') == flat and not c.endswith('_H')), snake(base))
    if stem.endswith('_H'):
        return [f'JT_{stem}'] if f'JT_{stem}' in codes else []
    plain, upper = f'JT_{stem}', f'JT_{stem}_H'
    if trans:
        return [upper] if upper in codes else []
    out = [plain] if plain in codes else []
    if not trans_split and upper in codes:
        out.append(upper)
    return out


def go_ints(d):
    return '{' + ', '.join(f'{k}: {v}' for k, v in sorted(d.items())) + '}'


def main():
    codes = load_codes()
    hp, sp, jb, tb, aspd, report = {}, {}, {}, {}, {}, []
    rows = json.load(open(os.path.join(SRC, 'src', 'assets', 'demo', 'data', 'hp_sp_table.json'), encoding='utf-8'))
    # Rows in level order: where two ranges meet (Lv200 of a 3rd class row and
    # a 4th class row), the higher range wins.
    rows.sort(key=lambda r: min(int(k) for k in r['baseHp']))
    for row in rows:
        for name in row['jobs']:
            got = codes_for(name, codes, True)
            if not got:
                report.append(f'hp: {name} skipped')
            for c in got:  # a class can be in several rows (level ranges): merge them
                hp.setdefault(c, {}).update({int(k): v for k, v in row['baseHp'].items()})
                sp.setdefault(c, {}).update({int(k): v for k, v in row['baseSp'].items()})
    jobs_dir = os.path.join(SRC, 'src', 'app', 'jobs')
    for f in sorted(os.listdir(jobs_dir)):
        if not f.endswith('.ts') or '.spec' in f or f.startswith('_'):
            continue
        src = open(os.path.join(jobs_dir, f), encoding='utf-8').read()
        for var, dest in (('jobBonusTable', jb), ('traitBonusTable', tb)):
            m = re.search(var + r'[^=]*=\s*\{(.*?)\n\};', src, re.S)
            if not m:
                continue
            table = {int(a): [int(x) for x in b.split(',')] for a, b in re.findall(r'(\d+)\s*:\s*\[([-\d,\s]+)\]', m.group(1))}
            for c in codes_for(f[:-3], codes, False):
                dest[c] = table
    asrc = open(os.path.join(jobs_dir, '_aspd-table.ts'), encoding='utf-8').read()
    for m in re.finditer(r'\[ClassName\.(\w+)\]:\s*\{([^}]*)\}', asrc):
        vals = {a.strip("' "): int(b) for a, b in re.findall(r"('?[\w -]+'?)\s*:\s*(-?\d+)", m.group(2))}
        if 'base' not in vals:
            continue
        # rAthena delays: ASPD = 196 - delay (status_base_amotion_pc renewal).
        delays = {'Fist': 196 - vals['base']}
        if 'shield' in vals:
            delays['Shield'] = -vals['shield']
        for w, d in vals.items():
            if w in WEAPON:
                delays[WEAPON[w]] = 196 - (vals['base'] + d)
        for c in codes_for(m.group(1), codes, False):
            aspd[c] = delays
    with open(OUT, 'w', encoding='utf-8', newline='\n') as fh:
        fh.write('// Code generated by tools/latamgen/tablegen.py from latam-ro-calc (MIT); DO NOT EDIT.\n\n')
        fh.write('package clientimport\n\n')
        fh.write('// latamBaseHP / latamBaseSP: base HP/SP by job code and base level.\n')
        fh.write('var latamBaseHP = map[string]map[int]int{\n')
        for c in sorted(hp):
            fh.write(f'\t"{c}": {go_ints(hp[c])},\n')
        fh.write('}\n\nvar latamBaseSP = map[string]map[int]int{\n')
        for c in sorted(sp):
            fh.write(f'\t"{c}": {go_ints(sp[c])},\n')
        fh.write('}\n\n// latamJobBonus / latamTraitBonus: total STR..LUK / POW..CRT by job level.\n')
        for var, dest in (('latamJobBonus', jb), ('latamTraitBonus', tb)):
            fh.write(f'var {var} = map[string]map[int][6]int{{\n')
            for c in sorted(dest):
                rows = ', '.join(f'{lv}: {{{", ".join(map(str, v))}}}' for lv, v in sorted(dest[c].items()))
                fh.write(f'\t"{c}": {{{rows}}},\n')
            fh.write('}\n\n')
        fh.write('// latamASPD: rAthena-style weapon delays (ASPD = 196 - delay) by job code.\n')
        fh.write('var latamASPD = map[string]map[string]int{\n')
        for c in sorted(aspd):
            fh.write('\t"' + c + '": {' + ', '.join(f'"{w}": {d}' for w, d in sorted(aspd[c].items())) + '},\n')
        fh.write('}\n')
    os.system('gofmt -w "' + OUT + '"')
    print(f'HP/SP {len(hp)} jobs, job bonus {len(jb)}, trait bonus {len(tb)}, ASPD {len(aspd)} -> {OUT}')
    for r in report:
        if 'Baby' not in r:
            print(' ', r)


if __name__ == '__main__':
    main()
