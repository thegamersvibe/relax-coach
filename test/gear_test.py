#!/usr/bin/env python3
"""Gear advice (1.7.2): both GearQuest layouts, battleground rewards, the trip-worth-it bar and level 60.

  python -I -B test/gear_test.py

Runs the real ForeverCoach files in Lua 5.1 (lupa) against the harness's mocked client. GearQuest data
here is synthetic (no GearQuest code or data is copied); the packed table follows the layout GearQuest
Forever 1.2.x ships. The real-shape case uses a level-51 warrior's equipped item ids from a saved
snapshot (names and realm removed) with synthetic scores. Exits non-zero on any failure."""
import ast, os, sys
try:
    import lupa.lua51 as lupa
except ImportError:
    import lupa

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..')
ADDON = os.path.join(ROOT, 'addon', 'ForeverCoach')
FILES = ['Rules', 'Core', 'Route', 'Session', 'Gear', 'Talents', 'Dungeons', 'Audit', 'Macros', 'Reactions', 'UI']
_tree = ast.parse(open(os.path.join(ROOT, 'test', 'harness.py'), encoding='utf-8').read())
MOCK = next(ast.literal_eval(n.value) for n in _tree.body
            if isinstance(n, ast.Assign) and getattr(n.targets[0], 'id', None) == 'MOCK')
SRC = {f: open(os.path.join(ADDON, f + '.lua'), encoding='utf-8').read() for f in FILES}
fails = []


def check(cond, msg):
    print(('PASS ' if cond else 'FAIL ') + msg)
    if not cond:
        fails.append(msg)


# Equipped items: slot -> (id, equip loc, score). Ids 274604 etc. are the real-snapshot shape below.
SLOT_LOC = {1: 'INVTYPE_HEAD', 2: 'INVTYPE_NECK', 3: 'INVTYPE_SHOULDER', 15: 'INVTYPE_CLOAK', 5: 'INVTYPE_CHEST',
            9: 'INVTYPE_WRIST', 10: 'INVTYPE_HAND', 6: 'INVTYPE_WAIST', 7: 'INVTYPE_LEGS', 8: 'INVTYPE_FEET',
            11: 'INVTYPE_FINGER', 12: 'INVTYPE_FINGER', 13: 'INVTYPE_TRINKET', 14: 'INVTYPE_TRINKET',
            16: 'INVTYPE_WEAPONMAINHAND', 17: 'INVTYPE_SHIELD', 18: 'INVTYPE_RANGEDRIGHT'}
GQ_SLOT = {1: 'Head', 2: 'Neck', 3: 'Shoulder', 15: 'Back', 5: 'Chest', 9: 'Wrist', 10: 'Hands', 6: 'Waist',
           7: 'Legs', 8: 'Feet', 11: 'Finger', 12: 'Finger', 13: 'Trinket', 14: 'Trinket', 16: 'MainHand',
           17: 'SecondaryHand', 18: 'Ranged'}
# A level-51 Dwarf protection warrior's equipped ids (from a saved snapshot; synthetic scores, total 960).
SNAPSHOT = {1: 275006, 2: 274385, 3: 287014, 5: 275005, 6: 10798, 7: 287013, 8: 10276, 9: 10282, 10: 287012,
            11: 12001, 12: 12002, 13: 17774, 14: 276743, 15: 17739, 16: 10837, 17: 279831, 18: 274604}
SCORE = {inv: 60.0 for inv in SNAPSHOT}
SCORE[18] = 13.2  # the ranged slot from the report (Frosty Blunderbuss)
PACKED_SLOTS = ["Back", "Chest", "Feet", "Finger", "Hands", "Head", "Legs", "MainHand", "Neck", "Ranged",
                "SecondaryHand", "Shoulder", "Trinket", "Waist", "Wrist"]
SPECS = ["arms", "fury", "protection"]


def lua_str(s):
    return '"' + s.replace('\\', '\\\\').replace('"', '\\"').replace('\n', '\\n') + '"'


def packed(rows, malformed=0):
    """rows: list of (id, slot, lo, hi, score, faction, spec). One packed table, both factions."""
    by = {'Alliance': [], 'Horde': []}
    rank = {}
    for (iid, slot, lo, hi, score, fac, spec) in rows:
        key = (fac, slot, spec)
        rank[key] = rank.get(key, 0) + 1
        by[fac].append('%d %d %d %d %d %d %s 0' % (iid, PACKED_SLOTS.index(slot) + 1, lo, hi, rank[key],
                                                  SPECS.index(spec) + 1, ('%g' % score)))
    for fac in by:
        by[fac][1:1] = ['not a row', '1 99 1 60 1 3 5 0', '7 1 1 60 1 9 5 0'][:malformed]
    rows_lua = ',\n'.join('["%s"] = %s' % (f, lua_str('\n'.join(by[f]) + '\n')) for f in by)
    return ('{ slots = {%s}, specs = {%s}, extras = { {suffix="of the Bear"} },\n rows = {\n%s\n} }'
            % (','.join(lua_str(s) for s in PACKED_SLOTS), ','.join(lua_str(s) for s in SPECS), rows_lua))


def array(rows):
    return '{' + ',\n'.join('{%d,%s,%d,%d,1,%s,%s,%g}' % (iid, lua_str(slot), lo, hi, lua_str(spec), lua_str(fac), score)
                            for (iid, slot, lo, hi, score, fac, spec) in rows) + '}'


def facts_lua(facts):
    out = []
    for iid, f in facts.items():
        out.append('[%d]={%s}' % (iid, ','.join('%s=%s' % (k, lua_str(v) if isinstance(v, str) else v)
                                                 for k, v in f.items())))
    return '{' + ',\n'.join(out) + '}'


def equipped_rows(level, fac='Alliance', spec='protection'):
    return [(iid, GQ_SLOT[inv], 1, 60, SCORE[inv], fac, spec) for inv, iid in SNAPSHOT.items()]


def client(level, rows, facts, layout='packed', zone="Un'Goro Crater", equipped=SNAPSHOT, malformed=0, fdj=None):
    L = lupa.LuaRuntime(unpack_returned_tuples=True)
    L.execute(MOCK)
    G = L.globals()
    G.STATE.level = level
    G.STATE.zone = zone
    G.STATE.talents = L.eval('{ {"Arms", {}}, {"Fury", {}}, {"Protection", {}} }')
    eq = L.table()
    for inv, iid in equipped.items():
        eq[inv] = iid
    G.STATE.equipped = eq
    data = 'warriorPicksPacked = %s' % packed(rows, malformed) if layout == 'packed' else \
        'warriorPicks = %s' % array(rows)
    if layout == 'neither':
        data = 'warriorSomethingNew = {}'
    L.execute('GearQuest = { Data = { %s, warriorItemFacts = %s } }' % (data, facts_lua(facts)))
    L.execute('SPEC_GQ = "protection"; GearQuest.Spec = { GetDisplaySpec = function() return SPEC_GQ end,'
              ' GetDefaultSpec = function() return "arms" end,'
              ' GetOptions = function() return {{id="arms"},{id="fury"},{id="protection"}} end }')
    for inv, iid in SNAPSHOT.items():
        G.LOCS[iid] = SLOT_LOC[inv]
    for (iid, slot, *_rest) in rows:
        if not G.LOCS[iid]:
            G.LOCS[iid] = SLOT_LOC[next(i for i, s in GQ_SLOT.items() if s == slot)]
    if fdj:
        L.execute('ForeverDungeonJournal_NS = { DB = %s }' % fdj)
    ns = L.table()
    for f in FILES:
        L.execute('return function(...) ' + SRC[f] + '\nend')('ForeverCoach', ns)
    G.FIRE('ADDON_LOADED', 'ForeverCoach')
    G.FIRE('PLAYER_LOGIN')
    FC = G.ForeverCoach
    FC.Refresh(FC, 'test')
    return L, G, FC, FC.Char(FC)


def texts(c):
    F = c.findings
    return [F[i].text + ' | ' + str(F[i].where or '') for i in range(1, len(F) + 1)] if F else []


def all_text(c):
    out = texts(c)
    for inv in range(1, 19):
        e = c.gear.slots[inv] if c.gear and c.gear.slots else None
        if e and e.best:
            out.append(str(e.best.where))
    return '\n'.join(out)


BOW, SHOTGUN, BEHIND_GUN, RAID_AXE, SOURCELESS, MARAUDON_GUN = 900001, 900002, 900003, 900004, 900005, 900006
FACTS = {
    BOW: dict(name="Twin Outrunner's Bow", reqLevel=48, sourceType="vendor", npc="Illiyana Moonblaze",
              zone="Ashenvale", instructions="Bought from Illiyana Moonblaze."),
    SHOTGUN: dict(name="Near Shotgun", reqLevel=50, sourceType="quest_reward", zone="Un'Goro Crater",
                  instructions="Reward from quest 'A Near Test'."),
    BEHIND_GUN: dict(name="Behind Rifle", reqLevel=40, sourceType="vendor", npc="Synthetic Gunsmith",
                     zone="Ashenvale", instructions="Bought from Synthetic Gunsmith."),
    RAID_AXE: dict(name="Raid Axe", reqLevel=60, sourceType="boss_drop", zone="Naxxramas",
                   instructions="Drops from a raid boss."),
    SOURCELESS: dict(name="Mystery Plate", reqLevel=55, sourceType="vendor", instructions="Bought from a vendor."),
    MARAUDON_GUN: dict(name="Maraudon Gun", reqLevel=45, sourceType="boss_drop", zone="Maraudon",
                       instructions="Drops from a dungeon boss."),
}
for inv, iid in SNAPSHOT.items():
    FACTS[iid] = dict(name="Equipped %d" % inv, reqLevel=40, sourceType="world_drop", zone="Tanaris",
                      instructions="Synthetic equipped item.")


def ranged(c):
    return c.gear.slots[18]


# 1. F1: both layouts give the same audit; malformed packed lines are skipped and counted.
print('== F1 packed and array layouts')
rows = equipped_rows(51) + [(SHOTGUN, 'Ranged', 50, 60, 30.0, 'Alliance', 'protection'),
                            (SHOTGUN, 'Ranged', 50, 60, 30.0, 'Horde', 'protection')]
_, _, _, cp = client(51, rows, FACTS, 'packed', malformed=3)
_, _, _, ca = client(51, rows, FACTS, 'array')
check(cp.gear.status == 'ok' and cp.gear.layout == 'packed', f'packed layout read ({cp.gear.status}/{cp.gear.layout})')
check(ca.gear.status == 'ok' and ca.gear.layout == 'array', f'array layout read ({ca.gear.status}/{ca.gear.layout})')
check(cp.gear.malformed == 3, f'three malformed Alliance lines counted, not fatal ({cp.gear.malformed})')
check(cp.gear.rows == ca.gear.rows and cp.gear.rows == len(SNAPSHOT) + 1, f'same usable rows ({cp.gear.rows}/{ca.gear.rows})')
check(ranged(cp).best and ranged(ca).best and ranged(cp).best.id == ranged(ca).best.id == SHOTGUN,
      'same ranged pick from both layouts')
_, _, _, ch = client(51, [(i, s, lo, hi, sc, 'Horde', sp) for (i, s, lo, hi, sc, f, sp) in rows],
                     FACTS, 'packed', zone='Tanaris')
L, G, FC, cx = client(51, rows, FACTS, 'packed')
G.STATE.faction = 'Horde'
FC.Refresh(FC, 'test')
check(cx.gear.status == 'ok' and cx.gear.rows > 0, 'Horde rows read from the packed table')
_, _, _, cn = client(51, rows, FACTS, 'neither')
check(cn.gear.status == 'layout', f'class module present, neither shape: status layout ({cn.gear.status})')
check(any('changed its data format' in t for t in texts(cn)) and not any('Enable' in t for t in texts(cn)),
      'layout message says the format changed, not "enable the module"')

# 2. F2 + F3: the bow case. Battleground reward blocked even with a huge gain; a small behind trip is not advice.
print('== F2/F3 bow case, warrior 51')
rows = equipped_rows(51) + [(BOW, 'Ranged', 48, 60, 24.9, 'Alliance', 'protection'),
                            (BEHIND_GUN, 'Ranged', 40, 60, 24.9, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS)
b = ranged(c).best
check(b and b.id == SNAPSHOT[18] and ranged(c).gapPct == 0,
      f'ranged stays on the equipped gun: bow is a battleground reward, the Ashenvale rifle a 1.2% behind-you trip ({b and b.name})')
check(not any('Bow' in t or 'Rifle' in t for t in texts(c)), 'no ranged finding printed')
check(c.gear.totalScore == 16 * 60 + 13.2 and c.gear.scoredSlots == 17, f'total over scored slots ({c.gear.totalScore})')
rows = equipped_rows(51) + [(BOW, 'Ranged', 48, 60, 400.0, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS)
check(ranged(c).best and ranged(c).best.id == SNAPSHOT[18], 'battleground reward blocked whatever its score')
rows = equipped_rows(51) + [(BEHIND_GUN, 'Ranged', 40, 60, 90.0, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS)
b = ranged(c).best
check(b and b.kind == 'behind' and 'behind you' in b.where, f'a large behind-you gain is still advice ({b and b.where})')
rows = equipped_rows(51) + [(MARAUDON_GUN, 'Ranged', 45, 60, 60.0, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS, fdj='{ ["Maraudon"] = { level = "40-49" } }')
b = ranged(c).best
check(b and b.kind == 'dungeon' and 'Maraudon' in b.where, f'behind-zone dungeon drop with a large gain still suggested ({b and b.where})')
pct = [t for t in texts(c) if 'Maraudon Gun' in t]
check(pct and '% better' in pct[0], f'percentage shown with 17 scored slots ({pct})')

# 3. F3: below 8 scored slots no bar and no percentage; the per-slot rule still applies.
print('== F3 few scored slots (level 20)')
few = {5: SNAPSHOT[5], 7: SNAPSHOT[7], 16: SNAPSHOT[16]}
rows = [(iid, GQ_SLOT[inv], 1, 60, 10.0, 'Alliance', 'protection') for inv, iid in few.items()] + \
       [(BEHIND_GUN, 'Chest', 15, 60, 20.0, 'Alliance', 'protection')]
FACTS[BEHIND_GUN]['reqLevel'] = 15
_, _, _, c = client(20, rows, FACTS, zone='Redridge Mountains', equipped=few)
e = c.gear.slots[5]
check(e.best and e.best.id == BEHIND_GUN and e.pctHidden, f'level 20: pick kept, percentage hidden ({e.best and e.best.where})')
check(any('Chest: Behind Rifle is better' in t for t in texts(c)), f'finding without a number ({[t for t in texts(c) if "Chest" in t]})')
FACTS[BEHIND_GUN]['reqLevel'] = 40

# 4. F4: level 60. No route wording anywhere; sourceless vendor and closed raid items are not advice.
print('== F4 level 60')
rows = equipped_rows(60) + [(SOURCELESS, 'Chest', 55, 60, 400.0, 'Alliance', 'protection'),
                            (RAID_AXE, 'MainHand', 60, 60, 400.0, 'Alliance', 'protection'),
                            (BEHIND_GUN, 'Ranged', 40, 60, 90.0, 'Alliance', 'protection')]
L, G, FC, c = client(60, rows, FACTS, zone='Winterspring')
txt = all_text(c)
check('your route' not in txt and 'behind you' not in txt and 'off your route' not in txt, 'no route wording at 60')
check(not (c.gear.slots[5].best and c.gear.slots[5].best.id == SOURCELESS), 'sourceless vendor item not suggested')
check(not (c.gear.slots[16].best and c.gear.slots[16].best.id == RAID_AXE), 'closed raid drop not suggested')
check(ranged(c).best and ranged(c).best.where == 'Ashenvale', f'zone named plainly at 60 ({ranged(c).best and ranged(c).best.where})')
L.execute('table.insert(ForeverCoach_Rules.raids.open, "Naxxramas")')
FC.Refresh(FC, 'test')
mh = c.gear.slots[16].best
check(mh and mh.id == RAID_AXE and mh.where == 'raid: Naxxramas', f'one rules entry opens a raid ({mh and mh.where})')

# 5. Real snapshot shape: level-51 warrior, protection, Un'Goro, packed data.
print('== real snapshot shape')
rows = equipped_rows(51) + [(BOW, 'Ranged', 48, 60, 24.9, 'Alliance', 'protection'),
                            (SHOTGUN, 'Ranged', 50, 60, 23.3, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS)
b = ranged(c).best
check(c.gear.status == 'ok' and b and b.id == SHOTGUN and b.kind == 'here',
      f'warrior 51: the bow is gone; the local quest shotgun is the ranged advice ({b and b.name} / {b and b.where})')
check(c.lastError is None, 'no addon error')

print()
print(f'{len(fails)} failed' if fails else 'ALL PASS')
sys.exit(1 if fails else 0)
