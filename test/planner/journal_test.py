"""Public synthetic journal adapter/engine regressions; no donor records."""
import pathlib as _p, sys as _s; _s.path.insert(0, str(_p.Path(__file__).resolve().parent))
import unittest
from pathlib import Path
from lupa.lua51 import LuaRuntime
from nil_quest_test import to_lua, q

ROOT = Path(__file__).resolve().parents[2]
A, B, C, T = 810001, 810002, 810003, 810004
OUTSIDE, INSIDE = 880001, 880002
OUT_NPC, IN_NPC = 881001, 881002


def array(t):
    return [t[i] for i in range(1, len(t) + 1)]


def plain(t):
    if isinstance(t, tuple):
        return [plain(v) for v in t]
    if hasattr(t, 'items'):
        return {k: plain(v) for k, v in t.items()}
    return t


class Harness:
    def __init__(self, rows=None, entries=None, chains=None, level=35, log=(), done=(), **player):
        self.lua = LuaRuntime(unpack_returned_tuples=True)
        self.g = self.lua.globals()
        snapshot = dict(level=level, race='Dwarf', raceID=3, faction='Alliance',
                        completed=list(done), log=list(log), professions={}, **{'class': 'WARRIOR'})
        snapshot.update(player)
        self.g.TEST_SNAPSHOT = to_lua(self.lua, snapshot)
        self.lua.execute((ROOT / 'test/planner/mocks/wow.lua').read_text())
        self.g.TEST_ROWS = to_lua(self.lua, rows or {})
        self.lua.execute('''
          function TEST_API(rows)
            return {Get=function(id,key) return rows[id] and rows[id][key] end,
              Exists=function(id) return rows[id]~=nil end,
              GetAllIds=function() local t={} for id in pairs(rows) do t[#t+1]=id end table.sort(t); return t end}
          end
          QuestDB=TEST_API(TEST_ROWS)
          TEST_NPCS={[881001]={name='Copper keeper',zoneID=880001},
                     [881002]={name='Tin keeper',zoneID=880002}}
          TEST_ITEMS={}; TEST_OBJECTS={}
          LibQuestieDB={Quest=QuestDB,Npc=TEST_API(TEST_NPCS),Item=TEST_API(TEST_ITEMS),
            Object=TEST_API(TEST_OBJECTS),RequireContract=function() return true end,
            Enum={raceMaskById={[1]=1,[3]=4,[7]=64,[8]=128},
              factionRaceMasks={Alliance=69,Horde=128},classKeys={WARRIOR=1,PALADIN=2,HUNTER=4,MAGE=128}}}
        ''')
        for name in ('Data', 'Player', 'Learn', 'Engine'):
            self.lua.execute((ROOT / f'addon/RelaxQuestPlanner/Core/{name}.lua').read_text())
        self.r = self.g.RelaxQuestPlanner
        # Names/aliases come from RQP's existing allowlist, not a donor fixture.
        self.name = self.r.Data.FOREVER_DUNGEONS[3].name
        self.alias = self.r.Data.FOREVER_DUNGEONS[3].aliases[1]
        self.g.TEST_DUNGEON = self.name
        self.lua.execute('''LibQuestieDB.Support={Get=function() return {private={
          dungeons={[880002]={name=TEST_DUNGEON}},areaIdToUiMapId={}}} end}''')
        if entries is not None:
            self.g.ForeverDungeonJournal_NS = to_lua(self.lua, dict(
                DB={self.name: dict(quests=entries)}, QUEST_PREREQ_CHAINS=chains or {}))
        self.r.Data.Reset()

    def route(self, **opts):
        return self.r.Engine.BuildPath(self.name, self.r.Player.Snapshot(), to_lua(self.lua, {'mode': 'all', **opts}))

    def states(self):
        p = self.r.Player.Snapshot()
        return {id: plain(self.r.Engine.State(id, p)) for id in array(self.r.Data.AllQuestIds())}


class JournalTests(unittest.TestCase):
    def test_alias_lookup_for_known_quest(self):
        h = Harness({A: q('Copper errand')}, [dict(id=A, name='Copper errand', pickup='At the copper gate')])
        self.assertTrue(callable(h.r.Data.JournalDungeon), 'JournalDungeon accessor missing')
        self.assertEqual(h.r.Data.JournalDungeon(h.alias)[1].id, A)
        self.assertEqual(h.r.Data.Journal(A).pickup, 'At the copper gate')
        self.assertEqual(h.r.Data.Quest(A).source, 'QuestieDB')

    def test_wings_merge_and_duplicate_ids_collapse(self):
        h = Harness()
        d = h.r.Data.FOREVER_DUNGEONS[13]
        h.g.ForeverDungeonJournal_NS = to_lua(h.lua, dict(DB={
            d.aliases[1]: dict(quests=[dict(id=A, name='Copper errand')]),
            d.aliases[2]: dict(quests=[dict(id=A, name='Copper errand'), dict(id=B, name='Tin errand')])}))
        h.r.Data.Reset()
        self.assertTrue(callable(h.r.Data.JournalDungeon), 'JournalDungeon accessor missing')
        self.assertEqual([v.id for v in array(h.r.Data.JournalDungeon(d.name))], [A, B])

    def test_non_numeric_id_skipped_and_missing_fdj_is_nil(self):
        h = Harness(entries=[dict(id='not-a-quest', name='Copper token'), dict(id=A, name='Copper errand')])
        self.assertTrue(callable(h.r.Data.Journal), 'Journal accessor missing')
        self.assertEqual(len(h.r.Data.JournalDungeon(h.name)), 1)
        self.assertIsNone(h.r.Data.Journal('not-a-quest'))
        h.g.ForeverDungeonJournal_NS = None; h.r.Data.Reset()
        self.assertIsNone(h.r.Data.Journal(A))
        self.assertIsNone(h.r.Data.JournalDungeon(h.name))

    def test_malformed_journal_fails_closed_in_all_consumers(self):
        h = Harness({A: q('Copper errand')})
        self.assertTrue(callable(h.r.Data.Journal), 'Journal accessor missing')
        for bad in ("{DB=true}", "{DB={[TEST_DUNGEON]={quests=false}}}",
                    "{DB={[TEST_DUNGEON]={quests={false}}}}",
                    "setmetatable({}, {__index=function() error('changed layout') end})",
                    "{DB={},QUEST_PREREQ_CHAINS={[810001]={false}}}"):
            with self.subTest(bad=bad):
                h.lua.execute('ForeverDungeonJournal_NS=' + bad)
                h.r.Data.Reset()
                self.assertIsNone(h.r.Data.JournalDungeon(h.name))
                self.assertIsNone(h.r.Data.Journal(A))
                h.route(); h.r.Data.Pin('quest', A)

    def test_string_level_and_item_step_survive_snapshot(self):
        h = Harness(entries=[dict(id=A, name='Copper errand', level='20-25')],
                    chains={A: [dict(id=B, name='Tin token', itemStep=True, choice=1)]})
        self.assertIsNotNone(h.r.Data.Journal(A))
        self.assertEqual(h.r.Data.Journal(A).level, '20-25')
        self.assertTrue(h.r.Data.FDJ().QUEST_PREREQ_CHAINS[A][1].itemStep)

    def test_overlay_is_snapshot_and_reset_invalidates(self):
        h = Harness(entries=[dict(id=A, name='Copper errand', requires=12,
            requiredItems=[[882001, 'Copper kit', 1, 'Bring two']], unavailableRaces={'Gnome': True})])
        self.assertTrue(callable(h.r.Data.Journal), 'Journal accessor missing')
        self.assertEqual(h.r.Data.Journal(A).requires, 12)
        h.lua.execute('ForeverDungeonJournal_NS.DB[TEST_DUNGEON].quests[1].requires=18')
        self.assertEqual(h.r.Data.Journal(A).requires, 12)
        h.r.Data.Reset()
        self.assertEqual(h.r.Data.Journal(A).requires, 18)
        self.assertEqual(h.r.Data.Journal(A).requiredItems[1][2], 'Copper kit')


def golden_routes():
    rows = {
        A: q('Copper errand', questLevel=20, startedBy={1: [OUT_NPC]}, finishedBy={1: [OUT_NPC]}, nextQuestInChain=B),
        B: q('Tin errand', questLevel=30, zoneOrSort=INSIDE, preQuestGroup=[A],
             startedBy={1: [OUT_NPC]}, finishedBy={1: [IN_NPC]}),
        C: q('Zinc errand', questLevel=40, requiredLevel=38, zoneOrSort=INSIDE,
             startedBy={1: [IN_NPC]}, finishedBy={1: [OUT_NPC]}),
    }
    result = {}
    for mode in ('all', 'game', 'strict'):
        for phase in ('fresh', 'active', 'done'):
            h = Harness(rows, log=[dict(id=B, complete=True)] if phase == 'active' else (),
                        done=[A] if phase == 'done' else ())
            result[mode + '-' + phase] = plain(h.route(mode=mode))
    return result


class MembershipTests(unittest.TestCase):
    def test_journal_membership_without_area_and_hidden_leadin(self):
        h = Harness({A: q('Copper lead', nextQuestInChain=B), B: q('Tin target')},
                    [dict(id=A, name='Copper lead', hideFromMainList=True), dict(id=B, name='Tin target')])
        h.lua.execute('LibQuestieDB.Support=nil'); h.r.Data.Reset()
        route = h.route()
        self.assertEqual(route.targets, 1)
        self.assertEqual(array(route.plan), [A, B])

    def test_non_journal_extras_are_counted_and_in_log_is_kept(self):
        rows = {A: q('Copper target', zoneOrSort=INSIDE), B: q('Tin extra', zoneOrSort=INSIDE),
                C: q('Zinc active', zoneOrSort=INSIDE)}
        h = Harness(rows, [dict(id=A, name='Copper target')], log=[dict(id=C, complete=False)])
        route = h.route()
        self.assertEqual(route.targets, 2)
        self.assertEqual(set(array(route.plan)), {A, C})
        self.assertEqual([v.q for v in array(route.extras)], [B])
        self.assertEqual(route.extraCount, 1)

    def test_faction_class_race_unavailable_and_completed_filters(self):
        rows = {A+i: q('Metal errand '+str(i)) for i in range(8)}
        rows[A+1]['requiredClasses'] = 1  # DB class beats journal class
        rows[A+5]['requiredRaces'] = 1   # same-faction race exclusion
        entries = [dict(id=A, name='Metal errand 0', faction='Horde'),
                   dict(id=A+1, name='Metal errand 1', classOnly='MAGE'),
                   dict(id=A+2, name='Metal errand 2', classOnly='MAGE'),
                   dict(id=A+3, name='Metal errand 3', unavailableRaces={'Dwarf': True}),
                   dict(id=A+4, name='Metal errand 4', unavailable=True),
                   dict(id=A+5, name='Metal errand 5', faction='Alliance'),
                   dict(id=A+6, name='Metal errand 6'), dict(id=A+7, name='Metal errand 7', requires=40)]
        h = Harness(rows, entries, done=[A+6])
        route = h.route()
        self.assertEqual(route.targets, 2)
        self.assertEqual(set(array(route.plan)), {A+1, A+7})
        self.assertEqual([s.q for s in array(route.later)], [A+7])

    def test_faction_conflict_keeps_target_with_label_without_changing_state(self):
        h = Harness({A: q('Copper disputed', requiredRaces=128)},
                    [dict(id=A, name='Copper disputed', faction='Alliance')])
        self.assertEqual(h.r.Engine.State(A)[0], 'race')
        route = h.route()
        self.assertEqual(route.targets, 1)
        self.assertEqual(array(route.plan), [A])
        self.assertIn('check faction', route.steps[1].note)
        self.assertEqual(h.r.Engine.State(A)[0], 'race')

    def test_no_journal_full_route_golden(self):
        import json
        expected = json.loads((ROOT / 'test/planner/fixtures/no_journal_routes.json').read_text())
        self.assertEqual(json.loads(json.dumps(golden_routes())), expected)


def chain_harness(**kw):
    rows = {id: q(name, zoneOrSort=INSIDE) for id, name in
            ((A, 'Copper errand'), (B, 'Tin errand'), (C, 'Zinc errand'), (T, 'Alloy errand'))}
    return Harness(rows, [dict(id=T, name='Alloy errand')],
                   {T: [dict(id=id, name=rows[id]['name']) for id in (C, B, A)]}, **kw)


class HintTests(unittest.TestCase):
    def test_three_step_chain_orders_turnin_before_next_accept(self):
        h = chain_harness(); route = h.route()
        keys = [(s.kind, s.q) for s in array(route.steps)]
        for before, after in ((C, B), (B, A), (A, T)):
            self.assertLess(keys.index(('turnin', before)), keys.index(('accept', after)))

    def test_hint_does_not_change_any_quest_state_or_gate(self):
        import json
        expected = json.loads((ROOT / 'test/planner/fixtures/journal_states.json').read_text())
        actual = {}
        for phase in ('fresh', 'active', 'done'):
            h = chain_harness(log=[dict(id=T, complete=False)] if phase == 'active' else (),
                              done=[C] if phase == 'done' else ())
            before = h.states(); h.route()
            self.assertEqual(h.states(), before)
            actual[phase] = before
        self.assertEqual(json.loads(json.dumps(actual)), expected)

    def test_item_and_unknown_steps_are_skipped_by_hints(self):
        h = chain_harness()
        h.g.ForeverDungeonJournal_NS.QUEST_PREREQ_CHAINS[T] = to_lua(h.lua, [
            dict(id=C, name='Zinc errand'), dict(id=1810001, name='Copper token', itemStep=True),
            dict(id=810090, name='Missing errand'), dict(id=A, name='Copper errand')])
        h.r.Data.Reset()
        route = h.route(); keys = [(s.kind, s.q) for s in array(route.steps)]
        self.assertLess(keys.index(('turnin', C)), keys.index(('accept', A)))
        self.assertNotIn(1810001, array(route.plan)); self.assertNotIn(810090, array(route.plan))
        self.assertFalse(any(i[1] == 'actionCycle' for i in array(route.issues)))

    def test_choice_group_satisfied_by_one_completed_member(self):
        h = chain_harness(done=[C])
        h.g.ForeverDungeonJournal_NS.QUEST_PREREQ_CHAINS[T] = to_lua(h.lua, [
            dict(id=C, name='Zinc errand', choice=1), dict(id=B, name='Tin errand', choice=1),
            dict(id=A, name='Copper errand')])
        h.r.Data.Reset()
        route = h.route()
        keys = [(s.kind, s.q) for s in array(route.steps)]
        # The old target claims still include B; the hint must not force B before A.
        self.assertLess(keys.index(('accept', A)), keys.index(('turnin', B)))
        self.assertTrue(route.orderHints is not None, 'route order-hint diagnostics missing')
        self.assertFalse(any(x.before == B and x.after == A for x in array(route.orderHints)))

    def test_conflicting_hint_is_dropped_not_action_cycle(self):
        h = chain_harness()
        h.g.TEST_ROWS[C].preQuestGroup = to_lua(h.lua, [B])  # opposes C -> B hint
        h.r.Data.Reset(); route = h.route()
        self.assertIsNotNone(route.droppedHints, 'dropped hint diagnostics missing')
        self.assertTrue(any(x.before == C and x.after == B for x in array(route.droppedHints)))
        self.assertFalse(any(i[1] == 'actionCycle' for i in array(route.issues)))
        self.assertEqual(len(route.steps), 12)


if __name__ == '__main__':
    unittest.main(verbosity=2)
