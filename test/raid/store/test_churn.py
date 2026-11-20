"""Real-limit store size regression; invented data only."""
from pathlib import Path
import sys
import time
import unittest
sys.path.insert(0, str(Path(__file__).resolve().parent))
from test_store import boot, large_report, night_bytes, literal, assert_references


class ChurnTests(unittest.TestCase):
    def test_night_budget_rotating_roster_bound_and_reload(self):
        c = boot()
        c.lua.globals().R = large_report(c)
        c.lua.globals().END_WINDOW = lambda: c.fire("ENCOUNTER_END")
        c.run("""
            local S=RelaxRaid.modules.Store
            for day=1,1 do -- one real 3200-identity churn; nights have independent budgets
                R.sentAt=9999600000+day*86400
                GetServerTime=function() return R.sentAt end
                for window=1,80 do
                    END_WINDOW()
                    for i=1,40 do
                        -- 3200 distinct maximum-length identities per night.
                        R.name='Alia'..string.rep('Z',85)..string.format('%04d',(window-1)*40+i)..'-EU'
                        assert(S.put(R.name,R))
                    end
                end
            end
        """)
        # Keep the full three-night serialized/reload workload. Clone only the
        # independently budgeted night fixture, not live session accounting;
        # 3-night real-put retention is separately covered by fixed/small fills.
        one = literal(c.run("return RelaxRaidDB.prep.nights[1]"))
        for day in (2, 3):
            c.run("local n=" + one + f"; n.day=n.day+{day - 1}; RelaxRaidDB.prep.nights[{day}]=n")
        raw = literal(c.lua.globals().RelaxRaidDB)
        nights = c.run("return RelaxRaidDB.prep.nights")
        bodies = sum(len(b) for n in nights.values() for b in n["reports"].values())
        body_bytes = sum(night_bytes(n) for n in nights.values())
        references = sum(len(list(w["reports"].keys())) for n in nights.values() for w in n["windows"].values())
        start = time.perf_counter()
        second = boot({"RelaxRaidDB": raw})
        elapsed = time.perf_counter() - start
        print("ROSTER_CHURN references=%d bodies=%d body_bytes=%d bytes=%d reload_seconds=%.3f" %
              (references, bodies, body_bytes, len(raw.encode()), elapsed))
        self.assertLess(len(raw.encode()), 16*1024*1024)
        self.assertEqual(len(nights), 3)
        self.assertLessEqual(body_bytes, 12000000)
        self.assertLessEqual(references, 9600)
        self.assertEqual(references, bodies)
        for night in nights.values():
            self.assertLessEqual(night_bytes(night), 4000000)
            size = len(next(iter(night["reports"].values()))[1].encode())
            self.assertEqual(len(list(night["reports"].keys())), 4000000 // size)
            self.assertLessEqual(len(night["windows"]), 80)
            for key, reports in night["reports"].items():
                self.assertEqual(list(reports.keys()), [1])
            for window in night["windows"].values():
                self.assertGreater(len(list(window["reports"].keys())), 0)
                self.assertLessEqual(len(list(window["reports"].keys())), 40)
                for key, row in window["reports"].items():
                    self.assertEqual(row["report"], 1)
                    self.assertIsNotNone(night["reports"][key])
        self.assertEqual(literal(second.lua.globals().RelaxRaidDB), raw)
        assert_references(self, c)
        assert_references(self, second)


if __name__ == "__main__":
    unittest.main()
