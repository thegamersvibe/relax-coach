#!/usr/bin/env python3
"""Farming name cache: real TOC modules, mocked client, synthetic data only."""
import ast
from pathlib import Path
import unittest
import lupa.lua51 as lupa

ROOT = Path(__file__).resolve().parents[1]
ADDON = ROOT / 'addon' / 'ForeverCoach'
TREE = ast.parse((ROOT / 'test' / 'harness.py').read_text())
MOCK = next(ast.literal_eval(n.value) for n in TREE.body
            if isinstance(n, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'MOCK' for t in n.targets))
TIMERS = r'''
TIMERS, LOOKUPS, REQUESTS, NAMES, QUALITIES = {}, {}, {}, {}, {}
C_Timer.After = function(delay, fn) TIMERS[#TIMERS+1] = {at=GetTime()+delay, fn=fn} end
function STEP(seconds)
  local finish = GetTime()+seconds
  while true do
    local pick
    for i,t in ipairs(TIMERS) do
      if t.at <= finish and (not pick or t.at < TIMERS[pick].at) then pick=i end
    end
    if not pick then break end
    local t=table.remove(TIMERS,pick)
    ADVANCE(t.at-GetTime()); t.fn()
  end
  ADVANCE(finish-GetTime())
end
C_Item.GetItemInfo = function(id)
  LOOKUPS[#LOOKUPS+1] = {id=id, at=GetTime()}
  return NAMES[id], nil, QUALITIES[id]
end
C_Item.RequestLoadItemDataByID = function(id) REQUESTS[#REQUESTS+1]=id end
function InCombatLockdown() return STATE.lockdown == true end
function issecretvalue(v) return rawequal(v,SECRET) end
getmetatable(SECRET).__index = function() error('secret accessed') end
'''


def client(setup=''):
    lua = lupa.LuaRuntime(unpack_returned_tuples=True)
    lua.execute(MOCK)
    lua.execute(TIMERS)
    lua.execute(setup)
    ns = lua.table()
    for entry in (ADDON / 'ForeverCoach.toc').read_text().splitlines():
        if entry.strip().endswith('.lua') and not entry.startswith('#'):
            lua.execute('return function(...) ' + (ADDON / entry.strip()).read_text() + '\nend')('ForeverCoach', ns)
    g = lua.globals()
    g.FIRE('ADDON_LOADED', 'ForeverCoach')
    # Dispatch just the real cache frame to prove its independent guards;
    # Core's own logout flag/timers must not conceal a missing cache guard.
    lua.execute('''
        local frame
        for _, f in ipairs(FRAMES) do
            if f.events.AUCTION_HOUSE_CLOSED and f.events.ITEM_DATA_LOAD_RESULT then
                assert(not frame, "ambiguous item-name frame")
                frame = f
            end
        end
        assert(frame, "missing item-name frame")
        function NAME_EVENT(event, ...) frame.scripts.OnEvent(frame, event, ...) end
    ''')
    return lua, g


class ItemNamesTests(unittest.TestCase):
    def test_login_reads_hist_and_saves_name_and_quality(self):
        lua, g = client('AHProfitDB={hist={house={["900001"]="packed"}}}; NAMES[900001]="Synthetic blade"; QUALITIES[900001]=3')
        g.FIRE('PLAYER_LOGIN')
        g.STEP(0.11)
        self.assertTrue(lua.eval('ForeverCoachDB.itemNames and ForeverCoachDB.itemNames[900001] and ForeverCoachDB.itemNames[900001].n == "Synthetic blade"'), 'login should cache the game name')
        self.assertEqual(g.ForeverCoachDB.itemNames[900001].q, 3)
        self.assertEqual(len(g.LOOKUPS), 1)
        self.assertEqual(len(g.REQUESTS), 0)

    def test_client_placeholder_is_not_a_known_name(self):
        lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}; NAMES[91]="Item 91"')
        g.FIRE('PLAYER_LOGIN'); g.STEP(0.11)
        self.assertIsNone(g.ForeverCoachDB.itemNames[91], 'temporary client item-id labels must not become permanent')
        lua.execute('NAMES[91]="Real synthetic label"')
        g.FIRE('ITEM_DATA_LOAD_RESULT', 91, True); g.STEP(0.11)
        self.assertEqual(g.ForeverCoachDB.itemNames[91].n, 'Real synthetic label')

    def test_synchronous_unreadable_completion_does_not_loop(self):
        lua, g = client('AHProfitDB={hist={house={["90"]="x"}}}; C_Item.RequestLoadItemDataByID=function(id) REQUESTS[#REQUESTS+1]=id; FIRE("ITEM_DATA_LOAD_RESULT",id,true) end')
        g.FIRE('PLAYER_LOGIN'); g.STEP(2)
        self.assertEqual(len(g.REQUESTS), 1, 'do not re-request a successful but unreadable result forever')
        self.assertLessEqual(len(g.LOOKUPS), 2)

    def test_async_completion_uses_same_throttle_and_ignores_other_items(self):
        lua, g = client('AHProfitDB={hist={house={["900002"]="packed"}}}')
        g.FIRE('PLAYER_LOGIN'); g.STEP(0.11)
        self.assertEqual(list(g.REQUESTS.values()), [900002], 'uncached item should request data')
        lua.execute('NAMES[900002]="Late blade"; QUALITIES[900002]=4')
        g.FIRE('ITEM_DATA_LOAD_RESULT', 900002, True)
        g.FIRE('ITEM_DATA_LOAD_RESULT', 900002, True)
        g.FIRE('ITEM_DATA_LOAD_RESULT', 900003, True)
        g.STEP(0.11)
        self.assertEqual(g.ForeverCoachDB.itemNames[900002].n, 'Late blade')
        self.assertEqual(len(g.LOOKUPS), 2)

    def test_throttle_survives_event_bursts(self):
        lua, g = client('AHProfitDB={hist={house={}}}; for i=1,120 do AHProfitDB.hist.house[tostring(i)]="packed"; NAMES[i]="Synthetic "..i end')
        for _ in range(10):
            g.FIRE('PLAYER_LOGIN'); g.FIRE('PLAYER_ENTERING_WORLD'); g.FIRE('AUCTION_HOUSE_CLOSED')
        self.assertLessEqual(len(g.LOOKUPS), 1, 'no unthrottled scan in event handler')
        g.STEP(13)
        self.assertEqual(len(g.LOOKUPS), 120)
        stamps = sorted(row.at for row in g.LOOKUPS.values())
        for start in stamps:
            self.assertLessEqual(sum(start <= s < start+1 for s in stamps), 50)

    def test_combat_blocks_work_and_resumes_even_for_late_data(self):
        for combat in ('combat', 'lockdown'):
            with self.subTest(combat=combat):
                lua, g = client('AHProfitDB={hist={house={["91"]="packed"}}}; STATE.'+combat+'=true')
                g.FIRE('PLAYER_LOGIN'); g.STEP(2)
                self.assertEqual(len(g.LOOKUPS), 0, 'combat must defer reads')
                lua.execute('STATE.'+combat+'=false')
                g.FIRE('PLAYER_REGEN_ENABLED'); g.STEP(0.11)
                self.assertEqual(list(g.REQUESTS.values()), [91])
                lua.execute('NAMES[91]="Combat deferred"; STATE.'+combat+'=true')
                g.FIRE('ITEM_DATA_LOAD_RESULT', 91, True); g.STEP(2)
                self.assertEqual(len(g.LOOKUPS), 1)
                lua.execute('STATE.'+combat+'=false')
                g.FIRE('PLAYER_REGEN_ENABLED'); g.STEP(0.11)
                self.assertEqual(g.ForeverCoachDB.itemNames[91].n, 'Combat deferred')

    def test_close_and_world_entry_only_query_missing_and_never_write_donor(self):
        lua, g = client('AHProfitDB={hist={house={["91"]="packed",["92"]="other"},other={["91"]="duplicate"}},private={1,2,3}}; NAMES[91]="Known"; NAMES[92]="New"; ForeverCoachDB={itemNames={[91]={n="Kept",q=2}}}')
        def snapshot(t):
            return sorted((str(k), snapshot(v) if lupa.lua_type(v) == 'table' else v) for k, v in t.items())
        before = snapshot(g.AHProfitDB)
        lua.execute('OLD_DB=AHProfitDB; OLD_HIST=AHProfitDB.hist; OLD_HOUSE=AHProfitDB.hist.house')
        g.FIRE('PLAYER_ENTERING_WORLD'); g.STEP(0.3)
        self.assertEqual(g.ForeverCoachDB.itemNames[92].n, 'New')
        g.FIRE('AUCTION_HOUSE_CLOSED'); g.STEP(0.3)
        self.assertEqual([r.id for r in g.LOOKUPS.values()], [92])
        self.assertEqual(g.ForeverCoachDB.itemNames[91].n, 'Kept')
        self.assertEqual(snapshot(g.AHProfitDB), before)
        self.assertTrue(lua.eval('AHProfitDB==OLD_DB and AHProfitDB.hist==OLD_HIST and AHProfitDB.hist.house==OLD_HOUSE'))
        lua.execute('AHProfitDB.hist.house["93"]="later"; NAMES[93]="After close"')
        g.FIRE('AUCTION_HOUSE_CLOSED'); g.STEP(0.11)
        self.assertEqual(g.ForeverCoachDB.itemNames[93].n, 'After close')

    def test_cap_no_new_entries_or_lookups_when_full(self):
        lua, g = client('ForeverCoachDB={itemNames={}}; for i=1,9999 do ForeverCoachDB.itemNames[i]={n="Known"} end; AHProfitDB={hist={house={["10000"]="x",["10001"]="x",["10002"]="x"}}}; for i=10000,10002 do NAMES[i]="Extra" end')
        g.FIRE('PLAYER_LOGIN'); g.STEP(1)
        self.assertEqual(len(list(g.ForeverCoachDB.itemNames.items())), 10000)
        self.assertEqual(len(g.LOOKUPS), 1)
        g.FIRE('AUCTION_HOUSE_CLOSED'); g.STEP(1)
        self.assertEqual(len(g.LOOKUPS), 1)

    def test_absent_malformed_and_secret_sources_are_harmless(self):
        for value in ('nil', 'false', '7', '"bad"', 'SECRET', '{}', '{hist=SECRET}', '{hist=7}', '{hist={bad=7,secret=SECRET}}', '{hist={house={["-1"]="x",["01"]="x",["1.2"]="x",["10000001"]="x",["2"]={},["3"]=SECRET,[SECRET]="x"}}}'):
            with self.subTest(value=value):
                lua, g = client('AHProfitDB='+value)
                g.FIRE('PLAYER_LOGIN'); g.FIRE('AUCTION_HOUSE_CLOSED'); g.STEP(1)
                self.assertEqual(len(g.LOOKUPS), 0)
                self.assertIsNone(g.ForeverCoach.Char(g.ForeverCoach).lastError)

    def test_name_and_quality_validation_and_legacy_api(self):
        lua, g = client('''AHProfitDB={hist={house={}}}
            local names={"",string.rep("x",129),"bad\\nname","|Hitem:1|h[x]|h",SECRET,42,"Valid","Unknown quality","Token","Fraction","Negative","Too high","Secret quality"}
            for i,n in ipairs(names) do AHProfitDB.hist.house[tostring(i)]="x"; NAMES[i]=n end
            QUALITIES[7]=0; QUALITIES[9]=8; QUALITIES[10]=1.5; QUALITIES[11]=-1; QUALITIES[12]=9; QUALITIES[13]=SECRET
            GetItemInfo=C_Item.GetItemInfo; C_Item.GetItemInfo=nil''')
        g.FIRE('PLAYER_LOGIN'); g.STEP(2)
        for i in range(1,7):
            self.assertIsNone(g.ForeverCoachDB.itemNames[i])
        self.assertEqual(g.ForeverCoachDB.itemNames[7].q, 0)
        self.assertEqual(g.ForeverCoachDB.itemNames[9].q, 8)
        for i in (8,10,11,12,13):
            self.assertIsNone(g.ForeverCoachDB.itemNames[i].q)
        lua.execute('AHProfitDB.hist.house["14"]="x"; NAMES[14]="Fallback quality"; C_Item.GetItemQualityByID=function() return 6 end')
        g.FIRE('AUCTION_HOUSE_CLOSED'); g.STEP(2)
        self.assertEqual(g.ForeverCoachDB.itemNames[14].q, 6)

    def test_failed_load_can_retry_on_next_close_and_logout_stops_work(self):
        lua, g = client('AHProfitDB={hist={house={["90"]="x"}}}')
        g.FIRE('PLAYER_LOGIN'); g.STEP(0.11)
        g.FIRE('ITEM_DATA_LOAD_RESULT', 90, False)
        lua.execute('NAMES[90]="Recovered"')
        g.FIRE('AUCTION_HOUSE_CLOSED'); g.STEP(0.11)
        self.assertEqual(g.ForeverCoachDB.itemNames[90].n, 'Recovered')
        lua.execute('AHProfitDB.hist.house["91"]="x"; NAMES[91]="After logout"')
        g.FIRE('AUCTION_HOUSE_CLOSED'); g.FIRE('PLAYER_LOGOUT'); g.STEP(1)
        self.assertIsNone(g.ForeverCoachDB.itemNames[91])


    def test_combat_gate_before_scheduling(self):
        for combat in ('combat', 'lockdown'):
            with self.subTest(combat=combat):
                lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}')
                lua.execute('STATE.'+combat+'=true; TIMERS={}')
                g.NAME_EVENT('PLAYER_LOGIN')
                self.assertEqual(len(g.TIMERS), 0, 'combat must not schedule a cache timer')
                lua.execute('STATE.'+combat+'=false')
                g.NAME_EVENT('PLAYER_REGEN_ENABLED'); g.STEP(0.11)
                self.assertEqual(list(g.REQUESTS.values()), [91])

    def test_combat_gate_rechecked_when_timer_fires(self):
        for combat in ('combat', 'lockdown'):
            with self.subTest(combat=combat):
                lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}')
                g.NAME_EVENT('PLAYER_LOGIN')
                self.assertGreater(len(g.TIMERS), 0, 'timer must actually be pending')
                lua.execute('STATE.'+combat+'=true')
                g.STEP(0.11)
                self.assertEqual(len(g.LOOKUPS), 0, 'combat began after scheduling')
                lua.execute('STATE.'+combat+'=false')
                g.NAME_EVENT('PLAYER_REGEN_ENABLED'); g.STEP(0.11)
                self.assertEqual(list(g.REQUESTS.values()), [91])

    def test_combat_unreadable_or_error_fails_closed(self):
        for api in ('InCombatLockdown', 'UnitAffectingCombat'):
            for result in ('return nil', 'return SECRET', 'return 0', 'error("unreadable")'):
                with self.subTest(api=api, result=result):
                    lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}')
                    lua.execute(api+'=function() '+result+' end; TIMERS={}')
                    g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(0.11)
                    self.assertEqual(len(g.LOOKUPS), 0, 'unreadable combat must block')
                    self.assertEqual(len(g.TIMERS), 0)
                    lua.execute(api+'=function() return false end')
                    g.NAME_EVENT('PLAYER_REGEN_ENABLED'); g.STEP(0.11)
                    self.assertEqual(list(g.REQUESTS.values()), [91])

    def test_combat_apis_absent_fails_closed(self):
        lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}')
        lua.execute('InCombatLockdown=nil; UnitAffectingCombat=nil; TIMERS={}')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(0.11)
        self.assertEqual(len(g.LOOKUPS), 0)
        self.assertEqual(len(g.TIMERS), 0)
        lua.execute('UnitAffectingCombat=function() return false end')
        g.NAME_EVENT('PLAYER_REGEN_ENABLED'); g.STEP(0.11)
        self.assertEqual(list(g.REQUESTS.values()), [91], 'one readable API suffices')

    def test_throttle_interval_is_one_tenth_second(self):
        lua, g = client('AHProfitDB={hist={house={["91"]="x",["92"]="x"}}}')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(0.099)
        self.assertEqual(len(g.LOOKUPS), 0, 'first lookup must wait 0.1s')
        g.STEP(0.002)
        self.assertEqual(len(g.LOOKUPS), 1)
        first = g.LOOKUPS[1].id
        lua.execute('NAMES['+str(first)+']="Late synthetic"')
        g.NAME_EVENT('ITEM_DATA_LOAD_RESULT', first, True)
        g.STEP(0.098)
        self.assertEqual(len(g.LOOKUPS), 1, 'completion cannot bypass the interval')
        g.STEP(0.202)
        self.assertEqual(len(g.LOOKUPS), 3)
        stamps = [r.at for r in g.LOOKUPS.values()]
        for a, b in zip(stamps, stamps[1:]):
            self.assertGreaterEqual(b-a, 0.1-1e-9)

    def test_scan_visits_at_most_eight_houses(self):
        lua, g = client('AHProfitDB={hist={}}; for i=1,9 do AHProfitDB.hist["house"..i]={[tostring(i)]="x"} end')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(1)
        self.assertEqual(len(g.REQUESTS), 8)
        self.assertEqual(len(set(g.REQUESTS.values())), 8)

    def test_newer_schema_cache_is_untouched(self):
        lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}; NAMES[91]="Fresh synthetic"')
        # Set after ADDON_LOADED: Core's legacy Char initialization sets schema.
        lua.execute('ForeverCoachDB.schema=ForeverCoach.SCHEMA+1; OLD={future=true}; ForeverCoachDB.itemNames=OLD')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(1)
        self.assertEqual(len(g.LOOKUPS), 0)
        self.assertTrue(lua.eval('ForeverCoachDB.itemNames==OLD and OLD.future==true'))

    def test_logout_stops_cache_without_core_flag(self):
        lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}; NAMES[91]="After logout"')
        g.NAME_EVENT('PLAYER_LOGIN')
        g.NAME_EVENT('PLAYER_LOGOUT')
        self.assertFalse(bool(g.ForeverCoach._loggingOut), 'exercise cache stop independently')
        g.STEP(1)
        g.NAME_EVENT('PLAYER_REGEN_ENABLED'); g.NAME_EVENT('AUCTION_HOUSE_CLOSED'); g.STEP(1)
        self.assertEqual(len(g.LOOKUPS), 0, 'pending and new work must remain stopped')

    def test_load_result_requires_waiting_id(self):
        lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}; NAMES[92]="Unrequested synthetic"')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(0.11)
        g.NAME_EVENT('ITEM_DATA_LOAD_RESULT', 92, True); g.STEP(0.11)
        self.assertEqual([r.id for r in g.LOOKUPS.values()], [91])
        self.assertIsNone(g.ForeverCoachDB.itemNames[92])

    def test_load_result_requires_readable_true(self):
        for result in (None, 1, 'true'):
            with self.subTest(result=result):
                lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}')
                g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(0.11)
                lua.execute('NAMES[91]="Not a successful result"')
                g.NAME_EVENT('ITEM_DATA_LOAD_RESULT', 91, result); g.STEP(0.11)
                self.assertEqual(len(g.LOOKUPS), 1)
                self.assertIsNone(g.ForeverCoachDB.itemNames[91])

    def test_numeric_fractional_history_id_is_rejected(self):
        lua, g = client('AHProfitDB={hist={house={[1.5]="x"}}}; NAMES[1.5]="Fractional synthetic"')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(1)
        self.assertEqual(len(g.LOOKUPS), 0)

    def test_saved_cache_is_capped_during_initialization(self):
        lua, g = client('ForeverCoachDB={itemNames={}}; for i=1,10001 do ForeverCoachDB.itemNames[i]={n="Saved synthetic"} end')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(0.11)
        self.assertEqual(len(list(g.ForeverCoachDB.itemNames.items())), 10000)

    def test_failed_request_releases_waiting_state(self):
        lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}; C_Item.RequestLoadItemDataByID=function(id) REQUESTS[#REQUESTS+1]=id; error("failed request") end')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(0.11)
        g.NAME_EVENT('AUCTION_HOUSE_CLOSED'); g.STEP(0.11)
        self.assertEqual(list(g.REQUESTS.values()), [91, 91], 'failed call must allow a later retry')

    def test_scan_work_is_bounded_without_resolved_names(self):
        lua, g = client('AHProfitDB={hist={house={}}}; for i=1,10001 do AHProfitDB.hist.house[tostring(i)]="x" end')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(1002)
        self.assertEqual(len(g.REQUESTS), 10000, 'waiting names still consume the work budget')

    def test_numeric_placeholder_is_rejected(self):
        lua, g = client('AHProfitDB={hist={house={["91"]="x"}}}; NAMES[91]="91"')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(0.11)
        self.assertIsNone(g.ForeverCoachDB.itemNames[91])

    def test_saved_aliases_count_once_toward_cap(self):
        lua, g = client('ForeverCoachDB={itemNames={}}; for i=1,9999 do ForeverCoachDB.itemNames[i]={n="Saved synthetic"} end; ForeverCoachDB.itemNames["1"]={n="Alias synthetic"}; AHProfitDB={hist={house={["10000"]="x"}}}; NAMES[10000]="Last synthetic"')
        g.NAME_EVENT('PLAYER_LOGIN'); g.STEP(0.11)
        self.assertEqual(len(g.LOOKUPS), 1, 'numeric and string keys for one ID use one slot')
        self.assertEqual(len(list(g.ForeverCoachDB.itemNames.items())), 10000)
        self.assertEqual(g.ForeverCoachDB.itemNames[10000].n, 'Last synthetic')


if __name__ == '__main__':
    unittest.main(verbosity=2)
