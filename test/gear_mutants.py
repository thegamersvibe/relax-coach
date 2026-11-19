#!/usr/bin/env python3
"""Behavioral mutants for the 1.7.2 gear fixes: each must turn test/gear_test.py red.

  python -I -B test/gear_mutants.py

Each mutant edits a scratch copy of addon/ and test/; the tree itself is never changed."""
import os, shutil, subprocess, sys, tempfile

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..'))
MUTANTS = [
    ('packed layout ignored', 'addon/ForeverCoach/Gear.lua',
     'local packed = D[p .. "PicksPacked"]', 'local packed = nil'),
    ('quartermaster block removed', 'addon/ForeverCoach/Gear.lua',
     'if f.npc == npc or ins:find("Bought from " .. npc, 1, true) then return "battleground reward" end', ''),
    ('trip bar zero', 'addon/ForeverCoach/Gear.lua',
     'local bar = (GA.tripPct or {})[c.kind] or GA.tripDefault or 5', 'local bar = 0'),
    ('route wording at 60', 'addon/ForeverCoach/Route.lua',
     'if maxLevel then return "zone", P.here or 1, zone, nil end', ''),
    ('closed raids allowed', 'addon/ForeverCoach/Gear.lua',
     'if raid and not FC.Route:RaidOpen(raid) then', 'if false then'),
    ('sourceless vendor allowed', 'addon/ForeverCoach/Route.lua',
     'return "unknown", 0, "location unknown", "no zone: nowhere to travel"', 'return "unknown", 0.7, "location unknown", nil'),
    ('raid bosses ignored', 'addon/ForeverCoach/Route.lua',
     'if f.npc == boss or ins:find("Drops from " .. boss, 1, true) then return raid end', ''),
    ('raid quests ignored', 'addon/ForeverCoach/Route.lua',
     'if ins:find("quest \'" .. quest .. "\'", 1, true) then return raid end', ''),
    ('layout status lost', 'addon/ForeverCoach/Gear.lua',
     'if facts or packed ~= nil then return nil, nil, "layout" end', ''),
]


def main():
    killed = 0
    for name, path, old, new in MUTANTS:
        with tempfile.TemporaryDirectory(prefix='gear-mut-') as d:
            for sub in ('addon', 'test'):
                shutil.copytree(os.path.join(ROOT, sub), os.path.join(d, sub))
            target = os.path.join(d, path)
            src = open(target, encoding='utf-8').read()
            if src.count(old) != 1:
                print(f'BROKEN {name}: anchor found {src.count(old)} times')
                return 1
            open(target, 'w', encoding='utf-8').write(src.replace(old, new))
            r = subprocess.run([sys.executable, '-I', '-B', os.path.join(d, 'test', 'gear_test.py')],
                               capture_output=True, text=True, timeout=300)
            ok = r.returncode != 0 and 'FAIL ' in r.stdout
            killed += ok
            print(('KILLED ' if ok else 'SURVIVED ') + name)
    print(f'RESULT: {killed}/{len(MUTANTS)} gear mutants killed')
    return 0 if killed == len(MUTANTS) else 1


if __name__ == '__main__':
    sys.exit(main())
