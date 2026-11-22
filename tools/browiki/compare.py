"""Compare the simulator's generated skill ratios with bROWiki's per-level
tables (base level 100, no stats/traits/masteries).

    DUMP_RATIOS=../../tmp/ours.json go test -run TestDumpRatios ./internal/calc
    python -I tools/browiki/compare.py tmp/ours.json
Writes tools/browiki/COMPARE.md.
"""
import json, os, re, sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..'))


# Differences checked by hand: why the simulator keeps its value.
TYPO = 'erro na tabela do wiki (contradiz a fórmula da própria página)'
OLD = 'tabela do wiki anterior ao cliente de 28/10/2026; vale o rebalanceamento medido pelo latam-ro-calc'
REVIEWED = {
    'CD_FRAMEN': TYPO + ': Nv4-5 da coluna Normal repetem a coluna Bônus',
    'GS_RAPIDSHOWER': TYPO + ' (Nv9)', 'HN_MEGA_SONIC_BLOW': TYPO + ' (Nv6)', 'SP_SPA': TYPO + ' (Nv7)',
    'SR_DRAGONCOMBO': TYPO + ' (Nv4-6)', 'SS_ANTENPOU': TYPO + ' (Nv3)', 'WL_HELLINFERNO': TYPO + ' (Nv5)',
    'DK_DRAGONIC_AURA': OLD + ' (tabela de 06/2026)', 'SJ_NEWMOONKICK': OLD + ' (tabela de 2022)',
    'LG_RAYOFGENESIS': OLD + ' (tabela de 06/2026)', 'LG_BANISHINGPOINT': OLD + ' (tabela de 06/2026)',
    'SP_SWHOO': OLD + ' (tabela de 2024)', 'KO_HAPPOKUNAI': OLD + ' (tabela de 2024)',
    'WL_CHAINLIGHTNING': OLD + ' (tabela de 2019); o simulador soma o bônus das correntes',
    'GC_COUNTERSLASH': 'a tabela lista o dano base sem o ÷120 da fórmula; a fórmula bate',
    'NC_AXEBOOMERANG': 'depende do peso do machado (a comparação roda sem arma); a fórmula bate',
    'WL_FROSTMISTY': 'a 1ª coluna não é o ATQM; a fórmula por cristal bate',
    'HN_SPIRAL_PIERCE_MAX': 'a tabela é o dano base antes do multiplicador de tamanho (×1,3 no médio da comparação); a fórmula bate',
}


# Attribute coefficients checked by hand.
STATE = 'o 2º coeficiente vale com o estado {}, que o simulador aplica'
COEF_REVIEWED = {
    'SHC_SAVAGE_IMPACT': STATE.format('Carrasco Sombrio'), 'SHC_ETERNAL_SLASH': STATE.format('Carrasco Sombrio'),
    'EM_CONFLAGRATION': STATE.format('Elemental ativo'), 'EM_DIAMOND_STORM': STATE.format('Elemental ativo'),
    'EM_LIGHTNING_LAND': STATE.format('Elemental ativo'), 'EM_TERRA_DRIVE': STATE.format('Elemental ativo'),
    'EM_VENOM_SWAMP': STATE.format('Elemental ativo'), 'CD_FRAMEN': 'FEI × 5 contra Demônio e Morto-vivo, que o simulador aplica',
    'SC_FATALMENACE': STATE.format('Ameaça'), 'LG_HESPERUSLIT': STATE.format('Consagração'),
    'NC_AXETORNADO': STATE.format('Esmagador'),
    'LG_RAYOFGENESIS': 'página anterior ao cliente de 28/10/2026; vale o rebalanceamento medido pelo latam-ro-calc (INT × 2)',
    'RA_CLUSTERBOMB': 'INT × 5 está na fórmula, multiplicado pelo nível de Perícia com Armadilhas (0 na comparação)',
    'TR_METALIC_FURY': 'FEI × 1,5 multiplicado pelo nível de Presença de Palco (0 na comparação)',
    'TR_RHYTHMSHOOTING': 'CON × 3 multiplicado pelo nível de Presença de Palco (0 na comparação)',
    'TR_ROSEBLOSSOM': 'CON × 3 multiplicado pelo nível de Presença de Palco (0 na comparação)',
    'SJ_PROMINENCEKICK': 'a fórmula com AGI/DES da página é do tempo de recarga, não do dano',
}


def pct(cell):
    """'1.100%' -> 1100.0; None when the cell isn't a single percentage."""
    m = re.fullmatch(r'\s*([\d.]+(?:,\d+)?)\s*%\s*', cell or '')
    if not m:
        return None
    return float(m.group(1).replace('.', '').replace(',', '.'))


def wiki_columns(w):
    """Every column holding a % per level: {index: {lv: value}}."""
    cols = {}
    for lv, cells in w['levels'].items():
        for i, cell in enumerate(cells):
            v = pct(cell)
            if v is not None:
                cols.setdefault(i, {})[int(lv)] = v
    return {i: c for i, c in cols.items() if len(c) >= max(1, len(w['levels']) // 2)}


STAT = re.compile(r'\b(FOR|AGI|VIT|INT|DES|SOR|POD|VIG|SAB|FEI|CON|PRE)(?:\s+base)?\s*[×x]\s*(\d+(?:,\d+)?)\b(?!\s*(?:÷|/|\d))')


def wiki_coefs(formula):
    """{'POD': 7.0, ...}: the 'STAT × k' terms of a formula's text."""
    out = {}
    for name, k in STAT.findall(formula):
        out[name] = out.get(name, 0) + float(k.replace(',', '.'))
    return out


def coef_check(o, w):
    """None when every wiki formula with stat terms matches one of our
    variants; else (wiki terms, ours) of the first that does not."""
    ours = list((o.get('coefs') or {}).values()) or [{}]
    for f in w['formulas']:
        want = wiki_coefs(f)
        if not want:
            continue
        if not any(all(abs(c.get(n, 0) - v) < 0.01 for n, v in want.items()) for c in ours):
            return want, ours
    return None


def matches(col, ratios):
    return all(lv > 10 or abs(ratios[lv - 1] - v) <= 0.5 for lv, v in col.items())


def main():
    ours = json.load(open(sys.argv[1], encoding='utf-8'))
    wiki = json.load(open(os.path.join(ROOT, 'data', 'browiki', 'skills.json'), encoding='utf-8'))
    ok, diff, nodata, missing, reviewed, coefdiff = [], [], [], [], [], []
    for code in sorted(set(ours) | set(wiki)):
        w = wiki.get(code)
        o = ours.get(code)
        if not w or not w['title']:
            nodata.append((code, 'sem página'))
            continue
        cols = wiki_columns(w)
        levels = cols[min(cols)] if cols else {}
        if not levels:
            nodata.append((code, f"{w['title']}: tabela sem % por nível"))
            continue
        if not o:
            missing.append((code, w['title'], levels, w['formulas']))
            continue
        if (bad := coef_check(o, w)) and code not in COEF_REVIEWED:
            coefdiff.append((code, w['title'], bad, w['formulas']))
        ours_all = [o['ratios']] + list(o.get('variants', {}).values()) + list(o.get('races', {}).values())
        if any(matches(c, r) for c in cols.values() for r in ours_all):
            ok.append((code, w['title'], {}, w['formulas']))
            continue
        bad = {lv: (v, o['ratios'][lv - 1]) for lv, v in levels.items()
               if lv <= 10 and abs(o['ratios'][lv - 1] - v) > 0.5}
        if code in REVIEWED:
            ok.append((code, w['title'], {}, [REVIEWED[code]]))
            reviewed.append((code, w['title'], REVIEWED[code]))
            continue
        diff.append((code, w['title'], bad, w['formulas']))
    lines = ['# Simulador × bROWiki', '',
             f'Iguais: {len(ok) - len(reviewed)} · diferença conferida: {len(reviewed)} · diferentes: {len(diff)} · coeficientes diferentes: {len(coefdiff)} · sem fórmula no simulador: {len(missing)} · sem dados no wiki: {len(nodata)}', '',
             '## Diferentes (wiki, simulador) por nível', '']
    for code, title, bad, f in diff:
        cells = ', '.join(f'Nv{lv}: {v:g} × {u:g}' for lv, (v, u) in sorted(bad.items()))
        lines.append(f'- **{title}** ({code}): {cells}')
        for x in f[:2]:
            lines.append(f'  - `{x}`')
    lines += ['', '## Coeficientes de atributos diferentes (wiki × simulador, por ponto)', '']
    for code, title, (want, ours), f in coefdiff:
        lines.append(f'- **{title}** ({code}): wiki {want} × simulador {ours}')
        for x in f[:2]:
            lines.append(f'  - `{x}`')
    lines += ['', '## Coeficientes conferidos à mão', '']
    lines += [f'- {c}: {why}' for c, why in sorted(COEF_REVIEWED.items())]
    lines += ['', '## Diferença conferida (o simulador mantém o seu valor)', '']
    lines += [f'- **{t}** ({c}): {why}' for c, t, why in reviewed]
    lines += ['', '## Sem fórmula no simulador', '']
    for code, title, levels, f in missing:
        lines.append(f'- **{title}** ({code}): ' + ', '.join(f'Nv{k}: {v:g}' for k, v in sorted(levels.items())))
        for x in f[:2]:
            lines.append(f'  - `{x}`')
    lines += ['', '## Sem dados comparáveis no wiki', ''] + [f'- {c}: {why}' for c, why in nodata]
    open(os.path.join(ROOT, 'tools', 'browiki', 'COMPARE.md'), 'w', encoding='utf-8').write('\n'.join(lines) + '\n')
    print(lines[2])


if __name__ == '__main__':
    main()
