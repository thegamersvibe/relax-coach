#!/usr/bin/env python3
import sys, unittest
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))
from cleanup_harness import client

class CleanupDataTests(unittest.TestCase):
    def test_provider_contract_flavor_and_unreliable_revision(self):
        lua=client()
        self.assertIsNotNone(lua.globals().RelaxQuestPlanner.CleanupData, 'cleanup adapter is missing')
        lua.execute('''
          local D=RelaxQuestPlanner.CleanupData
          local p=D.Provider(); assert(p.available and p.version=='1.1.1' and p.contract==4)
          assert(p.flavor=='Forever' and not p.revisionKnown)
          LibQuestieDB.RequireContract=function() return false end; assert(not D.Provider().available)
          LibQuestieDB.RequireContract=function() error('fail') end; assert(not D.Provider().available)
          LibQuestieDB.RequireContract=function() return true end
          LibQuestieDB.flavor=nil; assert(not D.Provider().available)
          LibQuestieDB=nil; assert(not D.Provider().available)
        ''')

    def test_fields_keep_nil_error_and_namespace_identity(self):
        lua=client()
        self.assertIsNotNone(lua.globals().RelaxQuestPlanner.CleanupData, 'cleanup adapter is missing')
        lua.execute('''
          QUEST(30,{sourceItemId=15885,requiredSourceItems={15882,15883},
            startedBy={{999},{888},{19973}},objectives={[3]={{6291}},[6]={{10,'spell',700}}},
            extraObjectives={{nil,nil,nil,nil,{{'item',701},{'monster',702}}}}})
          local D=RelaxQuestPlanner.CleanupData; local r=D.ReadQuest(30)
          assert(r.fields.sourceItemId.value==15885 and r.fields.sourceItemId.state=='known')
          assert(r.fields.startedBy.value[1][1]==999 and r.fields.startedBy.value[3][1]==19973)
          assert(r.fields.requiredSourceItems.value[2]==15883)
          assert(r.fields.objectives.value[6][1][3]==700)
          assert(r.fields.name.source=='QuestieDB' and r.fields.name.version=='1.1.1')
          assert(r.fields.preQuestGroup.state=='known' and r.fields.preQuestGroup.value==nil)
          local get=LibQuestieDB.Quest.Get
          LibQuestieDB.Quest.Get=function(id,key) if key=='sourceItemId' then error('bad') end return get(id,key) end
          r=D.ReadQuest(30); assert(not r.complete and r.fields.sourceItemId.state=='error')
          assert(r.fields.requiredSourceItems.state=='known')
          assert(not D.ReadQuest(-30).complete and not D.ReadQuest(404).complete)
        ''')

    def test_only_canonical_provider_fields_are_queried(self):
        lua=client()
        lua.execute('''
          QUEST(1); ITEM(1,{class=12})
          local qget,iget=LibQuestieDB.Quest.Get,LibQuestieDB.Item.Get
          LibQuestieDB.Quest.Get=function(id,key) assert(key~='type','not a QuestieDB field'); return qget(id,key) end
          LibQuestieDB.Item.Get=function(id,key) assert(key~='itemClass','field is class'); return iget(id,key) end
          local D=RelaxQuestPlanner.CleanupData
          assert(D.ReadQuest(1).complete)
          assert(D.ReadItem(1).complete and D.ReadItem(1).fields.class.value==12)
        ''')

    def test_unreadable_provider_method_lookup_is_not_a_crash(self):
        lua=client()
        lua.execute('''
          QUEST(1); ITEM(1)
          local D=RelaxQuestPlanner.CleanupData
          LibQuestieDB.Quest=setmetatable({}, {__index=function() error('unreadable') end})
          local ok,row=pcall(D.ReadQuest,1)
          assert(ok and not row.complete,'method lookup must fail closed')
          local ids,state=D.Ids(LibQuestieDB.Quest)
          assert(ids==nil and state=='unknown')
        ''')

    def test_exact_ids_reverse_rewards_and_unreadable_values(self):
        lua=client()
        self.assertIsNotNone(lua.globals().RelaxQuestPlanner.CleanupData, 'cleanup adapter is missing')
        lua.execute('''
          ITEM(268579,{name='Crest',relatedQuests={1},questRewards={3},startQuest=4})
          ITEM(275521,{name='Crest',relatedQuests={2}})
          local D=RelaxQuestPlanner.CleanupData
          assert(D.ReadItem(268579).fields.relatedQuests.value[1]==1)
          assert(D.ReadItem(275521).fields.relatedQuests.value[1]==2)
          assert(D.ReadItem(268579).fields.questRewards.value[1]==3)
          I[268579].relatedQuests={SECRET}
          assert(not D.ReadItem(268579).complete)
          I[268579].relatedQuests='bad shape'
          assert(not D.ReadItem(268579).complete)
        ''')

if __name__=='__main__': unittest.main(verbosity=2)
