"""Real-limit store size regression; invented data only."""
from pathlib import Path
import sys
import time
import unittest
sys.path.insert(0, str(Path(__file__).resolve().parent))
from test_store import boot, literal


class FillTests(unittest.TestCase):
    def test_full_store_serialized_bound_and_reload(self):
        c = boot()
        c.lua.globals().END_WINDOW = lambda: c.fire("ENCOUNTER_END")
        c.run("""
            local S,P=RelaxRaid.modules.Store,RelaxRaid.modules.Prep
            for day=1,3 do
                GetServerTime=function() return 9999600000+day*86400 end
                local latest={}
                -- Saturate each character's own budget with distinct maximum-field
                -- bodies in one window, then fill all 80 windows with references.
                for i=1,40 do
                    local r=P.build('encounter')
                    r.addonVersion=string.rep(string.char(92),24)
                    r.class,r.level,r.playedLevel='unreadable','unreadable','unreadable'
                    r.name='Alia'..string.rep('Z',87)..string.char(65+math.floor((i-1)/26))..string.char(65+(i-1)%26)..'-EU'
                    r.consumables={}; r.gear={}; r.notScored={}
                    for j=1,32 do r.consumables[j]={category='enhancement',itemID=999999999-j,count=999999} end
                    for j=1,19 do r.gear[j]={slot=j,wornID=999999998,bagID=999999999}; r.notScored[j]=j end
                    for revision=1,80 do
                        r.consumables[1].count=999919+revision
                        assert(S.put(r.name,r))
                    end
                    latest[i]=r
                end
                for window=1,80 do
                    END_WINDOW()
                    for _,r in ipairs(latest) do assert(S.put(r.name,r)) end
                end
            end
        """)
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(c.run("local count=0; for _,n in ipairs(RelaxRaidDB.prep.nights) do for _,w in ipairs(n.windows) do for _ in pairs(w.reports) do count=count+1 end end end; return count"), 9600)
        self.assertLess(len(raw.encode()), 16*1024*1024)
        bodies, body_bytes = c.run("""
            local count, bytes=0,0
            for _,n in ipairs(RelaxRaidDB.prep.nights) do
                for _,reports in pairs(n.reports) do
                    local own=0
                    for _,body in ipairs(reports) do own=own+#body; count=count+1 end
                    assert(own <= 100000 and own+#reports[1] > 100000)
                    bytes=bytes+own
                end
            end
            return count,bytes
        """)
        start=time.perf_counter()
        second=boot({"RelaxRaidDB":raw})
        elapsed=time.perf_counter()-start
        self.assertEqual(literal(second.lua.globals().RelaxRaidDB),raw)
        print("STORE_BOUND references=9600 bodies=%d body_bytes=%d bytes=%d reload_seconds=%.3f" %
              (bodies,body_bytes,len(raw.encode()),elapsed))


if __name__ == "__main__":
    unittest.main()
