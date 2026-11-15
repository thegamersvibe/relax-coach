#!/usr/bin/env python3
"""Player.LogKey: the cheap hidden-window key (level, map, zone, quest log, ready flags).

Run: uv run -q --with lupa python test/planner/player_logkey_test.py
Authored fixture only; no private data.
"""
from pathlib import Path
import unittest

from lupa.lua51 import LuaRuntime

ROOT = Path(__file__).resolve().parents[2]


class Mini:
    """The planner's wow.lua mock and the real Core/Player.lua, nothing else."""
    def __init__(self, snapshot):
        self.lua = LuaRuntime(unpack_returned_tuples=True)
        self.lua.globals().TEST_SNAPSHOT = self.lua.table_from({
            k: (self.lua.table_from([self.lua.table_from(e) if isinstance(e, dict) else e for e in v])
                if isinstance(v, list) else v) for k, v in snapshot.items()})
        self.lua.execute((ROOT / "test/planner/mocks/wow.lua").read_text())
        self.lua.execute("RelaxQuestPlanner={Data={Error=function(e) TEST_ERROR=e end, Enums=function() return {} end}}")
        self.lua.execute((ROOT / "addon/RelaxQuestPlanner/Core/Player.lua").read_text())
        self.rqp = self.lua.globals().RelaxQuestPlanner


class LogKeyTests(unittest.TestCase):
    def harness(self):
        snap = dict(level=30, race="Dwarf", raceID=3, faction="Alliance", completed=[5, 6], professions={},
                    log=[dict(id=11, title="a", level=30, complete=False), dict(id=12, title="b", level=30, complete=True)],
                    **{"class": "WARRIOR"})
        h = Mini(snap)
        h.lua.execute("C_Map.GetBestMapForUnit=function() return TEST_MAP end; GetRealZoneText=function() return TEST_ZONE_TEXT end")
        h.lua.execute('TEST_MAP, TEST_ZONE_TEXT = 1429, "Elwynn Forest"')
        return h

    def test_key_covers_level_map_zone_log_and_ready(self):
        h = self.harness()
        key = h.rqp.Player.LogKey
        self.assertEqual(key(), "30,1429,Elwynn Forest,11,12r")
        h.lua.execute("TEST_SNAPSHOT.log[1].complete=true")
        self.assertEqual(key(), "30,1429,Elwynn Forest,11r,12r", "a quest ready to turn in changes the key")
        h.lua.execute("table.remove(TEST_SNAPSHOT.log, 2)")
        self.assertEqual(key(), "30,1429,Elwynn Forest,11r", "a quest leaving the log changes the key")
        h.lua.execute('TEST_MAP, TEST_ZONE_TEXT = 1433, "Redridge Mountains"')
        self.assertEqual(key(), "30,1433,Redridge Mountains,11r")

    def test_objectives_complete_before_ready_for_turn_in(self):
        h = self.harness()
        h.lua.execute("C_QuestLog.IsComplete=function(q) return q==11 end")
        self.assertEqual(h.rqp.Player.LogKey(), "30,1429,Elwynn Forest,11c,12r",
                         "IsComplete without ReadyForTurnIn changes the key")

    def test_key_never_reads_completed_skills_or_reputation(self):
        h = self.harness()
        h.lua.execute("""
          C_QuestLog.GetAllCompletedQuestIDs=function() error('completed read') end
          C_SkillInfo.GetNumSkillLines=function() error('skills read') end
          C_Reputation.GetNumFactions=function() error('reputation read') end
        """)
        self.assertEqual(h.rqp.Player.LogKey(), "30,1429,Elwynn Forest,11,12r")

    def test_secret_or_missing_values_are_blank_not_errors(self):
        h = self.harness()
        h.lua.execute("issecretvalue=function(v) return v==TEST_SECRET end; TEST_SECRET={}")
        h.lua.execute("TEST_MAP=TEST_SECRET; TEST_ZONE_TEXT=nil")
        self.assertEqual(h.rqp.Player.LogKey(), "30,,,11,12r")


if __name__ == "__main__":
    unittest.main()
