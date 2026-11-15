"""Real Player/Data/Engine/UI correctness regressions; invented API-edge fixtures."""
from pathlib import Path
import sys
import unittest
from lupa.lua51 import LuaRuntime
sys.path.insert(0, str(Path(__file__).resolve().parent))
import ui_shell_test as ui
ADDON = ui.ADDON

API = r'''
STATE.complete={}
function UnitLevel() return STATE.level end
function UnitRace() return "Dwarf", "Dwarf", 3 end
function UnitClass() return "Warrior", "WARRIOR", 1 end
function UnitFactionGroup() return "Alliance" end
local function ids(t) local out={}; for q,v in pairs(t) do if v then out[#out+1]=q end end; table.sort(out); return out end
C_QuestLog.GetAllCompletedQuestIDs=function() return ids(STATE.done) end
C_QuestLog.GetNumQuestLogEntries=function() return #ids(STATE.log) end
C_QuestLog.GetInfo=function(i) local q=ids(STATE.log)[i]; if q then return {questID=q,isHeader=false,title="Probe "..q,level=30} end end
C_QuestLog.GetTrivialRange=function() return 7 end
C_QuestLog.IsComplete=function(q) return STATE.complete[q]==true end
C_Map.GetMapInfo=function(q) return {name=q==101 and "Home" or "Away"} end
function GetAreaInfo(q) return q==1 and "Home" or "Away" end
C_SkillInfo={GetNumSkillLines=function() return 0 end}
C_Reputation={GetNumFactions=function() return 0 end}
-- Clock budgets advance independently of the simulation's wall clock.
local budget=0
function debugprofilestop() budget=budget+0.001; return budget end
QUEST_ROWS={
 [12]={name="Probe first",requiredLevel=1,questLevel=30,zoneOrSort=1,
       startedBy={{1201}},finishedBy={{1201}},objectives={{{1202,1}}}},
 [13]={name="Probe next",requiredLevel=1,questLevel=30,zoneOrSort=1,
       preQuestGroup={12},startedBy={{1301}},finishedBy={{1301}},objectives={{{1302,1}}}}
}
NPC_ROWS={
 [1201]={name="First giver",zoneID=1,spawns={[1]={{40,50}}}},
 [1202]={name="First objective",zoneID=1,spawns={[1]={{45,55}}}},
 [1301]={name="Next giver",zoneID=1,spawns={[1]={{60,65}}}},
 [1302]={name="Next objective",zoneID=1,spawns={[1]={{70,75}}}}
}
QuestDB={Get=function(q,k) return QUEST_ROWS[q] and QUEST_ROWS[q][k] end,
 Exists=function(q) return QUEST_ROWS[q]~=nil end, GetAllIds=function() return {12,13} end}
LibQuestieDB={Quest=QuestDB,RequireContract=function() return true end,
 Enum={raceMaskById={[3]=4},classKeys={WARRIOR=1}},
 Npc={Get=function(q,k) return NPC_ROWS[q] and NPC_ROWS[q][k] end,Exists=function(q) return NPC_ROWS[q]~=nil end},
 Support={Get=function() return {private={dungeons={[1]={"Gnomeregan"}},areaIdToUiMapId={[1]=101}}} end}}
'''
HELPERS = r'''
U=RelaxQuestPlanner.UI; R=RelaxQuestPlanner
COUNT={snap=0,plan=0,render=0,key=0,hand=0}
BUILD_CALLS={}
local snapshot=R.Player.Snapshot
R.Player.Snapshot=function(...) COUNT.snap=COUNT.snap+1; return snapshot(...) end
if R.Player.LogKey then local key=R.Player.LogKey; R.Player.LogKey=function(...) COUNT.key=COUNT.key+1; return key(...) end end
local render=U.RenderRoute; U.RenderRoute=function(...) COUNT.render=COUNT.render+1; return render(...) end
local hand=U.HandoffJourney; U.HandoffJourney=function(...) COUNT.hand=COUNT.hand+1; return hand(...) end
for _,kind in ipairs({"BuildGoalAsync","BuildPathAsync"}) do
 local old=R.Engine[kind]
 R.Engine[kind]=function(target,P,opts,cb)
  COUNT.plan=COUNT.plan+1
  BUILD_CALLS[#BUILD_CALLS+1]={kind=kind,level=P.level,target=target}
  return old(target,P,opts,cb)
 end
end
function ADVANCE(dt,events)
 local count=math.floor(dt/0.25+0.5)
 for i=1,count do if events then FIRE("QUEST_LOG_UPDATE") end; TICK(0.25) end
end
function FLUSH() for i=1,80 do TICK(0.25); if not U.busy and #TIMERS==0 then return end end end
function FULL() U.Refresh(true); TICK(0.25); assert(not U.busy,"setup: full plan failed to settle"); EPOCH=U.fullAt or NOW end
function REC(label)
 local c=U.current
 RECORDS[#RECORDS+1]={label=label,now=NOW,fullAge=U.fullAt and NOW-U.fullAt or -1,
  q=c and c.q or -1,kind=c and c.kind or "none",pin=WP and WP.x or -1,
  snaps=COUNT.snap,plans=COUNT.plan,renders=COUNT.render,keys=COUNT.key,handoffs=COUNT.hand,
  dirty=U.dirty==true,visible=U.frame:IsShown(),levelPlanned=U.levelPlanned or -1,
  buildLevel=BUILD_CALLS[#BUILD_CALLS] and BUILD_CALLS[#BUILD_CALLS].level or -1,
  pending=#TIMERS,busy=U.busy==true,routeLevel=U.route and U.route.level or -1,
  availability13=U.route and U.route.availability and U.route.availability[13] or "unset",
  error=RelaxQuestPlannerDB.lastError or "none"}
end
RECORDS={}
FIRE("ADDON_LOADED","RelaxQuestPlanner"); FIRE("PLAYER_LOGIN"); ADVANCE(1)
if QA_MODE=="core" then SlashCmdList.RELAXQUESTPLANNER("goal 13")
else SlashCmdList.RELAXQUESTPLANNER("dungeon alpha keep") end
FLUSH(); FULL()
assert(U.current and U.current.q==12 and U.current.kind=="accept","setup: expected accept 12")
assert(WP and WP.x==0.4,"setup: expected first pin")
assert(not RelaxQuestPlannerDB.lastError,RelaxQuestPlannerDB.lastError)
'''
SCENARIOS = {
    'autopin_stale_dependency': r'''
 SlashCmdList.RELAXQUESTPLANNER("hide"); SlashCmdList.RELAXQUESTPLANNER("autopin")
 STATE.done[12]=true; REC("before_autopin")
 SlashCmdList.RELAXQUESTPLANNER("autopin"); ADVANCE(.5); REC("after_autopin")
 ''',
    'hidden_pin_dependency_unlock_on_time': r'''
STATE.log[12]=true; STATE.ready[12]=true; FIRE("QUEST_ACCEPTED",1,12); ADVANCE(0.5)
 SlashCmdList.RELAXQUESTPLANNER("hide"); REC("before_turnin")
 STATE.log[12]=nil; STATE.ready[12]=nil; STATE.done[12]=true
 FIRE("QUEST_TURNED_IN",12); FIRE("QUEST_REMOVED",12); ADVANCE(3.5); REC("after_retries")
 ADVANCE(60,true); REC("after60_unchanged_events")
 SlashCmdList.RELAXQUESTPLANNER("show"); ADVANCE(0.75); REC("shown_replanned")
''',
    'turnin_late_325_hidden_frequent': r'''
STATE.log[12]=true; STATE.ready[12]=true; FIRE("QUEST_ACCEPTED",1,12); ADVANCE(0.5)
 SlashCmdList.RELAXQUESTPLANNER("hide"); STATE.log[12]=nil; STATE.ready[12]=nil
 FIRE("QUEST_TURNED_IN",12); FIRE("QUEST_REMOVED",12); ADVANCE(3.25); REC("after_last_retry")
 STATE.done[12]=true; ADVANCE(15,true); REC("15s_after_flag"); ADVANCE(585,true); REC("600s_after_flag")
 SlashCmdList.RELAXQUESTPLANNER("show"); ADVANCE(0.75); REC("show_recovers")
''',
    'hidden_frequent_completion_then_show_dirty': r'''
SlashCmdList.RELAXQUESTPLANNER("hide"); STATE.done[12]=true
 ADVANCE(15,true); REC("hidden_15s"); ADVANCE(585,true); REC("hidden_600s")
 SlashCmdList.RELAXQUESTPLANNER("show"); ADVANCE(0.75); REC("shown_dirty_catchup")
 local snap=COUNT.snap; SlashCmdList.RELAXQUESTPLANNER("hide"); SlashCmdList.RELAXQUESTPLANNER("show"); ADVANCE(1)
 REC("second_show"); EXTRA=COUNT.snap-snap
''',
    'hidden_no_event_then_show_clean': r'''
SlashCmdList.RELAXQUESTPLANNER("hide"); REC("hide_clean"); STATE.done[12]=true
 ADVANCE(15); REC("hidden_15s"); SlashCmdList.RELAXQUESTPLANNER("show"); ADVANCE(1); REC("show_without_dirty")
 ADVANCE(60); REC("shown_no_events_60s"); FIRE("QUEST_LOG_UPDATE"); ADVANCE(0.5); REC("next_event")
''',
    'levels_lagging_real_eligibility': r'''
if QA_MODE=="core" then
  QUEST_ROWS[13].requiredLevel=31; RelaxQuestPlanner.Data.Reset()
  SlashCmdList.RELAXQUESTPLANNER("dungeon gnomeregan"); ADVANCE(0.5)
 end
 STATE.done[12]=true; U.Refresh(true); ADVANCE(0.5); REC("level30_blocked")
 FIRE("PLAYER_LEVEL_UP",31); ADVANCE(0.5); REC("event31_unit30_available")
 FIRE("PLAYER_LEVEL_UP",31); ADVANCE(0.5); REC("repeat31_downgrades")
 ADVANCE(60,true); REC("60s_later_unit_still_lagging")
''',
    'levels_same_refresh_out_of_order': r'''
SlashCmdList.RELAXQUESTPLANNER("hide"); REC("before_levels")
 FIRE("PLAYER_LEVEL_UP",32); FIRE("PLAYER_LEVEL_UP",31); ADVANCE(0.5); REC("32_then31_same_debounce")
''',
    'visible_no_events_completion_just_after_full': r'''
REC("baseline_full"); STATE.done[12]=true; REC("flag_arrives")
 ADVANCE(14.75); REC("15s_since_full"); ADVANCE(0.25); REC("15s_since_flag")
 ADVANCE(45); REC("60s_since_flag"); ADVANCE(540); REC("600s_since_flag")
 FIRE("QUEST_LOG_UPDATE"); ADVANCE(0.5); REC("next_event")
''',
    'turnin_late_325_hidden_no_events': r'''
STATE.log[12]=true; STATE.ready[12]=true; FIRE("QUEST_ACCEPTED",1,12); ADVANCE(0.5)
 SlashCmdList.RELAXQUESTPLANNER("hide"); STATE.log[12]=nil; STATE.ready[12]=nil
 FIRE("QUEST_TURNED_IN",12); FIRE("QUEST_REMOVED",12); ADVANCE(3.25); REC("after_last_retry")
 STATE.done[12]=true; ADVANCE(15); REC("15s_after_flag"); ADVANCE(585); REC("600s_after_flag")
 FIRE("QUEST_LOG_UPDATE"); ADVANCE(0.5); REC("next_unchanged_event")
''',
}

class RefreshCorrectness(unittest.TestCase):
    def run_scenario(self, name):
        lua = LuaRuntime(unpack_returned_tuples=True)
        lua.execute(ui.WOW)
        lua.execute(API)
        lua.globals()["QA_MODE"] = "core"
        for line in (ADDON / "RelaxQuestPlanner.toc").read_text().splitlines():
            rel = line.strip().replace("\\", "/")
            if rel.endswith(".lua") and not rel.startswith("Journey/"):
                lua.execute((ADDON / rel).read_text(), name=rel)
        lua.execute(HELPERS)
        lua.execute(SCENARIOS[name])
        records = lua.globals()["RECORDS"]
        result = {records[i]['label']: records[i] for i in range(1, len(records)+1)}
        for row in result.values(): self.assertEqual(row['error'], 'none')
        return result

    def test_hidden_dependency_and_late_flags(self):
        for name, label in [
            ('hidden_pin_dependency_unlock_on_time', 'after_retries'),
            ('turnin_late_325_hidden_frequent', '15s_after_flag'),
            ('hidden_frequent_completion_then_show_dirty', 'hidden_15s')]:
            with self.subTest(name=name):
                rows = self.run_scenario(name); r = rows[label]
                self.assertEqual(r['q'], 13)
                self.assertEqual(r['pin'], .6)
                self.assertEqual(r['availability13'], 'available')
                self.assertGreater(r['handoffs'], 0)
                if 'before_turnin' in rows:
                    self.assertGreater(r['handoffs'], rows['before_turnin']['handoffs'])

    def test_autopin_replans_stale_dependencies(self):
        rows = self.run_scenario('autopin_stale_dependency')
        r = rows['after_autopin']
        self.assertEqual((r['q'], r['pin'], r['availability13']), (13, .6, 'available'))
        self.assertGreater(r['handoffs'], rows['before_autopin']['handoffs'])

    def test_clean_show_reads_full_state(self):
        r = self.run_scenario('hidden_no_event_then_show_clean')['show_without_dirty']
        self.assertEqual((r['q'], r['pin']), (13, .6))

    def test_lagging_level_never_downgrades(self):
        rows = self.run_scenario('levels_lagging_real_eligibility')
        first = rows['event31_unit30_available']
        for label in ['repeat31_downgrades','60s_later_unit_still_lagging']:
            r = rows[label]
            self.assertEqual(r['buildLevel'],31)
            self.assertEqual(r['plans'],first['plans'])
            self.assertEqual((r['q'],r['pin']),(13,.6))

    def test_pending_levels_keep_maximum(self):
        r = self.run_scenario('levels_same_refresh_out_of_order')['32_then31_same_debounce']
        self.assertEqual(r['buildLevel'],32)

    def test_eventless_is_explicitly_not_guaranteed(self):
        for name, label in [('visible_no_events_completion_just_after_full','next_event'),
                            ('turnin_late_325_hidden_no_events','next_unchanged_event')]:
            with self.subTest(name=name):
                rows = self.run_scenario(name)
                stale = rows.get('600s_since_flag') or rows.get('600s_after_flag')
                assert stale is not None
                self.assertEqual(stale['q'],12) # accepted baseline limitation, not a timer promise
                r = rows[label]
                self.assertEqual((r['q'],r['pin']),(13,.6))

if __name__ == '__main__': unittest.main()
