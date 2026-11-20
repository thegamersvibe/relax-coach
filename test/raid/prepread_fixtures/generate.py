"""Real Prep/Wire/Store output; only mocked client inputs, invented names.
Run with --write to regenerate; default unittest compares without writing.
The v1/newer fixtures change ONLY a version tag on real v2 output, as Store's
own version-gate tests do. They are not claims of an old/future writer.
"""
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "test/raid/_client"))
from client import Client
from serialize import literal
from prep_store import assert_references

DEST = ROOT / "desktop/internal/raid/prepread/testdata"


def boot(saved=None):
    c = Client()
    c.guild = ("Invented Guild", "Rank F", 5)
    c.load_addon(str(ROOT / "addon/RelaxRaid"), saved)
    c.in_raid = True
    return c


def put(c, reason="ready"):
    return c.run("local r=RelaxRaid.modules.Prep.build('" + reason + "'); assert(RelaxRaid.modules.Store.put(r.name,r)); return r")


def saves():
    out = {}
    def save(name, c, valid=True):
        if valid:
            assert_references(unittest.TestCase(), c)
        out[name] = "-- Synthetic client output; regenerate with test/raid/prepread_fixtures/generate.py --write\nRelaxRaidDB = " + literal(c.lua.globals().RelaxRaidDB) + "\n"

    c = boot()
    c.player_name = "Élia-Stóne"
    put(c)
    save("accented", c)

    c = boot()
    c.bags = {0: [{"itemID": 101, "stackCount": 3}, {"itemID": 102, "stackCount": 5}, {"itemID": 103, "stackCount": 1}, {"itemID": 104, "stackCount": 2}]}
    c.item_types = {101: (0, 1), 102: (0, 5), 103: (0, 3), 104: (0, 2)}
    c.fire("TIME_PLAYED_MSG", 5000, 123)
    c.run("""
        ForeverCoach={Snapshot=function() return {baginfo={known=true}} end,
            Gear={Audit=function() return {status='ok',owned={{inv=2,id=202},{inv=3,id=203}},
                slots={[1]={id=201,scoreStatus='unknown'},[2]={id=200,scoreStatus='ok'},[3]={scoreStatus='empty'}}} end}}
    """)
    put(c, "join")
    save("normal", c)
    c.now = 20
    c.fire("ENCOUNTER_END")
    put(c, "join")
    save("played-refresh", c)

    c = boot()
    put(c)
    c.now = 10
    c.fire("ENCOUNTER_END")
    put(c)
    save("dedup", c)
    for count in (2, 1):
        c.now += 10
        c.fire("ENCOUNTER_END")
        c.bags = {0: [{"itemID": 101, "stackCount": count}]}
        c.item_types = {101: (0, 1)}
        put(c)
    save("changes", c)

    c = boot()
    put(c)
    c.now = 1
    c.player_name = "Bram-Oak"
    c.lua.globals().C_AddOns.GetAddOnMetadata = lambda *_: None
    c.player_class = "unreadable"
    c.level = None
    c.bags = {}
    c.run("ForeverCoach={Snapshot=function() error('unreadable') end, Gear={Audit=function() end}}")
    put(c)
    c.now = 2
    c.player_name = "Cato"
    c.level = 60
    c.player_class = "MAGE"
    c.bags = {0: [None]}
    c.run("ForeverCoach={Snapshot=function() return {baginfo={known=true}} end,Gear={Audit=function() return {status='ok',owned={},slots={}} end}}")
    c.fire("TIME_PLAYED_MSG", 0, 0)
    put(c)
    save("markers", c)

    c = boot()
    for day in range(4):
        c.wall = 1800000000 + day * 86400
        put(c)
    save("night-eviction", c)
    c = boot()
    for i in range(81):
        c.now = i
        c.fire("ENCOUNTER_END")
        put(c)
    save("window-eviction", c)
    c = boot()
    for i in range(41):
        c.now = i
        c.player_name = "Alia%02d" % i
        put(c)
    save("player-eviction", c)

    c = boot()
    for count in (1, 2, 1):
        c.fire("ENCOUNTER_END")
        c.bags = {0: [{"itemID": 101, "stackCount": count}]}
        c.item_types = {101: (0, 1)}
        put(c)
    save("aba", c)

    c = boot()
    c.run("""
        local P,S,W=RelaxRaid.modules.Prep,RelaxRaid.modules.Store,RelaxRaid.modules.Wire
        local classes={'WARRIOR','PALADIN','HUNTER','ROGUE','PRIEST','SHAMAN','MAGE','WARLOCK','DRUID'}
        for i=1,32 do
            local r=P.build('encounter')
            r.name=string.format('Alia%02d-PvP-EU',i)
            r.addonVersion=string.rep(string.char(92),24)
            r.class=classes[(i-1)%9+1]; r.level=(i-1)%60+1
            r.sentAt=9999999999; r.playedLevel=999999999
            r.consumables={}; r.gear={}; r.notScored={}
            for j=1,32 do r.consumables[j]={category=P.categories[(j-1)%16],itemID=999999999-j,count=999999} end
            for j=1,19 do
                r.gear[j]={slot=j,wornID=j==1 and 0 or 999999999,bagID=999999999}
                r.notScored[j]=20-j
            end
            local body=W.encode(r)
            assert(body and W.encode(W.decode(body))==body)
            assert(S.put(r.name,r))
        end
    """)
    save("wide", c)

    c = boot()
    put(c)
    c.run("RelaxRaidDB.prep.version=1")
    save("version-one", c, False)
    raw = literal(c.lua.globals().RelaxRaidDB)
    save("version-one-reloaded", boot({"RelaxRaidDB": raw}))
    c.run("RelaxRaidDB.prep.version=3")
    newer = boot({"RelaxRaidDB": literal(c.lua.globals().RelaxRaidDB)})
    save("newer", newer, False)
    return out


class FixtureTests(unittest.TestCase):
    def test_fixtures_are_real_addon_output(self):
        for name, raw in saves().items():
            with self.subTest(name=name):
                if name == "accented":
                    self.assertIn("Élia-Stóne", raw)
                else:
                    self.assertTrue(raw.isascii())
                self.assertEqual((DEST / (name + ".lua")).read_text(encoding="utf-8"), raw)
                # Execute only our generated fixture in the test Lua 5.1 VM.
                # The app reader never executes SavedVariables.
                reader = Client()
                self.assertEqual(reader.run("return _VERSION"), "Lua 5.1")
                reader.run(raw)
                self.assertEqual(literal(reader.lua.globals()["RelaxRaidDB"]),
                                 raw.split("RelaxRaidDB = ", 1)[1].rstrip("\n"))


if __name__ == "__main__":
    if sys.argv[1:] == ["--write"]:
        DEST.mkdir(parents=True, exist_ok=True)
        for name, raw in saves().items():
            (DEST / (name + ".lua")).write_text(raw, encoding="utf-8")
            print(name, len(raw.encode()), "bytes")
    else:
        unittest.main()
