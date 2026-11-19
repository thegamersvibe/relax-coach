"""Logger prep store; invented fixtures only."""
from pathlib import Path
import sys
import unittest
import time
ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "test/raid/_client"))
from client import Client
from serialize import literal
ADDON = ROOT / "addon/RelaxRaid"


def boot(saved=None):
    c = Client()
    c.guild = ("Fake Guild", "Rank F", 5)
    c.load_addon(str(ADDON), saved)
    c.in_raid = True
    c.enter("raid", 901)
    return c


class StoreTests(unittest.TestCase):
    def test_latest_per_player_window_receiver_time(self):
        c = boot()
        self.assertTrue(c.run("return RelaxRaid.modules.Store ~= nil"), "store module missing")
        self.assertTrue(c.run("""
            local r = RelaxRaid.modules.Prep.build('join')
            r.sentAt = 99
            return RelaxRaid.modules.Store.put(r.name, r)
        """))
        c.advance(2)
        c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.sentAt=1; RelaxRaid.modules.Store.put(r.name,r)")
        row = c.run("return RelaxRaidDB.prep.nights[1].windows[1].reports['alia-stone-pvp-eu']")
        self.assertEqual(row["receivedAt"], c.wall + 2)
        self.assertEqual(row["report"]["sentAt"], 1)
        self.assertEqual(row["report"]["reason"], "ready")
        self.assertEqual(c.run("local n=0; for _ in pairs(RelaxRaidDB.prep.nights[1].windows[1].reports) do n=n+1 end; return n"), 1)
        c.fire("ENCOUNTER_END")
        c.run("local r=RelaxRaid.modules.Prep.build('encounter'); RelaxRaid.modules.Store.put(r.name,r)")
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights[1].windows"), 2)

    def test_newer_save_is_byte_identical(self):
        for saved in ({"RelaxRaidDB": "{version=2,future={opaque='keep'}}"},
                      {"RelaxRaidDB": "{version=1,prep={version=2,nights={future='keep'}}}"},
                      {"RelaxRaidCharDB": "{version=2,logger=true,future='keep'}"}):
            c = Client()
            expected = {key: literal(c.run("return " + src)) for key,src in saved.items()}
            c.guild = ("Fake Guild", "Rank F", 5)
            c.load_addon(str(ADDON), saved)
            c.in_raid = True
            c.run("local r=RelaxRaid.modules.Prep.build('join'); RelaxRaid.modules.Store.put(r.name,r)")
            c.fire("ENCOUNTER_END")
            c.slash("/relaxraid logger on")
            for key, original in expected.items():
                self.assertEqual(literal(c.lua.globals()[key]), original)

    def test_store_caps_and_logger_choice(self):
        c = boot()
        c.lua.globals().END_WINDOW = lambda: c.fire("ENCOUNTER_END")
        c.run("""
            local S,P = RelaxRaid.modules.Store,RelaxRaid.modules.Prep
            for day=1,4 do
                GetServerTime=function() return 1800000000+day*86400 end
                for window=1,81 do
                    -- close one receiver pull window
                    END_WINDOW()
                    for i=1,41 do
                        local r=P.build('ready'); r.name='Alia-'..string.char(65+math.floor((i-1)/26))..string.char(65+(i-1)%26)..'-PvP-EU'
                        S.put(r.name,r)
                    end
                end
            end
        """)
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 3)
        self.assertTrue(c.run("""
            for _,n in ipairs(RelaxRaidDB.prep.nights) do
                if #n.windows ~= 80 then return false end
                for _,w in ipairs(n.windows) do
                    local count=0; for _ in pairs(w.reports) do count=count+1 end
                    if count ~= 40 then return false end
                end
            end
            return true
        """))
        c.slash("/relaxraid logger off")
        before = literal(c.lua.globals().RelaxRaidDB)
        self.assertFalse(c.run("local r=RelaxRaid.modules.Prep.build('ready'); return RelaxRaid.modules.Store.put(r.name,r)"))
        self.assertEqual(literal(c.lua.globals().RelaxRaidDB), before)

    def test_load_validates_retained_records(self):
        c = boot({"RelaxRaidDB": "{version=1,prep={version=1,nights={{day=1,windows={{openedAt=5,reports={forged={receivedAt=6,report={}}}}}}}}}"})
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
        c = boot()
        c.run("local r=RelaxRaid.modules.Prep.build('join'); RelaxRaid.modules.Store.put(r.name,r)")
        saved = literal(c.lua.globals().RelaxRaidDB)
        after = boot({"RelaxRaidDB": saved})
        self.assertEqual(literal(after.lua.globals().RelaxRaidDB), saved)
        for value in ("{version=1,prep={version=1,nights='bad'}}", "{version=1,prep={version=0}}"):
            c = boot({"RelaxRaidDB": value})
            self.assertTrue(c.run("local r=RelaxRaid.modules.Prep.build('join'); return RelaxRaid.modules.Store.put(r.name,r)"))
            self.assertEqual(c.run("return next(RelaxRaid.errors)"), None)

    def test_store_rejects_forgery_independently_and_copies(self):
        c = boot()
        self.assertFalse(c.run("local r=RelaxRaid.modules.Prep.build('ready'); return RelaxRaid.modules.Store.put('Bram-Oak-Hill-PvP-EU',r)"))
        self.assertEqual(c.run("return RelaxRaid.modules.Store.stats.forged"), 1)
        c.run("""
            local r=RelaxRaid.modules.Prep.build('ready')
            RelaxRaid.modules.Store.put(r.name,r)
            r.name='Bram-Oak-Hill-PvP-EU'
        """)
        self.assertEqual(c.run("return RelaxRaidDB.prep.nights[1].windows[1].reports['alia-stone-pvp-eu'].report.name"), "Alia-Stone-PvP-EU")

    def test_not_scored_validated_copied_and_reloaded(self):
        c = boot()
        self.assertTrue(c.run("""
            local r=RelaxRaid.modules.Prep.build('ready')
            r.gear={}; r.notScored={1,19}
            local ok=RelaxRaid.modules.Store.put(r.name,r)
            r.notScored[1]=2
            return ok
        """))
        row = c.run("return RelaxRaidDB.prep.nights[1].windows[1].reports['alia-stone-pvp-eu'].report")
        self.assertIsNotNone(row["notScored"])
        self.assertEqual(list(row["notScored"].values()), [1, 19])
        saved = literal(c.lua.globals().RelaxRaidDB)
        reloaded = boot({"RelaxRaidDB": saved})
        self.assertEqual(literal(reloaded.lua.globals().RelaxRaidDB), saved)
        for bad in ("{1,1}", "{0}", "{20}", "{1.5}", "{[2]=1}", "{secret()}", "secret()",
                    "{1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,19}"):
            self.assertFalse(c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.gear={}; r.notScored=" + bad +
                                   "; return RelaxRaid.modules.Store.put(r.name,r)"), bad)
            self.assertEqual(literal(c.lua.globals().RelaxRaidDB), saved)
        corrupt = saved.replace('["notScored"]={[1]=1,[2]=19}', '["notScored"]={[1]=0}')
        self.assertNotEqual(corrupt, saved)
        self.assertEqual(boot({"RelaxRaidDB": corrupt}).run("return #RelaxRaidDB.prep.nights"), 0)

    def test_read_only_latch_survives_external_version_reset(self):
        c = boot({"RelaxRaidDB": "{version=2,future='opaque'}"})
        c.run("RelaxRaidDB={version=1,prep={version=1,nights={}}}")
        before = literal(c.lua.globals().RelaxRaidDB)
        self.assertFalse(c.run("local r=RelaxRaid.modules.Prep.build('join'); return RelaxRaid.modules.Store.put(r.name,r)"))
        self.assertEqual(literal(c.lua.globals().RelaxRaidDB), before)

    def test_store_outside_raid_is_unchanged(self):
        c = boot()
        c.in_raid = False
        before = literal(c.lua.globals().RelaxRaidDB)
        self.assertFalse(c.run("local r=RelaxRaid.modules.Prep.build('join'); return RelaxRaid.modules.Store.put(r.name,r)"))
        self.assertEqual(literal(c.lua.globals().RelaxRaidDB), before)
        c.in_raid = True
        self.assertTrue(c.run("local r=RelaxRaid.modules.Prep.build('join'); return RelaxRaid.modules.Store.put(r.name,r)"))

    def test_reload_caps_oversized_player_map(self):
        c = boot()
        c.run("""
            local reports={}
            for i=1,41 do
                local r=RelaxRaid.modules.Prep.build('join'); r.name='Bram'..i..'-EU'
                reports[string.lower(r.name)]={receivedAt=5,report=r}
            end
            RelaxRaidDB.prep.nights={{day=1,windows={{openedAt=5,reports=reports}}}}
        """)
        saved = literal(c.lua.globals().RelaxRaidDB)
        c = boot({"RelaxRaidDB": saved})
        self.assertEqual(c.run("local n=0; for _ in pairs(RelaxRaidDB.prep.nights[1].windows[1].reports) do n=n+1 end; return n"), 40)

    def test_full_store_serialized_bound_and_reload(self):
        c = boot()
        c.lua.globals().END_WINDOW = lambda: c.fire("ENCOUNTER_END")
        c.run("""
            local S,P=RelaxRaid.modules.Store,RelaxRaid.modules.Prep
            for day=1,3 do
                GetServerTime=function() return 9999600000+day*86400 end
                for window=1,80 do
                    END_WINDOW()
                    for i=1,40 do
                        local r=P.build('encounter')
                        r.addonVersion=string.rep(string.char(92),24)
                        r.class,r.level,r.playedLevel='unreadable','unreadable','unreadable'
                        r.name='Alia'..string.rep('Z',87)..string.char(65+math.floor((i-1)/26))..string.char(65+(i-1)%26)..'-EU'
                        r.consumables={}; r.gear={}; r.notScored={}
                        for j=1,32 do r.consumables[j]={category='enhancement',itemID=999999999-j,count=999999} end
                        for j=1,19 do r.gear[j]={slot=j,wornID=999999998,bagID=999999999}; r.notScored[j]=j end
                        assert(S.put(r.name,r))
                    end
                end
            end
        """)
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(c.run("local count=0; for _,n in ipairs(RelaxRaidDB.prep.nights) do for _,w in ipairs(n.windows) do for _ in pairs(w.reports) do count=count+1 end end end; return count"), 9600)
        self.assertLess(len(raw.encode()), 50*1024*1024)
        start=time.perf_counter()
        second=boot({"RelaxRaidDB":raw})
        elapsed=time.perf_counter()-start
        self.assertEqual(literal(second.lua.globals().RelaxRaidDB),raw)
        print("STORE_BOUND records=9600 bytes=%d reload_seconds=%.3f" % (len(raw.encode()),elapsed))


if __name__ == "__main__":
    unittest.main()
