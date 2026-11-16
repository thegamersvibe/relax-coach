#!/usr/bin/env python3
import sys,unittest
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parent))
from cleanup_harness import policy
from ui_shell_test import client as planner_client

class CleanupLifecycleTests(unittest.TestCase):
    def test_throwing_cleanup_event_still_records_planner_turnin(self):
        # Review probe: a review-hook error must not skip the planner's handler.
        planner_client().execute('''
          RelaxQuestPlanner.Cleanup={OnEvent=function() error('review hook failed') end}
          local before=#LEARN_CALLS
          FIRE('QUEST_TURNED_IN',5,500,0); DRAIN()
          assert(#LEARN_CALLS==before+1,'review error suppressed planner turn-in')
          local record=LEARN_CALLS[#LEARN_CALLS]
          assert(record.kind=='turnin' and record.q==5 and record.npc==901)
          assert(RelaxQuestPlannerDB.lastError:find('review hook failed'))
        ''')

    def runtime(self):
        lua=policy()
        self.assertIsNotNone(lua.globals().RelaxQuestPlanner.Cleanup.Open,'review lifecycle is missing')
        lua.execute('''
          QUEST(1701,{sourceItemId=6842}); ITEM(6842); BAG(6842); DONE={1701}
          ROUTE_REFRESHES=0
          RelaxQuestPlanner.UI.Refresh=function() ROUTE_REFRESHES=ROUTE_REFRESHES+1 end
        ''')
        return lua

    def test_lazy_open_refresh_debounce_close_and_no_hidden_scans(self):
        self.runtime().execute('''
          local C=RelaxQuestPlanner.Cleanup
          FIRE('BAG_UPDATE_DELAYED'); DRAIN(); assert(READS==0 and ROUTE_REFRESHES==0)
          C.Open(); DRAIN(); local r,status=C.GetReport()
          assert(status=='Ready' and r.itemRows[1].reasonCodes[1]=='no-use-found')
          local before=READS
          for i=1,10 do FIRE('BAG_UPDATE_DELAYED') end
          assert(C.GetReport().itemRows[1].reasonCodes[1]=='state-unresolved')
          TICK(0.24); assert(READS==before)
          TICK(0.01); DRAIN(); assert(C.GetReport().itemRows[1].reasonCodes[1]=='no-use-found')
          assert(ROUTE_REFRESHES==0)
          C.Close(); before=READS
          for i=1,20 do FIRE('BAG_UPDATE_DELAYED'); TICK(10) end
          assert(READS==before)
          C.Open(); C.Close(); DRAIN(); assert(not C.visible)
        ''')

    def test_index_survives_frequent_snapshot_events_and_publishes_latest_labels(self):
        # Adapted from the reviewer's 6,500-row starvation probe. A synthetic
        # clock exercises the real 2 ms budget deterministically, not host speed.
        for event in ('BAG_UPDATE_DELAYED', 'QUEST_LOG_UPDATE', 'GET_ITEM_INFO_RECEIVED',
                      'QUEST_ACCEPTED', 'QUEST_TURNED_IN', 'QUEST_REMOVED'):
            with self.subTest(event=event):
                lua=self.runtime()
                lua.execute('SNAPSHOT_EVENT=...',event)
                lua.execute('''
                  for i=1,1500 do
                    QUEST(100000+i,{objectives={[3]={{200000+i}}},
                      preQuestSingle={100000+math.max(1,i-1)},zoneOrSort=12})
                  end
                  for i=1,5000 do
                    ITEM(200000+i,{relatedQuests={100000+(i%1500)+1},class=12,flags=0})
                  end
                  local enumerations=0
                  for _,api in ipairs({LibQuestieDB.Quest,LibQuestieDB.Item}) do
                    local ids=api.GetAllIds
                    api.GetAllIds=function() enumerations=enumerations+1; return ids() end
                  end
                  local ticks=0
                  debugprofilestop=function() ticks=ticks+0.01; return ticks end
                  local C=RelaxQuestPlanner.Cleanup
                  C.Open()
                  local firstReady,latestCount,active=nil,1,false
                  for frame=1,3600 do
                    TICK(0.016)
                    local report,status=C.GetReport()
                    if status=='Ready' then
                      firstReady=firstReady or frame
                      local row=report.itemRows[1]
                      assert(report.generation==C.generation,'stale snapshot generation published')
                      assert(row.slots[1].count==latestCount,'stale bag count published')
                      assert(row.reasonCodes[1]==(active and 'active-use' or 'no-use-found'),
                        'labels do not reflect the latest quest snapshot')
                    end
                    if frame%300==0 and frame<3600 then
                      latestCount=frame/300+1; active=not active
                      BAGS[0][1].stackCount=latestCount
                      LOG={}; DONE=active and {} or {1701}
                      if active then CURRENT(1701) end
                      FIRE(SNAPSHOT_EVENT,1701)
                    end
                  end
                  assert(firstReady and firstReady<=1200,'index starved under snapshot events')
                  assert(select(2,C.GetReport())=='Ready','latest snapshot did not settle')
                  assert(C.GetReport().itemRows[1].disposition=='KEEP')
                  assert(enumerations==2,'static provider was enumerated more than once')
                  assert(RelaxQuestPlannerDB.lastError==nil,RelaxQuestPlannerDB.lastError)
                ''')

    def test_generation_character_provider_and_native_changes_cannot_publish_stale_scan(self):
        for change in ("C.OnEvent('QUEST_LOG_UPDATE')", "UnitGUID=function() return 'Player-other' end",
                       "C_AddOns.GetAddOnMetadata=function() return 'new' end", "LibQuestieDB.Quest=API(Q)",
                       "BAGS[0][1].stackCount=7", "LOG={{questID=1701,title='Changed',isHeader=false}}",
                       "RelaxQuestPlannerDB.selection={kind='goal',q=1701}"):
            with self.subTest(change=change):
                self.runtime().execute('''
                  local C=RelaxQuestPlanner.Cleanup
                  for i=1,300 do QUEST(10000+i) end
                  C.Open(); TICK(0.01); assert(select(2,C.GetReport())=='Scanning')
                  local generation=C.generation
                  '''+change+'''
                  DRAIN()
                  local r,status=C.GetReport()
                  assert(r.generation~=generation or status~='Ready','stale snapshot published')
                ''')

    def test_bounded_job_yields_cancels_failure_and_combat_pauses(self):
        self.runtime().execute('''
          local C=RelaxQuestPlanner.Cleanup
          for i=1,250 do QUEST(10000+i) end
          local called=0
          local j=C.BuildIndexAsync(nil,function() called=called+1 end)
          TICK(0.01); assert(j.records<=100 and called==0)
          j:Cancel(); DRAIN(); assert(called==0)
          local ticks=0
          debugprofilestop=function() ticks=ticks+0.3; return ticks end
          j=C.BuildIndexAsync(nil,function() called=called+1 end)
          TICK(0.01); assert(j.records<10 and called==0)
          STATE.combat=true; local n=READS; DRAIN(); assert(READS==n)
          j:Cancel(); STATE.combat=false
          C.Open(); DRAIN(3000); assert(select(2,C.GetReport())=='Ready')
          STATE.combat=true; FIRE('PLAYER_REGEN_DISABLED')
          assert(select(2,C.GetReport())=='Paused in combat')
          n=READS; DRAIN(); assert(READS==n)
          STATE.combat=false; FIRE('PLAYER_REGEN_ENABLED'); DRAIN(3000)
          assert(select(2,C.GetReport())=='Ready')
          C.Close(); InCombatLockdown=nil; C.Open(); DRAIN()
          assert(select(2,C.GetReport())=='Limited data')
        ''')

    def test_pin_ids_only_and_newer_saves_untouched(self):
        self.runtime().execute('''
          local C=RelaxQuestPlanner.Cleanup
          C.Open(); DRAIN(); assert(C.SetPin('item',6842,true)); DRAIN()
          assert(C.GetReport().itemRows[1].disposition=='KEEP')
          assert(RelaxQuestPlannerDB.cleanup.schema==1 and RelaxQuestPlannerDB.cleanup.pins.item[6842])
          assert(not C.SetPin('item',-1,true) and not C.SetPin('slot',1,true))
          C.SetPin('item',6842,false); DRAIN()
          assert(C.GetReport().itemRows[1].disposition=='UNCERTAIN-KEEP')
          for id=1,250 do C.SetPin('item',id,true) end
          local count=0 for _ in pairs(RelaxQuestPlannerDB.cleanup.pins.item) do count=count+1 end
          assert(count<=100)
          local future={schema=2,pins={future=true},other='preserve'}
          RelaxQuestPlannerDB.cleanup=future
          assert(not C.SetPin('item',6842,true) and RelaxQuestPlannerDB.cleanup==future and future.other=='preserve')
        ''')

    def test_quest_removed_means_not_in_log_not_abandoned(self):
        self.runtime().execute('''
          local C=RelaxQuestPlanner.Cleanup
          DONE={}; CURRENT(1701); C.Open(); DRAIN()
          assert(C.GetReport().itemRows[1].disposition=='KEEP')
          LOG={}; FIRE('QUEST_REMOVED',1701); DRAIN()
          local row=C.GetReport().itemRows[1]
          assert(row.disposition=='KEEP' and table.concat(row.reasons,' '):find('not recorded turned in'))
          assert(not table.concat(row.reasons,' '):find('abandon'))
        ''')

if __name__=='__main__': unittest.main(verbosity=2)
