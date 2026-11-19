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
     'if f.npc == npc or FC.Route.HasWord(ins, npc) then return "battleground reward" end', ''),
    ('trip bar zero', 'addon/ForeverCoach/Gear.lua',
     'local bar = (GA.tripPct or {})[c.barKind or c.kind] or GA.tripDefault or 5', 'local bar = 0'),
    ('route wording at 60', 'addon/ForeverCoach/Route.lua',
     'if maxLevel then return "zone", P.here or 1, zone, nil end', ''),
    ('closed raids allowed', 'addon/ForeverCoach/Gear.lua',
     'if raid and not FC.Route:RaidOpen(raid) then', 'if false then'),
    ('sourceless vendor allowed', 'addon/ForeverCoach/Route.lua',
     'return "unknown", 0, "location unknown", "no zone: nowhere to travel"', 'return "unknown", 0.7, "location unknown", nil'),
    ('raid bosses ignored', 'addon/ForeverCoach/Route.lua',
     '    for boss, raid in pairs(rl.bosses or {}) do if source(boss) then return raid end end\n', ''),
    ('raid quests ignored', 'addon/ForeverCoach/Route.lua',
     'if text:find("quest \'" .. quest .. "\'", 1, true) then return raid end', ''),
    ('layout status lost', 'addon/ForeverCoach/Gear.lua',
     'if facts or packed ~= nil then return nil, nil, "layout" end', ''),
    ('dungeon base names ignored', 'addon/ForeverCoach/Route.lua',
     '        if zone == name or hasWord(zone, name) then return true end', '        if false then return true end'),
    ('world bosses ignored', 'addon/ForeverCoach/Route.lua',
     '        if source(boss) then return rl.worldBossName or "World bosses" end', '        if false then return rl.worldBossName or "World bosses" end'),
    ('raid boss in text ignored', 'addon/ForeverCoach/Route.lua',
     '    local function source(name) name = name:lower(); return drops[name] or npc == name end', '    local function source(name) name = name:lower(); return npc == name end'),
    ('raid mobs ignored', 'addon/ForeverCoach/Route.lua',
     '    for mob, raid in pairs(rl.mobs or {}) do if hasWord(text, mob) then return raid end end', ''),
    ('learned raid npcs ignored', 'addon/ForeverCoach/Route.lua',
     '    for name, raid in pairs(self._raidNpcs or {}) do if source(name) then return raid end end', ''),
    ('raid zone in text ignored', 'addon/ForeverCoach/Route.lua',
     '    for _, z in ipairs(rl.zones or {}) do if hasWord(text, z) then return z end end', ''),
    ('raid source kind ignored', 'addon/ForeverCoach/Route.lua',
     '    if f.sourceType == "raid" then return', '    if false then return'),
    ('unknown source kind allowed', 'addon/ForeverCoach/Gear.lua',
     'if f.sourceType ~= "raid" and not KNOWN_SOURCE[f.sourceType] then return "unknown source kind" end', 'if false then return "unknown source kind" end'),
    ('honor-text filter removed', 'addon/ForeverCoach/Gear.lua',
     '    for _, pat in ipairs(pvp.patterns or {}) do if ins:find(pat, 1, true) then return "battleground reward" end end', ''),
    ('second seller ignored', 'addon/ForeverCoach/Gear.lua',
     'if f.npc == npc or FC.Route.HasWord(ins, npc) then return "battleground reward" end', 'if f.npc == npc or ins:find("Bought from " .. npc, 1, true) then return "battleground reward" end'),
    ('bar off under eight slots', 'addon/ForeverCoach/Gear.lua',
     '    local barOn = scored > 0 and total > 0', '    local barOn = scored >= (GA.minScoredSlots or 8) and total > 0'),
    ('unknown equipped counts as zero', 'addon/ForeverCoach/Gear.lua',
     '        if have == nil and entry.scoreStatus == "unknown" then have = average end', ''),
    ('bag upgrades skip the bar', 'addon/ForeverCoach/Gear.lua',
     'local worth = not barOn or clears(s - (worstScore or 0), (GA.tripPct or {}).bags or 1)', 'local worth = true'),
    ('array before packed', 'addon/ForeverCoach/Gear.lua',
     '    -- packed first: an old or stub array beside it must not hide the real data\n', '    if type(D[p .. "Picks"]) == "table" then return { D[p .. "Picks"], D[p .. "Early1to9"] }, factTables, "array", 0 end\n'),
    ('extras index unchecked', 'addon/ForeverCoach/Gear.lua',
     'and (ex == 0 or (type(packed.extras) == "table" and packed.extras[ex] ~= nil))', 'and true'),
    ('display clamp removed', 'addon/ForeverCoach/Gear.lua',
     '    if n > 100 then return ">100%" end\n', ''),
    ('off-route dungeon uses the off-route bar', 'addon/ForeverCoach/Gear.lua',
     'then barKind = "dungeon"', 'then barKind = kind'),
    ('crafted item uses its place bar', 'addon/ForeverCoach/Gear.lua',
     '    elseif f and f.sourceType == "profession" then barKind = "craft" end', '    end'),
    ('boss matched anywhere in text', 'addon/ForeverCoach/Route.lua',
     '    local function source(name) name = name:lower(); return drops[name] or npc == name end', '    local function source(name) return hasWord(text, name) end'),
    ('npc contains the boss', 'addon/ForeverCoach/Route.lua',
     '    local function source(name) name = name:lower(); return drops[name] or npc == name end', '    local function source(name) name = name:lower(); return drops[name] or (npc and npc:find(name, 1, true)) end'),
    ('apostrophe splits words', 'addon/ForeverCoach/Route.lua',
     '    return text:find("%f[%w\']" .. esc(name) .. "%f[^%w\']") ~= nil', '    return text:find("%f[%w]" .. esc(name) .. "%f[^%w]") ~= nil'),
    ('drop list keeps places', 'addon/ForeverCoach/Route.lua',
     '            part = part:gsub("^%s*from%s+", ""):gsub("%s+in%s+.*$", ""):gsub("^%s+", ""):gsub("%s+$", "")', '            part = part:gsub("^%s*from%s+", ""):gsub("^%s+", ""):gsub("%s+$", "")'),
    ('Waking Legends not a world-boss quest', 'addon/ForeverCoach/Rules.lua',
     '      ["Waking Legends"] = "World bosses",\n', ''),
    ("Forever's dungeons missing", 'addon/ForeverCoach/Rules.lua',
     '      "Alcaz Prison",\n', ''),
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
