#!/usr/bin/env python3
"""Offline harness: runs the ForeverCoach addon in Lua 5.1 (lupa) against a mocked WoW Forever client.
Forever rules are mocked: health is a secret value (math on it throws), no combat log.

  uv run --with lupa python test/harness.py [GearQuestForever_<CLASS> dir] [ForeverDungeonJournal dir]

Each scenario builds a fresh client. Exits non-zero on any failed assertion.
Set SV_OUT=path to write the main scenario's ForeverCoachDB as a SavedVariables file (for the desktop tests)."""
import os, sys
try:
    import lupa.lua51 as lupa
except ImportError:
    import lupa

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..')
ADDON = os.path.join(ROOT, 'addon', 'ForeverCoach')
GQDIR = sys.argv[1] if len(sys.argv) > 1 else None
FDJDIR = sys.argv[2] if len(sys.argv) > 2 else None
FILES = ['Rules', 'Core', 'Route', 'Session', 'Gear', 'Talents', 'Dungeons', 'Audit', 'Macros', 'Reactions', 'UI']

MOCK = r'''
local now = 1000
STATE = { level = 20, class = "WARRIOR", race = "Dwarf", faction = "Alliance", unspent = 2, equipped = {}, bags = {},
          known = { [71] = true }, combat = false, dead = false, xp = 500, xpMax = 10000, rested = 0, zone = "Redridge Mountains",
          talents = { {"Arms", {}}, {"Fury", {}}, {"Protection", { ["Shield Specialization"] = 5, ["Improved Revenge"] = 3 }} },
          reps = {}, skills = { ["First Aid"] = 40 }, trainer = {}, tradeTrainer = false, quests = {}, freeSlots = 10, bagSlots = 4 }
LOCS = {}
function time() return 1700000000 + now end
function GetTime() return now end
function ADVANCE(s) now = now + s end
function UnitName() return "Willtank" end
function GetRealmName() return "TestRealm" end
function UnitClass() return "X", STATE.class end
function UnitRace() return STATE.race, STATE.race end
function UnitFactionGroup() return STATE.faction end
function UnitLevel() return STATE.level end
function UnitXP() return STATE.xp end
function UnitXPMax() return STATE.xpMax end
function GetXPExhaustion() return STATE.rested end
function IsResting() return STATE.resting == true end
function GetMoney() return 12345 end
function UnitCharacterPoints() return STATE.unspent end
function GetNumTalentTabs() return #STATE.talents end
function GetTalentTabInfo(i) local n, t = STATE.talents[i][1], STATE.talents[i][2]; local s = 0; for _, r in pairs(t) do s = s + r end; return n, "icon", s, "bg" end
function GetNumTalents(tab) local n = 0; for _ in pairs(STATE.talents[tab][2]) do n = n + 1 end; return n end
function GetTalentInfo(tab, i) local k = 0; for name, r in pairs(STATE.talents[tab][2]) do k = k + 1; if k == i then return name, "icon", 1, 1, r, 5 end end end
function GetNumSkillLines() local n = 0; for _ in pairs(STATE.skills) do n = n + 1 end; return n end
function GetSkillLineInfo(i) local k = 0; for name, r in pairs(STATE.skills) do k = k + 1; if k == i then return name, false, false, r, 0, 0, 300 end end end
function GetNumFactions() local n = 0; for _ in pairs(STATE.reps) do n = n + 1 end; return n end
function GetFactionInfo(i) local k = 0; for name, st in pairs(STATE.reps) do k = k + 1; if k == i then return name, "", st end end end
local function link(id) return "|cffffffff|Hitem:" .. id .. "::::|h[Item " .. id .. "]|h|r" end
function GetInventoryItemLink(_, slot) local id = STATE.equipped[slot]; return id and link(id) or nil end
function GetInventoryItemID(_, slot) return STATE.equipped[slot] end
C_Container = {
  GetContainerNumSlots = function(b) if b == 0 then return 16 end; return b <= STATE.bagSlots and 0 or 0 end,
  GetContainerNumFreeSlots = function(b) if b == 0 then return STATE.freeSlots end; return 0 end,
  GetContainerItemLink = function(b, s) if b == 0 and STATE.bags[s] then return link(STATE.bags[s]) end end }
C_Item = { GetItemInfoInstant = function(id) return id, "", "", LOCS[id] end, RequestLoadItemDataByID = function() end }
function IsPlayerSpell(id) return STATE.known[id] == true end
-- Spellbook: STATE.book = { {name, rankText, future}, ... }; by default it mirrors STATE.known
-- (names from SPELLNAMES or the rules). STATE.nobook = true: the client has no usable book.
BOOKCALLS = 0
local function ruleName(id)
  local r = ForeverCoach_Rules or {}
  for _, list in ipairs({ r.spellMilestones or {}, r.procSpells or {} }) do
    for _, m in ipairs(list) do
      for _, sid in ipairs(m.spellIds or { m.spellId }) do if sid == id then return m.name end end
    end
  end
end
local function bookEntries()
  if STATE.book then return STATE.book end
  local e = { { "Attack", "" } } -- every real spellbook has the General tab
  for id, v in pairs(STATE.known) do
    local n = v == true and (SPELLNAMES[id] or ruleName(id))
    if n then
      local base, rank = string.match(n, "^(.-) %((Rank %d+)%)$")
      e[#e + 1] = { base or n, rank or "" }
    end
  end
  return e
end
TABCALLS = 0
function GetNumSpellTabs() TABCALLS = TABCALLS + 1; if STATE.nobook then return nil end; return 1 end
function GetSpellTabInfo(t) return "General", "icon", 0, #bookEntries() end
function GetSpellBookItemName(i) BOOKCALLS = BOOKCALLS + 1; local e = bookEntries()[i]; if e then return e[1], e[2] end end
function GetSpellBookItemInfo(i) local e = bookEntries()[i]; if e then return e[3] and "FUTURESPELL" or "SPELL", 0 end end
function UnitAffectingCombat() return STATE.combat end
function UnitIsDeadOrGhost() return STATE.dead end
function UnitOnTaxi() return false end
SECRET = setmetatable({}, {})
local SM = getmetatable(SECRET)
local function boom() error("attempt to perform arithmetic on a secret number value") end
SM.__add, SM.__sub, SM.__mul, SM.__div, SM.__lt, SM.__le, SM.__unm = boom, boom, boom, boom, boom, boom, boom
function issecretvalue(v) return v == SECRET end
function UnitHealth() return SECRET end
function UnitHealthMax() return 300 end
function IsPlayerMoving() return STATE.moving == true end
USABLE = {}
SPELLNAMES = {[6572]="Revenge",[7384]="Overpower",[402927]="Victory Rush",[71]="Defensive Stance",[712]="Summon Succubus",[713]="Summon Incubus"}
C_Spell = { IsSpellUsable = function(n) return USABLE[n] == true, false end, GetSpellInfo = function(id) return { name = SPELLNAMES[id] } end }
function UnitPowerType() return STATE.ptype or 1 end
function UnitPower(_, t) if STATE.secretPower then return SECRET end; return STATE.rage or 0 end
function UnitPowerMax(_, t) if STATE.secretPower then return SECRET end; return 100 end
function GetRealZoneText() return STATE.zone end
function GetSubZoneText() return "" end
function GetNumTrainerServices() return #STATE.trainer end
function GetTrainerServiceInfo(i) return STATE.trainer[i][1], "", STATE.trainer[i][2] end
function IsTradeskillTrainer() return STATE.tradeTrainer end
C_QuestLog = { IsQuestFlaggedCompleted = function(id) return STATE.quests[id] == "done" end, IsOnQuest = function(id) return STATE.quests[id] == "log" end }
C_Timer = { After = function(_, f) f() end }
DEFAULT_CHAT_FRAME = { AddMessage = function(_, m) CHAT[#CHAT + 1] = m end }
CHAT = {}
UISpecialFrames = {}
function tinsert(t, v) table.insert(t, v) end
local function frame()
  local f = { scripts = {}, events = {}, shown = true }
  setmetatable(f, { __index = function(t, k) return function(...) return t end end })
  f.SetScript = function(self, k, fn) self.scripts[k] = fn end
  f.RegisterEvent = function(self, e) if not next(self.events) then FRAMES[#FRAMES + 1] = self end; self.events[e] = true end
  f.IsShown = function(self) return self.shown end
  f.Show = function(self) self.shown = true end
  f.Hide = function(self) self.shown = false end
  f.SetText = function(self, text) self._text = text end
  f.GetText = function(self) return rawget(self, "_text") end
  f.GetName = function(self) return rawget(self, "name") end
  f.CreateFontString = function() return frame() end
  f.GetStringHeight = function() return 10 end
  return f
end
FRAMES = {}
function CreateFrame(_, name)
  local f = frame()
  f.name = name
  if name then _G[name] = f end
  return f
end
UIParent = frame()
SlashCmdList = {}
function FIRE(event, ...)
  for _, f in ipairs(FRAMES) do if f.events[event] and f.scripts.OnEvent then f.scripts.OnEvent(f, event, ...) end end
end
'''

SERIALIZE = r"""function(v)
  local NL, TAB, b = string.char(10), string.char(9), {}
  local function w(x, ind)
    local t = type(x)
    if t == "table" then
      b[#b+1] = "{" .. NL
      for k, val in pairs(x) do
        b[#b+1] = string.rep(TAB, ind + 1) .. "[" .. (type(k) == "string" and string.format("%q", k) or tostring(k)) .. "] = "
        w(val, ind + 1); b[#b+1] = "," .. NL
      end
      b[#b+1] = string.rep(TAB, ind) .. "}"
    elseif t == "string" then b[#b+1] = string.format("%q", x)
    else b[#b+1] = tostring(x) end
  end
  b[#b+1] = NL .. "ForeverCoachDB = "; w(v, 0); b[#b+1] = NL
  return table.concat(b)
end"""

fails = []


def check(cond, msg):
    print(('PASS ' if cond else 'FAIL ') + msg)
    if not cond: fails.append(msg)


GQ_SRC = []
if GQDIR:
    GQ_SRC = [open(os.path.join(GQDIR, fn), encoding='utf-8').read() for fn in sorted(os.listdir(GQDIR)) if fn.endswith('.generated.lua')]
FDJ_SRC = open(os.path.join(FDJDIR, 'Data', 'Dungeons.lua'), encoding='utf-8').read() if FDJDIR else None
ADDON_SRC = {f: open(os.path.join(ADDON, f + '.lua'), encoding='utf-8').read() for f in FILES}


def client(state=None, gq=True, fdj=True, spec_hook=None, pre=None):
    L = lupa.LuaRuntime(unpack_returned_tuples=True)
    L.execute(MOCK)
    if pre: L.execute(pre)  # extra client API before the addon loads
    G = L.globals()
    for k, v in (state or {}).items():
        if isinstance(v, dict):
            t = L.table()
            for kk, vv in v.items(): t[kk] = vv
            G.STATE[k] = t
        elif isinstance(v, list):
            G.STATE[k] = L.table(*v)
        else:
            G.STATE[k] = v
    if gq and GQ_SRC:
        L.execute('GearQuest = { Data = {} }')
        for src in GQ_SRC: L.execute(src)
        L.execute('GearQuest.Spec = { GetDisplaySpec = function(_, c) return SPEC_GQ end, GetDefaultSpec = function() return "arms" end,'
                  ' GetOptions = function() return {{id="arms"},{id="fury"},{id="protection"}} end }')
        # equip locations for picked items: weapons default to one-handers, shields to off-hand
        L.execute('''
          local SL = { Head="INVTYPE_HEAD", Neck="INVTYPE_NECK", Shoulder="INVTYPE_SHOULDER", Back="INVTYPE_CLOAK", Chest="INVTYPE_CHEST",
            Wrist="INVTYPE_WRIST", Hands="INVTYPE_HAND", Waist="INVTYPE_WAIST", Legs="INVTYPE_LEGS", Feet="INVTYPE_FEET",
            Finger="INVTYPE_FINGER", Trinket="INVTYPE_TRINKET", MainHand="INVTYPE_WEAPON", SecondaryHand="INVTYPE_SHIELD", Ranged="INVTYPE_RANGED" }
          for _, list in ipairs({ GearQuest.Data.warriorPicks or {}, GearQuest.Data.warriorEarly1to9 or {} }) do
            for _, r in ipairs(list) do if not LOCS[r[1]] then LOCS[r[1]] = SL[r[2]] end end
          end''')
    if fdj and FDJ_SRC:
        L.execute('ForeverDungeonJournal_NS = { Constants = { ALBA_FAIRMOON_LOCATION = "Sentinel Hill" } }')
        L.execute('return function(...) ' + FDJ_SRC + '\nend')('ForeverDungeonJournal', G.ForeverDungeonJournal_NS)
    ns = L.table()
    for f in FILES:
        L.execute('return function(...) ' + ADDON_SRC[f] + '\nend')('ForeverCoach', ns)
    G.FIRE('ADDON_LOADED', 'ForeverCoach')
    G.FIRE('PLAYER_LOGIN')
    FC = G.ForeverCoach
    return L, G, FC, FC.Char(FC)


def findings(c):
    F = c.findings
    return [F[i] for i in range(1, len(F) + 1)] if F else []


def show(c, n=8):
    for f in findings(c)[:n]:
        print(f'    [{f.area} {f.impact}] {f.text}' + (f'  <{f.where}>' if f.where else '') + f'  ->  {f.fix}')


def refresh(FC):
    FC.Refresh(FC, 'test')


# 0. Visible branding stays separate from stable save, global, frame and command IDs.
print('== Relax Coach branding, no optional providers')
L, G, FC, c = client(gq=False, fdj=False)
check(G.Journey_Character is None and G.GearQuest is None and G.ForeverDungeonJournal_NS is None,
      'standalone scenario has no Journey, GearQuest or DungeonJournal globals')
check(c.lastError is None and len(findings(c)) > 0, 'standalone Coach loads and produces findings')
G.SlashCmdList.FOREVERCOACH('')
check(FC.UI.frame.IsShown(FC.UI.frame), '/coach opens the window')
check(FC.UI.frame.title.GetText(FC.UI.frame.title) == 'Relax Coach', 'window title is Relax Coach')
check(FC.UI.frame.GetName(FC.UI.frame) == 'ForeverCoachFrame' and 'ForeverCoachFrame' in G.UISpecialFrames.values(),
      'window keeps its global frame and Escape registration')
G.SlashCmdList.FOREVERCOACH('')
check(not FC.UI.frame.IsShown(FC.UI.frame), '/coach closes the window')
G.SlashCmdList.FOREVERCOACH('help')
check(G.CHAT[len(G.CHAT)].startswith('|cff4fc3f7Relax Coach|r: '), 'chat prefix is Relax Coach')
check(G.SLASH_FOREVERCOACH1 == '/coach' and '/coach macros' in G.CHAT[len(G.CHAT)], 'slash commands remain /coach')
check(FC.VERSION == '1.5.4' and G.ForeverCoachDB.addonVersion == '1.5.4', 'addon and saved version are 1.5.4')
check(FC.SCHEMA == 1 and G.ForeverCoachDB.schema == 1 and G.RelaxCoachDB is None and G.RelaxCoach is None,
      'SavedVariables, global and schema remain compatible')
L.execute('''
  local db = ForeverCoachDB
  db.privateSetting = "retained"
  local old = ForeverCoach:Char()
  old.specOverride = "arms"
  old.sessions = {{xp = 123, played = 300}}
  db.addonVersion = "1.3.1"
  FIRE("ADDON_LOADED", "ForeverCoach")
  SAVES_RETAINED = ForeverCoachDB == db and ForeverCoach:Char() == old
    and db.privateSetting == "retained" and old.specOverride == "arms"
    and #old.sessions == 1 and old.sessions[1].xp == 123
''')
check(G.SAVES_RETAINED, 'existing character settings and session history are retained')
check(c.snapshot.journey is None, 'no Journey context when the optional addon is absent')
L.execute('Journey_Character = {lastGuide="Test guide", bookmarks={["Test guide"]={stepID="42"}}}')
refresh(FC)
check(c.snapshot.journey.guide == 'Test guide' and c.snapshot.journey.step == '42',
      'optional guide context still reads the legacy Journey global')

# 1. Main: level 20 Dwarf prot warrior in Redridge, naked
print('== scenario: level 20 Dwarf protection warrior, Redridge, naked')
L, G, FC, c = client()
show(c)
texts = [f.text for f in findings(c)]
check(any('2 unspent talent points' in t for t in texts), 'unspent talents detected')
check(not any('Defensive Stance' in t for t in texts), 'known Defensive Stance not flagged')
check(any('not learned' in t and 'Revenge' in t for t in texts), 'Revenge (14) flagged as missing for prot')
check(c.lastError is None, f'no internal error ({c.lastError and c.lastError.err})')
check(all(1 <= f.impact <= 100 for f in findings(c)), 'impacts clamped to 1..100')
tp = c.talentPlan
check(tp is not None and tp.next is not None, 'talent plan found for WARRIOR:protection')
if tp and tp.next:
    print('   talent next:', tp.next.level, tp.next.name, '| unspent fix:', [f.fix for f in findings(c) if 'unspent' in f.text][0])
    check(tp.next.level == 18, f'talent plan: Shield Spec + Imp Revenge done, next is level 18 ({tp.next.level})')
if GQ_SRC:
    g = c.gear
    check(g.status == 'ok', f'gear status ok ({g.status})')
    check(g.spec == 'protection' and g.specSource == 'talents', f'spec from talents ({g.spec}/{g.specSource})')
    r1, r2 = g.slots[11].best, g.slots[12].best
    check(not (r1 and r2 and r1.id == r2.id), f'rings never recommend the same item twice ({r1 and r1.name} / {r2 and r2.name})')
    bests = [g.slots[s].best for s in range(1, 19) if g.slots[s] and g.slots[s].best]
    check(not any('Elunite' in (b.name or '') for b in bests), 'no Night Elf-only reward for Dwarf')
    check(not any(b.kind == 'denied' for b in bests), 'no items from Horde-only zones')
    mh = g.slots[16].best
    print('   best MainHand:', mh.name, '|', mh.where)
    G.STATE.equipped[16] = mh.id
    refresh(FC)
    check(c.gear.slots[16].gapPct == 0, 'equipping best MH clears gap')
    # unknown equipped item must not become a 100% gap
    G.STATE.equipped[5] = 999999
    refresh(FC)
    e5 = c.gear.slots[5]
    check(e5.scoreStatus == 'unknown' and e5.gapPct is None, f'unknown chest is "unknown", not 100% behind ({e5.scoreStatus}, {e5.gapPct})')
    check(not any('Chest' in f.text for f in findings(c)), 'no chest finding for an unknown item')
    # owned upgrade in bags ranks first among gear
    chest_best = c.gear.slots[5].best
    G.STATE.equipped[5] = None
    G.STATE.bags[1] = chest_best.id
    G.LOCS[chest_best.id] = 'INVTYPE_CHEST'
    refresh(FC)
    gearf = [f for f in findings(c) if f.area == 'Gear']
    check(gearf and gearf[0].text.startswith('Upgrade in your bags'), f'bag upgrade is the top gear tip ({gearf[0].text if gearf else None})')
    G.STATE.bags[1] = None
    # /coach spec validation
    CH = len(G.CHAT)
    G.SlashCmdList.FOREVERCOACH('spec tankk')
    check(c.specOverride is None and 'unknown spec' in (G.CHAT[len(G.CHAT)] or ''), 'invalid /coach spec rejected')
    G.SlashCmdList.FOREVERCOACH('spec arms')
    check(c.gear.spec == 'arms' and c.gear.specSource == 'manual', 'manual spec override')
    G.SlashCmdList.FOREVERCOACH('spec auto')

# trainer: purchases clear the reminder; profession trainers ignored
G.STATE.trainer = L.table(L.table('Rend', 'available'), L.table('Thunder Clap', 'available'))
G.FIRE('TRAINER_SHOW'); refresh(FC)
check(any('left to train' in f.text for f in findings(c)), 'trainer: available skills reported')
G.STATE.trainer = L.table(L.table('Rend', 'used'), L.table('Thunder Clap', 'used'))
G.FIRE('TRAINER_UPDATE'); G.FIRE('TRAINER_CLOSED')
check(not any('left to train' in f.text for f in findings(c)), 'trainer: reminder clears after buying')

# Session: secret health, procs via usability, AFK split, XP across level-up
S = FC.Session
G.STATE.known[6572] = True
S.Start(S)
G.STATE.combat = True
for i in range(10):
    G.USABLE['Revenge'] = True; S.PollProcs(S)
    if i < 3:
        G.USABLE['Revenge'] = False; S.PollProcs(S)
        G.FIRE('UNIT_SPELLCAST_SUCCEEDED', 'player', 'cast-guid', 6572)
    G.ADVANCE(6); S.PollProcs(S); S.Tick(S, 6)
    G.USABLE['Revenge'] = False; S.PollProcs(S)
G.FIRE('PLAYER_DEAD'); G.FIRE('PLAYER_DEAD')
G.STATE.combat = False; G.STATE.moving = False
for _ in range(60): S.Tick(S, 1)
G.STATE.moving = True
for _ in range(500): S.Tick(S, 1)
G.STATE.moving = False
S.Tick(S, 121)  # one long frame: must not double count
cur = S.Current(S)
check(abs(cur.recovery - 60) < 2 and abs(cur.afk - 121) < 1, f'AFK split exact for long frames (stopped {cur.recovery}, afk {cur.afk})')
G.STATE.moving = True
for _ in range(400): S.Tick(S, 1)
# XP across a level-up: 9500/10000 at 20, +1600 -> level 21 with 1100/11000
G.STATE.xp = 9500; G.FIRE('PLAYER_XP_UPDATE')
before = cur.xp
G.FIRE('PLAYER_LEVEL_UP', 21)
G.STATE.level = 21; G.STATE.xp = 1100; G.STATE.xpMax = 11000
G.FIRE('PLAYER_XP_UPDATE')
check(cur.xp - before == 1600, f'XP across level-up counted ({cur.xp - before})')
G.STATE.level = 20; G.STATE.xp = 500; G.STATE.xpMax = 10000
refresh(FC)
show(c, 20)
texts = [f.text for f in findings(c)]
check(any(t.startswith('Revenge: cast 3 of 10') for t in texts), 'Revenge usage 3/10 via usability')
check(any('deaths per hour' in t for t in texts), 'death rate flagged')
check(c.lastError is None, f'no refresh error ({c.lastError and c.lastError.err})')
G.FIRE('PLAYER_LOGOUT')
check(len(c.sessions) == 1, 'session saved on logout')
out = os.environ.get('SV_OUT')
if out:
    os.makedirs(os.path.dirname(out), exist_ok=True)
    open(out, 'w', encoding='utf-8').write(L.eval(SERIALIZE)(G.ForeverCoachDB))
    print('wrote', out)

# 2. Tarek now: level 10 Dwarf in Dun Morogh. Rusty Cutlass (no zone) must not beat a route item.
if GQ_SRC:
    print('== scenario: level 10 Dwarf in Dun Morogh')
    L, G, FC, c = client({'level': 10, 'zone': 'Dun Morogh', 'unspent': 1, 'xp': 100, 'xpMax': 6500,
                          'talents': [['Arms', {}], ['Fury', {}], ['Protection', {}]]})
    show(c, 10)
    mh = c.gear.slots[16].best
    print('   MainHand pick:', mh.name, '|', mh.where)
    check(mh.name != 'Rusty Cutlass', 'Rusty Cutlass (location unknown) no longer the top main hand')
    check(c.gear.specSource == 'default', 'no talents yet -> default spec, flagged')
    t = [f.text for f in findings(c)]
    check(any('default spec' in x for x in t), 'default-spec warning shown at 10')
    dg = c.dungeons
    names = [dg.list[i].name for i in range(1, len(dg.list) + 1)] if dg and dg.list else []
    print('   dungeons:', names)
    if FDJ_SRC:
        check('Hall of Thanes' in names, 'Hall of Thanes suggested near level 10-13 for a Dwarf')
        check('Ragefire Chasm' not in names, 'Horde-only Ragefire Chasm hidden for Alliance')

# 3. Level 1: no gear nagging
print('== scenario: level 1')
L, G, FC, c = client({'level': 1, 'zone': 'Coldridge Valley', 'unspent': 0, 'talents': [['Arms', {}], ['Fury', {}], ['Protection', {}]]})
gear_tips = [f for f in findings(c) if f.area == 'Gear']
check(not gear_tips, f'no gear findings at level 1 ({[f.text for f in gear_tips][:3]})')

# 4. Druid at rage cap: no warrior advice
print('== scenario: druid bear with rage capped, priest standing around')
L, G, FC, c = client({'class': 'DRUID', 'race': 'NightElf', 'level': 20, 'zone': 'Darkshore', 'rage': 100, 'known': {5487: True},
                      'talents': [['Balance', {}], ['Feral Combat', {'Thick Hide': 5}], ['Restoration', {}]]}, gq=False)
S = FC.Session; S.Start(S); G.STATE.combat = True
for _ in range(700): S.Tick(S, 1)
refresh(FC)
t = ' '.join((f.text or '') + ' ' + (f.fix or '') for f in findings(c))
check('Heroic Strike' not in t and 'Victory Rush' not in t, 'no warrior-only advice for a druid')
check(any('GearQuest' in f.text for f in findings(c)), 'missing GearQuest shown as checks off')
check(c.gear.status == 'missing', 'gear status missing')

# 5. Warlock with Incubus instead of Succubus
L, G, FC, c = client({'class': 'WARLOCK', 'race': 'Human', 'level': 22, 'zone': 'Duskwood', 'known': {697: True, 713: True, 712: False},
                      'talents': [['Affliction', {}], ['Demonology', {}], ['Destruction', {}]]}, gq=False)
check(not any('Succubus' in f.text for f in findings(c)), 'Incubus counts for the level 20 demon')

# 6. Secret power + level 60
L, G, FC, c = client({'level': 60, 'secretPower': True, 'zone': 'Silithus', 'unspent': 0}, gq=False)
S = FC.Session; S.Start(S); G.STATE.combat = True
for _ in range(700): S.Tick(S, 1)
refresh(FC)
check(c.lastError is None, f'secret power at 60: no error ({c.lastError and c.lastError.err})')
check(not any('rested' in f.text.lower() for f in findings(c)), 'no rested tip at 60')

# 7. Corrupt GearQuest rows don't crash or wipe other advice
if GQ_SRC:
    L, G, FC, c = client({'level': 12, 'zone': 'Dun Morogh'})
    L.execute('table.insert(GearQuest.Data.warriorPicks, 1, { "x", "MainHand", "1", 60, 1, "protection", "Alliance", 5 }); table.insert(GearQuest.Data.warriorPicks, 1, {})')
    refresh(FC)
    check(c.lastError is None and c.findings is not None and len(findings(c)) > 0, 'malformed GearQuest rows ignored')

# 8. Baleful: Skyborne druid, level 10 on Zephras Isle, Bear Form not learned yet
print('== scenario: level 10 Skyborne druid on Zephras Isle')
L, G, FC, c = client({'class': 'DRUID', 'race': 'Skyborne', 'faction': 'Alliance', 'level': 10, 'zone': 'Zephras Isle', 'unspent': 1,
                      'known': {5487: False, 768: False}, 'talents': [['Balance', {}], ['Feral Combat', {}], ['Restoration', {}]]}, gq=False)
show(c, 10)
t = [(f.text or '') + ' | ' + (f.fix or '') for f in findings(c)]
bear = [x for x in t if x.startswith('Bear Form')]
check(len(bear) == 1 and "Urs'endris" in bear[0], f'Skyborne gets the Zephras Isle Bear Form quest, once ({bear})')
check(not any('Darnassus' in x or 'Thunder Bluff' in x for x in bear), 'no Night Elf/Tauren Bear Form route for Skyborne')
check(not any(x.startswith('Cat Form') for x in t), 'Cat Form (20) not nagged at 10')
route = FC.Route.ForPlayer(FC.Route, c.snapshot)
check(route['Zephras Isle'] is not None, 'Zephras Isle on the Skyborne route')
check(c.lastError is None, f'no internal error ({c.lastError and c.lastError.err})')
L.execute('ForeverCoach:Char().specOverride = "feral"'); refresh(FC)
check(c.talentPlan is not None and 'Feral' in c.talentPlan.source, 'feral talent plan available')
check(any('Ferocity' in (f.fix or '') for f in findings(c)), 'unspent point names the next Feral talent (Ferocity)')

# 9. Every class and spec: plan builds, no errors, milestone tips only for own class
import json as _json
RULES = _json.load(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'rules', 'rules.json'), encoding='utf-8'))
TREES = {}
for key, b in RULES['talentBuilds'].items():
    TREES.setdefault(key.split(':')[0], set()).add(b['tree'])
print('== sweep: every class/spec at levels 12, 30, 55')
sweep_fail = []
for key, b in sorted(RULES['talentBuilds'].items()):
    cls, spec = key.split(':')
    race = {'PALADIN': 'Human', 'SHAMAN': 'Orc', 'DRUID': 'Skyborne', 'HUNTER': 'Skyborne'}.get(cls, 'Human' if cls != 'WARLOCK' else 'Gnome')
    fac = 'Horde' if race == 'Orc' else 'Alliance'
    for lvl in (12, 30, 55):
        spent = {}
        for L_, name in b['order']:
            if L_ <= lvl: spent[name] = spent.get(name, 0) + 1
        tabs = [[tree, ({n: r for n, r in spent.items()} if tree == b['tree'] else {})] for tree in sorted(TREES[cls])]
        L, G, FC, c = client({'class': cls, 'race': race, 'faction': fac, 'level': lvl, 'zone': 'Ashenvale', 'unspent': 0, 'talents': tabs}, gq=False)
        L.execute(f'ForeverCoach:Char().specOverride = "{spec}"'); refresh(FC)
        ok = c.lastError is None and c.talentPlan is not None
        other = [f.text for f in findings(c) if any(m['name'] in (f.text or '') for m in RULES['spellMilestones'] if m['class'] != cls and m['name'] not in ('Shoot',))
                 and not any(m['name'] in (f.text or '') for m in RULES['spellMilestones'] if m['class'] == cls)]
        if not ok or other:
            sweep_fail.append((key, lvl, c.lastError and c.lastError.err, other[:2]))
check(not sweep_fail, f'{len(RULES["talentBuilds"])} specs x 3 levels: plan built, no errors, no other-class tips {sweep_fail[:4]}')

# 10. /coach macros: creates, updates its own (FC ) macros, never touches others, blocked in combat
print('== /coach macros')
L, G, FC, c = client(gq=False)
L.execute('''
  MACROS = { {name="FC Taunt", body="old shift version"}, {name="My Taunt", body="mine"} }
  function GetMacroIndexByName(n) for i, m in ipairs(MACROS) do if m.name == n then return i end end return 0 end
  function GetMacroInfo(i) local m = MACROS[i]; return m.name, "icon", m.body end
  function GetNumMacros() return 0, #MACROS end
  function CreateMacro(n, icon, body, perChar)
    assert(#n <= 16, "name too long: " .. n); assert(#body <= 255, "body too long: " .. n); assert(perChar == true)
    MACROS[#MACROS + 1] = {name=n, body=body}; return #MACROS
  end
  function EditMacro(i, n, icon, body) assert(#body <= 255); MACROS[i].body = body; return i end
  INCOMBAT = false
  function InCombatLockdown() return INCOMBAT end
''')
SlashCmd = G.SlashCmdList.FOREVERCOACH
SlashCmd('macros')
ms = G.MACROS
names = [ms[i].name for i in range(1, len(ms) + 1)]
bodies = ' '.join(ms[i].body for i in range(1, len(ms) + 1))
check(len(names) == 10 and names.count('FC Taunt') == 1, f'8 new warrior macros + 1 updated ({names})')
check('mod:ctrl' in ms[1].body and 'mod:shift' not in bodies, 'old FC Taunt updated to Ctrl; no Shift modifiers left')
check(ms[2].body == 'mine', "user's own macro untouched")
SlashCmd('macros')
check(len(G.MACROS) == 10, 'running twice creates no duplicates')
L.execute('INCOMBAT = true; MACROS = {}'); SlashCmd('macros')
check(len(G.MACROS) == 0 and any('combat' in str(m) for m in G.CHAT.values()), 'blocked in combat with a message')
check(c.lastError is None, 'no internal error')
sets = L.eval('ForeverCoach.Macros.SETS')
bad, total = [], 0
for cls in ['WARRIOR', 'PALADIN', 'HUNTER', 'ROGUE', 'PRIEST', 'SHAMAN', 'MAGE', 'WARLOCK', 'DRUID']:
    st = sets[cls]
    if not st: bad.append((cls, 'missing')); continue
    seen = set()
    for i in range(1, len(st) + 1):
        n, lv, b = st[i][1], st[i][2], st[i][3]; total += 1
        if len(n) > 16 or len(b) > 255 or n in seen or 'mod:shift]' in b.split(';')[0] and 'mod:ctrl' not in b: bad.append((cls, n))
        seen.add(n)
    if len(seen) > 30: bad.append((cls, 'over 30 macros'))
check(not bad and total >= 80, f'{total} macros for all 9 classes: names <=16, bodies <=255, unique ({bad})')
for cls in ['DRUID', 'MAGE']:
    L.execute(f'INCOMBAT = false; MACROS = {{}}; STATE.class = "{cls}"'); SlashCmd('macros')
    check(len(G.MACROS) == len(sets[cls]), f'{cls}: /coach macros creates its set')

# 11. Logout can occur after the client has torn down inventory/XP APIs.
print('== logout must not replace a good snapshot with cleared client data')
L, G, FC, c = client({'equipped': {5: 123456, 16: 654321}, 'bags': {1: 11111}}, gq=False)
before = L.eval('function(v) return v end')(c.snapshot)
G.ADVANCE(90)
FC.Session.Tick(FC.Session, 90)
L.execute('''
  function GetInventoryItemLink() return nil end
  function GetInventoryItemID() return nil end
  C_Container.GetContainerNumSlots = function() return 0 end
  C_Container.GetContainerNumFreeSlots = function() return 0 end
  function GetMoney() return 0 end
  STATE.xp, STATE.xpMax = 0, 0
''')
G.FIRE('PLAYER_LOGOUT')
check(c.snapshot.equipped[5] is not None and c.snapshot.equipped[5].id == 123456,
      'logout retains equipped chest after inventory API becomes empty')
check(c.snapshot.xpMax == 10000 and c.snapshot.money == 12345 and c.snapshot.baginfo.slots == 16,
      'logout retains valid XP, money and bags instead of teardown zeros')
check(len(c.sessions) == 1 and c.sessions[1].finish is not None,
      'logout still finalizes the actual play session')
check(c.lastError is None, 'logout preservation has no internal error')

# 12. Item names/links are cached separately from the equipped item's identity.
print('== uncached new item IDs stay equipped for every class')
uncached_fail = []
for cls in ['WARRIOR', 'PALADIN', 'HUNTER', 'ROGUE', 'PRIEST', 'SHAMAN', 'MAGE', 'WARLOCK', 'DRUID']:
    L, G, FC, c = client({'class': cls, 'equipped': {5: 987654, 16: 987655}}, gq=False)
    L.execute('''
      function GetInventoryItemID(_, slot) return STATE.equipped[slot] end
      function GetInventoryItemLink() return nil end
      REQUESTED = {}
      C_Item.RequestLoadItemDataByID = function(id) REQUESTED[id] = true end
    ''')
    refresh(FC)
    chest = c.snapshot.equipped[5]
    if chest is None or chest.id != 987654 or not G.REQUESTED[987654] or c.lastError is not None:
        uncached_fail.append(cls)
check(not uncached_fail, f'uncached inventory IDs are retained and requested for all 9 classes ({uncached_fail})')

# 13. API failure is not evidence of an empty equipment slot.
print('== unreadable equipment differs from a confirmed empty slot')
L, G, FC, c = client(gq=False)
L.execute("""
  GearQuest = {Data={warriorPicks={{99991,"Chest",1,60,1,"protection","Alliance",100}},
    warriorItemFacts={[99991]={name="Test chest",sourceType="world",instructions="Synthetic test source"}}}}
  function GetInventoryItemID() error("inventory unavailable") end
  function GetInventoryItemLink() error("inventory unavailable") end
""")
refresh(FC)
check(c.gear.slots[5].scoreStatus == 'unreadable' and c.gear.slots[5].gapPct is None,
      'failed inventory APIs never produce an empty slot or a 100% gap')
check(not any('Empty Chest' in f.text for f in findings(c)), 'no false empty-chest advice after API failure')
L.execute('function GetInventoryItemID() return nil end; function GetInventoryItemLink() return nil end')
refresh(FC)
check(c.gear.slots[5].scoreStatus == 'empty', 'a successful nil inventory-ID read confirms a genuinely empty slot')
check(c.lastError is None, 'unreadable inventory is handled without an addon error')

# 14. Loading-screen reads must preserve the last known inventory.
print('== out-of-world capture waits and recovers on entering world')
L, G, FC, c = client({'equipped': {5: 12345}}, gq=False)
L.execute('WORLD_READY = false; function IsPlayerInWorld() return WORLD_READY end; STATE.equipped = {}')
refresh(FC)
check(c.snapshot.equipped[5] is not None and c.snapshot.equipped[5].id == 12345,
      'loading-screen capture cannot wipe last known gear')
L.execute('WORLD_READY = true; STATE.equipped = {[5]=54321}')
G.FIRE('PLAYER_ENTERING_WORLD', False, False)
check(c.snapshot.equipped[5] is not None and c.snapshot.equipped[5].id == 54321,
      'entering-world event captures fresh inventory after loading')
check(c.lastError is None, 'temporarily unavailable world is not reported as an addon exception')

# 15. The asynchronous item-data event resolves names without losing identity.
print('== item-load completion refreshes the equipped hyperlink')
L, G, FC, c = client({'equipped': {5: 987654}}, gq=False)
L.execute('function GetInventoryItemLink() return nil end')
refresh(FC)
L.execute('function GetInventoryItemLink(_, slot) if slot == 5 then return "|Hitem:987654::::|h[New Forever Test Chest]|h" end end')
G.FIRE('ITEM_DATA_LOAD_RESULT', 987654, True)
check(c.snapshot.equipped[5].link is not None and 'New Forever Test Chest' in c.snapshot.equipped[5].link,
      'item-data completion replaces pending name with the game-provided hyperlink')

# 16. Unknown-to-provider is not the same as uncached game data.
print('== linked items outside provider tables remain equipped and unscored')
L, G, FC, c = client({'equipped': {5: 987654}}, gq=False)
L.execute("""
  GearQuest = {Data={warriorPicks={{99991,"Chest",1,60,1,"protection","Alliance",100}},
    warriorItemFacts={[99991]={name="Test chest",sourceType="world",instructions="Synthetic test source"}}}}
  REQUEST_COUNT = 0
  C_Item.RequestLoadItemDataByID = function() REQUEST_COUNT = REQUEST_COUNT + 1 end
""")
refresh(FC)
check(c.snapshot.equipped[5].id == 987654 and c.gear.slots[5].scoreStatus == 'unknown'
      and c.gear.slots[5].gapPct is None, 'unknown provider item stays occupied without an invented score')
check(G.REQUEST_COUNT == 0, 'cached item absent from scoring tables is not re-requested endlessly')

# 17. Stat-weight estimates and pipeline recommendation scores are different units.
print('== incomparable fallback scores are never turned into upgrade percentages')
L.execute('SCORE_CALLS=0; GearQuest.Compare={ScoreItemStats=function() SCORE_CALLS=SCORE_CALLS+1; return 2.15 end}')
refresh(FC)
check(c.gear.slots[5].scoreStatus == 'unknown' and c.gear.slots[5].score is None
      and c.gear.slots[5].gapPct is None, 'an occupied unindexed item never mixes stat weights with pipeline scores')
check(G.SCORE_CALLS == 0, 'provider preview/stat scorer is not used as a comparable pipeline score')

# 18. Level-up checklist: once per level, never in combat, built from existing findings, can be turned off.
print('== level-up checklist')
L, G, FC, c = client({'level': 20, 'unspent': 1, 'combat': False})
LU = FC.LevelUp
check(LU.Enabled(LU), 'level-up checklist is on by default')
check(c.levelUpShown is None and LU.frame is None, 'nothing pops up on a normal login')
G.FIRE('PLAYER_LEVEL_UP', 20)
check(c.levelUpShown == 20 and LU.frame is not None and LU.frame.IsShown(LU.frame), 'checklist opens after a level-up')
rows = [LU.rows[i] for i in range(1, len(LU.rows) + 1)]
kinds = [r.kind for r in rows]
print('   rows:', [(r.kind, r.text) for r in rows])
check(kinds and kinds[0] == 'talent' and 'unspent talent point' in rows[0].text, 'unspent talent point is the first row')
check(len([k for k in kinds if k == 'gear']) <= 2 and len([k for k in kinds if k in ('train', 'classquest')]) <= 2,
      'at most 2 training and 2 gear rows')
check(all(k in ('talent', 'train', 'classquest', 'gear', 'dungeon') for k in kinds), 'only checklist kinds are shown')
title = LU.frame.title.GetText(LU.frame.title)
check(title.startswith('Level 20'), f'title names the new level ({title})')
check(not LU.frame.guide.IsShown(LU.frame.guide), 'Pick my guide is hidden when Quest Planner is not loaded')
LU.frame.Hide(LU.frame)
G.FIRE('PLAYER_LEVEL_UP', 20)
check(not LU.frame.IsShown(LU.frame), 'same level does not pop up twice')
G.STATE.level = 21; G.STATE.combat = True
G.FIRE('PLAYER_LEVEL_UP', 21)
check(not LU.frame.IsShown(LU.frame) and LU.pending, 'in combat: waits instead of popping up')
G.STATE.combat = False
G.FIRE('PLAYER_REGEN_ENABLED')
check(LU.frame.IsShown(LU.frame) and c.levelUpShown == 21 and not LU.pending, 'opens once combat ends')
LU.frame.Hide(LU.frame)
L.execute('SlashCmdList.RELAXQUESTPLANNER = function(m) RQP_CALLED = m end')
G.SlashCmdList.FOREVERCOACH('levelup')
check(LU.frame.IsShown(LU.frame) and LU.frame.guide.IsShown(LU.frame.guide), '/coach levelup reopens it; guide button shows with Quest Planner')
LU.frame.guide.scripts.OnClick(LU.frame.guide)
check(G.RQP_CALLED == 'guide' and not LU.frame.IsShown(LU.frame), 'Pick my guide calls /rqp guide and closes')
G.SlashCmdList.FOREVERCOACH('levelup off')
check(not LU.Enabled(LU) and G.ForeverCoachDB.levelUpChecklist is False, '/coach levelup off saves the setting')
LU.frame.Hide(LU.frame)
G.STATE.level = 22
G.FIRE('PLAYER_LEVEL_UP', 22)
check(not LU.frame.IsShown(LU.frame) and c.levelUpShown == 21, 'turned off: no pop-up on the next level')
G.SlashCmdList.FOREVERCOACH('levelup on')
check(LU.Enabled(LU), '/coach levelup on turns it back on')
L.execute('FC_EMPTY = ForeverCoach.LevelUp:Build({ findings = {} })')
check(len(G.FC_EMPTY) == 0, 'no findings -> empty checklist (prints "all set" instead of a window)')
check(c.lastError is None, f'no errors recorded ({c.lastError and c.lastError.err})')


# 14. Superseded spell ranks (Tarek, level 35 Protection warrior): the rank-1 IDs answer false once a
# higher rank is learned; the spellbook name decides. Unknown stays unknown.
print('== scenario: level 35 prot warrior with higher spell ranks')
L, G, FC, c = client({'level': 35, 'unspent': 0, 'known': {71: True, 355: True, 2458: True}}, gq=False)
L.execute("""STATE.book = { {"Attack",""}, {"Defensive Stance",""}, {"Battle Stance",""}, {"Berserker Stance",""}, {"Taunt",""},
  {"Sunder Armor","Rank 4"}, {"Revenge","Rank 3"}, {"Overpower","Rank 2"}, {"Shield Bash","Rank 2"},
  {"Tactical Mastery",""}, {"Shield Block",""}, {"Slam","Rank 2"}, {"Victory Rush",""}, {"Shield Wall",""} }""")
G.BOOKCALLS = 0
refresh(FC)
t = [(f.text or '') for f in findings(c)]
miss = [x for x in t if 'not learned' in x]
print('   not learned:', miss)
for name in ('Sunder Armor', 'Revenge', 'Overpower', 'Shield Bash'):
    check(not any(name in x for x in miss), f'{name} known at a higher rank is not flagged')
check(any('Berserker Rage' in x for x in miss), 'Berserker Rage (quest done, not trained) is still flagged')
check(c.snapshot.spells[7386] is True and c.snapshot.spells[18499] is False, 'snapshot: Sunder known by name, Berserker Rage not known')
check(G.BOOKCALLS == 14, f'one spellbook pass per snapshot ({G.BOOKCALLS} reads for 14 entries)')
L.execute('table.insert(STATE.book, {"Berserker Rage", "", true})'); refresh(FC)
check(any('Berserker Rage' in (f.text or '') and 'not learned' in (f.text or '') for f in findings(c)), 'a spell not learned yet (future) in the book does not count')
L.execute('STATE.nobook = true'); refresh(FC)
miss = [(f.text or '') for f in findings(c) if 'not learned' in (f.text or '')]
check(not any(n in x for x in miss for n in ('Sunder Armor', 'Revenge', 'Overpower', 'Berserker Rage')), f'no usable spellbook: unknown, not missing ({miss})')
check(c.snapshot.spells[7386] is None, 'no usable spellbook: no answer recorded')
check(c.lastError is None, f'no internal error ({c.lastError and c.lastError.err})')
L.execute('STATE.nobook = false; GetSpellBookItemName = function() error("broken client") end'); refresh(FC)
check(c.lastError is None and c.snapshot.spells[7386] is None, 'a failing spellbook API is caught and means unknown')

print('== scenario: level 14 mage with Frost Nova rank 2')
L, G, FC, c = client({'class': 'MAGE', 'race': 'Human', 'level': 14, 'unspent': 0, 'talents': [['Arcane', {}], ['Fire', {}], ['Frost', {}]]}, gq=False)
L.execute("""STATE.book = { {"Attack",""}, {"Fireball","Rank 3"}, {"Frost Nova","Rank 2"}, {"Conjure Water","Rank 2"}, {"Conjure Food","Rank 2"},
  {"Polymorph",""}, {"Slow Fall",""}, {"Comprehend Scroll",""} }""")
refresh(FC)
miss = [(f.text or '') for f in findings(c) if 'not learned' in (f.text or '')]
print('   not learned:', miss)
check(not any('Frost Nova' in x or 'Conjure Water' in x for x in miss), 'mage: ranked Frost Nova / Conjure Water known by name')
check(any('Arcane Explosion' in x for x in miss), 'mage: Arcane Explosion (not in the book) still flagged')

print('== scenario: level 44 rogue, "Shank (Rank 2)" needs rank 2')
L, G, FC, c = client({'class': 'ROGUE', 'race': 'Human', 'level': 44, 'unspent': 0, 'talents': [['Assassination', {}], ['Combat', {}], ['Subtlety', {}]]}, gq=False)
ROGUE_BOOK = ''.join('{%s,""}, ' % _json.dumps(m['name']) for m in RULES['spellMilestones'] if m['class'] == 'ROGUE' and m['level'] <= 44 and not m['name'].startswith('Shank'))
L.execute('STATE.book = { {"Attack",""}, ' + ROGUE_BOOK + '{"Shank","Rank 1"} }'); refresh(FC)
print('   not learned:', [(f.text or '') for f in findings(c) if 'not learned' in (f.text or '')])
miss = [(f.text or '') for f in findings(c) if 'not learned' in (f.text or '')]
check(any('Shank (Rank 2)' in x for x in miss), 'rogue: Shank rank 1 only -> Shank (Rank 2) flagged')
check(not any(('Shank,' in x or x.endswith('Shank') or ': Shank' in x) and 'Rank 2' not in x for x in miss), 'rogue: base Shank counts as learned')
L.execute('STATE.book = { {"Attack",""}, ' + ROGUE_BOOK + '{"Shank","Rank 2"} }'); refresh(FC)
miss = [(f.text or '') for f in findings(c) if 'not learned' in (f.text or '')]
check(not any('Shank' in x for x in miss), f'rogue: Shank rank 2 -> neither Shank tip ({[x for x in miss if "Shank" in x]})')
check(c.lastError is None, f'no internal error ({c.lastError and c.lastError.err})')


print('== scenario: spellbook failure paths (review S1-S3)')
L, G, FC, c = client({'level': 35, 'unspent': 0, 'known': {71: True, 355: True, 2458: True}}, gq=False)
L.execute("""STATE.book = { {"Attack",""}, {"Defensive Stance",""}, {"Sunder Armor","Rank 4"}, {"Revenge","Rank 3"}, {"Overpower","Rank 2"} }""")
# S1: the name lookup fails for one spell -> only that spell is unanswered
L.execute("""local lower = string.lower
  string.lower = function(s) if s == "Berserker Rage" then error("lookup failed") end return lower(s) end""")
refresh(FC)
check(c.lastError is None, f'S1: a failing name lookup does not break the snapshot ({c.lastError and c.lastError.err})')
check(c.snapshot.spells[18499] is None and c.snapshot.spells[7386] is True, 'S1: only that spell has no answer; the others still do')
check(not any('Berserker Rage' in (f.text or '') for f in findings(c)), 'S1: no warning for the spell without an answer')
# S2: a spell learned a moment after a scan is seen by SPELLS_CHANGED; one scan per burst
L, G, FC, c = client({'level': 35, 'unspent': 0, 'known': {71: True}}, gq=False)
L.execute('C_Timer = { After = function() end }')  # keep the delayed refresh out of the counts
L.execute('STATE.book = { {"Attack",""}, {"Revenge","Rank 3"} }')
FC.SpellBook(FC)
check(FC.spellBookCache['overpower'] is None, 'S2: Overpower not in the first book')
L.execute('table.insert(STATE.book, {"Overpower","Rank 2"})'); G.ADVANCE(0.3)
G.FIRE('SPELLS_CHANGED')
check(FC.spellBookCache['overpower'] == 2, 'S2: SPELLS_CHANGED reads the current book, not the 0.3 s old one')
L.execute('STATE.nobook = true; ForeverCoach.spellBookCache = nil'); G.ADVANCE(1); G.TABCALLS = 0
for _ in range(20):
    G.FIRE('SPELLS_CHANGED'); G.ADVANCE(0.01)
check(G.TABCALLS == 1, f'S2: a failed scan is reused for a second ({G.TABCALLS} scans for 20 events)')
# S3: a partly readable book (secret count on the class tab) is unknown, not "not learned"
L, G, FC, c = client({'level': 35, 'unspent': 0, 'known': {71: True, 355: True, 2458: True}}, gq=False)
L.execute("""STATE.book = { {"Attack",""}, {"Sunder Armor","Rank 4"} }
  GetNumSpellTabs = function() return 2 end
  GetSpellTabInfo = function(t) if t == 1 then return "General", "i", 0, 1 end; return "Warrior", "i", 1, SECRET end""")
refresh(FC)
miss = [(f.text or '') for f in findings(c) if 'not learned' in (f.text or '')]
check(not miss, f'S3: secret class-tab count -> no "not learned" warning ({miss})')
check(c.snapshot.spells[6572] is None and c.lastError is None, 'S3: unknown, no error')
L.execute('GetSpellTabInfo = function(t) if t == 1 then return "General", "i", 0, 1 end; return "Warrior", "i", 1, 1 end; GetSpellBookItemName = function(i) if i == 1 then return "Attack", "" end return SECRET end')
refresh(FC)
check(not [(f.text or '') for f in findings(c) if 'not learned' in (f.text or '')], 'S3: a slot without a readable name -> no "not learned" warning')

# Journey replaced by another updater (CurseForge, Wago, Journey Guide Manager):
# one chat warning per session at login; nothing when Journey is ours, absent
# or disabled, or when the metadata API is missing or throws.
J_WARN = 'is not Relax Journey'
def journey_case(pre):
    L, G, FC, c = client({'level': 20}, gq=False, fdj=False, pre=pre)
    return L, G, c, [str(m) for m in G.CHAT.values() if J_WARN in str(m)]
LOADED = 'C_AddOns = C_AddOns or {}; C_AddOns.IsAddOnLoaded = function(n) return n == "Journey" end\n'
L, G, c, warn = journey_case(LOADED + 'C_AddOns.GetAddOnMetadata = function(n, f) if n == "Journey" and f == "Title" then return "Journey" end end')
check(len(warn) == 1 and 'Repair' in warn[0] and 'ignore this' in warn[0] and 'was replaced' not in warn[0], f'Journey foreign: one neutral line with Repair and ignore advice ({warn})')
G.FIRE('PLAYER_LOGIN'); G.FIRE('PLAYER_ENTERING_WORLD')
check(len([m for m in G.CHAT.values() if J_WARN in str(m)]) == 1, 'Journey foreign: still one warning per session')
L, G, c, warn = journey_case(LOADED + 'C_AddOns.GetAddOnMetadata = function(n, f) if f == "Title" then return "Relax Journey" end end')
check(not warn, 'Journey ours: no warning')
L, G, c, warn = journey_case(LOADED + 'C_AddOns.GetAddOnMetadata = function(n, f) if f == "Title" then return "|cff00ccffRelax Journey|r" end end')
check(not warn, 'Journey ours with a color code: no warning')
L, G, c, warn = journey_case('C_AddOns = { IsAddOnLoaded = function() return false end, GetAddOnMetadata = function() return nil end }')
check(not warn, 'Journey absent: no warning')
L, G, c, warn = journey_case('C_AddOns = { IsAddOnLoaded = function() return false end, GetAddOnMetadata = function(n, f) return f == "Title" and "Journey" or nil end }')
check(not warn, 'Journey foreign but disabled (not loaded): no warning')
L, G, c, warn = journey_case('IsAddOnLoaded = function(n) return n == "Journey" end; GetAddOnMetadata = function(n, f) return f == "Title" and "Journey" or nil end')
check(len(warn) == 1, 'no C_AddOns: falls back to the global GetAddOnMetadata/IsAddOnLoaded')
L, G, c, warn = journey_case(LOADED + 'C_AddOns.GetAddOnMetadata = function() error("boom") end; GetAddOnMetadata = function(n, f) return f == "Title" and "Journey" or nil end')
check(len(warn) == 1, 'C_AddOns.GetAddOnMetadata throws: falls back to the global')
L, G, c, warn = journey_case(LOADED + 'C_AddOns.GetAddOnMetadata = function() error("boom") end; GetAddOnMetadata = function() error("boom") end')
check(not warn and c.lastError is None, 'both metadata APIs throw: no warning, no error')
L, G, c, warn = journey_case('')
check(not warn and c.lastError is None, 'no addon API at all: no warning, no error')

print('ALL OK' if not fails else f'{len(fails)} FAILED')
sys.exit(1 if fails else 0)
