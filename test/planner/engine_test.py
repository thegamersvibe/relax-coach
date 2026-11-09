"""Lua 5.1 real-data acceptance suite (private fixtures are never vendored)."""
import pathlib as _p, sys as _s; _s.path.insert(0, str(_p.Path(__file__).resolve().parent))
import unittest
from engine_harness import AVAILABLE, Harness, MiniHarness, PRIVATE, array, to_lua

DUNGEONS = ("Gnomeregan", "The Deadmines", "Uldaman", "Razorfen Kraul",
            "Scarlet Monastery", "Wailing Caverns", "The Drowned City")


@unittest.skipUnless(AVAILABLE, "SKIP: RQP_PRIVATE_DATA not installed")
class EngineTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.h = Harness()

    def test_dungeon_list_is_forever_only(self):
        h = Harness()
        names = [h.rqp.Data.Dungeons()[i].name for i in range(1, len(h.rqp.Data.Dungeons()) + 1)]
        for bad in ("Utgarde Keep", "Karazhan", "Molten Core", "Onyxia's Lair", "Zul'Gurub", "Naxxramas",
                    "Icecrown Citadel", "Warsong Gulch", "Deeprun Tram", "The Black Morass", "Scarlet Halls"):
            self.assertNotIn(bad, names)
        for good in ("Gnomeregan", "The Deadmines", "The Drowned City", "The Hall of Thanes", "Scarlet Monastery",
                     "Blackrock Spire", "Dire Maul", "Stratholme", "Sunken Temple", "The Stockade"):
            self.assertIn(good, names)
        self.assertNotIn("Ragefire Chasm", names)            # Alliance test character
        self.assertEqual(len(names), len(set(names)))         # wings collapsed, no duplicates

    def test_grey_dungeon_quests_hidden_unless_all(self):
        h = Harness(); p = h.player()
        count = lambda r: len(set(r.steps[i].q for i in range(1, len(r.steps) + 1))) if r.steps else 0
        self.assertEqual(count(h.route("The Deadmines", p)), 0, "level-12-18 quests are grey at level 30")
        self.assertGreater(count(h.route("The Deadmines", p, mode="all")), 0)
        stock = h.route("The Stockade", p)
        self.assertEqual(count(stock), 6, "quests already in the log are kept even if low")
        self.assertIn("quest log", stock.steps[1].note)

    def test_player_snapshot(self):
        h = self.h
        self.assertIsNotNone(h.rqp.Player, "Player snapshot is not implemented")
        p = h.player()
        self.assertEqual(p.level, h.snapshot["level"])
        self.assertEqual(p.raceMask, 4)
        self.assertEqual(p.classMask, 1)
        self.assertEqual(set(p.done.keys()), set(h.snapshot["completed"]))
        self.assertEqual(set(p.log.keys()), {q["id"] for q in h.snapshot["log"]})
        self.assertEqual(set(p.ready.keys()), {q["id"] for q in h.snapshot["log"] if q["complete"]})
        self.assertEqual(dict(p.professions.items()), h.snapshot["professions"])
        self.assertEqual(p.trivial, 7)
        self.assertEqual({name: [row.standing, row.value, row.id] for name, row in p.reputation.items()},
                         h.snapshot["reputation"])

    def test_all_quest_states_match_reference(self):
        from collections import Counter
        from graph import Graph
        h = self.h
        self.assertIsNotNone(h.rqp.Engine, "Quest engine is not implemented")
        ref = Graph(h.snapshot)
        actual_ids = array(h.rqp.Data.AllQuestIds())
        self.assertEqual(set(actual_ids), set(ref.db.q))
        p = h.player()
        actual = {q: h.rqp.Engine.State(q, p)[0] for q in actual_ids}
        expected = {q: ref.state(q) for q in ref.db.q}
        mismatch = [(q, actual[q], expected[q]) for q in expected if actual[q] != expected[q]]
        self.assertEqual(mismatch, [])
        self.assertEqual(Counter(actual.values()), Counter(expected.values()))
        # Exercise the reference's reporting entrypoint, not a copied golden count.
        import ast
        from contextlib import redirect_stdout
        import io
        import os
        import runpy
        output, cwd = io.StringIO(), os.getcwd()
        try:
            os.chdir(PRIVATE)
            with redirect_stdout(output):
                runpy.run_path(str(PRIVATE / "engine/report.py"))
        finally:
            os.chdir(cwd)
        report = next(line for line in output.getvalue().splitlines() if line.startswith("states "))
        self.assertEqual(dict(Counter(actual.values())), ast.literal_eval(report[len("states "):]))
        print("states", dict(sorted(Counter(actual.values()).items())), "quests", len(actual))

    def test_routes_and_prerequisites(self):
        from graph import Graph
        from entities import Entities
        from path import build
        h = self.h
        self.assertIsNotNone(h.rqp.Engine.BuildPathAsync, "Async planner is not implemented")
        ref, entities = Graph(h.snapshot), Entities()
        # API enumeration is ascending; normalize only incidental dict iteration in
        # the reference so equal-level target ties have a reproducible order.
        ref.db.q = dict(sorted(ref.db.q.items()))
        ref.extra = {q: dict(sorted(v.items())) for q, v in ref.extra.items()}
        for dungeon in DUNGEONS:
            with self.subTest(dungeon=dungeon):
                expected = build(ref, entities, dungeon)
                # Reference parity is checked without the dungeon-target level filter.
                route = h.route(dungeon, mode="all")
                plan = array(route.plan)
                self.assertEqual(plan, [r["id"] for r in expected["steps"]])
                self.assertEqual(len(plan), len(set(plan)))
                self.assertEqual(h.route(dungeon).floor, expected["floor"])
                steps, later = array(route.steps), array(route.later)
                self.assertEqual({s.q for s in steps} | {s.q for s in later}, set(plan))
                self.assertEqual({s.q for s in later}, {s["id"] for s in expected["steps"] if not s["levelOK"]})
                self.assert_action_dependencies(route, h.player(), h)
                print("route", dungeon, "quests", len(plan), "actions", len(steps), "later", len(later))
        self.assertGreater(h.g.TEST_TICKS, 1)
        self.assertLessEqual(h.g.TEST_MAX_FRAME, 8.0 + 1e-8)

    def assert_action_dependencies(self, route, p, h=None):
        h = h or self.h
        accepted, completed, turned = set(p.log.keys()), set(p.ready.keys()), set(p.done.keys())
        for step in array(route.steps):
            for prereq in array(step.needs):
                self.assertIn(prereq, accepted if step.activeNeeds and step.activeNeeds[prereq] else turned,
                              f"{step.kind} {step.q} precedes prerequisite {prereq}")
            if step.kind == "accept":
                state, reasons = h.rqp.Engine.State(step.q, p, to_lua(h.lua, {
                    "done": {q: True for q in turned}, "log": {q: True for q in accepted - turned}}))
                self.assertIn(state, ("available", "unknownGate"),
                              f"Cannot accept {step.q}: {state} {[array(r) if not isinstance(r, str) else r for r in array(reasons)]}")
                accepted.add(step.q)
            elif step.kind == "complete":
                self.assertIn(step.q, accepted)
                completed.add(step.q)
            else:
                self.assertEqual(step.kind, "turnin")
                self.assertIn(step.q, completed)
                turned.add(step.q)
            self.assertIn(step.source, ("QuestieDB", "FDJ", "DungeonQuestie"))
            self.assertIn(step.confidence, ("db", "addon", "observed"))

    def test_unlocks_match_reference(self):
        from graph import Graph
        from unlock import unlocks
        h, ref = self.h, Graph(self.h.snapshot)
        self.assertIsNotNone(h.rqp.Engine.Unlocks, "Unlock simulation is not implemented")
        p = h.player()
        for q in ref.db.q:
            self.assertEqual(set(array(h.rqp.Engine.Unlocks(q, p))), set(unlocks(ref, q)), q)

    def test_fdj_only_routes_use_canonical_picker_names(self):
        snap = dict(level=60, race="Dwarf", raceID=3, faction="Alliance",
                    completed=[], log=[], professions={}, **{"class": "WARRIOR"})
        h = Harness(snap, qdb=False, dungeon_global=False)
        p = h.player()
        for name in ("The Hall of Thanes", "Dire Maul", "Scarlet Monastery",
                     "Stratholme", "Blackrock Spire"):
            with self.subTest(dungeon=name):
                route = h.route(name, p, mode="all")
                self.assertGreater(len(route.steps), 0, name)
                if name == "The Hall of Thanes":
                    self.assertIn(96394, array(route.plan))
        # This FDJ source has only Horde quests here; never invent Alliance ones.
        self.assertEqual(len(h.route("The Shaper's Terrace", p, mode="all").steps), 0)
        horde = Harness(dict(snap, faction="Horde", race="Tauren", raceID=6),
                        qdb=False, dungeon_global=False)
        self.assertGreater(len(horde.route("The Shaper's Terrace", mode="all").steps), 0)
        for dungeon in array(h.rqp.Data.FOREVER_DUNGEONS):
            for alias in array(dungeon.aliases) if dungeon.aliases else []:
                self.assertEqual(h.rqp.Data.CanonicalDungeon(alias), dungeon.name)

    def test_synthetic_characters_and_fdj_fallback(self):
        cases = [("Horde", "Tauren", 6, "DRUID", 20, "Wailing Caverns"),
                 ("Alliance", "Skyborne", 95, "HUNTER", 10, "The Deadmines")]
        for faction, race, race_id, cls, level, dungeon in cases:
            snap = dict(level=level, race=race, raceID=race_id, faction=faction,
                        completed=[], log=[], professions={}, **{"class": cls})
            for qdb in (True, False):
                with self.subTest(faction=faction, qdb=qdb):
                    h = Harness(snap, qdb=qdb, dungeon_global=False)
                    self.assertEqual(h.fdj_errors, [])
                    p = h.player()
                    route = h.route(dungeon, p)
                    self.assertGreater(len(route.steps), 0)
                    for q in array(route.plan):
                        row = h.rqp.Data.Quest(q)
                        self.assertNotIn(h.rqp.Engine.State(q, p)[0], ("race", "class"))
                        if qdb and row.races:
                            self.assertTrue(h.rqp.Engine.MaskContains(row.races, p.raceMask))
                        if row.faction in ("Alliance", "Horde"):
                            self.assertEqual(row.faction, faction)
                        # Verify fallback output against the full real DB too;
                        # missing fallback metadata must not hide faction leakage.
                        full = self.h.rqp.Data.Quest(q)
                        race_mask = self.h.g.LibQuestieDB.Enum.raceMaskById[race_id]
                        class_mask = self.h.g.LibQuestieDB.Enum.classKeys[cls]
                        if full and full.races:
                            self.assertTrue(int(full.races) & int(race_mask), q)
                        if full and full.classes:
                            self.assertTrue(int(full.classes) & int(class_mask), q)
                    self.assert_action_dependencies(route, p, h)
                    if race_id == 95 and qdb:
                        self.assertEqual(p.raceMask, 4294967296)
                    if not qdb:
                        self.assertEqual(h.rqp.Data.Source(), "FDJ-only")
                        self.assertIn("limited", h.rqp.Data.Ready()[1])
                    self.assertLessEqual(h.g.TEST_MAX_FRAME, 8.0 + 1e-8)
                    print("synthetic", faction, race, cls, level, h.rqp.Data.Source(), "actions", len(route.steps))

    def test_learn_observations_refutations_and_event_guards(self):
        h = Harness()
        self.assertIsNotNone(h.rqp.Learn, "Learning store is not implemented")
        learn = h.rqp.Learn
        # Quest 19 has a real missing prerequisite in an empty-history character.
        h.g.TEST_SNAPSHOT.completed = h.lua.table()
        h.g.TEST_SNAPSHOT.log = h.lua.table()
        learn.Record("turnin", 20, 382)
        learn.Record("accept", 19, 382)
        self.assertEqual(set(learn.Observed(19).keys()), {20})
        self.assertTrue(learn.Refuted(19, 98387))
        self.assertIsNotNone(h.g.RelaxQuestPlannerDB.learn)
        learn.Record("turnin", 100, 1)
        learn.Record("accept", 101, 2)
        self.assertEqual(set(learn.Observed(101).keys()), set())
        h.lua.execute("TEST_Event('GOSSIP_CLOSED')")
        learn.Record("accept", 102, 1)
        self.assertEqual(set(learn.Observed(102).keys()), set())
        # Accept/turn-in events are forwarded by Init.lua (covered in ui_shell_test); Core only resets sessions.
        learn.Record("turnin", 20, 382); learn.Record("accept", 19, 382)
        self.assertTrue(learn.Observed(19)[20])
        h.lua.execute("TEST_Event('PLAYER_ENTERING_WORLD')")
        learn.Record("accept", 103, 382)
        self.assertEqual(set(learn.Observed(103).keys()), set())

    def test_modes_goal_errors_and_secret_numbers(self):
        h = Harness()
        p = h.player()
        for mode, floor in (("strict", 25), ("game", 23), ("all", 0)):
            route = h.route(19, p, goal=True, mode=mode)
            self.assertEqual(route.floor, floor)
            self.assert_action_dependencies(route, p, h)
        route = h.route(999999999, p, goal=True)
        self.assertEqual(len(route.steps), 0)
        self.assertEqual(route.issues[1][1], "unknownQuest")
        h.lua.execute("TEST_SNAPSHOT.level={secret=true}; TEST_SNAPSHOT.raceID={secret=true}; C_QuestLog.GetTrivialRange=function() return {secret=true} end")
        secret_p = h.player()
        self.assertEqual(secret_p.level, 0)
        self.assertEqual(secret_p.raceMask, 0)
        self.assertEqual(secret_p.trivial, 7)

    def test_private_dungeonquestie_namespace_is_not_assumed(self):
        from unittest.mock import patch
        from collections import Counter
        from graph import Graph
        from entities import Entities
        from path import build
        import dungeon as reference_dungeon
        h = Harness(dungeon_global=False)
        self.assertIsNone(h.rqp.Data.DungeonQuestie())
        # Model the documented runtime limitation explicitly. The reference's
        # sources.fdj() omits Dungeons.lua, so it misses QuestUpdates' conditional
        # lead-in removal (95809). Supply the actually loaded runtime claims for
        # this degradation test only; the report.py test above is unmodified.
        live_claims = {q: [s.id for s in array(chain)]
                       for q, chain in h.g.FDJ.QUEST_PREREQ_CHAINS.items()}
        with patch("sources.dgf", return_value=({}, {})), patch("sources.fdj", return_value=live_claims), patch.object(reference_dungeon, "_AL", None):
            ref, entities = Graph(h.snapshot), Entities()
            ref.db.q = dict(sorted(ref.db.q.items()))
            ref.extra = {q: dict(sorted(v.items())) for q, v in ref.extra.items()}
            p = h.player()
            actual = {q: h.rqp.Engine.State(q, p)[0] for q in ref.db.q}
            self.assertEqual(actual, {q: ref.state(q) for q in ref.db.q})
            for name in DUNGEONS:
                route = h.route(name, p, mode="all")
                self.assertEqual(array(route.plan), [r["id"] for r in build(ref, entities, name)["steps"]])
                self.assert_action_dependencies(route, p, h)
            print("runtime-without-DungeonQuestie-global", dict(sorted(Counter(actual.values()).items())))

    def test_cold_async_budget_queue_and_callback_errors(self):
        h = Harness()
        self.assertEqual(h.g.TEST_TICKS, 0)
        h.lua.execute("""
          TEST_CALLBACKS=0
          local p=RelaxQuestPlanner.Player.Snapshot()
          RelaxQuestPlanner.Engine.BuildPathAsync('Gnomeregan',p,{sliceMs=3},function(route)
            TEST_CALLBACKS=TEST_CALLBACKS+1; TEST_FIRST_ROUTE=route
          end)
          RelaxQuestPlanner.Engine.BuildGoalAsync(19,p,{sliceMs=3},function()
            TEST_CALLBACKS=TEST_CALLBACKS+1; error('callback guard probe')
          end)
          local cancelled=RelaxQuestPlanner.Engine.BuildPathAsync('Uldaman',p,{},function()
            error('cancelled job ran')
          end)
          cancelled:Cancel()
          TEST_DriveFrames(100000)
        """)
        self.assertEqual(h.g.TEST_CALLBACKS, 2)
        self.assertGreater(len(h.g.TEST_FIRST_ROUTE.steps), 0)
        self.assertIn("callback guard probe", h.g.RelaxQuestPlannerDB.lastError)
        self.assertGreater(h.g.TEST_TICKS, 1)
        self.assertLessEqual(h.g.TEST_MAX_FRAME, 3.0 + 1e-8)
        print("cold async frames", h.g.TEST_TICKS, "max budget-clock ms", round(h.g.TEST_MAX_FRAME, 3))

    def test_contract_rejection_and_late_addon_loading(self):
        h = Harness()
        h.lua.execute("LibQuestieDB.RequireContract=function() return false,'too old' end")
        h.rqp.Data.Reset()
        self.assertEqual(h.rqp.Data.Source(), "FDJ-only")
        self.assertIn("contract 4", h.rqp.Data.Ready()[1])
        h.lua.execute("LibQuestieDB.RequireContract=function() return true end; TEST_Event('ADDON_LOADED','QuestieDB')")
        self.assertEqual(h.rqp.Data.Source(), "QuestieDB")
        h.lua.execute("LibQuestieDB=nil; ForeverDungeonJournal_NS=nil; DungeonGuideForever_NS=nil; RelaxQuestPlanner.Data.Reset()")
        ok, reason = h.rqp.Data.Ready()
        self.assertFalse(ok)
        self.assertIn("missing", reason)
        route = h.route("Gnomeregan")
        self.assertEqual(len(route.steps), 0)
        self.assertEqual(route.issues[1][1], "missingData")

    def test_gate_rules_and_dependency_edge_cases(self):
        def q(name, **kw):
            return dict(name=name, requiredLevel=1, questLevel=10, **kw)
        h = MiniHarness({1: q("one"), 2: q("two"), 3: q("both", preQuestGroup=[1, 2]),
                         4: q("either", preQuestSingle=[1, 2]), 5: q("next", preQuestSingle=[4]),
                         6: q("crumb", breadcrumbForQuestId=7), 7: q("target", breadcrumbs=[6]),
                         8: q("profession", requiredSkill=[202, 100]),
                         9: q("holiday", zoneOrSort=-366),
                         10: q("expired", availableUntilCompleted=1),
                         11: q("limited", requiredMaxLevel=20),
                         12: q("startwith", availableStartingWith=1),
                         13: q("disabled", disabledByQuest=1)})
        p = h.player()
        self.assertEqual(h.rqp.Engine.State(3, p)[0], "blocked")
        self.assertEqual(set(array(h.rqp.Engine.Unlocks(1, p))), {4, 5, 12})
        self.assertEqual(array(h.rqp.Engine.Unlocks(6, p)), [])
        for qid, expected in ((8, "unknownGate"), (9, "holiday"), (11, "overLevel")):
            self.assertEqual(h.rqp.Engine.State(qid, p)[0], expected)
        p.done[1] = True
        self.assertEqual(h.rqp.Engine.State(10, p)[0], "expired")
        self.assertEqual(h.rqp.Engine.State(4, p)[0], "available")
        p.log[1] = True
        self.assertEqual(h.rqp.Engine.State(13, p)[0], "blocked")
        # A parentActive child starts inside: the parent must not be turned in
        # while batching preparatory turn-ins before the child's acceptance.
        h = MiniHarness({1: q("Parent"), 2: q("Child", parentQuest=1, startedBy={3: [100]})})
        route = h.route(2, goal=True)
        self.assert_action_dependencies(route, h.player(), h)
        # A chosen exclusive alternative must be turned in, not just accepted.
        h = MiniHarness({1: q("Excluded", exclusiveTo=[9]), 9: q("Alternative"),
                         3: q("Child", preQuestGroup=[1]), 4: q("Goal", preQuestGroup=[9, 3])})
        route = h.route(4, goal=True)
        self.assertEqual(array(route.plan), [9, 3, 4])
        self.assert_action_dependencies(route, h.player(), h)
        # Negative IDs intentionally do not accept the exclusive alternative.
        h.g.TEST_ROWS[3].preQuestGroup = to_lua(h.lua, [-1])
        h.rqp.Data.Reset()
        p = h.player(); p.done[9] = True
        self.assertEqual(h.rqp.Engine.State(3, p)[0], "blocked")
        h = MiniHarness({1: q("Cycle A", preQuestSingle=[2]), 2: q("Cycle B", preQuestSingle=[1])})
        route = h.route(1, goal=True)
        self.assertEqual(len(route.steps), 0)
        self.assertGreater(len(route.issues), 0)

    def test_goal_maximum_level_uses_player_not_future_lookahead(self):
        snap = dict(level=30, race="Dwarf", raceID=3, faction="Alliance",
                    completed=[], log=[], professions={}, **{"class": "WARRIOR"})
        h = Harness(snap, dungeon_global=False)
        p = h.player()
        self.assertEqual(h.rqp.Engine.State(8167, p)[0], 'available')
        route = h.route(8167, p, goal=True, mode='all')
        self.assertGreater(len(route.steps), 0, 'eligible level 30-39 quest must not be evaluated at level 60')
        self.assert_action_dependencies(route, p, h)
        p.level = 40
        expired = h.route(8167, p, goal=True, mode='all')
        self.assertEqual(len(expired.steps), 0)
        self.assertIn('overLevel', [issue[1] for issue in array(expired.issues)])
        # The minimum-level lookahead must still retain future goals as later.
        h = MiniHarness({1: dict(name='Future', requiredLevel=40, questLevel=40, requiredMaxLevel=50)})
        later = h.route(1, goal=True, mode='all')
        self.assertEqual(len(later.steps), 0)
        self.assertEqual([s.q for s in array(later.later)], [1])

    def test_parent_active_metadata_and_ui_order_survive_local_repick(self):
        from ui_shell_test import WOW, ADDON
        def q(name, **kw):
            return dict(name=name, requiredLevel=1, questLevel=10, **kw)
        h = MiniHarness({1: q('Parent'), 2: q('Child', parentQuest=1, startedBy={3: [100]}),
                         3: q('Starting with', availableStartingWith=1),
                         4: q('Both', preQuestGroup=[1, 2])})
        p = h.player()
        route = h.route(2, p, goal=True)
        h.g.ORDER_ROUTE, h.g.ORDER_PLAYER = route, p
        h.lua.execute(WOW)
        h.lua.execute((ADDON / 'Init.lua').read_text())
        h.lua.execute((ADDON / 'UI/Steps.lua').read_text())
        h.lua.execute('''
            STATE.log[1]=true; STATE.ready[1]=true
            ORDER_PLAYER.log[1]=true; ORDER_PLAYER.ready[1]=true
            for _,s in ipairs(ORDER_ROUTE.steps) do s.zone=s.q==1 and 'Home' or 'Away' end
            local U=RelaxQuestPlanner.UI
            local i=U.PickStep(ORDER_ROUTE,ORDER_PLAYER)
            assert(ORDER_ROUTE.steps[i].q==2 and ORDER_ROUTE.steps[i].kind=='accept',
                'never turn in the local parent before accepting its pending child')
            RelaxQuestPlannerDB.skipped={[2]=true}
            i=U.PickStep(ORDER_ROUTE,ORDER_PLAYER)
            assert(ORDER_ROUTE.steps[i].q==1 and ORDER_ROUTE.steps[i].kind=='turnin')
            RelaxQuestPlannerDB.skipped=nil; STATE.log[2]=true; ORDER_PLAYER.log[2]=true
            i=U.PickStep(ORDER_ROUTE,ORDER_PLAYER)
            assert(ORDER_ROUTE.steps[i].q==1 and ORDER_ROUTE.steps[i].kind=='turnin')
        ''')
        p.log[2] = None
        rebuilt = h.route(2, p, goal=True)
        child = next(s for s in array(rebuilt.steps) if s.q == 2 and s.kind == 'accept')
        self.assertTrue(child.parentActive[1], 'retain an already-active parent constraint after replan')
        self.assertIn(1, array(child.needs))
        both = h.route(4, p, goal=True)
        order = [(s.q, s.kind) for s in array(both.steps)]
        self.assertLess(order.index((2, 'accept')), order.index((1, 'turnin')))
        self.assertNotIn('actionCycle', [issue[1] for issue in array(both.issues)])
        p.log[1] = None
        starting = next(s for s in array(h.route(3, p, goal=True).steps) if s.q == 3 and s.kind == 'accept')
        self.assertTrue(starting.activeNeeds[1])
        self.assertFalse(starting.parentActive and starting.parentActive[1], 'startingWith also permits a turned-in parent')

    def test_slow_provider_reads_are_time_sliced(self):
        h = Harness()
        h.lua.execute("""
          for _,api in ipairs({QuestDB,NpcDB,ItemDB,ObjectDB}) do
            local original=api.Get
            api.Get=function(id,key) TEST_CLOCK=TEST_CLOCK+0.25; return original(id,key) end
          end
        """)
        route = h.route("Gnomeregan")
        self.assertGreater(len(route.steps), 0)
        self.assertLessEqual(h.g.TEST_MAX_FRAME, 8.0 + 1e-8)
        print("slow provider max budget-clock ms", round(h.g.TEST_MAX_FRAME, 3))

    def test_data_runtime_adapter(self):
        h = self.h
        self.assertIsNotNone(h.rqp, "Core data adapter is not implemented")
        self.assertTrue(h.rqp.Data.Ready()[0])
        self.assertEqual(h.g.TEST_CONTRACT, 4)
        self.assertEqual(h.rqp.Data.Source(), "QuestieDB")
        q = h.rqp.Data.Quest(19)
        self.assertEqual(q.reqLevel, 18)
        self.assertEqual(array(q.grp), [20, 98387])
        for key in ("grp", "single", "excl", "bcs", "startedBy", "finishedBy", "objectives", "skill", "ranks", "minRep", "maxRep"):
            self.assertIsNotNone(q[key], key)
        self.assertIsNone(h.rqp.Data.Quest(999999999))
        names = {d.name for d in array(h.rqp.Data.Dungeons())}
        self.assertIn("The Drowned City", names)
        self.assertIn("Gnomeregan", names)
        self.assertFalse(names & {"Warsong Gulch", "Arathi Basin", "Alterac Valley"})
        pin = h.rqp.Data.Pin("npc", 234)
        self.assertTrue(pin.map > 0 and 0 <= pin.x <= 100 and 0 <= pin.y <= 100)
        self.assertIsNotNone(h.rqp.Data.Npc(234))
        self.assertIsNotNone(h.rqp.Data.Object(31))
        self.assertIsNotNone(h.rqp.Data.Item(16305))


if __name__ == "__main__":
    if not AVAILABLE:
        print(f"SKIP: private quest data missing: {PRIVATE}")
    unittest.main(verbosity=2)
