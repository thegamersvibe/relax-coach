"""Run: uv run -q --with lupa python -m unittest discover -s test/planner -p 'journey_*.py' -v"""
import pathlib as _p, sys as _s; _s.path.insert(0, str(_p.Path(__file__).resolve().parent))
import hashlib
import unittest
from journey_harness import runtime, planner_runtime, load_rqp, table, JOURNEY


class JourneyIntegration(unittest.TestCase):
    def test_route_replans_replace_journeys_cached_guide_every_time(self):
        lua, env = planner_runtime()
        lua.execute("SlashCmdList.RELAXQUESTPLANNER('dungeon Alpha Keep'); DRAIN()")
        for level in (31, 32):
            lua.execute('''
                local level=...
                ROUTES['Alpha Keep']={title='Level '..level,steps={
                    {q=level,kind='accept',name='New',needs={}}},issues={},later={}}
                STATE.level=level; RelaxQuestPlanner.UI.Refresh(); DRAIN()
            ''', level)
            self.assertEqual(env.GuideNavigationService.GetGuide().info.description, f'Level {level}')
            self.assertEqual(env.GuideNavigationService.GetStep().id, f'0001-{level}-accept')
        self.assertIsNone(lua.globals().RelaxQuestPlannerDB.lastError)

    def test_skip_unskip_reload_and_start_at_first_undone_step(self):
        lua, env = planner_runtime()
        lua.execute("STATE.map=102; STATE.zone='Away'; SlashCmdList.RELAXQUESTPLANNER('dungeon Alpha Keep'); DRAIN()")
        def ids():
            guide = env.GuideNavigationService.GetGuide()
            return [guide[i].id for i in range(1, len(guide) + 1)]
        self.assertIn('0001-11-accept', ids())
        lua.execute("SlashCmdList.RELAXQUESTPLANNER('skip')")
        self.assertNotIn('0001-11-accept', ids())
        self.assertEqual(env.GuideNavigationService.GetStep().id, '0002-12-accept')
        lua.execute("SlashCmdList.RELAXQUESTPLANNER('unskip')")
        self.assertIn('0001-11-accept', ids())
        self.assertEqual(env.GuideNavigationService.GetStep().id, '0001-11-accept')
        lua.execute('''
            STATE.done[11]=true; STATE.log[12]=true
            RelaxQuestPlanner.UI.OpenInJourney()
        ''')
        self.assertEqual(env.GuideNavigationService.GetStep().id, '0003-12-complete')
        self.assertIsNone(lua.globals().RelaxQuestPlannerDB.lastError)

    def test_route_sync_defers_in_combat_and_respects_other_guides(self):
        lua, env = planner_runtime()
        lua.execute("STATE.map=102; STATE.zone='Away'; SlashCmdList.RELAXQUESTPLANNER('dungeon Alpha Keep'); DRAIN()")
        lua.execute("STATE.combat=true; SlashCmdList.RELAXQUESTPLANNER('skip')")
        self.assertEqual(env.GuideNavigationService.GetStep().id, '0001-11-accept')
        lua.execute("STATE.combat=false; FIRE('PLAYER_REGEN_ENABLED'); DRAIN()")
        self.assertEqual(env.GuideNavigationService.GetStep().id, '0002-12-accept')
        lua.execute("STATE.combat=true; SlashCmdList.RELAXQUESTPLANNER('unskip')")
        self.assertTrue(lua.globals().RelaxJourneyAPI.Goto('A_TEST', 'one'))
        lua.execute("STATE.combat=false; FIRE('PLAYER_REGEN_ENABLED'); DRAIN()")
        self.assertEqual(env.GuideNavigationService.GetGuide().id, 'A_TEST')
        lua.execute("SlashCmdList.RELAXQUESTPLANNER('skip'); STATE.level=32; RelaxQuestPlanner.UI.Refresh(); DRAIN()")
        self.assertEqual(env.GuideNavigationService.GetGuide().id, 'A_TEST')
        self.assertIsNone(lua.globals().RelaxQuestPlannerDB.lastError)

    def test_late_route_callback_preserves_newer_manual_guide_choice(self):
        for active_route in (False, True):
            with self.subTest(active_route=active_route):
                lua, env = planner_runtime()
                if active_route:
                    lua.execute("SlashCmdList.RELAXQUESTPLANNER('dungeon Alpha Keep'); DRAIN()")
                lua.execute("HOLD=true; SlashCmdList.RELAXQUESTPLANNER('dungeon Beta Hall')")
                self.assertTrue(lua.globals().RelaxJourneyAPI.Goto('A_TEST', 'one'))
                lua.execute('PENDING[#PENDING].cb(PENDING[#PENDING].route)')
                self.assertEqual(env.GuideNavigationService.GetGuide().id, 'A_TEST')
                self.assertEqual(lua.globals().RelaxQuestPlanner.UI.route.title, 'Beta Hall')
                self.assertIsNone(lua.globals().RelaxQuestPlannerDB.lastError)
        # Choosing a route intentionally supersedes an unchanged prior guide.
        lua, env = planner_runtime()
        self.assertTrue(lua.globals().RelaxJourneyAPI.Goto('A_TEST', 'one'))
        lua.execute("HOLD=true; SlashCmdList.RELAXQUESTPLANNER('dungeon Alpha Keep')")
        lua.execute('PENDING[#PENDING].cb(PENDING[#PENDING].route)')
        self.assertEqual(env.GuideNavigationService.GetGuide().id, 'RQP_ROUTE')

    def test_combat_delayed_auto_handoff_rechecks_manual_choice(self):
        lua, env = planner_runtime()
        lua.execute("STATE.combat=true; SlashCmdList.RELAXQUESTPLANNER('dungeon Alpha Keep'); DRAIN()")
        self.assertIsNone(env.GuideNavigationService.GetGuide())
        self.assertTrue(lua.globals().RelaxJourneyAPI.Goto('A_TEST', 'one'))
        lua.execute("STATE.combat=false; FIRE('PLAYER_REGEN_ENABLED'); DRAIN()")
        self.assertEqual(env.GuideNavigationService.GetGuide().id, 'A_TEST')

    def test_signed_admission_bytes_preserved(self):
        # Fingerprints of this repository's pre-patch source, not content data.
        hashes = {
            'services/Catalog.lua': 'cc363be717b11348390a30f0432ae9b2956f3e9e60230a33a65f8221950c94e9',
            'lib/TomCats/HTMLDataLoader.lua': 'f560b55e8c168203bff76fb6c20c12b277dc251c36cdf1f67f1dc0c13654d2bd',
            'lib/TomCats/Crypto.lua': '26b82a2f79f31c9b254b8a4e7a72af2c5062cf168efad3bbdec5a3637adf6600',
            'GuideModules.lua': 'a893f75d562c0a002ed18fadfee8686574e9e10d3c9e9a2bda289c8093a65be3',
        }
        for path, expected in hashes.items():
            self.assertEqual(hashlib.sha256((JOURNEY / path).read_bytes()).hexdigest(), expected, path)
        source = (JOURNEY / 'services/Guide.lua').read_bytes()
        block = source[source.index(b'        local guideTemplate ='):source.index(b'        guide.info = guideInfo')]
        block = block.replace(b'        end -- Relax local guides: shared normalization starts here.\r\n', b'')
        self.assertEqual(hashlib.sha256(block).hexdigest(), '7ff63e0aa0dd646d1fde5c604c292915d08baeec5e87588723a54012c1db9ee6')

    def test_catalog_signed_loader_contract_and_hash_rejection(self):
        lua, env = runtime()
        guide = env.GuideService.GetGuide('A_TEST')
        self.assertEqual(guide.id, 'A_TEST')
        self.assertEqual(guide[1][1][1].taskType.type, 'accept')
        call = lua.globals().LOAD_CALLS[1]
        self.assertEqual((call.template, call.key, call.hash), ('template:A_TEST', 'test-key', 'test-hash'))
        lua, env = runtime()
        env.HASH_FAIL = True
        self.assertIsNone(env.GuideService.GetGuide('A_TEST'))
        self.assertEqual(len(lua.globals().LOAD_CALLS), 2)
        self.assertEqual(env.RELOADS, 1)
        for flag in ('EXPIRED', 'INCOMPATIBLE'):
            lua, env = runtime()
            env[flag] = True
            self.assertIsNone(env.GuideService.GetGuide('A_TEST'))
            self.assertEqual(len(lua.globals().LOAD_CALLS), 0)

    def test_local_registration_normalizes_evaluates_and_navigates(self):
        lua, env = runtime()
        api = lua.globals().RelaxJourneyAPI
        self.assertIsNotNone(api, 'RelaxJourneyAPI must cross the private facade')
        self.assertIsNone(lua.globals().GuideService, 'Journey services stay private')
        self.assertTrue(api.RegisterLocalGuide('RQP_TEST', table(lua, {'description': 'Local route'}), env.BUILD))
        # Catalog expiry/compatibility cannot admit/reject unsigned local routes.
        env.EXPIRED, env.INCOMPATIBLE = True, True
        guide = env.GuideService.GetGuide('RQP_TEST')
        self.assertEqual(guide.info.flags, 'B')
        self.assertTrue(guide.moduleInfo.installed and guide.moduleInfo.compatible)
        self.assertEqual(guide[1].idx, 1)
        self.assertEqual(guide.stepLUT.one.id, 'one')
        self.assertEqual(guide[1][1][1].root.id, 'one')
        self.assertIn('Errors Found: 0', list(lua.globals().MESSAGES.values()))
        self.assertEqual(len(lua.globals().LOAD_CALLS), 0)
        for state in range(4):
            lua.globals().STATES[101] = state
            env.EvaluationService.EvaluateStep(guide[1])
            self.assertEqual(bool(guide[1].completedPassed), state >= 1)
        self.assertTrue(api.Goto('RQP_TEST', 'one'))
        self.assertEqual(env.GuideNavigationService.GetGuide().id, 'RQP_TEST')
        self.assertEqual(env.GuideNavigationService.GetStep().id, 'one')
        self.assertTrue(api.UnregisterLocalGuide('RQP_TEST'))
        self.assertIsNone(env.GuideService.GetGuide('RQP_TEST'))
        self.assertFalse(api.RegisterLocalGuide('A_TEST', table(lua, {}), env.BUILD)[0])
        self.assertFalse(api.RegisterLocalGuide('RQP_bad-id', table(lua, {}), env.BUILD)[0])

    def test_route_bridge_uses_real_locations_conditions_and_tasks(self):
        lua, env = runtime()
        load_rqp(lua, 'Bridge.lua')
        journey = lua.globals().RelaxQuestPlanner.Journey
        route = table(lua, {'title': 'A route', 'steps': [
            {'q': 201, 'kind': 'accept', 'name': 'One', 'who': 'A giver', 'zone': 'A zone', 'map': 1433, 'x': 12.25, 'y': 57.5, 'needs': [200]},
            {'q': 201, 'kind': 'complete', 'name': 'One'},
            {'q': 201, 'kind': 'turnin', 'name': 'One', 'below': True, 'note': 'Optional detour'},
        ]})
        guide = journey.ToGuide(route)
        self.assertEqual(len(guide), 4, 'always-passing final step protects Journey navigation')
        self.assertEqual(guide[1].locations[1][1], '|1433|12.25|57.50')
        lua.globals().STATES[200] = 3
        result = journey.LoadRoute(route)
        self.assertTrue(result[0] if isinstance(result, tuple) else result)
        guide = env.GuideNavigationService.GetGuide()
        self.assertTrue(guide.id.startswith('RQP_'))
        self.assertEqual(guide[1].locations[1].x, 12.25)
        self.assertEqual(guide[1].locations[1].y, 57.5)
        for i, kind in enumerate(('accept', 'complete', 'turnin'), 1):
            step = guide[i]
            self.assertEqual(step[1][2].taskType.type, kind)
            for state in range(4):
                lua.globals().STATES[201] = state
                env.EvaluationService.EvaluateStep(step)
                if step.conditionPassed:
                    self.assertEqual(bool(step.completedPassed), state >= i)
        self.assertIn('Errors Found: 0', list(lua.globals().MESSAGES.values()))
        lua.globals().STATES[201] = 0
        lua.globals().STATES[200] = 0
        env.EvaluationService.EvaluateStep(guide[1])
        self.assertFalse(guide[1].conditionPassed, 'unfinished prerequisite gates accept')
        lua.globals().RelaxJourneyAPI = None
        result = journey.LoadRoute(route)
        self.assertFalse(result[0])
        self.assertIn('Journey', result[1])

    def test_parent_active_condition_excludes_turned_in_but_starting_with_does_not(self):
        lua, env = runtime()
        load_rqp(lua, 'Bridge.lua')
        bridge = lua.globals().RelaxQuestPlanner.Journey
        for parent_active in (True, False):
            route = table(lua, {'title': 'Dependency test', 'steps': [
                {'q': 2, 'kind': 'accept', 'needs': [1], 'activeNeeds': {1: True},
                 'parentActive': {1: parent_active}}]})
            step = bridge.ToGuide(route)[1]
            for status in range(4):
                lua.globals().STATES[1] = status
                self.assertEqual(bool(env.CheckCondition(step)),
                                 status in ((1, 2) if parent_active else (1, 2, 3)),
                                 (parent_active, status, step.condition))

    def test_location_and_button_normalization_share_async_checkpoints(self):
        lua, env = runtime()
        lua.globals().CHECKPOINTS = 0
        callback = lua.eval('function() CHECKPOINTS=CHECKPOINTS+1 end')
        for service, name in ((env.LocationsInitService, 'InitGuide'),
                              (env.ButtonGenerationService, 'GenerateButtons')):
            guide = env.BUILD()
            guide.quests = lua.table()
            service[name](guide, callback)
        self.assertEqual(lua.globals().CHECKPOINTS, 4)

    def test_async_scan_yields_during_normalization_not_only_extraction(self):
        lua, env = runtime()
        lua.globals().API = lua.globals().RelaxJourneyAPI
        env.DEBUG_VALIDATOR = False
        env.BUILD = lua.eval('''function()
            local guide={} for i=1,1000 do guide[i]={id=tostring(i),{{accept=i}}} end return guide
        end''')
        lua.execute('''
            SCAN_WORKER=coroutine.create(function()
                SCAN_RESULT=API.ScanGuideQuests('A_TEST', coroutine.yield)
            end)
            SCAN_YIELDS=0
            while coroutine.status(SCAN_WORKER)~='dead' do
                local ok,err=coroutine.resume(SCAN_WORKER)
                assert(ok,err)
                SCAN_YIELDS=SCAN_YIELDS+1
            end
        ''')
        self.assertEqual(len(lua.globals().SCAN_RESULT.steps), 1000)
        self.assertGreater(lua.globals().SCAN_YIELDS, 1001,
                           'normalization must share the async budget too')

    def test_startwork_respects_real_default_optional_semantics(self):
        lua, env = runtime()
        env.BUILD = lua.eval("function()return {{id='start',{{startwork=101}}},{id='end',{{text='End'}}}} end")
        result = lua.globals().RelaxJourneyAPI.ScanGuideQuests('A_TEST')
        self.assertTrue(result.steps[1].actions[1].optional)

    def test_scan_uses_normalized_order_and_conditions_without_navigation(self):
        lua, env = runtime()
        api = lua.globals().RelaxJourneyAPI
        self.assertIsNotNone(api.ListCatalogGuides, 'catalog scanner API is required')
        self.assertIsNotNone(api.ScanGuideQuests)
        env.INFOS[2] = table(lua, {'guideID': 'H_TEST', 'condition': 'HORDE'})
        self.assertEqual(len(api.ListCatalogGuides()), 1)
        env.BUILD = lua.eval('''function()return {
            {id='done', completed='true', {{accept=100}}},
            {id='class', condition='MAGE', {{accept=102}}},
            {id='mixed', {{condition='ALLIANCE',accept=101}, {complete=101,seq=2}, {turnin=101}}},
            {id='unknown', condition='MISSING[1]', {{accept=103}}},
            {id='end',{{text='End'}}}
        } end''')
        result = api.ScanGuideQuests('A_TEST')
        self.assertEqual(len(result.steps), 5)
        self.assertTrue(result.accept[101] and result.turnin[101] and result.complete[101])
        self.assertEqual(result.steps[3].id, 'mixed')
        self.assertEqual(len(result.steps[3].actions), 4, 'real loader expands seq')
        self.assertFalse(result.steps[2].actions[1].active)
        self.assertIsNone(result.steps[4].actions[1].active, 'unknown is not eligible')
        self.assertTrue(result.steps[1].actions[1].completed)
        self.assertIsNone(env.GuideNavigationService.GetGuide())
        self.assertIsNone(env.LAST_GUIDE)


if __name__ == '__main__':
    unittest.main(verbosity=2)
