"""Fetch skill pages from bROWiki (https://browiki.org) for the skills the
simulator has formulas for, politely (1 request/second, cached), and extract
the per-level damage table and the formula text.

Pages are found by searching the client skill code and accepted only when
the page's `Sdbname` is exactly that code.

    python -I tools/browiki/fetch.py            # fetch missing pages
Output: data/browiki/<CODE>.json (raw wikitext cache) and
        data/browiki/skills.json (parsed tables), both gitignored.
"""
import json, os, re, sys, time, urllib.parse, urllib.request

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..'))
OUT = os.path.join(ROOT, 'data', 'browiki')
API = 'https://browiki.org/api.php'
UA = 'ragnarok-of-building/0.1 (simulador de builds)'
_last = [0.0]


def get(params):
    wait = 1.0 - (time.time() - _last[0])
    if wait > 0:
        time.sleep(wait)
    _last[0] = time.time()
    url = API + '?' + urllib.parse.urlencode({**params, 'format': 'json'})
    req = urllib.request.Request(url, headers={'User-Agent': UA})
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.load(r)


def codes_wanted():
    gen = open(os.path.join(ROOT, 'internal', 'calc', 'latam_gen.go'), encoding='utf-8').read()
    codes = set(re.findall(r'Code:\s+"([A-Z0-9_]+)"', gen))
    for extra in ('latam_wiki.go', 'latam_wiki_2026.go'):  # formulas written from the wiki
        path = os.path.join(ROOT, 'internal', 'calc', extra)
        if os.path.exists(path):
            codes |= set(re.findall(r'Code:\s+"([A-Z0-9_]+)"', open(path, encoding='utf-8').read()))
    rep = open(os.path.join(ROOT, 'tools', 'latamgen', 'REPORT.md'), encoding='utf-8').read()
    codes |= set(re.findall(r'\(([A-Z][A-Z0-9_]+)\): not translated', rep))
    return sorted(codes)


def fetch(code):
    path = os.path.join(OUT, code + '.json')
    if os.path.exists(path):
        return json.load(open(path, encoding='utf-8'))
    res = get({'action': 'query', 'list': 'search', 'srsearch': f'"{code}"', 'srwhat': 'text', 'srlimit': 5})
    page = None
    for hit in res.get('query', {}).get('search', []):
        d = get({'action': 'parse', 'page': hit['title'], 'prop': 'wikitext', 'redirects': 1})
        wt = d.get('parse', {}).get('wikitext', {}).get('*', '')
        if re.search(r'\|\s*Sdbname\s*=\s*' + re.escape(code) + r'\s*$', wt, re.M):
            page = {'code': code, 'title': d['parse']['title'], 'wikitext': wt}
            break
    page = page or {'code': code, 'title': None, 'wikitext': ''}
    json.dump(page, open(path, 'w', encoding='utf-8'), ensure_ascii=False)
    return page


def clean(cell):
    cell = re.sub(r'\[\[(?:[^|\]]*\|)?([^\]]*)\]\]', r'\1', cell)  # [[link|text]] -> text
    cell = re.sub(r'<[^>]+>', '', cell)
    cell = re.sub(r"'''?", '', cell)
    return cell.strip()


def parse_tables(wt):
    """Wikitables as lists of rows (header row first), cells cleaned."""
    tables = []
    for m in re.finditer(r'\{\|(.*?)\n\|\}', wt, re.S):
        rows, cur = [], []
        for line in m.group(1).split('\n')[1:]:
            line = line.strip()
            if line.startswith('|-'):
                if cur:
                    rows.append(cur)
                cur = []
            elif line.startswith('!') or line.startswith('|'):
                sep = '!!' if line.startswith('!') else '||'
                body = line[1:]
                cells = [c.split('|', 1)[-1] if ('=' in c.split('|', 1)[0] and '|' in c) else c for c in body.split(sep)]
                cur.extend(clean(c) for c in cells)
        if cur:
            rows.append(cur)
        tables.append(rows)
    return tables


def parse(page):
    wt = page['wikitext']
    out = {'code': page['code'], 'title': page['title'], 'formulas': [], 'levels': {}, 'columns': []}
    if not wt:
        return out
    out['formulas'] = [clean(f) for f in re.findall(r'<code>(.*?)</code>', wt, re.S)]
    info = re.search(r'\{\{Skill Info(.*?)\n\}\}', wt, re.S)
    if info:
        for k in ('tipo', 'nvmax', 'arma', 'alvo', 'ENname'):
            m = re.search(r'\|\s*' + k + r'\s*=\s*(.*)', info.group(1))
            if m:
                out[k] = m.group(1).strip()
    for rows in parse_tables(wt):
        if not rows or not rows[0] or not rows[0][0].lower().startswith('nv'):
            continue
        header = rows[0]
        if not any(re.search(r'ATQ|Dano', h, re.I) for h in header):
            continue
        out['columns'] = header
        for r in rows[1:]:
            if r and re.fullmatch(r'\d+', r[0]):
                out['levels'][r[0]] = r[1:]
        break
    return out


def main():
    os.makedirs(OUT, exist_ok=True)
    codes = codes_wanted() if len(sys.argv) < 2 else sys.argv[1:]
    parsed = {}
    for i, code in enumerate(codes):
        try:
            parsed[code] = parse(fetch(code))
        except Exception as ex:  # network hiccup: keep going, retry next run
            print(f'{code}: {ex}', file=sys.stderr)
            continue
        if (i + 1) % 25 == 0:
            print(f'{i + 1}/{len(codes)}', flush=True)
    json.dump(parsed, open(os.path.join(OUT, 'skills.json'), 'w', encoding='utf-8'), ensure_ascii=False, indent=1)
    found = sum(1 for p in parsed.values() if p['title'])
    tables = sum(1 for p in parsed.values() if p['levels'])
    print(f'{len(parsed)} skills: {found} pages found, {tables} with a damage table')


if __name__ == '__main__':
    main()
