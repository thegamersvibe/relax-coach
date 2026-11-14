#!/usr/bin/env python3
"""Deterministic offline tests of real addon Lua 5.1, not a Python reimplementation.
Run: python test/reactions_test.py (requires lupa with lua51).
Game event delivery/audio/aura readability still require separate in-client QA.
"""
import ast
import pathlib
import unittest
from typing import Any
import lupa.lua51 as lupa

ROOT = pathlib.Path(__file__).resolve().parents[1]
ADDON = ROOT / 'addon' / 'ForeverCoach'
# Reuse only the established client API fixture, not the legacy suite's execution.
TREE = ast.parse((ROOT / 'test' / 'harness.py').read_text())
BASE_MOCK = next(ast.literal_eval(n.value) for n in TREE.body
                 if isinstance(n, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'MOCK' for t in n.targets))
FILES = ['Rules', 'Core', 'Route', 'Session', 'Gear', 'Talents', 'Dungeons', 'Audit', 'Macros', 'Reactions', 'UI']

STRICT_MOCK = r'''
FRAMES = {}
local Frame = {}
Frame.__index = Frame
function Frame:SetScript(k,v) self.scripts[k] = v end
function Frame:RegisterEvent(e) self.events[e] = true end
function Frame:RegisterUnitEvent(e,u) self.events[e] = true; self.units[e] = u end
function Frame:UnregisterEvent(e) self.events[e] = nil end
function Frame:SetSize(w,h) self.width,self.height = w,h end
function Frame:SetPoint(...) self.point = {...} end
function Frame:SetFrameStrata(v) self.strata = v end
function Frame:EnableMouse(v) self.mouse = v end
function Frame:EnableMouseWheel(v) self.wheel = v end
function Frame:SetAlpha(v) self.alpha = v end
function Frame:Show() self.shown = true end
function Frame:Hide() self.shown = false end
function Frame:IsShown() return self.shown end
-- window methods used by the level-up checklist (Coach UI), not by Fun Mode itself
function Frame:SetMovable(v) self.movable = v end
function Frame:RegisterForDrag(...) end
function Frame:StartMoving() end
function Frame:StopMovingOrSizing() end
function Frame:SetHeight(h) self.height = h end
function Frame:SetText(v) self.label = v end
function Frame:CreateFontString()
  local fs = {}
  function fs:SetPoint(...) end
  function fs:SetWidth(v) end
  function fs:SetJustifyH(v) end
  function fs:SetSpacing(v) end
  function fs:SetText(v) self.text = v end
  function fs:GetText() return self.text end
  function fs:GetStringHeight() return 12 end
  return fs
end
UISpecialFrames = UISpecialFrames or {}
tinsert = tinsert or table.insert
function Frame:CreateTexture()
  local t = {}
  function t:SetTexture(v) self.path = v end
  function t:SetAllPoints() self.allPoints = true end
  function t:SetTexCoord(...) self.coords = {...} end
  self.texture = t
  return t
end
function CreateFrame(kind,name,parent)
  local f = setmetatable({scripts={}, events={}, units={}, shown=true, name=name, parent=parent},Frame)
  FRAMES[#FRAMES+1] = f
  if name then _G[name] = f end
  return f
end
UIParent = CreateFrame('Frame','UIParent')
function FIRE(e,...)
  local unit = select(1,...)
  for _,f in ipairs(FRAMES) do
    if f.events[e] and f.scripts.OnEvent and (not f.units[e] or f.units[e] == unit) then
      f.scripts.OnEvent(f,e,...)
    end
  end
end
function issecretvalue(v) return rawequal(v,SECRET) end
local function secretBoom() error('secret touched') end
getmetatable(SECRET).__tostring = secretBoom
getmetatable(SECRET).__concat = secretBoom
getmetatable(SECRET).__index = secretBoom
getmetatable(SECRET).__newindex = secretBoom
STATE.epochShift = 0
local oldTime = time
function time() return oldTime() + STATE.epochShift end
function GetServerTime() return time() end
function IsPlayerInWorld() return STATE.inWorld ~= false end
STATE.containers = {[0]={slots=16,free=10,family=0}}
C_Container.GetContainerNumSlots = function(b)
  if STATE.bagError then error('inventory unavailable') end
  if STATE.bagSecret then return SECRET end
  if STATE.bagUnknown then return nil end
  return STATE.containers[b] and STATE.containers[b].slots or 0
end
C_Container.GetContainerNumFreeSlots = function(b)
  if STATE.bagError then error('inventory unavailable') end
  if STATE.bagSecret then return SECRET,SECRET end
  if STATE.bagUnknown then return nil,nil end
  local x=STATE.containers[b]
  if x then return x.free,x.family end
  return 0,0
end
C_UnitAuras = {GetPlayerAuraBySpellID = function(id)
  assert(id == 1289723 or id == 1229739,'wrong aura')
  if STATE.auraNoReturn then return end
  if STATE.auraError then error('aura unavailable') end
  if STATE.auraSecret then return SECRET end
  if id == 1289723 then return STATE.aura end
  return STATE.alternateAura
end}
-- No guessed GameError enum: Forever's verified caller uses the localized second argument.
ERR_INV_FULL = 'localized inventory-full text'
SOUNDS, ATTEMPTS, STOPS, ACTIVE = {}, {}, {}, {}
DURATIONS = {['level_up.mp3']=1.66,['bags_1.mp3']=2.62,['bags_2.mp3']=1.27,['death.mp3']=2.12,['campfire_1.mp3']=60.13,['campfire_2.mp3']=63.33}
function PlaySoundFile(path,channel)
  local file = path:match('([^\\]+)$')
  assert(DURATIONS[file], 'unexpected asset '..tostring(path))
  assert(channel == 'Master', 'unexpected sound channel')
  ATTEMPTS[#ATTEMPTS+1] = {file=file,time=GetTime()}
  if STATE.soundThrows then error('audio unavailable') end
  if STATE.soundSecret then return SECRET,SECRET end
  if STATE.soundFail then return false,nil end
  for _,a in pairs(ACTIVE) do assert(GetTime() >= a.ends, 'owned audio overlaps') end
  local handle = #SOUNDS+100
  local entry = {file=file,handle=handle,time=GetTime(),ends=GetTime()+DURATIONS[file]}
  SOUNDS[#SOUNDS+1] = entry
  ACTIVE[handle] = entry
  return true,handle
end
function StopSound(handle)
  assert(type(handle)=='number','invalid owned sound handle')
  STOPS[#STOPS+1] = handle
  ACTIVE[handle] = nil
end
local timers={}
C_Timer.After = function(delay,fn) timers[#timers+1]={at=GetTime()+delay,fn=fn} end
function WAIT(dt)
  local target=GetTime()+dt
  while GetTime() < target - 0.0000001 do
    local step=math.min(0.01,target-GetTime())
    ADVANCE(step)
    -- Snapshot completions before dispatch: callbacks may add a new ACTIVE entry.
    -- Mutating a pairs() table by adding keys during iteration is invalid in Lua 5.1.
    local finished={}
    for handle,a in pairs(ACTIVE) do
      if GetTime()+0.0000001 >= a.ends then finished[#finished+1]=handle end
    end
    for _,handle in ipairs(finished) do
      ACTIVE[handle]=nil
      if not STATE.noSoundFinished then FIRE('SOUNDKIT_FINISHED',handle) end
    end
    local due={}
    for i=#timers,1,-1 do if timers[i].at <= GetTime() then due[#due+1]=table.remove(timers,i) end end
    for _,t in ipairs(due) do t.fn() end
    for _,f in ipairs(FRAMES) do if f.shown and f.scripts.OnUpdate then f.scripts.OnUpdate(f,step) end end
  end
end
function SET_FREE(n) STATE.containers[0].free=n; STATE.freeSlots=n end
function DIE() STATE.dead=true; FIRE('PLAYER_DEAD') end
function REVIVE() STATE.dead=false; FIRE('PLAYER_UNGHOST'); FIRE('PLAYER_ALIVE') end
function AURA_ON(id) STATE.aura={spellId=1289723,auraInstanceID=id or 1}; FIRE('UNIT_AURA','player',{}) end
function AURA_OFF() STATE.aura=nil; FIRE('UNIT_AURA','player',{}) end
function FULL() SET_FREE(0); FIRE('BAG_UPDATE_DELAYED') end
function ROOM() SET_FREE(1); FIRE('BAG_UPDATE_DELAYED') end
function FULL_ERROR() FIRE('UI_ERROR_MESSAGE',17,ERR_INV_FULL) end
function LEVEL(n) STATE.level=n; FIRE('PLAYER_LEVEL_UP',n) end
'''


def lua_value(L, obj):
    if isinstance(obj, dict):
        return L.table_from({k: lua_value(L, v) for k, v in obj.items()})
    return obj


def python_value(obj):
    if lupa.lua_type(obj) == 'table':
        return {k: python_value(v) for k, v in obj.items()}
    return obj


class Client:
    def __init__(self, setup='', db=None):
        self.L: Any = lupa.LuaRuntime(unpack_returned_tuples=True)
        self.L.execute(BASE_MOCK + '\n' + STRICT_MOCK)
        self.g: Any = self.L.globals()
        if db is not None:
            self.g.ForeverCoachDB = lua_value(self.L, db)
        self.L.execute(setup)
        ns = self.L.table()
        for stem in FILES:
            src = (ADDON / (stem + '.lua')).read_text(encoding='utf-8')
            self.L.execute('return function(...)\n' + src + '\nend')('ForeverCoach', ns)
        self.g.FIRE('ADDON_LOADED', 'ForeverCoach')
        self.g.FIRE('PLAYER_LOGIN')
        self.g.FIRE('PLAYER_ENTERING_WORLD', True, False)
        self.fc = self.g.ForeverCoach
        self.r = self.fc.Reactions
        self.c = self.fc.Char(self.fc)

    def run(self, code):
        return self.L.execute(code)

    def wait(self, seconds):
        self.g.WAIT(seconds)

    def sounds(self):
        return [self.g.SOUNDS[i].file for i in range(1, len(self.g.SOUNDS)+1)]

    def save(self):
        return python_value(self.g.ForeverCoachDB)


class ReactionsTest(unittest.TestCase):
    def make(self, **kwargs):
        c = Client(**kwargs)
        self.addCleanup(lambda: self.assertIsNone(c.c.lastError, 'addon exception: '+str(c.c.lastError)))
        return c

    def test_bag_count_changes_without_game_error_are_silent(self):
        c = self.make(setup='SET_FREE(1)')
        # Simulate inventory settling across a reward; no rejection was emitted.
        c.run('FULL(); ROOM(); FIRE("PLAYER_ENTERING_WORLD",false,false)')
        c.wait(66)
        self.assertEqual(c.sounds(), [])
        self.assertEqual(len(c.g.ATTEMPTS), 0)
        self.assertEqual(len(c.r.queue), 0)
        self.assertEqual(c.c.reactions.bagNext, 1)

    def test_repeated_full_error_with_unchanged_spare_slot_is_one_episode(self):
        c = self.make(setup='SET_FREE(1)')
        c.run('FULL_ERROR()')
        for _ in range(4):
            c.wait(4)
            c.run('FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
        self.assertEqual(c.sounds(), ['bags_1.mp3'])
        self.assertEqual(c.c.reactions.bagNext, 2)

    def test_full_error_is_required_even_after_last_slot_fills(self):
        c = self.make(setup='SET_FREE(1)')
        c.run('FULL()')
        self.assertEqual(c.sounds(), [])
        c.run('FULL_ERROR(); for i=1,100 do FULL_ERROR(); FULL() end')
        c.wait(4)
        self.assertEqual(c.sounds(), ['bags_1.mp3'])
        c.run('ROOM(); FULL()')
        self.assertEqual(c.sounds(), ['bags_1.mp3'])
        c.run('FULL_ERROR()')
        self.assertEqual(c.sounds(), ['bags_1.mp3', 'bags_2.mp3'])

    def test_rejection_with_space_rearms_only_after_space_increases(self):
        c = self.make(setup='SET_FREE(1)')
        c.run('FULL_ERROR()'); c.wait(4)
        c.run('SET_FREE(2); FIRE("BAG_UPDATE_DELAYED")')
        self.assertEqual(c.sounds(), ['bags_1.mp3'])
        c.run('FULL_ERROR()')
        self.assertEqual(c.sounds(), ['bags_1.mp3', 'bags_2.mp3'])

    def test_filling_then_freeing_space_can_rearm_a_rejection(self):
        c = self.make(setup='SET_FREE(1)')
        c.run('FULL_ERROR()'); c.wait(4)
        c.run('FULL(); ROOM()')
        self.assertEqual(c.sounds(), ['bags_1.mp3'])
        c.run('FULL_ERROR()')
        self.assertEqual(c.sounds(), ['bags_1.mp3', 'bags_2.mp3'])

    def test_unknown_rejection_capacity_recovers_without_false_rearm(self):
        for flag in ('bagUnknown', 'bagSecret', 'bagError'):
            with self.subTest(flag=flag):
                c = self.make(setup=f'SET_FREE(1); STATE.{flag}=true')
                c.run('FULL_ERROR()'); c.wait(4)
                c.run(f'STATE.{flag}=false; FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
                self.assertEqual(c.sounds(), ['bags_1.mp3'])
                c.run('SET_FREE(2); FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
                self.assertEqual(c.sounds(), ['bags_1.mp3', 'bags_2.mp3'])

    def test_muted_rejection_with_spare_capacity_stays_suppressed(self):
        c = self.make(setup='SET_FREE(1)')
        c.run('SlashCmdList.FOREVERCOACH("fun off"); FULL_ERROR(); SlashCmdList.FOREVERCOACH("fun on"); FULL_ERROR(); FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
        self.assertEqual(c.sounds(), [])
        self.assertEqual(c.c.reactions.bagNext, 1)
        c.run('SET_FREE(2); FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
        self.assertEqual(c.sounds(), ['bags_1.mp3'])

    def test_rejection_with_spare_capacity_stays_deduplicated_after_reload(self):
        c = self.make(setup='SET_FREE(1)')
        c.run('FULL_ERROR()')
        reloaded = self.make(db=c.save(), setup='SET_FREE(1)')
        reloaded.run('FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
        self.assertEqual(reloaded.sounds(), [])
        reloaded.run('SET_FREE(2); FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
        self.assertEqual(reloaded.sounds(), ['bags_2.mp3'])

    def test_count_only_cycles_cannot_replace_a_pending_rejection(self):
        c = self.make()
        c.run('AURA_ON(); FULL(); FULL_ERROR()')
        token = c.r.pending.bags.token
        c.run('ROOM(); FULL()')
        self.assertEqual(c.r.pending.bags.token, token)
        c.wait(66)
        self.assertEqual(c.sounds(), ['campfire_1.mp3', 'bags_1.mp3'])
        self.assertFalse(c.c.reactions.bagsAnnounced)
        c.run('FULL_ERROR()')
        self.assertEqual(c.sounds(), ['campfire_1.mp3', 'bags_1.mp3', 'bags_2.mp3'])

    def test_specialty_and_bank_space_cannot_rearm_general_rejection(self):
        c = self.make(setup='SET_FREE(1); STATE.containers[1]={slots=20,free=0,family=32}; STATE.containers[5]={slots=20,free=0,family=0}')
        c.run('FULL_ERROR()'); c.wait(4)
        c.run('STATE.containers[1].free=20; STATE.containers[5].free=20; FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
        self.assertEqual(c.sounds(), ['bags_1.mp3'])
        c.run('SET_FREE(2); FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
        self.assertEqual(c.sounds(), ['bags_1.mp3', 'bags_2.mp3'])

    def test_unchanged_space_cannot_restart_failed_rejection_retries(self):
        c = self.make(setup='SET_FREE(1); STATE.soundFail=true')
        c.run('FULL_ERROR()'); c.wait(4)
        self.assertEqual(len(c.g.ATTEMPTS), 3)
        c.run('STATE.soundFail=false; for i=1,100 do FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR() end')
        c.wait(4)
        self.assertEqual(len(c.g.ATTEMPTS), 3)
        self.assertEqual(c.sounds(), [])
        c.run('SET_FREE(2); FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
        self.assertEqual(c.sounds(), ['bags_1.mp3'])

    def test_full_error_requires_the_exact_localized_second_payload(self):
        c = self.make(setup='SET_FREE(1)')
        c.run('FIRE("UI_ERROR_MESSAGE",ERR_INV_FULL,"different error"); FIRE("UI_ERROR_MESSAGE",17,ERR_INV_FULL.." extra"); FIRE("UI_ERROR_MESSAGE",17,SECRET)')
        self.assertEqual(c.sounds(), [])
        c.run('ERR_INV_FULL="another locale"; FULL_ERROR()')
        self.assertEqual(c.sounds(), ['bags_1.mp3'])

    def test_new_and_existing_enabled_by_default(self):
        for db in (None, {'schema':1,'characters':{},'privateSetting':'keep'}):
            with self.subTest(existing=db is not None):
                c=self.make(db=db)
                self.assertTrue(c.g.ForeverCoachDB.reactionsEnabled)
                self.assertEqual(c.sounds(), [])
                if db:
                    self.assertEqual(c.g.ForeverCoachDB.privateSetting,'keep')

    def test_existing_false_stays_muted(self):
        c=self.make(db={'reactionsEnabled':False})
        c.run('DIE(); FULL(); FULL_ERROR(); LEVEL(21); AURA_ON()')
        c.wait(66)
        self.assertEqual(c.sounds(),[])
        self.assertFalse(c.g.ForeverCoachDB.reactionsEnabled)

    def test_full_bags_at_login_are_silent_until_error(self):
        c=self.make(setup='SET_FREE(0)')
        self.assertEqual(c.sounds(),[])
        c.run('FULL_ERROR(); FULL_ERROR(); FIRE("BAG_UPDATE_DELAYED")')
        self.assertEqual(c.sounds(),['bags_1.mp3'])

    def test_full_edge_and_loot_error_share_one_episode(self):
        c=self.make()
        c.run('FULL(); for i=1,100 do FULL_ERROR(); FIRE("BAG_UPDATE_DELAYED") end')
        c.wait(5)
        self.assertEqual(c.sounds(),['bags_1.mp3'])

    def test_error_can_catch_a_missed_bag_event(self):
        c=self.make()
        c.run('SET_FREE(0); FULL_ERROR(); FIRE("BAG_UPDATE_DELAYED")')
        self.assertEqual(c.sounds(),['bags_1.mp3'])

    def test_unrelated_errors_are_not_full_bags(self):
        c=self.make()
        c.run('FIRE("UI_ERROR_MESSAGE",1,"not that error"); FIRE("UI_ERROR_MESSAGE",SECRET,SECRET)')
        self.assertEqual(c.sounds(),[])

    def test_bag_alternation_survives_reload(self):
        c=self.make()
        c.run('FULL(); FULL_ERROR()'); c.wait(4); c.run('ROOM(); FULL(); FULL_ERROR()'); c.wait(3)
        self.assertEqual(c.sounds(),['bags_1.mp3','bags_2.mp3'])
        reloaded=self.make(db=c.save(),setup='SET_FREE(0)')
        reloaded.run('FULL_ERROR()')
        self.assertEqual(reloaded.sounds(),[])
        reloaded.run('ROOM(); FULL(); FULL_ERROR()')
        self.assertEqual(reloaded.sounds(),['bags_1.mp3'])

    def test_unknown_inventory_does_not_rearm(self):
        c=self.make(); c.run('FULL(); FULL_ERROR()'); c.wait(4)
        c.run('STATE.bagUnknown=true; FIRE("BAG_UPDATE_DELAYED"); STATE.bagUnknown=false; FULL(); FULL_ERROR()')
        self.assertEqual(c.sounds(),['bags_1.mp3'])

    def test_secret_and_failed_inventory_do_not_invent_full(self):
        for flag in ('bagSecret','bagError','bagUnknown'):
            with self.subTest(flag=flag):
                c=self.make()
                c.run(f'STATE.{flag}=true; FIRE("BAG_UPDATE_DELAYED")')
                self.assertEqual(c.sounds(),[])
                c.run(f'STATE.{flag}=false; FULL(); FULL_ERROR()')
                self.assertEqual(c.sounds(),['bags_1.mp3'])

    def test_zero_containers_are_not_full(self):
        c=self.make(setup='STATE.containers[0].slots=0')
        c.run('FIRE("BAG_UPDATE_DELAYED")')
        self.assertEqual(c.sounds(),[])

    def test_specialty_and_bank_space_cannot_hide_full_normal_bags(self):
        c=self.make(setup='STATE.containers[1]={slots=20,free=20,family=32}; STATE.containers[5]={slots=20,free=20,family=0}; STATE.containers[-1]={slots=20,free=20,family=0}')
        c.run('FULL()')
        self.assertEqual(c.sounds(), [])
        c.run('FULL_ERROR()')
        self.assertEqual(c.sounds(),['bags_1.mp3'])

    def test_other_normal_bag_space_prevents_false_full(self):
        c=self.make(setup='STATE.containers[1]={slots=8,free=1,family=0}')
        c.run('FULL()')
        self.assertEqual(c.sounds(),[])
        c.run('STATE.containers[1].free=0; FIRE("BAG_UPDATE_DELAYED")')
        self.assertEqual(c.sounds(), [])
        c.run('FULL_ERROR()')
        self.assertEqual(c.sounds(),['bags_1.mp3'])

    def test_loading_screen_zeros_do_not_make_full(self):
        c=self.make()
        c.run('STATE.inWorld=false; FULL(); FIRE("PLAYER_LEAVING_WORLD")')
        self.assertEqual(c.sounds(),[])
        c.run('STATE.inWorld=true; ROOM(); FIRE("PLAYER_ENTERING_WORLD",false,false); FULL(); FULL_ERROR()')
        self.assertEqual(c.sounds(),['bags_1.mp3'])

    def test_one_sound_per_death_including_ghost_release(self):
        c=self.make()
        c.run('DIE(); for i=1,30 do FIRE("PLAYER_DEAD") end; FIRE("PLAYER_ALIVE")')
        c.wait(4); c.run('FIRE("PLAYER_DEAD")')
        self.assertEqual(c.sounds(),['death.mp3'])
        c.run('REVIVE(); DIE()')
        self.assertEqual(c.sounds(),['death.mp3','death.mp3'])

    def test_login_dead_is_not_a_new_death(self):
        c=self.make(setup='STATE.dead=true')
        c.run('FIRE("PLAYER_DEAD"); FIRE("PLAYER_ALIVE"); FIRE("PLAYER_DEAD")')
        self.assertEqual(c.sounds(),[])
        c.run('REVIVE(); DIE()')
        self.assertEqual(c.sounds(),['death.mp3'])

    def test_unknown_alive_read_does_not_rearm_death(self):
        c=self.make(); c.run('DIE()'); c.wait(4)
        c.run('STATE.dead=SECRET; FIRE("PLAYER_ALIVE"); STATE.dead=true; FIRE("PLAYER_DEAD")')
        self.assertEqual(c.sounds(),['death.mp3'])

    def test_campfire_alternate_variant_gain(self):
        c = self.make()
        c.run('STATE.alternateAura={spellId=1229739}; FIRE("UNIT_AURA","player",{})')
        self.assertEqual(c.sounds(), ['campfire_1.mp3'])
        self.assertTrue(c.r.campPresent)
        self.assertIsNotNone(c.c.reactions.campfireLastPlayedAt)

    def dual_campfire(self, first='absent', second='absent', db=None):
        return self.make(db=db, setup=r'''
            CAMP_MODES={}; OPAQUE={}
            function canaccesstable(v) return not rawequal(v,OPAQUE) end
            C_UnitAuras.GetPlayerAuraBySpellID=function(id)
                assert(id==1289723 or id==1229739,'unexpected spell lookup')
                local mode=CAMP_MODES[id]
                if mode=='present' then return {spellId=id} end
                if mode=='zero' then return end
                if mode=='error' then error('aura query unavailable') end
                if mode=='secret' then return SECRET end
                if mode=='opaque' then return OPAQUE end
                return nil
            end
            function CAMP_MODES_SET(a,b)
                CAMP_MODES[1289723]=a; CAMP_MODES[1229739]=b
                FIRE('UNIT_AURA','player',{})
            end
        ''' + f'CAMP_MODES_SET("{first}","{second}")')

    def test_campfire_variant_observation_matrix(self):
        modes = ('absent', 'present', 'zero', 'error', 'secret', 'opaque')
        for first in modes:
            for second in modes:
                with self.subTest(first=first, second=second):
                    c = self.dual_campfire(first, second)
                    present = 'present' in (first, second)
                    self.assertEqual(c.sounds(), ['campfire_1.mp3'] if present else [])
                    self.assertEqual(c.r.campPresent, True if present else False if first == second == 'absent' else None)

    def test_campfire_variant_unknown_cannot_rearm_known_presence(self):
        for mode in ('zero', 'error', 'secret', 'opaque'):
            for side in (0, 1):
                with self.subTest(mode=mode, side=side):
                    c = self.dual_campfire('present', 'absent'); c.wait(61)
                    pair = ['absent', 'absent']; pair[side] = mode
                    c.run(f'STATE.epochShift=3601; CAMP_MODES_SET("{pair[0]}","{pair[1]}")')
                    self.assertTrue(c.r.campPresent)
                    c.run('CAMP_MODES_SET("absent","present")')
                    self.assertEqual(c.sounds(), ['campfire_1.mp3'])
                    c.run('CAMP_MODES_SET("absent","absent"); CAMP_MODES_SET("absent","present")')
                    self.assertEqual(c.sounds(), ['campfire_1.mp3', 'campfire_2.mp3'])

    def test_campfire_variant_overlap_and_switch_are_one_episode(self):
        for first, second in (('present','absent'), ('absent','present')):
            with self.subTest(first=first):
                c = self.dual_campfire(first, second); c.wait(61)
                stamp = c.c.reactions.campfireLastPlayedAt
                c.run('STATE.epochShift=3601; CAMP_MODES_SET("present","present")')
                c.run(f'CAMP_MODES_SET("{second}","{first}")')
                c.run(f'CAMP_MODES_SET("{first}","{second}")')
                self.assertEqual(c.sounds(), ['campfire_1.mp3'])
                self.assertEqual(c.c.reactions.campfireLastPlayedAt, stamp)
                self.assertEqual(c.r.campEpisode, 1)

    def test_campfire_variants_share_exact_thirty_minute_guard(self):
        for first, second in (('present','absent'), ('absent','present')):
            with self.subTest(first=first):
                c = self.dual_campfire(first, second); c.wait(61)
                stamp = c.c.reactions.campfireLastPlayedAt
                c.run(f'STATE.epochShift=STATE.epochShift+{stamp}+1799-time(); CAMP_MODES_SET("absent","absent"); CAMP_MODES_SET("{second}","{first}")')
                self.assertEqual(c.sounds(), ['campfire_1.mp3'])
                self.assertEqual(c.c.reactions.campfireLastPlayedAt, stamp)
                c.run(f'STATE.epochShift=STATE.epochShift+1; CAMP_MODES_SET("absent","absent"); CAMP_MODES_SET("{second}","{first}")')
                self.assertEqual(c.sounds(), ['campfire_1.mp3', 'campfire_2.mp3'])
                self.assertEqual(c.c.reactions.campfireLastPlayedAt, stamp + 1800)

    def campfire_again(self, c, after=1800):
        """Next eligible campfire episode: move the clock past the cooldown, aura off then on."""
        stamp = c.c.reactions.campfireLastPlayedAt
        c.run(f'STATE.epochShift=STATE.epochShift+{stamp}+{after}-time(); AURA_OFF(); AURA_ON()')

    def test_campfire_songs_take_turns_across_reload(self):
        c = self.make(); c.run('AURA_ON()'); c.wait(61)
        self.assertEqual(c.sounds(), ['campfire_1.mp3'])
        self.assertEqual(c.c.reactions.campfireNext, 2)
        self.campfire_again(c); c.wait(64)
        self.assertEqual(c.sounds(), ['campfire_1.mp3', 'campfire_2.mp3'])
        self.assertEqual(c.c.reactions.campfireNext, 1)
        self.campfire_again(c); c.wait(61)
        self.assertEqual(c.sounds(), ['campfire_1.mp3', 'campfire_2.mp3', 'campfire_1.mp3'])
        # the index survives a reload: the next campfire after it is song 2
        saved = c.save()
        stamp = c.c.reactions.campfireLastPlayedAt
        reloaded = self.make(db=saved, setup=f'STATE.epochShift=STATE.epochShift+{stamp}+1800-time()')
        reloaded.run('AURA_ON()')
        self.assertEqual(reloaded.sounds(), ['campfire_2.mp3'])
        self.assertEqual(reloaded.c.reactions.campfireNext, 1)

    def test_campfire_thirty_minute_boundary(self):
        c = self.make(); c.run('AURA_ON()'); c.wait(61)
        self.campfire_again(c, after=1799); c.wait(1)
        self.assertEqual(c.sounds(), ['campfire_1.mp3'])
        self.assertEqual(c.c.reactions.campfireNext, 2)
        self.campfire_again(c, after=1800)
        self.assertEqual(c.sounds(), ['campfire_1.mp3', 'campfire_2.mp3'])

    def test_campfire_bad_or_missing_index_starts_at_song_one(self):
        for bad in ('nil', '3', '0', '"2"', '1.5', '{}', 'true'):
            with self.subTest(bad=bad):
                c = self.make(setup=f'ForeverCoachDB=nil')
                c.run(f'ForeverCoach:Char().reactions.campfireNext={bad}; AURA_ON()')
                self.assertEqual(c.sounds(), ['campfire_1.mp3'])
                self.assertEqual(c.c.reactions.campfireNext, 2)

    def test_campfire_failed_or_blocked_play_keeps_song_and_time(self):
        # three failed attempts: no stamp, no turn
        c = self.make(setup='STATE.soundFail=true'); c.run('AURA_ON()'); c.wait(4)
        self.assertEqual(c.sounds(), [])
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
        self.assertIsNone(c.c.reactions.campfireNext)
        c.run('STATE.soundFail=false; AURA_OFF(); AURA_ON()')
        self.assertEqual(c.sounds(), ['campfire_1.mp3'])
        # a throwing or secret playback result is not a start either
        for flag in ('soundThrows', 'soundSecret'):
            with self.subTest(flag=flag):
                c = self.make(setup=f'STATE.{flag}=true'); c.run('AURA_ON()'); c.wait(4)
                self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
                self.assertIsNone(c.c.reactions.campfireNext)
        # blocked by the cooldown: the song stays where it was
        c = self.make(); c.run('AURA_ON()'); c.wait(61)
        self.campfire_again(c, after=600); c.wait(1)
        self.assertEqual(c.c.reactions.campfireNext, 2)
        self.campfire_again(c); self.assertEqual(c.sounds(), ['campfire_1.mp3', 'campfire_2.mp3'])

    def test_campfire_variant_reload_preserves_shared_guard(self):
        for first, second in (('present','absent'), ('absent','present')):
            with self.subTest(first=first):
                c = self.dual_campfire(first, second)
                reloaded = self.dual_campfire(second, first, db=c.save())
                self.assertEqual(reloaded.sounds(), [])
                self.assertEqual(reloaded.c.reactions.campfireLastPlayedAt, c.c.reactions.campfireLastPlayedAt)

    def test_campfire_alternate_variant_muted_baseline_and_fresh_gain(self):
        c = self.dual_campfire()
        c.run('SlashCmdList.FOREVERCOACH("fun off"); CAMP_MODES_SET("absent","present"); SlashCmdList.FOREVERCOACH("fun on")')
        self.assertEqual(c.sounds(), [])
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
        c.run('CAMP_MODES_SET("absent","absent"); CAMP_MODES_SET("absent","present")')
        self.assertEqual(c.sounds(), ['campfire_1.mp3'])

    def test_campfire_variant_unknown_unmute_retains_silent_baseline(self):
        c = self.dual_campfire()
        c.run('SlashCmdList.FOREVERCOACH("fun off"); CAMP_MODES_SET("absent","zero"); SlashCmdList.FOREVERCOACH("fun on")')
        self.assertTrue(c.r.campBaselinePending)
        c.run('CAMP_MODES_SET("absent","present")')
        self.assertEqual(c.sounds(), [])
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
        c.run('CAMP_MODES_SET("absent","absent"); CAMP_MODES_SET("absent","present")')
        self.assertEqual(c.sounds(), ['campfire_1.mp3'])

    def test_campfire_variant_playback_failure_does_not_consume_guard(self):
        c = self.dual_campfire()
        c.run('STATE.soundFail=true; CAMP_MODES_SET("absent","present")'); c.wait(4)
        self.assertEqual(c.sounds(), [])
        self.assertEqual(len(c.g.ATTEMPTS), 3)
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
        c.run('STATE.soundFail=false; CAMP_MODES_SET("present","absent")')
        self.assertEqual(c.sounds(), [])
        c.run('CAMP_MODES_SET("absent","absent"); CAMP_MODES_SET("present","absent")')
        self.assertEqual(c.sounds(), ['campfire_1.mp3'])

    def test_campfire_alternate_variant_queues_once_behind_other_audio(self):
        c = self.dual_campfire()
        c.run('DIE(); CAMP_MODES_SET("absent","present"); CAMP_MODES_SET("present","present")')
        self.assertEqual(c.sounds(), ['death.mp3'])
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
        self.assertEqual(len(c.r.queue), 1)
        c.wait(3)
        self.assertEqual(c.sounds(), ['death.mp3', 'campfire_1.mp3'])

    def test_campfire_other_same_name_spell_is_not_accepted(self):
        c = self.make(setup='''C_UnitAuras.GetPlayerAuraBySpellID=function(id)
            if id==999999 then return {spellId=id,name='Welcoming Campfire'} end
            return nil
        end''')
        c.run('FIRE("UNIT_AURA","player",{})')
        self.assertEqual(c.sounds(), [])
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)

    def test_campfire_gain_and_refreshes(self):
        c=self.make(); c.run('AURA_ON(1)')
        stamp=c.c.reactions.campfireLastPlayedAt
        c.run('for i=1,100 do AURA_ON(i) end')
        c.wait(66)
        self.assertEqual(c.sounds(),['campfire_1.mp3'])
        self.assertEqual(c.c.reactions.campfireLastPlayedAt,stamp)

    def test_campfire_initial_aura_detected_once_and_persisted(self):
        c=self.make(setup='STATE.aura={spellId=1289723,auraInstanceID=55}')
        self.assertEqual(c.sounds(),['campfire_1.mp3'])
        reloaded=self.make(db=c.save(),setup='STATE.aura={spellId=1289723,auraInstanceID=99}')
        reloaded.run('AURA_ON(100)')
        self.assertEqual(reloaded.sounds(),[])

    def test_campfire_hour_is_start_based_and_not_expiry_driven(self):
        c=self.make(); c.run('AURA_ON()'); c.wait(61)
        c.run('AURA_OFF(); AURA_ON()'); c.wait(1)
        self.assertEqual(c.sounds(),['campfire_1.mp3'])
        c.run('STATE.epochShift=3601; AURA_ON(88)')
        c.wait(1)
        self.assertEqual(c.sounds(),['campfire_1.mp3'])
        c.run('AURA_OFF(); AURA_ON(89)')
        self.assertEqual(c.sounds(),['campfire_1.mp3','campfire_2.mp3'])

    def test_campfire_unknown_is_not_removal_even_after_an_hour(self):
        for flag in ('auraSecret','auraError'):
            with self.subTest(flag=flag):
                c=self.make(); c.run('AURA_ON()'); c.wait(61)
                c.run(f'STATE.epochShift=3601; STATE.{flag}=true; FIRE("UNIT_AURA","player",{{}}); STATE.{flag}=false; AURA_ON(2)')
                self.assertEqual(c.sounds(),['campfire_1.mp3'])

    def test_campfire_initial_unknown_can_recover(self):
        c=self.make(setup='STATE.auraError=true; STATE.aura={spellId=1289723,auraInstanceID=1}')
        self.assertEqual(c.sounds(),[])
        c.run('STATE.auraError=false; FIRE("UNIT_AURA","player",{})')
        self.assertEqual(c.sounds(),['campfire_1.mp3'])

    def test_nonplayer_aura_and_secret_unit_are_ignored(self):
        c=self.make()
        c.run('STATE.aura={spellId=1289723}; FIRE("UNIT_AURA","target",{}); FIRE("UNIT_AURA",SECRET,{})')
        self.assertEqual(c.sounds(),[])

    def test_clock_rollback_does_not_bypass_campfire_guard(self):
        c=self.make(); c.run('AURA_ON()'); c.wait(61)
        c.run('STATE.epochShift=-7200; AURA_OFF(); AURA_ON()')
        self.assertEqual(c.sounds(),['campfire_1.mp3'])

    def test_no_combat_gating_or_random_rolls(self):
        c=self.make(setup='STATE.combat=true; math.random=function() error("no random reactions") end')
        c.run('FULL(); FULL_ERROR(); DIE(); LEVEL(21); AURA_ON()'); c.wait(72)
        self.assertCountEqual(c.sounds(),['bags_1.mp3','death.mp3','level_up.mp3','campfire_1.mp3'])

    def test_busy_song_queues_distinct_reactions_once(self):
        c=self.make(); c.run('AURA_ON(); DIE(); FULL(); FULL_ERROR(); LEVEL(21)')
        c.run('for i=1,100 do FIRE("PLAYER_DEAD"); FULL_ERROR(); FIRE("PLAYER_LEVEL_UP",21); AURA_ON(i) end')
        self.assertEqual(c.sounds(),['campfire_1.mp3'])
        c.wait(71)
        self.assertEqual(c.sounds(),['campfire_1.mp3','death.mp3','bags_1.mp3','level_up.mp3'])
        self.assertEqual(len(c.g.STOPS),0)

    def test_repeated_pending_bag_cycles_are_coalesced(self):
        c=self.make(); c.run('AURA_ON(); for i=1,50 do FULL(); FULL_ERROR(); ROOM() end; FULL(); FULL_ERROR()')
        c.wait(71)
        self.assertEqual(c.sounds(),['campfire_1.mp3','bags_1.mp3'])
        c.run('FULL_ERROR()')
        self.assertEqual(c.sounds(),['campfire_1.mp3','bags_1.mp3'])

    def test_genuine_new_death_can_wait_behind_active_death(self):
        c=self.make(); c.run('DIE(); REVIVE(); DIE()'); c.wait(6)
        self.assertEqual(c.sounds(),['death.mp3','death.mp3'])

    def test_unrelated_finished_handle_cannot_release_audio_slot(self):
        c=self.make(); c.run('AURA_ON(); DIE(); FIRE("SOUNDKIT_FINISHED",99999)')
        self.assertEqual(c.sounds(),['campfire_1.mp3'])
        c.wait(66)
        self.assertEqual(c.sounds(),['campfire_1.mp3','death.mp3'])

    def test_missing_finished_event_has_duration_fallback(self):
        c=self.make(setup='STATE.noSoundFinished=true')
        c.run('AURA_ON(); DIE()'); c.wait(66)
        self.assertEqual(c.sounds(),['campfire_1.mp3','death.mp3'])

    def test_level_popup_is_three_seconds_and_click_through(self):
        c=self.make(); c.run('LEVEL(21)')
        f=c.r.popup
        self.assertIsNotNone(f)
        self.assertTrue(f.IsShown(f))
        self.assertFalse(f.mouse)
        self.assertEqual(f.point[1],'TOP')
        self.assertTrue(f.texture.path.endswith('level_up.tga'))
        c.wait(2.95)
        self.assertTrue(f.IsShown(f))
        c.wait(0.12)
        self.assertFalse(f.IsShown(f))
        self.assertEqual(c.sounds(),['level_up.mp3'])

    def test_other_reactions_have_no_popup(self):
        c=self.make(); c.run('DIE(); FULL(); FULL_ERROR(); AURA_ON()'); c.wait(71)
        self.assertTrue(c.r.popup is None or not c.r.popup.IsShown(c.r.popup))

    def test_level_duplicate_and_invalid_payloads(self):
        c=self.make(); c.run('LEVEL(21); FIRE("PLAYER_LEVEL_UP",21); FIRE("PLAYER_LEVEL_UP",SECRET); FIRE("PLAYER_LEVEL_UP","nope")')
        c.wait(5)
        self.assertEqual(c.sounds(),['level_up.mp3'])
        c.run('LEVEL(22)')
        self.assertEqual(c.sounds(),['level_up.mp3','level_up.mp3'])

    def test_master_mute_stops_owned_sound_clears_queue_and_popup(self):
        c=self.make(); c.run('LEVEL(21); DIE(); FULL(); FULL_ERROR(); AURA_ON(); SlashCmdList.FOREVERCOACH("fun off")')
        self.assertFalse(c.g.ForeverCoachDB.reactionsEnabled)
        self.assertFalse(c.r.popup.IsShown(c.r.popup))
        self.assertEqual(list(c.g.STOPS.values()),[100])
        c.wait(71)
        c.run('SlashCmdList.FOREVERCOACH("fun on")'); c.wait(71)
        self.assertEqual(c.sounds(),['level_up.mp3'])
        c.run('ROOM(); FULL(); FULL_ERROR()')
        self.assertEqual(c.sounds(),['level_up.mp3','bags_1.mp3'])

    def test_muted_triggers_are_not_replayed_on_unmute(self):
        c=self.make()
        c.run('SlashCmdList.FOREVERCOACH("fun off"); FULL(); FULL_ERROR(); DIE(); AURA_ON(); LEVEL(21); SlashCmdList.FOREVERCOACH("fun on"); FULL_ERROR(); FIRE("PLAYER_DEAD"); AURA_ON(2)')
        c.wait(66)
        self.assertEqual(c.sounds(),[])
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
        c.run('AURA_OFF(); AURA_ON(3)')
        self.assertEqual(c.sounds(),['campfire_1.mp3'])

    def test_unknown_aura_at_unmute_does_not_replay_muted_gain(self):
        for flag in ('auraNoReturn', 'auraSecret', 'auraError'):
            with self.subTest(flag=flag):
                c = self.make()
                c.run(f'SlashCmdList.FOREVERCOACH("fun off"); STATE.{flag}=true; AURA_ON(); SlashCmdList.FOREVERCOACH("fun on")')
                self.assertEqual(c.sounds(), [])
                self.assertFalse(c.r.campPresent)
                c.run(f'STATE.{flag}=false; FIRE("UNIT_AURA","player",{{}})')
                self.assertEqual(c.sounds(), [], 'same aura gained while muted must not be replayed after an unreadable unmute baseline')
                self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
                self.assertEqual(len(c.g.ATTEMPTS), 0)
                self.assertEqual(len(c.r.queue), 0)
                c.run('AURA_OFF(); AURA_ON()')
                self.assertEqual(c.sounds(), ['campfire_1.mp3'])

    def test_unknown_inventory_at_unmute_does_not_replay_muted_full(self):
        for flag in ('bagUnknown', 'bagSecret', 'bagError'):
            with self.subTest(flag=flag):
                c = self.make()
                c.run(f'SlashCmdList.FOREVERCOACH("fun off"); STATE.{flag}=true; FULL(); SlashCmdList.FOREVERCOACH("fun on")')
                self.assertEqual(c.sounds(), [])
                self.assertFalse(c.r.full)
                c.run(f'STATE.{flag}=false; FIRE("BAG_UPDATE_DELAYED")')
                self.assertEqual(c.sounds(), [], 'same fullness reached while muted must not be replayed after an unreadable unmute baseline')
                self.assertEqual(c.c.reactions.bagNext, 1)
                self.assertEqual(len(c.g.ATTEMPTS), 0)
                self.assertEqual(len(c.r.queue), 0)
                c.run('ROOM(); FULL(); FULL_ERROR()')
                self.assertEqual(c.sounds(), ['bags_1.mp3'])

    def test_full_error_cannot_bypass_deferred_mute_baseline(self):
        for flag in ('bagUnknown', 'bagSecret', 'bagError'):
            for error_before_read in (True, False):
                with self.subTest(flag=flag, error_before_read=error_before_read):
                    c = self.make()
                    c.run(f'SlashCmdList.FOREVERCOACH("fun off"); STATE.{flag}=true; FULL(); SlashCmdList.FOREVERCOACH("fun on")')
                    if error_before_read:
                        c.run('FULL_ERROR(); FULL_ERROR()')
                        self.assertEqual(c.sounds(), [], 'full-error fallback must stay silent while the mute baseline is pending')
                    c.run(f'STATE.{flag}=false; FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR(); FULL_ERROR()')
                    self.assertEqual(c.sounds(), [], 'the delayed full baseline must suppress later errors in the same episode')
                    self.assertEqual(c.c.reactions.bagNext, 1)
                    self.assertFalse(c.c.reactions.bagsAnnounced)
                    self.assertEqual(len(c.g.ATTEMPTS), 0)
                    self.assertEqual(len(c.r.queue), 0)
                    c.run('ROOM(); FULL_ERROR()')
                    self.assertEqual(c.sounds(), ['bags_1.mp3'])

    def test_deferred_aura_baseline_can_first_confirm_absence(self):
        for flag in ('auraNoReturn', 'auraSecret', 'auraError'):
            with self.subTest(flag=flag):
                c = self.make()
                c.run(f'SlashCmdList.FOREVERCOACH("fun off"); STATE.{flag}=true; AURA_ON(); SlashCmdList.FOREVERCOACH("fun on"); AURA_OFF()')
                self.assertFalse(c.r.campPresent)
                c.run(f'STATE.{flag}=false; FIRE("UNIT_AURA","player",{{}})')
                self.assertEqual(c.sounds(), [])
                self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
                self.assertEqual(len(c.g.ATTEMPTS), 0)
                # Already enabled: another "on" must not arm a new silent baseline.
                c.run(f'STATE.{flag}=true; AURA_ON(); SlashCmdList.FOREVERCOACH("fun on"); STATE.{flag}=false; FIRE("UNIT_AURA","player",{{}})')
                self.assertEqual(c.sounds(), ['campfire_1.mp3'])

    def test_deferred_bag_baseline_can_first_confirm_room(self):
        for flag in ('bagUnknown', 'bagSecret', 'bagError'):
            with self.subTest(flag=flag):
                c = self.make()
                c.run(f'SlashCmdList.FOREVERCOACH("fun off"); STATE.{flag}=true; FULL(); SlashCmdList.FOREVERCOACH("fun on"); ROOM()')
                self.assertFalse(c.r.full)
                c.run(f'STATE.{flag}=false; FIRE("BAG_UPDATE_DELAYED")')
                self.assertEqual(c.sounds(), [])
                self.assertEqual(c.c.reactions.bagNext, 1)
                self.assertEqual(len(c.g.ATTEMPTS), 0)
                c.run(f'STATE.{flag}=true; FULL(); SlashCmdList.FOREVERCOACH("fun on"); STATE.{flag}=false; FIRE("BAG_UPDATE_DELAYED"); FULL_ERROR()')
                self.assertEqual(c.sounds(), ['bags_1.mp3'])

    def test_deferred_bag_and_aura_baselines_recover_independently(self):
        aura = ('STATE.auraNoReturn=false; FIRE("UNIT_AURA","player",{})', 'AURA_OFF(); AURA_ON()', 'campfire_1.mp3')
        bags = ('STATE.bagUnknown=false; FIRE("BAG_UPDATE_DELAYED")', 'ROOM(); FULL(); FULL_ERROR()', 'bags_1.mp3')
        for first, second in ((aura, bags), (bags, aura)):
            with self.subTest(first=first[2]):
                c = self.make()
                c.run('SlashCmdList.FOREVERCOACH("fun off"); STATE.auraNoReturn=true; STATE.bagUnknown=true; AURA_ON(); FULL(); SlashCmdList.FOREVERCOACH("fun on")')
                c.run(first[0])
                self.assertEqual(c.sounds(), [])
                c.run(first[1])
                self.assertEqual(c.sounds(), [first[2]])
                c.run(second[0])
                self.assertEqual(len(c.r.queue), 0, 'one confirmed baseline must not clear the other pending baseline')
                self.assertEqual(c.c.reactions.bagNext, 1 if first == aura else 2)
                if first == bags:
                    self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
                c.wait(66)
                self.assertEqual(c.sounds(), [first[2]])
                c.run(second[1])
                self.assertEqual(c.sounds(), [first[2], second[2]])

    def test_synchronous_completion_on_mute_stop_does_not_drain_queue(self):
        c = self.make(setup='local stop=StopSound; function StopSound(h) stop(h); FIRE("SOUNDKIT_FINISHED",h) end')
        c.run('AURA_ON(); DIE(); FULL(); FULL_ERROR(); LEVEL(21); SlashCmdList.FOREVERCOACH("fun off"); SlashCmdList.FOREVERCOACH("fun off")')
        c.wait(71)
        self.assertEqual(c.sounds(), ['campfire_1.mp3'])
        self.assertEqual(list(c.g.STOPS.values()), [100])
        self.assertEqual(len(c.r.queue), 0)
        self.assertIsNone(c.r.active)
        self.assertIsNone(c.r.frame.scripts.OnUpdate)

    def test_synchronous_completion_on_fallback_stop_is_reentrant_safe(self):
        c = self.make(setup='STATE.noSoundFinished=true; local stop=StopSound; function StopSound(h) stop(h); FIRE("SOUNDKIT_FINISHED",h) end')
        c.run('AURA_ON(); DIE(); FULL(); FULL_ERROR(); LEVEL(21)')
        c.wait(71)
        self.assertEqual(c.sounds(), ['campfire_1.mp3', 'death.mp3', 'bags_1.mp3', 'level_up.mp3'])
        self.assertEqual(list(c.g.STOPS.values()), [100, 101, 102, 103])
        self.assertIsNone(c.r.frame.scripts.OnUpdate)

    def test_synchronous_completion_on_logout_stop_is_reentrant_safe(self):
        c = self.make(setup='local stop=StopSound; function StopSound(h) stop(h); FIRE("SOUNDKIT_FINISHED",h) end')
        c.run('AURA_ON(); DIE(); FULL(); FULL_ERROR(); FIRE("PLAYER_LOGOUT")')
        c.wait(71)
        self.assertEqual(c.sounds(), ['campfire_1.mp3'])
        self.assertEqual(list(c.g.STOPS.values()), [100])
        self.assertFalse(c.r.ready)

    def test_status_and_bad_option_do_not_toggle(self):
        c=self.make()
        for arg in ('fun','fun status','fun banana'):
            c.g.SlashCmdList.FOREVERCOACH(arg)
        self.assertTrue(c.g.ForeverCoachDB.reactionsEnabled)
        self.assertEqual(c.sounds(),[])
        chat=' '.join(c.g.CHAT.values())
        self.assertNotIn('1289723',chat)
        self.assertNotIn('bags_1',chat)

    def test_failed_playback_does_not_consume_rotation_or_hour(self):
        c=self.make(setup='STATE.soundFail=true')
        c.run('FULL(); FULL_ERROR(); AURA_ON()'); c.wait(15)
        self.assertEqual(c.sounds(),[])
        self.assertEqual(c.c.reactions.bagNext,1)
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
        self.assertLessEqual(len(c.g.ATTEMPTS),6)
        c.run('STATE.soundFail=false; ROOM(); FULL(); FULL_ERROR(); AURA_OFF(); AURA_ON()'); c.wait(66)
        self.assertEqual(c.sounds(),['bags_1.mp3','campfire_1.mp3'])

    def test_transient_failure_retries_without_consuming_or_blocking_forever(self):
        c=self.make(setup='STATE.soundFail=true')
        c.run('FULL(); FULL_ERROR(); DIE()'); c.wait(0.25)
        self.assertEqual(c.c.reactions.bagNext,1)
        c.run('STATE.soundFail=false'); c.wait(8)
        self.assertEqual(c.sounds(),['bags_1.mp3','death.mp3'])

    def test_throwing_or_secret_sound_api_is_safe_and_bounded(self):
        for flag in ('soundThrows','soundSecret'):
            with self.subTest(flag=flag):
                c=self.make(setup=f'STATE.{flag}=true')
                c.run('AURA_ON()'); c.wait(15)
                self.assertEqual(c.sounds(),[])
                self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
                self.assertLessEqual(len(c.g.ATTEMPTS),3)

    def test_logout_clears_pending_and_stops_owned_playback(self):
        c=self.make(); c.run('AURA_ON(); DIE(); FIRE("PLAYER_LOGOUT")'); c.wait(71)
        self.assertEqual(c.sounds(),['campfire_1.mp3'])
        self.assertEqual(list(c.g.STOPS.values()),[100])

    def test_campfire_zero_returns_do_not_manufacture_absence(self):
        c=self.make(); c.run('AURA_ON()'); c.wait(61)
        c.run('STATE.epochShift=3601; STATE.auraNoReturn=true; FIRE("UNIT_AURA","player",{}); STATE.auraNoReturn=false; AURA_ON(2)')
        self.assertEqual(c.sounds(),['campfire_1.mp3'])
        c.run('AURA_OFF(); AURA_ON(3)')
        self.assertEqual(c.sounds(),['campfire_1.mp3','campfire_2.mp3'])

    def test_level_uses_event_before_unit_level_is_applied(self):
        c=self.make()
        c.run('FIRE("PLAYER_LEVEL_UP",21); FIRE("PLAYER_LEVEL_UP",21)')
        c.wait(5)
        self.assertEqual(c.sounds(),['level_up.mp3'])

    def test_enable_command_is_idempotent_and_does_not_interrupt(self):
        c=self.make(); c.run('AURA_ON(); DIE(); SlashCmdList.FOREVERCOACH("fun on")')
        self.assertEqual(len(c.g.STOPS),0)
        c.wait(66)
        self.assertEqual(c.sounds(),['campfire_1.mp3','death.mp3'])

    def test_nullable_bag_family_is_unknown_not_general_capacity(self):
        c=self.make(setup='STATE.containers[1]={slots=8,free=0}')
        c.run('FULL()')
        self.assertEqual(c.sounds(),[])
        c.run('FULL_ERROR()')
        self.assertEqual(c.sounds(),['bags_1.mp3'])

    def test_campfire_guard_starts_on_playback_not_queueing(self):
        c=self.make(); c.run('DIE(); FULL(); FULL_ERROR(); AURA_ON()')
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
        queued_at=c.g.time()
        c.wait(6)
        self.assertGreater(c.c.reactions.campfireLastPlayedAt,queued_at)
        self.assertEqual(c.sounds(),['death.mp3','bags_1.mp3','campfire_1.mp3'])

    def test_campfire_guard_is_per_character(self):
        c=self.make(); c.run('AURA_ON()')
        other=self.make(db=c.save(),setup='function UnitName() return "OtherCharacter" end; STATE.aura={spellId=1289723}')
        self.assertEqual(other.sounds(),['campfire_1.mp3'])

    def test_mute_persists_through_actual_save_reload(self):
        c=self.make(); c.run('SlashCmdList.FOREVERCOACH("fun off")')
        reloaded=self.make(db=c.save(),setup='STATE.aura={spellId=1289723}')
        reloaded.run('DIE(); FULL(); FULL_ERROR(); LEVEL(21)')
        self.assertEqual(reloaded.sounds(),[])
        self.assertFalse(reloaded.g.ForeverCoachDB.reactionsEnabled)

    def test_opaque_aura_tables_do_not_become_presence(self):
        c=self.make(setup='OPAQUE={}; STATE.aura=OPAQUE; function canaccesstable(v) return not rawequal(v,OPAQUE) end')
        self.assertEqual(c.sounds(),[])
        c.run('AURA_ON()')
        self.assertEqual(c.sounds(),['campfire_1.mp3'])

    def test_nonfinite_clock_cannot_create_campfire_eligibility(self):
        c=self.make()
        c.run('time=function() return math.huge end; AURA_ON()')
        self.assertEqual(c.sounds(),[])
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)

    def test_missing_sound_function_is_quiet_and_bounded(self):
        c=self.make(setup='PlaySoundFile=nil')
        c.run('FULL(); FULL_ERROR(); AURA_ON(); DIE()'); c.wait(15)
        self.assertEqual(c.sounds(),[])
        self.assertEqual(c.c.reactions.bagNext,1)
        self.assertIsNone(c.c.reactions.campfireLastPlayedAt)
        self.assertEqual(len(c.r.queue),0)

    def test_mute_stops_only_its_own_handle(self):
        c=self.make(); c.run('AURA_ON(); ACTIVE[888888]={ends=GetTime()+100}; SlashCmdList.FOREVERCOACH("fun off")')
        self.assertEqual(list(c.g.STOPS.values()),[100])
        self.assertIsNotNone(c.g.ACTIVE[888888])

    def test_toc_wires_reactions_before_ui_without_dependency(self):
        toc=(ADDON/'ForeverCoach.toc').read_text()
        self.assertIn('Reactions.lua',toc)
        self.assertLess(toc.index('Reactions.lua'),toc.index('UI.lua'))
        self.assertNotIn('SoundyFun',toc)


if __name__ == '__main__':
    unittest.main(verbosity=2)
