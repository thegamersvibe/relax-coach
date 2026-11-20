"""Last-reference sizing/retention regressions, real limits, synthetic data."""
from pathlib import Path
import sys
import time
import unittest
sys.path.insert(0, str(Path(__file__).resolve().parent))
from test_store import boot, large_report, literal, night_bytes, assert_references, report_at


def measure(test, c, label):
    raw = literal(c.lua.globals().RelaxRaidDB)
    nights = c.run("return RelaxRaidDB.prep.nights")
    bodies = sum(len(b) for n in nights.values() for b in n["reports"].values())
    references = sum(len(list(w["reports"].keys())) for n in nights.values() for w in n["windows"].values())
    body_bytes = sum(night_bytes(n) for n in nights.values())
    start = time.perf_counter()
    second = boot({"RelaxRaidDB": raw})
    elapsed = time.perf_counter() - start
    print(f"{label} bodies={bodies} references={references} body_bytes={body_bytes} "
          f"serialized={len(raw.encode())} reload_seconds={elapsed:.3f}", flush=True)
    test.assertLess(len(raw.encode()), 16 * 1024 * 1024)
    assert_references(test, c)
    assert_references(test, second)
    test.assertEqual(literal(second.lua.globals().RelaxRaidDB), raw)
    return bodies, references


class LifetimeFillTests(unittest.TestCase):
    def test_small_report_fill_one_window_three_nights(self):
        c = boot()
        c.run("""
            local S,P=RelaxRaid.modules.Store,RelaxRaid.modules.Prep
            for day=1,3 do
                GetServerTime=function() return 1800000000+day*86400 end
                for i=1,40 do
                    local r=P.build('ready')
                    r.name=string.format('Alia%02d-EU',i)
                    r.addonVersion=string.rep(string.char(92),24)
                    r.class='MAGE'; r.level=1; r.sentAt=1
                    r.consumables={{category='flask',itemID=1,count=1}}
                    r.gear='not checked'; r.notScored=nil
                    -- Change actual prep CONTENT, not the now-excluded played time.
                    for revision=1,2000 do
                        r.consumables[1].count=revision
                        assert(S.put(r.name,r))
                    end
                end
            end
        """)
        self.assertEqual(measure(self, c, "SMALL_FILL"), (120, 120))
        for night in c.run("return RelaxRaidDB.prep.nights").values():
            self.assertEqual(len(night["windows"]), 1)
            self.assertEqual(len(list(night["reports"].keys())), 40)
            for bodies in night["reports"].values():
                self.assertEqual(c.lua.globals().RelaxRaid.modules.Wire.decode(bodies[1])["consumables"][1]["count"], 2000)

    def test_sixty_pull_max_size_keeps_every_window(self):
        c = boot()
        c.lua.globals().R = large_report(c)
        c.lua.globals().END_WINDOW = lambda: c.fire("ENCOUNTER_END")
        c.run("""
            local S=RelaxRaid.modules.Store
            GetServerTime=function() return 9999700000 end
            R.sentAt=9999700000; R.level=60; R.playedLevel=999999999
            for pull=1,60 do
                END_WINDOW()
                for i=1,40 do
                    R.name='Alia'..string.rep('Z',87)..string.format('%02d',i)..'-EU'
                    R.reason='ready'; R.consumables[1].count=999900+pull
                    assert(S.put(R.name,R))
                    R.reason='encounter'
                    assert(S.put(R.name,R))
                end
            end
        """)
        # All fields at maximum encoded width; each distinct content has two
        # receipts in its pull. Only the final encounter report is referenced.
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        self.assertEqual(len(night["windows"]), 60)
        size = len(next(iter(night["reports"].values()))[1].encode())
        self.assertEqual(size, 1572)
        self.assertLessEqual(60 * size, 100000)
        self.assertLessEqual(60 * size * 40, 4000000)
        self.assertIsNone(c.lua.globals().RelaxRaid.modules.Store.stats.budgetEvicted)
        self.assertIsNone(c.lua.globals().RelaxRaid.modules.Store.stats.nightBudgetEvicted)
        for wi, window in night["windows"].items():
            self.assertEqual(len(list(window["reports"].keys())), 40)
            for key, row in window["reports"].items():
                self.assertEqual(row["report"], wi)
                self.assertEqual(len(night["reports"][key]), 60)
                r = report_at(c, key, window=wi)
                self.assertEqual(r["reason"], "encounter")
                self.assertEqual(r["consumables"][1]["count"], 999900 + wi)
        self.assertEqual(measure(self, c, "SIXTY_PULL"), (2400, 2400))


if __name__ == "__main__":
    unittest.main()
