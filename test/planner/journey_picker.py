"""Synthetic picker acceptance tests, exercising patched real Journey code."""
import pathlib as _p, sys as _s; _s.path.insert(0, str(_p.Path(__file__).resolve().parent))
import unittest
from journey_harness import runtime, load_rqp, table, drain


class JourneyPicker(unittest.TestCase):
    def fixture(self):
        lua, env = runtime()
        env.DEBUG_VALIDATOR = False
        load_rqp(lua, 'Bridge.lua')
        load_rqp(lua, 'Picker.lua')
        env.INFOS = table(lua, [
            {'guideID': 'A_LOW', 'description': 'Old guide', 'levels': '20-24', 'condition': 'ALLIANCE'},
            {'guideID': 'A_GOOD', 'description': 'Useful guide', 'levels': '27-32', 'condition': 'ALLIANCE'},
            {'guideID': 'A_OTHER', 'description': 'Boundary-level guide', 'levels': '24-25', 'condition': 'ALLIANCE'},
            {'guideID': 'H_STUB', 'description': 'Coming later', 'levels': '30-35', 'condition': 'HORDE', 'stub': True},
        ])
        env.BUILD = lua.eval('''function(template)
            if template=='template:A_GOOD' then return {
                {id='done',{{accept=301}}},
                {id='blocked',{{accept=302}}},
                {id='work',{{accept=303},{complete=303}}},
                {id='later',{{accept=304}}},
                {id='end',{{text='End'}}}
            } end
            return {{id='one',{{accept=305}}},{id='end',{{text='End'}}}}
        end''')
        lua.execute('''
            STATES[301]=3; STATES[303]=1
            RelaxQuestPlanner.Player={Snapshot=function()return {
                level=30,faction='Alliance',done={[301]=true},log={[303]=true},ready={},trivial=7
            } end}
            RelaxQuestPlanner.Data={Quest=function(id)return {id=id,qLevel=id==305 and 25 or 30,reqLevel=25} end}
            RelaxQuestPlanner.Engine={
                State=function(q)
                    if STATES[q]==3 then return 'done',{} end
                    if STATES[q] then return 'inlog',{} end
                    if q==302 then return 'blocked',{'needs 300'} end
                    return 'available',{}
                end,
                Unlocks=function(q) if q==304 then return {401,402} end return {} end
            }
            PICK_RESULT=nil; PICK_ERROR=nil; CALLBACKS=0
            function PickCallback(result, err) PICK_RESULT=result; PICK_ERROR=err; CALLBACKS=CALLBACKS+1 end
        ''')
        return lua, env

    def test_async_ranking_strict_filter_dependencies_and_goto(self):
        lua, env = self.fixture()
        journey = lua.globals().RelaxQuestPlanner.Journey
        journey.PickGuide(lua.globals().PickCallback)
        self.assertIsNone(lua.globals().PICK_RESULT, 'work must start on a later frame')
        ticks = drain(lua)
        result = lua.globals().PICK_RESULT
        self.assertGreater(ticks, 0)
        self.assertEqual(lua.globals().CALLBACKS, 1)
        self.assertEqual(len(result), 2)
        self.assertEqual(result[1].guideID, 'A_GOOD')
        self.assertEqual(result[1].startStepID, 'work')
        self.assertTrue(any('unlock' in str(v).lower() for v in result[1].reasons.values()))
        self.assertNotIn('template:A_LOW', [v.template for v in lua.globals().LOAD_CALLS.values()])
        self.assertIsNone(env.GuideNavigationService.GetGuide(), 'preview must not navigate')
        self.assertTrue(journey.GotoBest(result[1]))
        self.assertEqual(env.GuideNavigationService.GetStep().id, 'work')
        # A state change before the explicit click must be rechecked.
        lua.globals().STATES[303] = 3
        lua.execute("RelaxQuestPlanner.Player.Snapshot=function()return {level=30,faction='Alliance',done={[301]=true,[303]=true},log={},ready={},trivial=7} end")
        self.assertTrue(journey.GotoBest(result[1]))
        self.assertEqual(env.GuideNavigationService.GetStep().id, 'later')

    def test_fallback_and_stub_only_horde_are_honest(self):
        lua, env = self.fixture()
        lua.execute("RelaxQuestPlanner.Player=nil; RelaxQuestPlanner.Engine=nil; RelaxQuestPlanner.Data=nil")
        lua.globals().RelaxQuestPlanner.Journey.PickGuide(lua.globals().PickCallback)
        drain(lua)
        result = lua.globals().PICK_RESULT
        self.assertGreater(len(result), 0)
        self.assertTrue(result[1].provisional)
        self.assertTrue(any('prerequisite' in str(v).lower() for v in result[1].reasons.values()))
        env.ConditionContext.ALLIANCE = None
        env.ConditionContext.HORDE = True
        lua.globals().FACTION = 'Horde'
        lua.globals().RelaxQuestPlanner.Journey.PickGuide(lua.globals().PickCallback)
        drain(lua)
        self.assertEqual(len(lua.globals().PICK_RESULT), 0)
        self.assertIn('stub', lua.globals().PICK_ERROR.lower())

    def test_game_and_all_modes_relax_only_the_level_floor(self):
        lua, env = self.fixture()
        for mode in ('game', 'all'):
            lua.globals().RelaxQuestPlannerDB.mode = mode
            lua.globals().RelaxQuestPlanner.Journey.PickGuide(lua.globals().PickCallback)
            drain(lua)
            ids = {c.guideID for c in lua.globals().PICK_RESULT.values()}
            self.assertIn('A_LOW', ids)
            self.assertNotIn('H_STUB', ids)
        self.assertIsNone(env.GuideNavigationService.GetGuide())

    def test_engine_loaded_but_questdb_missing_uses_provisional_fallback(self):
        lua, _ = self.fixture()
        lua.execute('''
            RelaxQuestPlanner.Data.Quest=function()return nil end
            RelaxQuestPlanner.Engine.State=function()return 'blocked',{'unknownQuest'} end
        ''')
        lua.globals().RelaxQuestPlanner.Journey.PickGuide(lua.globals().PickCallback)
        drain(lua)
        result = lua.globals().PICK_RESULT
        self.assertEqual(len(result), 2)
        self.assertTrue(all(c.provisional for c in result.values()))

    def test_verified_work_level_fit_not_only_guide_band(self):
        lua, env = self.fixture()
        env.INFOS = table(lua, [
            {'guideID': 'A_HARD', 'description': 'Hard work', 'levels': '27-35'},
            {'guideID': 'Z_FIT', 'description': 'Level fit', 'levels': '27-35'},
            {'guideID': 'A_OLDWORK', 'description': 'Old opening', 'levels': '27-35'},
        ])
        env.BUILD = lua.eval('''function(t)
            local q=t=='template:A_HARD' and 501 or t=='template:A_OLDWORK' and 502 or 503
            return {{id='first',{{accept=q}}},{id='end',{{text='End'}}}}
        end''')
        lua.execute('''RelaxQuestPlanner.Data.Quest=function(q)
            return {qLevel=q==501 and 35 or q==502 and 24 or 30}
        end''')
        lua.globals().RelaxQuestPlanner.Journey.PickGuide(lua.globals().PickCallback)
        drain(lua)
        result = lua.globals().PICK_RESULT
        self.assertEqual(len(result), 2, 'high guide upper bound cannot admit old quest work')
        self.assertEqual(result[1].guideID, 'Z_FIT')

    def test_engine_graph_work_is_yielded_and_checkpoint_restored(self):
        lua, _ = self.fixture()
        lua.execute('''
            ENGINE_OPS=0
            ORIGINAL_CHECKPOINT=function() end
            RelaxQuestPlanner.Engine.Checkpoint=ORIGINAL_CHECKPOINT
            local old=RelaxQuestPlanner.Engine.State
            RelaxQuestPlanner.Engine.State=function(q,p,o)
                for i=1,12000 do
                    ENGINE_OPS=ENGINE_OPS+1
                    RelaxQuestPlanner.Engine.Checkpoint()
                end
                return old(q,p,o)
            end
        ''')
        lua.globals().RelaxQuestPlanner.Journey.PickGuide(lua.globals().PickCallback)
        lua.globals().Tick()
        self.assertIsNone(lua.globals().PICK_RESULT)
        self.assertLess(lua.globals().ENGINE_OPS, 12000)
        self.assertTrue(lua.eval('RelaxQuestPlanner.Engine.Checkpoint == ORIGINAL_CHECKPOINT'))
        drain(lua)
        self.assertEqual(len(lua.globals().PICK_RESULT), 2)

    def test_secret_level_and_callback_errors_are_contained(self):
        lua, _ = self.fixture()
        lua.execute("RelaxQuestPlanner.Player=nil; LEVEL=SECRET")
        lua.globals().RelaxQuestPlanner.Journey.PickGuide(lua.globals().PickCallback)
        drain(lua)
        self.assertEqual(len(lua.globals().PICK_RESULT), 0)
        self.assertIn('level', lua.globals().PICK_ERROR.lower())
        lua.globals().RelaxQuestPlanner.Journey.PickGuide(lua.eval("function()error('callback test')end"))
        drain(lua)
        self.assertIn('callback test', lua.globals().RelaxQuestPlannerDB.lastError)


if __name__ == '__main__':
    unittest.main(verbosity=2)
