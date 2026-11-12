"""Quest ids QuestieDB lists but cannot name must not break the planner (public, synthetic).

QuestieDB 1.0.16 (Forever, baked) lists ids whose row has no name, e.g. stub rows for
quests Questie blacklists as not in the game. Data.Quest returns nil for them.
"""
import pathlib as _p, sys as _s; _s.path.insert(0, str(_p.Path(__file__).resolve().parent))
import unittest
from pathlib import Path

from lupa.lua51 import LuaRuntime

ROOT = Path(__file__).resolve().parents[2]


def to_lua(lua, value):
    if isinstance(value, dict):
        return lua.table_from({k: to_lua(lua, v) for k, v in value.items()})
    if isinstance(value, (list, tuple)):
        return lua.table_from([to_lua(lua, v) for v in value])
    return value


def q(name, **kw):
    row = dict(name=name, requiredLevel=1, questLevel=10, requiredRaces=0, requiredClasses=0,
               startedBy={1: [100]}, finishedBy={1: [100]})
    row.update(kw)
    return row


# 9001 and 9002 mirror the real stub rows: present in GetAllIds, Exists true, no name.
ROWS = {1: q("First"), 2: q("Second", preQuestSingle=[1]), 3: q("Third", preQuestGroup=[1, 2]),
        9001: dict(startedBy={1: [100]}, finishedBy={2: [200]}), 9002: dict(questLevel=60)}


class NilQuestTests(unittest.TestCase):
    def setUp(self):
        lua = self.lua = LuaRuntime(unpack_returned_tuples=True)
        g = self.g = lua.globals()
        g.TEST_SNAPSHOT = to_lua(lua, dict(level=39, race="Dwarf", raceID=3, faction="Alliance",
                                           completed=[1], log=[], professions={}, **{"class": "WARRIOR"}))
        lua.execute((ROOT / "test/planner/mocks/wow.lua").read_text())
        g.TEST_ROWS = to_lua(lua, ROWS)
        lua.execute("""
          QuestDB={}
          function QuestDB.Get(id,key) return TEST_ROWS[id] and TEST_ROWS[id][key] end
          function QuestDB.Exists(id) return TEST_ROWS[id]~=nil end
          function QuestDB.GetAllIds()
            local out={}; for k in pairs(TEST_ROWS) do out[#out+1]=k end; table.sort(out); return out
          end
          LibQuestieDB={Quest=QuestDB,RequireContract=function() return true end,
            Enum={raceMaskById={[3]=4},classKeys={WARRIOR=1}}}
          LibQuestieDB.Support={Get=function() return {private={dungeons={[1]={'Test Dungeon'}}}} end}
        """)
        for name in ("Data", "Player", "Learn", "Engine"):
            p = ROOT / f"addon/RelaxQuestPlanner/Core/{name}.lua"
            # Chunk names as the client reports them, so a failure reads like the in-game error.
            lua.execute(p.read_text(), name=f"@Interface/AddOns/RelaxQuestPlanner/Core/{name}.lua")
        self.rqp = g.RelaxQuestPlanner

    def test_nameless_ids_resolve_to_nil(self):
        self.assertIsNone(self.rqp.Data.Quest(9001))
        self.assertIsNone(self.rqp.Data.Quest(9002))

    def test_all_quest_ids_lists_only_resolvable_ids(self):
        ids = self.rqp.Data.AllQuestIds()
        self.assertEqual([ids[i] for i in range(1, len(ids) + 1)], [1, 2, 3])

    def test_state_does_not_fail_on_a_nameless_id(self):
        # The guide picker asks Engine.State for each guide quest; on 37884c5 this raised
        # "Core/Engine.lua:70: attempt to index local 'r' (a nil value)".
        ok, err = self.lua.eval("""function()
            local p=RelaxQuestPlanner.Player.Snapshot()
            local out={}
            local ok,err=pcall(function()
              for _,q in ipairs({1,2,3}) do out[q]=RelaxQuestPlanner.Engine.State(q,p) end end)
            return ok, err
        end""")()
        self.assertTrue(ok, err)
        st = self.lua.eval("function(q) return (RelaxQuestPlanner.Engine.State(q)) end")
        self.assertEqual(st(1), "done")
        self.assertEqual(st(2), "available")
        self.assertEqual(st(3), "blocked")  # needs 2 as well

    def test_graph_still_skips_nameless_ids_if_listed(self):
        # Defence in depth: even if an unresolvable id reaches the graph, it is skipped.
        self.lua.execute("""
          local D=RelaxQuestPlanner.Data; local all=D.AllQuestIds
          D.AllQuestIds=function() local t={} for i,v in ipairs(all()) do t[i]=v end
            t[#t+1]=9001; t[#t+1]=9002; return t end
          RelaxQuestPlanner.Engine.Invalidate()
        """)
        ok, err = self.lua.eval("""function()
            local ok,err=pcall(RelaxQuestPlanner.Engine.State,3,RelaxQuestPlanner.Player.Snapshot())
            return ok,err end""")()
        self.assertTrue(ok, err)

    def test_path_build_skips_nameless_ids(self):
        self.g.TEST_ROUTE = None
        ok, err = self.lua.eval("""function()
            local ok,err=pcall(RelaxQuestPlanner.Engine.BuildPathAsync,'Test Dungeon',
              RelaxQuestPlanner.Player.Snapshot(),{},function(r) TEST_ROUTE=r end)
            return ok,err end""")()
        self.assertTrue(ok, err)
        self.lua.execute("TEST_DriveFrames(100000)")
        self.assertIsNotNone(self.g.TEST_ROUTE, "async callback did not complete")
        errors = self.lua.eval("""function(route)
            local out={}
            for _,issue in ipairs(route.issues or {}) do
              if issue[1]=='error' then out[#out+1]=tostring(issue[2]) end end
            return table.concat(out,' | ') end""")(self.g.TEST_ROUTE)
        self.assertEqual(errors, "")


if __name__ == "__main__":
    unittest.main(verbosity=2)
