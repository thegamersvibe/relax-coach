#!/usr/bin/env python3
"""rules/rules.json -> addon/ForeverCoach/Rules.lua (the addon can't read JSON).
desktop/update.go does the same conversion when it downloads a newer rules.json.
Both use real Lua string escaping: UTF-8 stays as raw bytes (Lua 5.1 has no \\u escapes)."""
import json, os, sys

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..')


def lua_str(s):
    out = []
    for ch in s:
        if ch == '\\': out.append('\\\\')
        elif ch == '"': out.append('\\"')
        elif ch == '\n': out.append('\\n')
        elif ch == '\r': out.append('\\r')
        elif ord(ch) < 32 or ord(ch) == 127: out.append('\\%03d' % ord(ch))
        else: out.append(ch)
    return '"' + ''.join(out) + '"'


def lua(v, ind=0):
    p = '  ' * ind
    if isinstance(v, bool): return 'true' if v else 'false'
    if v is None: return 'nil'
    if isinstance(v, (int, float)): return repr(v)
    if isinstance(v, str): return lua_str(v)
    if isinstance(v, list): return '{\n' + ''.join(f'{p}  {lua(x, ind + 1)},\n' for x in v) + p + '}'
    if isinstance(v, dict):
        return '{\n' + ''.join(f'{p}  [{lua_str(k)}] = {lua(x, ind + 1)},\n' for k, x in sorted(v.items())) + p + '}'
    raise TypeError(type(v))


def convert(rules):
    return '-- Generated from rules.json. Do not edit; edit rules.json.\nForeverCoach_Rules = ' + lua(rules) + '\n'


def convert_avoidable(rules):
    """Export enabled raid mechanics only; name-only spells cannot label recap IDs."""
    if rules.get('schema') != 1 or not isinstance(rules.get('encounters'), list):
        raise ValueError('unsupported enabled-encounter rules')
    ids = set()
    for encounter in rules['encounters']:
        mechanics = encounter.get('mechanics')
        if not isinstance(mechanics, list):
            raise ValueError('mechanics must be an array')
        for mechanic in mechanics:
            avoidable, spell = mechanic.get('avoidable'), mechanic.get('spell')
            if type(avoidable) is not bool or not isinstance(spell, dict):
                raise ValueError('mechanic requires avoidable and spell')
            spell_id = spell.get('id')
            if spell_id is not None and (type(spell_id) is not int or not 1 <= spell_id <= 2147483647):
                raise ValueError('spell id must be a positive integer')
            if spell_id is not None:
                source = spell.get('source')
                if (not isinstance(source, dict) or source.get('kind') not in ('real_log', 'forever_source')
                        or not isinstance(source.get('note'), str) or not source['note'].strip()):
                    raise ValueError('spell id requires a source')
            if avoidable and spell_id is not None:
                ids.add(spell_id)
    return ('-- Generated from rules/raid/encounters-enabled.json. Do not edit.\n'
            '-- Only enabled mechanics with avoidable=true and a sourced spell ID.\n'
            'local _, RR = ...\nRR.avoidableSpells = {\n' +
            ''.join(f'    [{spell_id}] = true,\n' for spell_id in sorted(ids)) + '}\n')


def main():
    import argparse
    from pathlib import Path
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('src', nargs='?', default=os.path.join(ROOT, 'rules', 'rules.json'))
    parser.add_argument('dst', nargs='?', default=os.path.join(ROOT, 'addon', 'ForeverCoach', 'Rules.lua'))
    parser.add_argument('--raid-source', default=os.path.join(ROOT, 'rules', 'raid', 'encounters-enabled.json'))
    parser.add_argument('--avoidable-output', default=os.path.join(ROOT, 'addon', 'RelaxRaid', 'AvoidableSpells.lua'))
    parser.add_argument('--check', action='store_true', help='check both generated files without writing')
    args = parser.parse_args()
    # Generate both before writing either: invalid raid input cannot partially export.
    outputs = [(Path(args.dst), convert(json.loads(Path(args.src).read_text(encoding='utf-8')))),
               (Path(args.avoidable_output), convert_avoidable(json.loads(Path(args.raid_source).read_text(encoding='utf-8'))))]
    for path, content in outputs:
        if args.check:
            if not path.exists() or path.read_text(encoding='utf-8') != content:
                raise SystemExit(f'generated file differs: {path.name}')
        else:
            path.write_text(content, encoding='utf-8', newline='\n')
        print(('checked' if args.check else 'wrote'), path.name)


if __name__ == '__main__':
    main()
