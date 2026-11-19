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
OUTDOOR_RAID, RAID_QUEST, NOZONE_VENDOR, NOZONE_CRAFT = 900007, 900008, 900009, 900010
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
    OUTDOOR_RAID: dict(name="Glyph Trinket", reqLevel=60, sourceType="boss_drop", npc="Sapphiron",
                       zone="Eastern Plaguelands", instructions="Drops from Sapphiron."),
    RAID_QUEST: dict(name="Dragonslayer Neck", reqLevel=60, sourceType="quest_reward", zone="Stormwind City",
                     instructions="Reward from the quest 'The Lord of Blackrock'. Starts in Stormwind City."),
    NOZONE_VENDOR: dict(name="Zoneless Gizmo", reqLevel=60, sourceType="vendor", npc="Synthetic Goblin",
                        instructions="Bought from Synthetic Goblin."),
    NOZONE_CRAFT: dict(name="Crafted Helm", reqLevel=55, sourceType="profession", profession="Blacksmithing",
                       instructions="Crafted with Blacksmithing."),
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
                            (BEHIND_GUN, 'Ranged', 40, 60, 90.0, 'Alliance', 'protection'),
                            (OUTDOOR_RAID, 'Trinket', 60, 60, 400.0, 'Alliance', 'protection'),
                            (RAID_QUEST, 'Neck', 60, 60, 400.0, 'Alliance', 'protection'),
                            (NOZONE_VENDOR, 'Trinket', 60, 60, 900.0, 'Alliance', 'protection'),
                            (NOZONE_CRAFT, 'Head', 55, 60, 400.0, 'Alliance', 'protection')]
L, G, FC, c = client(60, rows, FACTS, zone='Winterspring')
check(not any(c.gear.slots[i].best and c.gear.slots[i].best.id == NOZONE_VENDOR for i in (13, 14)),
      'named seller with no zone is not a pick (location unknown)')
check('location unknown' not in all_text(c), 'no "location unknown" pick anywhere')
check(c.gear.slots[1].best and c.gear.slots[1].best.id == NOZONE_CRAFT, 'crafted item with no zone is still a pick')
check(not any(c.gear.slots[i].best and c.gear.slots[i].best.id == OUTDOOR_RAID for i in (13, 14)),
      'raid boss drop filed under an outdoor zone not suggested')
check(not (c.gear.slots[2].best and c.gear.slots[2].best.id == RAID_QUEST), 'raid-kill reward quest not suggested')
txt = all_text(c)
check('your route' not in txt and 'behind you' not in txt and 'off your route' not in txt, 'no route wording at 60')
check(not (c.gear.slots[5].best and c.gear.slots[5].best.id == SOURCELESS), 'sourceless vendor item not suggested')
check(not (c.gear.slots[16].best and c.gear.slots[16].best.id == RAID_AXE), 'closed raid drop not suggested')
check(ranged(c).best and ranged(c).best.where == 'Ashenvale', f'zone named plainly at 60 ({ranged(c).best and ranged(c).best.where})')
L.execute('table.insert(ForeverCoach_Rules.raids.open, "Naxxramas")')
FC.Refresh(FC, 'test')
mh = c.gear.slots[16].best
check(mh and mh.id == RAID_AXE and mh.where == 'raid: Naxxramas', f'one rules entry opens a raid ({mh and mh.where})')

# 4b. With the open-raids list empty, no pick at any level has a raid source; zero usable packed rows is nodata.
print('== F4 raids closed at every level; F1 nodata')
rows = equipped_rows(60) + [(RAID_AXE, 'MainHand', 1, 60, 900.0, 'Alliance', 'protection'),
                            (OUTDOOR_RAID, 'Trinket', 1, 60, 900.0, 'Alliance', 'protection'),
                            (RAID_QUEST, 'Neck', 1, 60, 900.0, 'Alliance', 'protection')]
saved = {i: FACTS[i]['reqLevel'] for i in (RAID_AXE, OUTDOOR_RAID, RAID_QUEST)}
for i in saved:
    FACTS[i]['reqLevel'] = 1
raid_picks = []
for lvl in (10, 20, 35, 51, 59, 60):
    _, _, _, c = client(lvl, rows, FACTS, zone='Tanaris')
    raid_picks += [(lvl, inv) for inv in range(1, 19) if c.gear.slots[inv] and c.gear.slots[inv].best
                   and c.gear.slots[inv].best.id in saved]
for i, v in saved.items():
    FACTS[i]['reqLevel'] = v
check(not raid_picks, f'no raid pick at levels 10-60 with the list empty ({raid_picks})')
_, _, _, c = client(51, [(SHOTGUN, 'Ranged', 50, 60, 30.0, 'Horde', 'protection')], FACTS)
check(c.gear.status == 'nodata' and c.gear.layout == 'packed', f'packed with zero Alliance rows: nodata ({c.gear.status})')

# 5. Real snapshot shape: level-51 warrior, protection, Un'Goro, packed data.
print('== real snapshot shape')
rows = equipped_rows(51) + [(BOW, 'Ranged', 48, 60, 24.9, 'Alliance', 'protection'),
                            (SHOTGUN, 'Ranged', 50, 60, 23.3, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS)
b = ranged(c).best
check(c.gear.status == 'ok' and b and b.id == SHOTGUN and b.kind == 'here',
      f'warrior 51: the bow is gone; the local quest shotgun is the ranged advice ({b and b.name} / {b and b.where})')
check(c.lastError is None, 'no addon error')

# 6. Review round 1.
def refresh(G, FC):
    FC.Refresh(FC, 'test')
    return FC.Char(FC)


def eligibility(G, FC, c, iid):
    f = G.GearQuest.Data.warriorItemFacts[iid]
    why = FC.Gear.Eligibility(FC.Gear, f, c.snapshot)
    return why[0] if isinstance(why, tuple) else why


def total_of(level=60, zone='Winterspring'):
    _, _, _, c = client(level, equipped_rows(level), FACTS, zone=zone)
    return c.gear.totalScore


print('== round 1: dungeon bar for dungeon base names (Blackrock Spire, Stratholme, Dire Maul, Scarlet Monastery)')
D_BRS, D_STRAT, D_UBRS, D_DM = 900101, 900102, 900103, 900104
FACTS[D_BRS] = dict(name="Twin Bloodmoon Cloak", reqLevel=58, sourceType="boss_drop", npc="The Beast", zone="Blackrock Spire",
                    instructions="Drops from The Beast.")
FACTS[D_STRAT] = dict(name="Twin Carapace Crossbow", reqLevel=58, sourceType="boss_drop", npc="Nerub'enkan", zone="Stratholme",
                      instructions="Drops from Nerub'enkan.")
FACTS[D_UBRS] = dict(name="Twin Draconian Deflector", reqLevel=58, sourceType="boss_drop", npc="General Drakkisath",
                     zone="Upper Blackrock Spire", instructions="Drops from General Drakkisath.")
FACTS[D_DM] = dict(name="Dire Maul Shoulders", reqLevel=58, sourceType="boss_drop", npc="Immol'thar", zone="Dire Maul East",
                   instructions="Drops from Immol'thar.")
T60 = total_of()
# the three level-60 picks from the review, as twins: 1.93%, 1.61% and 2.25% of the total, all under the 3% dungeon bar
rows = equipped_rows(60) + [(D_BRS, 'Back', 58, 60, SCORE[15] + 0.0193 * T60, 'Alliance', 'protection'),
                            (D_STRAT, 'Ranged', 58, 60, SCORE[18] + 0.0161 * T60, 'Alliance', 'protection'),
                            (D_UBRS, 'SecondaryHand', 58, 60, SCORE[17] + 0.0225 * T60, 'Alliance', 'protection'),
                            (D_DM, 'Shoulder', 58, 60, SCORE[3] + 0.032 * T60, 'Alliance', 'protection')]
L, G, FC, c = client(60, rows, FACTS, zone='Winterspring')
for inv, iid, what in ((15, D_BRS, 'Blackrock Spire 1.93%'), (18, D_STRAT, 'Stratholme 1.61%'), (17, D_UBRS, 'Upper Blackrock Spire 2.25%')):
    b = c.gear.slots[inv].best
    check(not (b and b.id == iid), f'{what} is under the 3% dungeon bar: hidden ({b and b.name})')
b = c.gear.slots[3].best
check(b and b.id == D_DM and b.kind == 'dungeon', f'Dire Maul East 3.2% passes as a dungeon ({b and b.kind} / {b and b.where})')
for z in ('Blackrock Spire', 'Upper Blackrock Spire', 'Lower Blackrock Spire', 'Dire Maul', 'Dire Maul: North', 'Stratholme',
          'Scarlet Monastery', 'Scarlet Monastery Armory'):
    check(FC.Route.IsDungeon(z), f'{z} is a dungeon without the Dungeon Journal')
for z in ('Blackrock Mountain', 'Searing Gorge', 'Ashenvale', 'Tirisfal Glades'):
    check(not FC.Route.IsDungeon(z), f'{z} is not a dungeon')

print('== round 1: raids and world bosses fail closed')
RAIDISH = {
    900201: dict(name="Twin Blazefury Medallion", reqLevel=1, sourceType="boss_drop", npc="Lord Kazzak", zone="The Tainted Scar",
                 instructions="Drops from Lord Kazzak."),
    900202: dict(name="Twin Jagged Obsidian Shield", reqLevel=1, sourceType="profession", profession="Blacksmithing",
                 instructions="Crafted with Blacksmithing (requires skill 300). Bought from Lieutenant General Andorov."),
    900203: dict(name="Twin Persuader", reqLevel=1, sourceType="profession", profession="Blacksmithing",
                 instructions="Crafted with Blacksmithing (requires skill 300). The Blacksmithing pattern drops from Hakkari Shadow Hunter and Vekniss Stinger."),
    900204: dict(name="Twin Hammer of Bestial Fury", reqLevel=1, sourceType="boss_drop", npc="Emeriss", zone="Duskwood",
                 instructions="Drops from Emeriss."),
    900205: dict(name="Outdoor Loatheb", reqLevel=1, sourceType="boss_drop", npc="Loatheb", zone="Eastern Plaguelands",
                 instructions="Drops from Loatheb."),
    900206: dict(name="Kel'Thuzad Reward", reqLevel=1, sourceType="quest_reward", zone="Eastern Plaguelands",
                 instructions="Reward from quest 'The Fall of Kel'Thuzad'."),
    900207: dict(name="Future Raid Kind", reqLevel=1, sourceType="raid", zone="Future Raid", instructions="Drops from the Future Raid boss."),
    900208: dict(name="Second Source Sapphiron", reqLevel=1, sourceType="boss_drop", npc="Field Scout", zone="Eastern Plaguelands",
                 instructions="Drops from Field Scout; Sapphiron (Naxxramas)."),
    900211: dict(name="Raid Zone Named Only", reqLevel=1, sourceType="boss_drop", npc="Field Scout", zone="Eastern Plaguelands",
                 instructions="Drops from Field Scout; also from a mob in Naxxramas."),
    900209: dict(name="Learned Raid Mob Pattern", reqLevel=1, sourceType="profession", profession="Blacksmithing",
                 instructions="Crafted with Blacksmithing. The pattern drops from Synthetic Plague Brute."),
}
FACTS.update(RAIDISH)
FACTS[900210] = dict(name="Inside Naxx", reqLevel=60, sourceType="raid_trash", npc="Synthetic Plague Brute", zone="Naxxramas",
                     instructions="Drops from Synthetic Plague Brute.")
for lvl in (10, 20, 35, 51, 59, 60):
    rows = equipped_rows(lvl) + [(iid, 'Trinket', 1, 60, 900.0, 'Alliance', 'protection') for iid in RAIDISH]
    L, G, FC, c = client(lvl, rows, FACTS, zone='Tanaris')
    picked = [c.gear.slots[i].best.id for i in (13, 14) if c.gear.slots[i].best and c.gear.slots[i].best.id in RAIDISH]
    check(not picked, f'level {lvl}: no raid or world-boss pick ({picked})')
for iid in RAIDISH:
    why = eligibility(G, FC, c, iid)
    check(why in ('raid not open in Forever yet', 'world boss: not open in Forever yet'), f'{FACTS[iid]["name"]}: blocked as raid ({why})')
check(eligibility(G, FC, c, 900201) == 'world boss: not open in Forever yet', 'Kazzak is a world boss')
L.execute('table.insert(ForeverCoach_Rules.raids.open, "Naxxramas")')
check(eligibility(G, FC, c, 900205) is None, 'opening Naxxramas opens its outdoor-filed Loatheb drop')
check(eligibility(G, FC, c, 900201) is not None, 'opening a raid does not open world bosses')
L.execute('table.insert(ForeverCoach_Rules.raids.open, "World bosses")')
check(eligibility(G, FC, c, 900201) is None, 'one entry opens the world bosses')

print('== round 1: honor and battleground sources, unknown source kinds')
FACTS[900301] = dict(name="Honor Text Item", reqLevel=1, sourceType="vendor", npc="Synthetic Officer", zone="Stormwind City",
                     instructions="Costs Honor Points and tokens.")
FACTS[900302] = dict(name="Second Seller", reqLevel=1, sourceType="vendor", npc="Synthetic Seller", zone="Ashenvale",
                     instructions="Bought from Synthetic Seller; Illiyana Moonblaze.")
FACTS[900303] = dict(name="Mystery Kind", reqLevel=1, sourceType="mystery_kind", zone="Tanaris", instructions="Somewhere.")
L, G, FC, c = client(51, equipped_rows(51), FACTS)
check(eligibility(G, FC, c, 900301) == 'battleground reward', f'honor text alone blocks ({eligibility(G, FC, c, 900301)})')
check(eligibility(G, FC, c, 900302) == 'battleground reward', f'a quartermaster as second seller blocks ({eligibility(G, FC, c, 900302)})')
check(eligibility(G, FC, c, 900303) == 'unknown source kind', f'unknown source kind fails closed ({eligibility(G, FC, c, 900303)})')

print('== round 1: bars by source, decimals, few slots, unknown equipped, bags')
OFF_D, CRAFT_Z, EXACT = 900401, 900402, 900403
FACTS[OFF_D] = dict(name="Off Dungeon", reqLevel=40, sourceType="boss_drop", npc="Boss", zone="Synthetic Dungeon",
                    instructions="Drops from a dungeon boss.")
FACTS[CRAFT_Z] = dict(name="Craft With Zone", reqLevel=40, sourceType="profession", profession="Blacksmithing", zone="Ashenvale",
                      instructions="Crafted with Blacksmithing.")
FACTS[EXACT] = dict(name="Exact Five", reqLevel=40, sourceType="quest_reward", zone="Ashenvale", instructions="Reward from quest 'X'.")
T51 = total_of(51, "Un'Goro Crater")
rows = equipped_rows(51) + [(OFF_D, 'Ranged', 40, 60, SCORE[18] + 0.04 * T51, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS, fdj='{ ["Synthetic Dungeon"] = { level = "40-49" } }')
b = ranged(c).best
check(b and b.id == OFF_D, f'off-route dungeon at 4% uses the 3% dungeon bar ({b and b.kind})')
rows = equipped_rows(51) + [(CRAFT_Z, 'Ranged', 40, 60, SCORE[18] + 0.02 * T51, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS)
b = ranged(c).best
check(b and b.id == CRAFT_Z, f'crafted item with a zone at 2% uses the 1% craft bar ({b and b.kind})')
rows = equipped_rows(51) + [(EXACT, 'Ranged', 40, 60, SCORE[18] + 0.05 * T51, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS)
b = ranged(c).best
check(b and b.id == EXACT, f'a gain exactly on the 5% bar passes despite decimals ({b and b.where}, total {T51})')
# fewer than 8 scored slots: the bar stays, only the percentage goes
few = {inv: SNAPSHOT[inv] for inv in (1, 2, 3, 5, 6, 18)}
rows_few = [(iid, GQ_SLOT[inv], 1, 60, SCORE[inv], 'Alliance', 'protection') for inv, iid in few.items()]
tot_few = sum(SCORE[inv] for inv in few)
HERE_R = 900406
FACTS[HERE_R] = dict(name="Here Rifle", reqLevel=40, sourceType="quest_reward", zone="Un'Goro Crater", instructions="Reward from quest 'Here'.")
rows = rows_few + [(HERE_R, 'Ranged', 40, 60, SCORE[18] + 0.005 * tot_few, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS, equipped=few)
check(not (ranged(c).best and ranged(c).best.id == HERE_R), 'six scored slots: a 0.5% gain here is under the 1% bar (the bar stays)')
rows = rows_few + [(HERE_R, 'Ranged', 40, 60, SCORE[18] + 0.02 * tot_few, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS, equipped=few)
check(ranged(c).best and ranged(c).best.id == HERE_R and c.gear.slots[18].pctHidden, 'six scored slots: 2% passes, percentage hidden')
# an equipped item with no score counts as an average scored slot, not as zero
UNSCORED = dict(SNAPSHOT); UNSCORED[13] = 700001
rows = [r for r in equipped_rows(51) if r[0] != SNAPSHOT[13]]
tot = sum(SCORE[inv] for inv in SNAPSHOT if inv != 13)
avg = tot / (len(SNAPSHOT) - 1)
NEAR_T = 900404
FACTS[NEAR_T] = dict(name="Near Trinket", reqLevel=40, sourceType="quest_reward", zone="Un'Goro Crater", instructions="Reward from quest 'Near'.")
_, _, _, c = client(51, rows + [(NEAR_T, 'Trinket', 40, 60, avg + 0.005 * tot, 'Alliance', 'protection')], FACTS, equipped=UNSCORED)
check(not (c.gear.slots[13].best and c.gear.slots[13].best.id == NEAR_T), 'unscored trinket: a 0.5% gain over an average slot is under the 1% bar')
_, _, _, c = client(51, rows + [(NEAR_T, 'Trinket', 40, 60, avg + 0.02 * tot, 'Alliance', 'protection')], FACTS, equipped=UNSCORED)
check(c.gear.slots[13].best and c.gear.slots[13].best.id == NEAR_T, 'unscored trinket: 2% over an average slot passes')
# bag upgrades must pass the 1% total bar
BAG = 900405
FACTS[BAG] = dict(name="Bag Chest", reqLevel=40, sourceType="quest_reward", zone="Un'Goro Crater", instructions="Reward from quest 'Bag'.")
for share, want in ((0.005, False), (0.02, True)):
    L, G, FC, c = client(51, equipped_rows(51) + [(BAG, 'Chest', 40, 60, SCORE[5] * 1.04 if share < 0.01 else SCORE[5] + share * T51,
                                                   'Alliance', 'protection')], FACTS)
    G.STATE.bags[1] = BAG
    c = refresh(G, FC)
    owned = [c.gear.owned[i].id for i in range(1, len(c.gear.owned) + 1)] if c.gear.owned else []
    check((BAG in owned) == want, f'bag upgrade at {share * 100:.1f}% of total: {"shown" if want else "not shown"} ({owned})')
    G.STATE.bags[1] = None

print('== round 1: packed first, malformed rows, display clamp')
L, G, FC, c = client(51, equipped_rows(51), FACTS)
L.execute('GearQuest.Data.warriorPicks = {}')
c = refresh(G, FC)
check(c.gear.layout == 'packed' and c.gear.status == 'ok', f'an empty array beside valid packed data: packed wins ({c.gear.layout} {c.gear.status})')
L.execute('BADP = { slots = {"Ranged","NotASlot"}, specs = {"protection"}, extras = {}, rows = { Alliance = '
          '"9001 1 1 60 1 1 140 0\\n9002 1 1 60 1 1 140 999\\n9003 2 1 60 1 1 140 0\\n9004 1 1 60 1 1 ' + '9' * 400 + ' 0\\n" } }')
rows_bad, bad = L.eval('ForeverCoach.Gear.Unpack(BADP, "Alliance", "protection")')
check(len(rows_bad) == 1 and bad == 3, f'extras 999, an unknown slot and a 400-digit score are malformed ({len(rows_bad)} rows, {bad} bad)')
check(FC.Gear.PctText(300) == '>100%' and FC.Gear.PctText(100) == '100%' and FC.Gear.PctText(45) == '45%', 'one clamp rule: >100%')
rows = equipped_rows(51) + [(SHOTGUN, 'Ranged', 50, 60, SCORE[18] * 4, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS)
check(any('>100% better' in t for t in texts(c)), f'the addon says ">100% better" ({[t for t in texts(c) if "Ranged" in t]})')

# 7. Review round 2: real GearQuest warrior records.
REAL = {
    20600: dict(name="Malfurion's Signet Ring", reqLevel=60, sourceType="quest_reward", zone="Moonglade",
                instructions="Reward from the quest 'Waking Legends'."),
    19852: dict(name="Ancient Hakkari Manslayer", reqLevel=60, sourceType="boss_drop", npc="Hakkar", zone="Zul'Gurub",
                instructions="Drops from Hakkar."),
    22384: dict(name="Persuader", reqLevel=58, sourceType="profession", profession="Blacksmithing",
                instructions="Crafted with Blacksmithing (requires skill 300). The Blacksmithing pattern drops from Hakkari Shadow Hunter and Vekniss Stinger."),
}
NOT_RAID = {
    10838: dict(name="Might of Hakkar", reqLevel=49, sourceType="boss_drop", npc="Avatar of Hakkar", zone="Sunken Temple",
                instructions="Drops from Avatar of Hakkar."),
    10845: dict(name="Warrior's Embrace", reqLevel=49, sourceType="boss_drop", npc="Avatar of Hakkar", zone="Sunken Temple",
                instructions="Drops from Avatar of Hakkar."),
    10846: dict(name="Bloodshot Greaves", reqLevel=49, sourceType="boss_drop", npc="Avatar of Hakkar", zone="Sunken Temple",
                instructions="Drops from Avatar of Hakkar."),
    10749: dict(name="Avenguard Helm", reqLevel=40, sourceType="quest_reward", zone="Sunken Temple",
                instructions="Reward from the quest 'The God Hakkar'. Pick it over the other reward choices. Starts with Yeh'kinya in Tanaris. Objective: Bring the Filled Egg of Hakkar to Yeh'kinya in Tanaris."),
    10750: dict(name="Lifeforce Dirk", reqLevel=40, sourceType="quest_reward", zone="Sunken Temple",
                instructions="Reward from the quest 'The God Hakkar'. Pick it over the other reward choices. Starts with Yeh'kinya in Tanaris. Objective: Bring the Filled Egg of Hakkar to Yeh'kinya in Tanaris."),
    10701: dict(name="Boots of Zua'tec", reqLevel=38, sourceType="quest_reward", zone="Dustwallow Marsh",
                instructions="Reward from the quest 'The Brood of Onyxia'. Pick it over the other reward choices. Starts with Draz'Zilb in Dustwallow Marsh. Objective: Speak with Overlord Mok'Morokk in Brackenwall Village."),
    250551: dict(name="Officer's Gloves", reqLevel=50, sourceType="profession", profession="Blacksmithing",
                 instructions="Crafted with Blacksmithing (requires skill 250). The Blacksmithing pattern drops from Archaedas in Uldaman, from Chief Ukorz Sandscalp, Witch Doctor Zum'rah, Gahz'rilla, and Ruuzlu in Zul'Farrak, from Shade of Eranikus in The Temple of Atal'Hakkar, and from Celebras the Cursed, Razorlash, and Tinkerer Gizlock in Maraudon."),
    252586: dict(name="Skirmisher's Mail Gauntlets", reqLevel=50, sourceType="profession", profession="Leatherworking",
                 instructions="Crafted with Leatherworking (requires skill 250). The Leatherworking pattern drops from Chief Ukorz Sandscalp, Gahz'rilla, and Shadowpriest Sezz'ziz in Zul'Farrak, from Dreamscythe in The Temple of Atal'Hakkar, and from Lord Roccor in Blackrock Depths."),
}
FACTS.update(REAL); FACTS.update(NOT_RAID)
RAID_REASONS = ('raid not open in Forever yet', 'world boss: not open in Forever yet')

print('== round 2: Waking Legends is a world-boss quest; the raid match is not wider than its sources')
L, G, FC, c = client(60, equipped_rows(60), FACTS, zone='Winterspring')
eq_ = eligibility(G, FC, c, 20600)
check(eq_ == 'world boss: not open in Forever yet', f"Malfurion's Signet Ring (Waking Legends) blocked as a world boss ({eq_})")
check(eligibility(G, FC, c, 19852) == 'raid not open in Forever yet', 'Ancient Hakkari Manslayer (Hakkar, Zul\'Gurub) still blocked')
check(eligibility(G, FC, c, 22384) == 'raid not open in Forever yet', 'Persuader (Hakkari/Vekniss pattern) still blocked')
for lvl in (51, 60):
    L, G, FC, c = client(lvl, equipped_rows(lvl), FACTS, zone="Un'Goro Crater")
    for iid, f in NOT_RAID.items():
        why = eligibility(G, FC, c, iid)
        check(why not in RAID_REASONS, f'level {lvl}: {f["name"]} is not raid loot ({why})')
check(FC.Route.Raid(FC.Route, 'Sunken Temple', FACTS[10838]) is None, 'Avatar of Hakkar is not Hakkar')
check(FC.Route.HasWord("in The Temple of Atal'Hakkar", 'Hakkar') is False, "Atal'Hakkar is one word")
check(FC.Route.HasWord('Drops from Hakkar.', 'Hakkar') is True, 'Hakkar alone is')
names = FC.Route.DropNames(NOT_RAID[250551]['instructions'])
for n in ('archaedas', 'chief ukorz sandscalp', "witch doctor zum'rah", "gahz'rilla", 'ruuzlu', 'shade of eranikus',
          'celebras the cursed', 'razorlash', 'tinkerer gizlock'):
    check(names[n] is True, f'drop list names {n}')
check(names['hakkar'] is None and names["the temple of atal'hakkar"] is None, 'places are not sources')

print("== round 2: Forever's own dungeons")
for z in ('Alcaz Prison', 'Blackmaw Hold', 'The Drowned City', 'City of Dalaran', 'Ruins of Lordaeron', 'Hall of Thanes',
          'Excavation Site', 'Excavation Site: Wetlands'):
    check(FC.Route.IsDungeon(z), f'{z} is a dungeon without the Dungeon Journal')
check(not FC.Route.IsDungeon('Dalaran') and not FC.Route.IsDungeon('Lordaeron'), 'the bare place names are not')
# the review's case at 51: Alcaz Prison Waist 2.06% and Back 1.04% are under the 3% dungeon bar
AW, AB = 900501, 900502
FACTS[AW] = dict(name="Alcaz Twin Waist", reqLevel=45, sourceType="boss_drop", npc="Warden", zone="Alcaz Prison", instructions="Drops from Warden.")
FACTS[AB] = dict(name="Alcaz Twin Back", reqLevel=45, sourceType="boss_drop", npc="Warden", zone="Alcaz Prison", instructions="Drops from Warden.")
T51 = total_of(51, "Un'Goro Crater")
rows = equipped_rows(51) + [(AW, 'Waist', 45, 60, SCORE[6] + 0.0206 * T51, 'Alliance', 'protection'),
                            (AB, 'Back', 45, 60, SCORE[15] + 0.0104 * T51, 'Alliance', 'protection')]
_, _, _, c = client(51, rows, FACTS, zone="Un'Goro Crater")
for inv, iid, what in ((6, AW, 'Waist 2.06%'), (15, AB, 'Back 1.04%')):
    b = c.gear.slots[inv].best
    check(not (b and b.id == iid), f'Alcaz Prison {what} hidden under the dungeon bar ({b and b.name})')

print()
print(f'{len(fails)} failed' if fails else 'ALL PASS')
sys.exit(1 if fails else 0)
