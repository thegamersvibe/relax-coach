#!/usr/bin/env python3
import sys,unittest
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parent))
from cleanup_harness import policy

class CleanupPolicyTests(unittest.TestCase):
    def runtime(self, before=''):
        lua=policy(before)
        self.assertIsNotNone(lua.globals().RelaxQuestPlanner.Cleanup.Evaluate,'review evaluator is missing')
        return lua

    def test_furen_no_use_is_information_not_permission_and_sorted_first(self):
        self.runtime().execute('''
          QUEST(1701,{sourceItemId=6842}); ITEM(6842,{name="Furen's Instructions"}); DONE={1701}; BAG(6842)
          QUEST(2200,{sourceItemId=7667}); ITEM(7667); BAG(7667); DONE[2]=2200
          SCAN()
          local r=ROW(6842)
          assert(r.disposition=='UNCERTAIN-KEEP' and r.reasonCodes[1]=='no-use-found')
          assert(r.reasons[1]=='No use found in our data' and REPORT.itemRows[1]==r)
          assert(r.detail:find('destroy side effects') and r.detail:find('had again') and r.detail:find('70888'))
          assert(r.detail:find('1.1.1') and r.detail:find('Keep or Toss 1.7.0'))
          assert(ROW(7667).disposition=='KEEP' and table.concat(ROW(7667).reasonCodes,','):find('tool'))
          assert(REPORT.candidates==nil and r.profileID==nil)
        ''')

    def test_typed_union_preserves_source_auxiliary_spell_extra_and_starters(self):
        self.runtime().execute('''
          QUEST(30,{objectives={[3]={{15885}}},requiredSourceItems={15882,15883}})
          QUEST(31,{sourceItemId=15885}); QUEST(76240,{requiredSourceItems={6291}})
          QUEST(8227,{startedBy={{9901},{9902},{19973}}})
          QUEST(40,{objectives={[6]={{123,'spell',700}}},extraObjectives={{nil,nil,nil,nil,{{'item',701},{'monster',702}}}}})
          CURRENT(31); CURRENT(76240); CURRENT(40); DONE={30}
          for _,id in ipairs({15885,15882,15883,6291,19973,700,701}) do ITEM(id); BAG(id,2) end
          SCAN()
          assert(INDEX.uses[9901]==nil and INDEX.uses[9902]==nil and INDEX.uses[702]==nil)
          local roles={} for _,e in ipairs(INDEX.uses[15885]) do roles[e.role]=true end
          assert(roles.objective and roles.source)
          assert(INDEX.uses[15882][1].role=='required-source')
          assert(ROW(6291).disposition=='KEEP' and ROW(15885).disposition=='KEEP')
          assert(ROW(19973).disposition=='KEEP' and table.concat(ROW(19973).reasonCodes,','):find('starter'))
          assert(ROW(700).disposition=='KEEP' and ROW(701).disposition=='KEEP')
        ''')

    def test_shared_all_stacks_and_no_import_absence_lowering(self):
        self.runtime().execute('''
          QUEST(1,{sourceItemId=6842}); QUEST(2,{requiredSourceItems={6842}})
          DONE={1}; CURRENT(2); ITEM(6842); BAG(6842,1); BAG(6842,10)
          SCAN(); local r=ROW(6842); assert(r.disposition=='KEEP' and #r.slots==2)
          LOG={}; DONE={1,2}; SCAN(); assert(ROW(6842).reasonCodes[1]=='no-use-found')
          local imported=RelaxQuestPlanner.CleanupUses
          for k in pairs(imported) do if k~='vanilla' then imported[k]={} end end
          SCAN(); assert(ROW(6842).disposition=='UNCERTAIN-KEEP' and ROW(6842).reasonCodes[1]=='no-use-found')
          QUEST(3,{sourceItemId=999999}); ITEM(999999); DONE[3]=3; BAG(999999); SCAN()
          assert(ROW(999999).reasonCodes[1]=='new-forever')
          assert(ROW(999999).reasons[1]=='New in Forever, no use data')
        ''')

    def test_read_failures_keep_positive_evidence_and_never_infer_no_use(self):
        for failure in ('LibQuestieDB=nil', 'LibQuestieDB.flavor={expansion="Classic"}',
                        'C_QuestLog.GetAllCompletedQuestIDs=nil','C_QuestLog.GetInfo=nil',
                        'C_QuestLog.IsQuestTrivial=nil','C_QuestLog.ReadyForTurnIn=nil',
                        'C_Container.GetContainerItemQuestInfo=nil','UnitLevel=nil',
                        'LibQuestieDB.Quest.GetAllIds=function() error("ids") end',
                        'LibQuestieDB.Item.Get=function() error("item") end'):
            with self.subTest(failure=failure):
                self.runtime().execute('''
                  QUEST(1701,{sourceItemId=6842}); ITEM(6842); BAG(6842); DONE={1701}; CURRENT(512); QUEST(512)
                  '''+failure+'''
                  SCAN(); assert(ROW(6842).reasonCodes[1]=='state-unresolved')
                  assert(ROW(6842).reasons[1]=='Quest state could not be resolved')
                  assert(REPORT.status:find('Limited data'))
                ''')
        self.runtime().execute('''
          BAG(7667); LibQuestieDB=nil; SCAN()
          assert(ROW(7667).disposition=='KEEP')
        ''')

    def test_grey_quests_ready_pins_and_unresolved_are_reviewed_not_candidates(self):
        self.runtime().execute('''
          for _,id in ipairs({512,513,514,515,516}) do QUEST(id); CURRENT(id) end
          C_QuestLog.ReadyForTurnIn=function(id) return id==512 end
          C_QuestLog.IsQuestTrivial=function(id) if id==516 then return nil end return id~=515 end
          RelaxQuestPlannerDB.cleanup={schema=1,pins={quest={[514]=true},item={}}}
          SCAN()
          assert(ROW(512,'quest').disposition=='KEEP' and ROW(512,'quest').reasonCodes[1]=='ready-turnin')
          assert(ROW(513,'quest').disposition=='UNCERTAIN-KEEP')
          assert(ROW(514,'quest').disposition=='KEEP')
          assert(ROW(515,'quest')==nil,'non-grey rows do not crowd the review')
          assert(ROW(516,'quest').reasonCodes[1]=='state-unresolved')
        ''')

    def test_multihop_future_and_old_dungeons_ignore_route_filters(self):
        self.runtime().execute('''
          QUEST(1); QUEST(2,{preQuestSingle={1}}); QUEST(3,{preQuestGroup={2},questLevel=60})
          QUEST(4); QUEST(5,{preQuestGroup={4},questLevel=10,zoneOrSort=99})
          LibQuestieDB.Support={Get=function() return {private={dungeons={[99]={name='Old dungeon'}}}} end}
          CURRENT(1); CURRENT(4); RelaxQuestPlannerDB.mode='strict'; RelaxQuestPlannerDB.skipped={[1]=true}
          SCAN()
          assert(ROW(1,'quest').disposition=='KEEP' and ROW(4,'quest').disposition=='KEEP')
          assert(table.concat(ROW(1,'quest').relatedIDs,','):find('3'))
          assert(table.concat(ROW(4,'quest').relatedIDs,','):find('5'))
          RelaxQuestPlannerDB.mode='all'; SCAN(); assert(ROW(1,'quest').disposition=='KEEP')
        ''')

    def test_typed_or_and_active_parent_and_no_negative_fdj_lookup(self):
        self.runtime().execute('''
          QUEST(1654); QUEST(1655,{parentQuest=1654}); CURRENT(1654); CURRENT(1655)
          QUEST(338); QUEST(339,{parentQuest=338}); CURRENT(338); CURRENT(339)
          QUEST(10,{exclusiveTo={11}}); QUEST(11); QUEST(12)
          QUEST(20,{preQuestGroup={10,-12},questLevel=60}); QUEST(21,{preQuestSingle={10,11},questLevel=60})
          CURRENT(10); CURRENT(11); CURRENT(12)
          ForeverDungeonJournal_NS={DB={Test={quests={{id=99,name='Target',level=60}}}},
            QUEST_PREREQ_CHAINS={[99]={{id=-88,name='Placeholder'},{id=10,name='Real'}}}}
          local get=LibQuestieDB.Quest.Get
          LibQuestieDB.Quest.Get=function(id,key) assert(id~=88 and id>0,'placeholder must not be queried'); return get(id,key) end
          SCAN()
          assert(ROW(1654,'quest').disposition=='KEEP' and ROW(338,'quest').disposition=='KEEP')
          local kinds={} for _,e in ipairs(INDEX.edges) do kinds[e.kind]=true end
          assert(kinds['active-parent'] and kinds['or-completed'] and kinds['and-completed-no-substitution'])
          assert(kinds['exclusive-substitute'])
          assert(INDEX.unknown[99] and not INDEX.quests[88])
        ''')

    def test_cycles_self_missing_eligibility_and_repeatable_item_use(self):
        self.runtime().execute('''
          QUEST(1,{preQuestGroup={2}}); QUEST(2,{preQuestGroup={1}})
          QUEST(3,{preQuestGroup={3}}); QUEST(4,{nextQuestInChain=404})
          QUEST(5,{questLevel=nil}); Q[5].questLevel=nil
          for _,id in ipairs({1,2,3,4,5}) do CURRENT(id) end
          QUEST(579,{sourceItemId=3898,specialFlags=1}); ITEM(3898); BAG(3898); DONE={579}
          QUEST(2200,{sourceItemId=7667}); ITEM(7667); BAG(7667); DONE[2]=2200
          SCAN()
          for _,id in ipairs({1,2,3,4,5}) do assert(ROW(id,'quest').reasonCodes[1]=='state-unresolved') end
          assert(ROW(3898).disposition=='KEEP' and table.concat(ROW(3898).reasonCodes,','):find('repeatable'))
          assert(ROW(7667).disposition=='KEEP')
        ''')

    def test_donor_scope_class_reasons_exact_ids_and_missing_starter(self):
        self.runtime().execute('''
          ITEM(268579,{name='Crest',relatedQuests={1}}); ITEM(275521,{name='Crest',relatedQuests={2}})
          QUEST(1); QUEST(2); DONE={1}; CURRENT(2); BAG(268579); BAG(275521)
          local U=RelaxQuestPlanner.CleanupUses
          local pet for id in pairs(U.petFood) do if not U.food[id] and not U.reagent[id] then pet=id; break end end
          assert(pet); ITEM(pet); BAG(pet)
          U.starts[999998]=999997; ITEM(999998); BAG(999998)
          SCAN()
          assert(ROW(268579)~=ROW(275521) and ROW(275521).disposition=='KEEP')
          assert(ROW(pet).disposition=='KEEP' and not table.concat(ROW(pet).reasonCodes,','):find('petFood'))
          assert(ROW(999998).disposition=='KEEP' and ROW(999998).reasonCodes[1]=='starter-missing')
          UnitClass=function() return 'Hunter','HUNTER',3 end; SCAN()
          assert(table.concat(ROW(pet).reasonCodes,','):find('petFood'))
        ''')

    def test_only_two_labels_across_every_reason_code(self):
        self.runtime().execute('''
          local C=RelaxQuestPlanner.Cleanup
          for code in pairs(C.Reasons) do
            assert(C.DispositionForReason(code)=='KEEP' or C.DispositionForReason(code)=='UNCERTAIN-KEEP')
          end
          assert(C.DispositionForReason('no-use-found')=='UNCERTAIN-KEEP')
          assert(C.DispositionForReason('unknown code')=='UNCERTAIN-KEEP')
          QUEST(1); CURRENT(1); ITEM(6842); BAG(6842); SCAN()
          for _,rows in ipairs({REPORT.questRows,REPORT.itemRows}) do
            for _,r in ipairs(rows) do assert(r.disposition=='KEEP' or r.disposition=='UNCERTAIN-KEEP') end
          end
        ''')

if __name__=='__main__': unittest.main(verbosity=2)
