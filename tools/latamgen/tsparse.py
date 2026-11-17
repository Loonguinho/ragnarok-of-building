"""Helpers to pull skill entries out of latam-ro-calc job files (TypeScript)."""
import re

BACKSLASH = chr(92)


def skip_string(src, i):
    q = src[i]
    i += 1
    while i < len(src) and src[i] != q:
        if src[i] == BACKSLASH:
            i += 1
        i += 1
    return i


def match_brace(src, i):
    """src[i] is an opening bracket; return index of the matching close."""
    pairs = {'{': '}', '[': ']', '(': ')'}
    stack = [pairs[src[i]]]
    i += 1
    while i < len(src) and stack:
        ch = src[i]
        if ch in '"\'`':
            i = skip_string(src, i)
        elif src.startswith('//', i):
            while i < len(src) and src[i] != '\n':
                i += 1
            continue
        elif src.startswith('/*', i):
            i = src.find('*/', i) + 1
        elif ch in pairs:
            stack.append(pairs[ch])
        elif ch in '}])':
            stack.pop()
            if not stack:
                return i
        i += 1
    return -1


def array_items(src, open_idx):
    """Top-level {...} objects inside the array starting at src[open_idx] == '['."""
    end = match_brace(src, open_idx)
    i, out = open_idx + 1, []
    while i < end:
        ch = src[i]
        if ch in '"\'`':
            i = skip_string(src, i) + 1
            continue
        if src.startswith('//', i):
            while i < end and src[i] != '\n':
                i += 1
            continue
        if src.startswith('/*', i):
            i = src.find('*/', i) + 2
            continue
        if ch == '{':
            j = match_brace(src, i)
            out.append(src[i:j + 1])
            i = j + 1
            continue
        i += 1
    return out


def atk_lists(src):
    """All AtkSkillModel[] arrays in a file, as lists of entry sources."""
    out = []
    for m in re.finditer(r'AtkSkillModel\[\]\s*=\s*\[', src):
        out.extend(array_items(src, m.end() - 1))
    return out


def strip_comments(s):
    out, i = [], 0
    while i < len(s):
        if s[i] in '"\'`':
            j = skip_string(s, i)
            out.append(s[i:j + 1])
            i = j + 1
        elif s.startswith('//', i):
            while i < len(s) and s[i] != '\n':
                i += 1
        elif s.startswith('/*', i):
            i = s.find('*/', i) + 2
        else:
            out.append(s[i])
            i += 1
    return ''.join(out)


def top_fields(entry):
    """Map of top-level `key: value` in an object literal (value as raw source)."""
    body = strip_comments(entry)[1:-1]
    fields, i, n = {}, 0, len(body)
    while i < n:
        m = re.match(r'\s*([A-Za-z_][A-Za-z0-9_]*)\s*:', body[i:])
        if not m:
            i += 1
            continue
        key = m.group(1)
        j = i + m.end()
        start = j
        depth = 0
        while j < n:
            ch = body[j]
            if ch in '"\'`':
                j = skip_string(body, j) + 1
                continue
            if ch in '{[(':
                depth += 1
            elif ch in '}])':
                depth -= 1
            elif ch == ',' and depth == 0:
                break
            j += 1
        fields[key] = body[start:j].strip()
        i = j + 1
    return fields
