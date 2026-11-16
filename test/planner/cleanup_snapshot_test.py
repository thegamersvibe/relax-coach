#!/usr/bin/env python3
import sys, unittest
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))
from cleanup_harness import client
MODULES=('Core/Data.lua','Core/CleanupData.lua','Core/CleanupSnapshot.lua')

class CleanupSnapshotTests(unittest.TestCase):
    def runtime(self):
        lua=client(MODULES)
        self.assertIsNotNone(lua.globals().RelaxQuestPlanner.Cleanup, 'cleanup snapshot is missing')
        return lua

    def test_complete_snapshot_and_distinct_ready_history_log(self):
        self.runtime().execute('''
          CURRENT(512); DONE={1701}; BAG(6842,3)
          local s=RelaxQuestPlanner.Cleanup.Capture()
          assert(s.complete and s.log[512].trivial==true and s.log[512].ready==false)
          assert(s.done[1701] and not s.done[512] and s.identity=='Player-fixture')
          assert(s.slots[1].itemID==6842 and s.slots[1].count==3 and s.slots[1].valid)
          assert(s.build=='70938' and s.provider.version=='1.1.1')
          assert(s.reads.history=='known' and s.reads.log=='known')
        ''')

    def test_nil_secret_error_and_missing_reads_are_not_false_or_empty(self):
        for api in ('C_QuestLog.GetInfo','C_QuestLog.GetAllCompletedQuestIDs',
                    'C_QuestLog.IsQuestTrivial','C_QuestLog.ReadyForTurnIn','C_QuestLog.IsRepeatableQuest',
                    'C_QuestLog.GetNumQuestLogEntries','UnitGUID','UnitLevel','UnitClass','UnitFactionGroup',
                    'GetBuildInfo','time','C_Container.GetContainerNumSlots','C_Container.GetContainerNumFreeSlots',
                    'C_Container.GetContainerItemInfo','C_Container.GetContainerItemID','C_Container.GetContainerItemQuestInfo',
                    'C_Item.GetItemInfoInstant'):
            for replacement in ('nil','function() error("read failed") end','function() return SECRET end'):
                with self.subTest(api=api, replacement=replacement):
                    lua=self.runtime()
                    lua.execute('CURRENT(512); BAG(6842); '+api+'='+replacement)
                    lua.execute('''
                      local s=RelaxQuestPlanner.Cleanup.Capture()
                      assert(not s.complete, 'failed read must be visible')
                      assert(next(s.failures), 'must name failed reads')
                    ''')
        self.runtime().execute('''
          CURRENT(512); C_QuestLog.ReadyForTurnIn=function() return nil end
          C_QuestLog.GetAllCompletedQuestIDs=function() error('history') end
          local s=RelaxQuestPlanner.Cleanup.Capture()
          assert(s.log[512].ready==nil and s.log[512].readyState=='unknown')
          assert(s.reads.history=='error' and s.done==nil)
          C_QuestLog.GetNumQuestLogEntries=function() return nil end
          assert(RelaxQuestPlanner.Cleanup.Capture().log==nil)
        ''')

    def test_slots_lock_missing_count_change_and_partial_enumeration(self):
        for mutation in ('BAGS[0][1].isLocked=true','BAGS[0][1].stackCount=nil',
                         'C_Container.GetContainerNumSlots=function(b) if b==2 then error("bag") end return #BAGS[b] end',
                         'C_Container.GetContainerItemInfo=function() return nil end',
                         'local old=C_Container.GetContainerItemInfo; local n=0; C_Container.GetContainerItemInfo=function(...) n=n+1; if n>1 then return {itemID=9,stackCount=1,isLocked=false} end return old(...) end'):
            with self.subTest(mutation=mutation):
                self.runtime().execute('BAG(6842); '+mutation+'''
                  local s=RelaxQuestPlanner.Cleanup.Capture(); assert(not s.complete and s.reads.bags~='known')
                ''')

    def test_empty_slot_requires_consistent_free_slot_enumeration(self):
        self.runtime().execute('''
          C_Container.GetContainerNumSlots=function(b) return b==0 and 1 or 0 end
          C_Container.GetContainerNumFreeSlots=function(b) return b==0 and 1 or 0 end
          local s=RelaxQuestPlanner.Cleanup.Capture(); assert(s.complete and #s.slots==0)
          C_Container.GetContainerNumFreeSlots=function() return 0 end
          assert(not RelaxQuestPlanner.Cleanup.Capture().complete)
        ''')

if __name__=='__main__': unittest.main(verbosity=2)
