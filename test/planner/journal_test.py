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


if __name__ == '__main__':
    unittest.main(verbosity=2)
