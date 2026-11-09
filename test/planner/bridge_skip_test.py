"""Skipped quests (per character) are not exported to the Journey guide."""
import pathlib, unittest
import lupa.lua51 as lua51

ROOT = pathlib.Path(__file__).resolve().parents[2]
BRIDGE = ROOT / "addon/RelaxQuestPlanner/Journey/Bridge.lua"

ROUTE = r'''{title="T", steps={
  {q=1,kind="accept",name="One"},{q=2,kind="accept",name="Two"},{q=2,kind="turnin",name="Two"},{q=3,kind="accept",name="Three",needs={1}}}}'''

class BridgeSkip(unittest.TestCase):
    def load(self, db):
        L = lua51.LuaRuntime()
        L.execute("RelaxQuestPlanner={Journey={}}; RelaxQuestPlannerDB=" + db)
        L.execute(BRIDGE.read_text(encoding="utf-8-sig"))
        return L

    def ids(self, L):
        return list(L.eval('''(function() local g=assert(RelaxQuestPlanner.Journey.ToGuide(%s)); local o={}
          for i,s in ipairs(g) do o[#o+1]=s.id end; return table.concat(o," ") end)()''' % ROUTE).split())

    def test_no_skips_exports_all(self):
        ids = self.ids(self.load("{}"))
        self.assertEqual(len(ids), 5)  # 4 actions + end marker

    def test_skipped_quest_dropped(self):
        ids = self.ids(self.load("{skipped={[2]=true}}"))
        self.assertEqual(len(ids), 3)
        self.assertFalse(any("-2-" in i for i in ids))
        self.assertEqual(ids[-1], "RQP-end")

if __name__ == "__main__":
    unittest.main(verbosity=1)
