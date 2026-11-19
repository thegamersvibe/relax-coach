"""Logger prep store; invented fixtures only."""
from pathlib import Path
from functools import lru_cache
import sys
import unittest
ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "test/raid/_client"))
from client import Client
from serialize import literal
from prep_store import report_at
ADDON = ROOT / "addon/RelaxRaid"


def boot(saved=None):
    c = Client()
    c.guild = ("Fake Guild", "Rank F", 5)
    c.load_addon(str(ADDON), saved)
    c.in_raid = True
    c.enter("raid", 901)
    return c


def large_report(c):
    return c.run("""
        local r=RelaxRaid.modules.Prep.build('encounter')
        r.addonVersion=string.rep(string.char(92),24)
        r.class,r.level,r.playedLevel='unreadable','unreadable','unreadable'
        r.consumables={}; r.gear={}; r.notScored={}
        for j=1,32 do r.consumables[j]={category='enhancement',itemID=999999999-j,count=999999} end
        for j=1,19 do r.gear[j]={slot=j,wornID=999999998,bagID=999999999}; r.notScored[j]=j end
        return r
    """)


@lru_cache(maxsize=1)
def exact_forty_save():
    """Real puts: 40 characters x 80 distinct 1250-byte bodies, no cap overrides."""
    c = boot()
    c.lua.globals().R = large_report(c)
    c.lua.globals().END_WINDOW = lambda: c.fire("ENCOUNTER_END")
    c.run("""
        local S,W=RelaxRaid.modules.Store,RelaxRaid.modules.Wire
        R.name='Alia00-EU'
        while #W.encode(R)>1250 do table.remove(R.consumables) end
        local padding=string.rep('Z',1250-#W.encode(R))
        for revision=1,80 do
            END_WINDOW()
            R.consumables[1].count=999900+revision
            for i=1,40 do
                R.name=string.format('Alia%02d',i)..padding..'-EU'
                assert(S.put(R.name,R))
            end
        end
    """)
    return literal(c.lua.globals().RelaxRaidDB)


def night_bytes(night):
    return sum(len(body.encode()) for bodies in night["reports"].values() for body in bodies.values())


class StoreTests(unittest.TestCase):
    def test_night_budget_largest_holder_oldest_body_and_remapping(self):
        c = boot()
        store = c.lua.globals().RelaxRaid.modules.Store
        encode = c.lua.globals().RelaxRaid.modules.Wire.encode
        older = c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.name='Alia-EU'; return r")
        holder = large_report(c)
        holder["name"] = "Bram-EU"
        newest = c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.name='Cato-EU'; return r")
        # The oldest window/character has MORE bodies, but far fewer bytes.
        for revision in range(3):
            older["level"] = 58 + revision
            self.assertTrue(store.put(older["name"], older))
        c.fire("ENCOUNTER_END")
        holder["consumables"][1]["count"] = 999901
        self.assertTrue(store.put(holder["name"], holder))
        c.fire("ENCOUNTER_END")
        self.assertTrue(store.put(holder["name"], holder))
        self.assertTrue(store.put(older["name"], older))
        c.fire("ENCOUNTER_END")
        holder["consumables"][1]["count"] = 999902
        self.assertTrue(store.put(holder["name"], holder))
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        old_bodies = literal(night["reports"]["alia-eu"])
        old_row = literal(night["windows"][3]["reports"]["alia-eu"])
        before = night_bytes(night)
        store.MAX_NIGHT_BYTES = before
        c.fire("ENCOUNTER_END")
        self.assertTrue(store.put(newest["name"], newest))
        self.assertLessEqual(night_bytes(night), before)
        self.assertEqual(night_bytes(night), before - len(encode(holder).encode()) + len(encode(newest).encode()))
        self.assertEqual(literal(night["reports"]["alia-eu"]), old_bodies)
        self.assertEqual(len(night["reports"]["bram-eu"]), 1)
        self.assertEqual(night["reports"]["bram-eu"][1], encode(holder))
        self.assertEqual(night["reports"]["cato-eu"][1], encode(newest))
        # The holder-only old window is pruned; its shared window keeps Alia.
        self.assertEqual(len(night["windows"]), 4)
        self.assertEqual(literal(night["windows"][2]["reports"]["alia-eu"]), old_row)
        self.assertIsNone(night["windows"][2]["reports"]["bram-eu"])
        self.assertEqual(night["windows"][3]["reports"]["bram-eu"]["report"], 1)
        self.assertEqual(report_at(c, "bram-eu", window=3)["consumables"][1]["count"], 999902)
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": raw}).lua.globals().RelaxRaidDB), raw)

    def test_night_budget_repeats_and_reselects_largest(self):
        c = boot()
        store = c.lua.globals().RelaxRaid.modules.Store
        encode = c.lua.globals().RelaxRaid.modules.Wire.encode
        a = c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.name='Alia-EU'; return r")
        b = c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.name='Bram-EU'; return r")
        a["consumables"] = c.run("return {{category='enhancement',itemID=999999999,count=999999}}")
        self.assertTrue(store.put(a["name"], a))
        a["consumables"] = c.lua.table()
        for revision in range(3):
            c.fire("ENCOUNTER_END")
            a["level"] = 57 + revision
            self.assertTrue(store.put(a["name"], a))
        for revision in range(4):
            c.fire("ENCOUNTER_END")
            b["level"] = 56 + revision
            self.assertTrue(store.put(b["name"], b))
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        before = night_bytes(night)
        store.MAX_NIGHT_BYTES = before
        incoming = c.run("""
            local r=RelaxRaid.modules.Prep.build('ready'); r.name='Cato-EU'
            r.consumables={}
            for i=1,3 do r.consumables[i]={category='enhancement',itemID=999999999-i,count=999999} end
            return r
        """)
        # Independent small reference model, using actual canonical byte lengths.
        expected = {key: list(bodies.values()) for key, bodies in night["reports"].items()}
        expected["cato-eu"] = [encode(incoming)]
        victims = []
        while sum(len(body.encode()) for bodies in expected.values() for body in bodies) > before:
            victim = min(expected, key=lambda key: (-sum(len(body.encode()) for body in expected[key]), key))
            victims.append(victim)
            expected[victim].pop(0)
            if not expected[victim]:
                del expected[victim]
        self.assertGreater(len(victims), 1)
        self.assertEqual(victims[:2], ["alia-eu", "bram-eu"])
        c.fire("ENCOUNTER_END")
        self.assertTrue(store.put(incoming["name"], incoming))
        self.assertLessEqual(night_bytes(night), before)
        self.assertEqual({key: list(bodies.values()) for key, bodies in night["reports"].items()}, expected)
        self.assertEqual(len(night["windows"]), sum(len(bodies) for bodies in expected.values()))
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": raw}).lua.globals().RelaxRaidDB), raw)

    def test_night_budget_ties_use_lowest_name_key(self):
        for cohort in (("Alia-EU", "Bram-EU", "Zora-EU"), ("Alia-EU", "Dara-EU", "Zora-EU"),
                       ("Bram-EU", "Dara-EU", "Zora-EU")):
            saved = []
            for names in (cohort, tuple(reversed(cohort))):
                c = boot()
                store = c.lua.globals().RelaxRaid.modules.Store
                for level in (59, 60):
                    c.fire("ENCOUNTER_END")
                    for name in names:
                        r = c.run("return RelaxRaid.modules.Prep.build('ready')")
                        r["name"], r["level"] = name, level
                        self.assertTrue(store.put(name, r))
                night = c.run("return RelaxRaidDB.prep.nights[1]")
                self.assertEqual(len({sum(len(b.encode()) for b in bodies.values()) for bodies in night["reports"].values()}), 1)
                store.MAX_NIGHT_BYTES = night_bytes(night)
                c.fire("ENCOUNTER_END")
                self.assertTrue(c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.name='Enid-EU'; return RelaxRaid.modules.Store.put(r.name,r)"))
                victim = min(name.lower() for name in names)
                for name in names:
                    self.assertEqual(len(night["reports"][name.lower()]), 1 if name.lower() == victim else 2, names)
                self.assertIsNone(night["windows"][1]["reports"][victim])
                self.assertEqual(night["windows"][2]["reports"][victim]["report"], 1)
                saved.append(literal(c.lua.globals().RelaxRaidDB))
                self.assertEqual(literal(boot({"RelaxRaidDB": saved[-1]}).lua.globals().RelaxRaidDB), saved[-1])
            self.assertEqual(*saved)

    def test_night_budget_reload_enforced(self):
        c = boot({"RelaxRaidDB": exact_forty_save()})
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        keys = sorted(night["reports"].keys())
        newcomer = c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.name='Zora-EU'; return r")
        body = c.lua.globals().RelaxRaid.modules.Wire.encode(newcomer)
        night["reports"]["zora-eu"] = c.lua.table_from([body])
        rows = night["windows"][80]["reports"]
        rows[keys[-1]] = None  # Still referenced by the other 79 windows.
        rows["zora-eu"] = c.lua.table_from({"receivedAt": c.wall, "report": 1})
        self.assertGreater(night_bytes(night), 4000000)
        reloaded = boot({"RelaxRaidDB": literal(c.lua.globals().RelaxRaidDB)})
        night = reloaded.run("return RelaxRaidDB.prep.nights[1]")
        self.assertLessEqual(night_bytes(night), 4000000)
        self.assertEqual(night_bytes(night), 4000000 - 1250 + len(body.encode()))
        self.assertEqual(len(night["reports"][keys[0]]), 79)
        self.assertIsNone(night["windows"][1]["reports"][keys[0]])
        self.assertEqual(night["windows"][2]["reports"][keys[0]]["report"], 1)
        self.assertEqual(night["reports"]["zora-eu"][1], body)
        raw = literal(reloaded.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": raw}).lua.globals().RelaxRaidDB), raw)

    def test_night_budget_exact_forty_players_evicts_nothing(self):
        raw = exact_forty_save()
        c = boot({"RelaxRaidDB": raw})
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        self.assertEqual(night_bytes(night), 4000000)
        self.assertEqual(len(list(night["reports"].keys())), 40)
        self.assertEqual(len(night["windows"]), 80)
        for bodies in night["reports"].values():
            self.assertEqual(len(bodies), 80)
            self.assertEqual(set(bodies.keys()), set(range(1, 81)))
            self.assertEqual({len(body.encode()) for body in bodies.values()}, {1250})
            self.assertEqual(sum(len(body.encode()) for body in bodies.values()), 100000)
        for i, window in night["windows"].items():
            self.assertEqual(len(list(window["reports"].keys())), 40)
            self.assertEqual({row["report"] for row in window["reports"].values()}, {i})
        self.assertEqual(literal(c.lua.globals().RelaxRaidDB), raw)

    def test_night_budget_late_substitute_is_kept(self):
        c = boot({"RelaxRaidDB": exact_forty_save()})
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        keys = sorted(night["reports"].keys())
        # Omit the final receipt from the synthetic full-raid fixture: 39 full
        # histories and one 79-body history, with room for the late substitute.
        night["reports"][keys[-1]][80] = None
        night["windows"][80]["reports"][keys[-1]] = None
        c = boot({"RelaxRaidDB": literal(c.lua.globals().RelaxRaidDB)})
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        self.assertEqual(night_bytes(night), 3998750)
        before = {key: list(bodies.values()) for key, bodies in night["reports"].items()}
        late = large_report(c)
        late["name"] = "Zora-Late-PvP-EU"
        body = c.lua.globals().RelaxRaid.modules.Wire.encode(late)
        self.assertGreater(night_bytes(night) + len(body.encode()), 4000000)
        self.assertTrue(c.lua.globals().RelaxRaid.modules.Store.put(late["name"], late))
        self.assertLessEqual(night_bytes(night), 4000000)
        self.assertIsNotNone(night["reports"]["zora-late-pvp-eu"])
        self.assertEqual(list(night["reports"]["zora-late-pvp-eu"].values()), [body])
        self.assertEqual(night["windows"][80]["reports"]["zora-late-pvp-eu"]["report"], 1)
        for key, bodies in before.items():
            self.assertEqual(list(night["reports"][key].values()), bodies[1:] if key == keys[0] else bodies)
        self.assertIsNone(night["windows"][1]["reports"][keys[0]])
        for index in range(2, 81):
            self.assertEqual(night["windows"][index]["reports"][keys[0]]["report"], index - 1)
        self.assertEqual(night_bytes(night), 3998750 - 1250 + len(body.encode()))
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": raw}).lua.globals().RelaxRaidDB), raw)

    def test_night_budget_put_does_not_rescan_all_holders(self):
        c = boot()
        c.lua.globals().END_WINDOW = lambda: c.fire("ENCOUNTER_END")
        c.run("""
            for window=1,3 do
                END_WINDOW()
                for i=1,20 do
                    local r=RelaxRaid.modules.Prep.build('ready')
                    r.name='Alia'..((window-1)*20+i)..'-EU'
                    assert(RelaxRaid.modules.Store.put(r.name,r))
                end
            end
            END_WINDOW()
            local target=RelaxRaidDB.prep.nights[1].reports
            local original=pairs
            HOLDER_WALKS=0
            pairs=function(t)
                if t==target then HOLDER_WALKS=HOLDER_WALKS+1 end
                return original(t)
            end
            for i=61,80 do
                local r=RelaxRaid.modules.Prep.build('ready'); r.name='Alia'..i..'-EU'
                assert(RelaxRaid.modules.Store.put(r.name,r))
            end
            pairs=original
        """)
        self.assertLessEqual(c.run("return HOLDER_WALKS"), 1)
        self.assertEqual(len(list(c.run("return RelaxRaidDB.prep.nights[1].reports").keys())), 80)

    def test_night_budget_accounting_follows_other_evictions(self):
        c = boot()
        store = c.lua.globals().RelaxRaid.modules.Store
        r = c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.name='Alia-EU'; return r")
        size = len(c.lua.globals().RelaxRaid.modules.Wire.encode(r).encode())
        store.MAX_BYTES, store.MAX_NIGHT_BYTES = size * 2, size * 4
        for level in range(51, 61):
            r["level"] = level
            self.assertTrue(store.put(r["name"], r))
        self.assertEqual(night_bytes(c.run("return RelaxRaidDB.prep.nights[1]")), size * 2)
        # Player-cap pruning releases Alia's whole body history from accounting.
        store.MAX_PLAYERS, store.MAX_WINDOWS = 1, 2
        for name in ("Bram-EU", "Cato-EU", "Dara-EU"):
            r["name"] = name
            self.assertTrue(store.put(name, r))
            self.assertEqual(night_bytes(c.run("return RelaxRaidDB.prep.nights[1]")), size)
        # Window-cap pruning also releases the final reference and history.
        for name in ("Enid-EU", "Fara-EU", "Gala-EU", "Hana-EU"):
            c.fire("ENCOUNTER_END")
            r["name"] = name
            self.assertTrue(store.put(name, r))
            night = c.run("return RelaxRaidDB.prep.nights[1]")
            self.assertEqual(night_bytes(night), size * 2)
            self.assertEqual(len(night["windows"]), 2)
        self.assertIsNone(store.stats.nightBudgetEvicted)
        store.MAX_PLAYERS, store.MAX_WINDOWS = 40, 80
        for name in ("Ilya-EU", "Jora-EU", "Kara-EU"):
            r["name"] = name
            self.assertTrue(store.put(name, r))
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        self.assertEqual(night_bytes(night), size * 4)
        self.assertIsNone(night["reports"]["gala-eu"])
        self.assertIsNotNone(night["reports"]["kara-eu"])
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": raw}).lua.globals().RelaxRaidDB), raw)

    def test_budget_evicts_only_senders_oldest(self):
        c = boot()
        r = large_report(c)
        other = c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.name='Bram-EU'; return r")
        store = c.lua.globals().RelaxRaid.modules.Store
        encode = c.lua.globals().RelaxRaid.modules.Wire.encode
        sizes = []
        for i in range(80):
            c.fire("ENCOUNTER_END")
            r["consumables"][1]["count"] = 999900 + i
            sizes.append(len(encode(r).encode()))
            self.assertTrue(store.put(r["name"], r))
            self.assertTrue(store.put(other["name"], other))
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        bodies = night["reports"]["alia-stone-pvp-eu"]
        self.assertLessEqual(sum(len(body.encode()) for body in bodies.values()), 100000)
        dropped = 0
        while sum(sizes[dropped:]) > 100000:
            dropped += 1
        self.assertGreater(dropped, 0)
        self.assertEqual(len(bodies), 80 - dropped)
        self.assertEqual(len(night["windows"]), 80)
        self.assertEqual(len(night["reports"]["bram-eu"]), 1)
        self.assertEqual(night["reports"]["bram-eu"][1], encode(other))
        for i in range(1, 81):
            rows = night["windows"][i]["reports"]
            self.assertEqual(rows["bram-eu"]["report"], 1)
            if i <= dropped:
                self.assertIsNone(rows["alia-stone-pvp-eu"])
            else:
                self.assertEqual(rows["alia-stone-pvp-eu"]["report"], i - dropped)
                self.assertEqual(report_at(c, window=i)["consumables"][1]["count"], 999899 + i)
        saved = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": saved}).lua.globals().RelaxRaidDB), saved)

    def test_eviction_prunes_empty_windows(self):
        c = boot()
        r = large_report(c)
        store = c.lua.globals().RelaxRaid.modules.Store
        for i in range(80):
            c.fire("ENCOUNTER_END")
            r["consumables"][1]["count"] = 999900 + i
            self.assertTrue(store.put(r["name"], r))
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        bodies = night["reports"]["alia-stone-pvp-eu"]
        self.assertLess(len(bodies), 80)
        self.assertEqual(len(night["windows"]), len(bodies))
        for i in range(1, len(bodies) + 1):
            self.assertEqual(night["windows"][i]["reports"]["alia-stone-pvp-eu"]["report"], i)
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": raw}).lua.globals().RelaxRaidDB), raw)

    def test_reload_enforces_body_budget_and_remaps(self):
        c = boot()
        c.lua.globals().R = large_report(c)
        c.run("""
            local bodies, windows={}, {}
            for i=1,80 do
                R.consumables[1].count=999900+i
                bodies[i]=RelaxRaid.modules.Wire.encode(R)
                windows[i]={openedAt=i,reports={['alia-stone-pvp-eu']={receivedAt=i,report=i}}}
            end
            RelaxRaidDB.prep.nights={{day=1,reports={['alia-stone-pvp-eu']=bodies},windows=windows}}
        """)
        raw = literal(c.lua.globals().RelaxRaidDB)
        c = boot({"RelaxRaidDB": raw})
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        bodies = night["reports"]["alia-stone-pvp-eu"]
        self.assertLessEqual(sum(len(b.encode()) for b in bodies.values()), 100000)
        self.assertEqual(len(bodies), 67)
        self.assertEqual(len(night["windows"]), 67)
        self.assertEqual(report_at(c)["consumables"][1]["count"], 999914)
        self.assertEqual(night["windows"][1]["openedAt"], 14)
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": raw}).lua.globals().RelaxRaidDB), raw)

    def test_exact_budget_is_allowed(self):
        c = boot()
        r = large_report(c)
        encode = c.lua.globals().RelaxRaid.modules.Wire.encode
        while len(encode(r).encode()) > 1250:
            r["consumables"][len(r["consumables"])] = None
        r["name"] = "Z" * (1250 - len(encode(r).encode())) + r["name"]
        self.assertEqual(len(encode(r).encode()), 1250)
        for i in range(80):
            c.fire("ENCOUNTER_END")
            r["consumables"][1]["count"] = 999900 + i
            self.assertTrue(c.lua.globals().RelaxRaid.modules.Store.put(r["name"], r))
        bodies = c.run("return RelaxRaidDB.prep.nights[1].reports")[r["name"].lower()]
        self.assertEqual(len(bodies), 80)
        self.assertEqual(sum(len(b.encode()) for b in bodies.values()), 100000)
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": raw}).lua.globals().RelaxRaidDB), raw)

    def test_caps_do_not_leave_orphan_character_bodies(self):
        c = boot()
        c.run("local r=RelaxRaid.modules.Prep.build('ready'); assert(RelaxRaid.modules.Store.put(r.name,r))")
        c.fire("ENCOUNTER_END")
        c.run("""
            for i=1,41 do
                local r=RelaxRaid.modules.Prep.build('ready'); r.name='Bram'..i..'-EU'
                assert(RelaxRaid.modules.Store.put(r.name,r))
            end
        """)
        self.assertEqual(c.run("local n=0; for _ in pairs(RelaxRaidDB.prep.nights[1].reports) do n=n+1 end; return n"), 41)
        self.assertEqual(report_at(c)["name"], "Alia-Stone-PvP-EU")
        for _ in range(80):
            c.fire("ENCOUNTER_END")
            c.run("local r=RelaxRaid.modules.Prep.build('ready'); r.name='Cato-EU'; assert(RelaxRaid.modules.Store.put(r.name,r))")
        self.assertEqual(c.run("local n=0; for _ in pairs(RelaxRaidDB.prep.nights[1].reports) do n=n+1 end; return n"), 1)
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": raw}).lua.globals().RelaxRaidDB), raw)

    def test_version_one_store_is_discarded_not_migrated(self):
        c = boot()
        r = c.run("return RelaxRaid.modules.Prep.build('ready')")
        old = "{version=1,unrelated='keep',prep={version=1,nights={{day=1,windows={{openedAt=5,reports={['alia-stone-pvp-eu']={receivedAt=6,report=" + literal(r) + "}}}}}}}}"
        c = boot({"RelaxRaidDB": old})
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
        self.assertEqual(c.run("return RelaxRaidDB.prep.version"), 2)
        self.assertEqual(c.run("return RelaxRaidDB.version"), 1)
        self.assertEqual(c.run("return RelaxRaidDB.unrelated"), "keep")
        self.assertTrue(c.run("local r=RelaxRaid.modules.Prep.build('join'); return RelaxRaid.modules.Store.put(r.name,r)"))
        self.assertIsNone(c.run("return next(RelaxRaid.errors)"))
        # A v1 marker must discard even otherwise-valid v2-shaped content: the
        # version gate is independent of the old layout failing validation.
        c.run("RelaxRaidDB.prep.version=1")
        c = boot({"RelaxRaidDB": literal(c.lua.globals().RelaxRaidDB)})
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)

    def test_unchanged_eighty_windows_share_one_body(self):
        c = boot()
        c.lua.globals().END_WINDOW = lambda: c.fire("ENCOUNTER_END")
        c.run("""
            local r=RelaxRaid.modules.Prep.build('ready')
            for i=1,80 do
                END_WINDOW()
                GetServerTime=function() return 1800000000+i end
                assert(RelaxRaid.modules.Store.put(r.name,r))
            end
        """)
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        self.assertIsNotNone(night["reports"], "per-character report bodies missing")
        bodies = night["reports"]["alia-stone-pvp-eu"]
        self.assertEqual(len(bodies), 1)
        self.assertEqual(len(night["windows"]), 80)
        self.assertIsInstance(bodies[1], str)
        for i in range(1, 81):
            row = night["windows"][i]["reports"]["alia-stone-pvp-eu"]
            self.assertEqual(row["report"], 1)
            self.assertEqual(row["receivedAt"], c.wall + i)
            self.assertEqual(sorted(row.keys()), ["receivedAt", "report"])
        raw = literal(c.lua.globals().RelaxRaidDB)
        self.assertEqual(literal(boot({"RelaxRaidDB": raw}).lua.globals().RelaxRaidDB), raw)

    def test_sent_at_does_not_add_body(self):
        c = boot()
        c.run("""
            R=RelaxRaid.modules.Prep.build('ready'); R.sentAt=1
            assert(RelaxRaid.modules.Store.put(R.name,R))
        """)
        c.fire("ENCOUNTER_END")
        c.run("GetServerTime=function() return 1800000007 end; R.sentAt=9999999999; assert(RelaxRaid.modules.Store.put(R.name,R))")
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights[1].reports['alia-stone-pvp-eu']"), 1)
        self.assertEqual(report_at(c, window=2)["sentAt"], 1)
        row = c.run("return RelaxRaidDB.prep.nights[1].windows[2].reports['alia-stone-pvp-eu']")
        self.assertEqual(row["receivedAt"], 1800000007)
        self.assertEqual(row["report"], 1)

    def test_only_latest_body_deduplicates(self):
        c = boot()
        c.run("""
            R=RelaxRaid.modules.Prep.build('ready')
            R.consumables={{category='flask',itemID=901,count=1}}
            assert(RelaxRaid.modules.Store.put(R.name,R))
        """)
        for count in (2, 1):
            c.fire("ENCOUNTER_END")
            c.run(f"R.consumables[1].count={count}; assert(RelaxRaid.modules.Store.put(R.name,R))")
        night = c.run("return RelaxRaidDB.prep.nights[1]")
        self.assertEqual(len(night["reports"]["alia-stone-pvp-eu"]), 3)
        for i, count in enumerate((1, 2, 1), 1):
            self.assertEqual(night["windows"][i]["reports"]["alia-stone-pvp-eu"]["report"], i)
            self.assertEqual(report_at(c, window=i)["consumables"][1]["count"], count)
        self.assertEqual(night["reports"]["alia-stone-pvp-eu"][1], night["reports"]["alia-stone-pvp-eu"][3])

    def test_equality_uses_all_validated_fields(self):
        changes = ("r.addonVersion='other'", "r.name='ALia-Stone-PvP-EU'", "r.class='MAGE'",
                   "r.level=59", "r.playedLevel=1", "r.reason='ready'",
                   "r.consumables[1].count=1", "r.consumables[1].itemID=901",
                   "r.consumables[1].category='flask'", "r.consumables='unreadable'",
                   "r.gear[1].slot=2", "r.gear[1].wornID=0", "r.gear[1].bagID=901",
                   "r.notScored={1}", "r.gear='not checked'; r.notScored=nil")
        for change in changes:
            c = boot()
            c.lua.globals().r = large_report(c)
            c.run("assert(RelaxRaid.modules.Store.put(r.name,r))")
            c.run(change + "; assert(RelaxRaid.modules.Store.put(r.name,r))")
            self.assertEqual(c.run("return #RelaxRaidDB.prep.nights[1].reports['alia-stone-pvp-eu']"), 2, change)
        c = boot()
        c.run("r=RelaxRaid.modules.Prep.build('ready'); assert(RelaxRaid.modules.Store.put(r.name,r)); r.ignored='not validated'; assert(RelaxRaid.modules.Store.put(r.name,r))")
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights[1].reports['alia-stone-pvp-eu']"), 1)
        self.assertIsNone(report_at(c)["ignored"])

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
        self.assertEqual(report_at(c)["sentAt"], 1)
        self.assertEqual(report_at(c)["reason"], "ready")
        self.assertEqual(c.run("local n=0; for _ in pairs(RelaxRaidDB.prep.nights[1].windows[1].reports) do n=n+1 end; return n"), 1)
        c.fire("ENCOUNTER_END")
        c.run("local r=RelaxRaid.modules.Prep.build('encounter'); RelaxRaid.modules.Store.put(r.name,r)")
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights[1].windows"), 2)

    def test_newer_save_is_byte_identical(self):
        for saved in ({"RelaxRaidDB": "{version=2,future={opaque='keep'}}"},
                      {"RelaxRaidDB": "{version=1,prep={version=3,nights={future='keep'}}}"},
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

    def test_writers_recheck_live_versions(self):
        for change in ("RelaxRaidDB.version=2", "RelaxRaidDB.prep.version=3", "RelaxRaidCharDB={version=2}"):
            c = boot()
            c.run(change)
            before = {name: literal(c.lua.globals()[name]) for name in ("RelaxRaidDB", "RelaxRaidCharDB")}
            c.fire("ENCOUNTER_END")
            self.assertFalse(c.run("local r=RelaxRaid.modules.Prep.build('ready'); return RelaxRaid.modules.Store.put(r.name,r)"))
            self.assertEqual({name: literal(c.lua.globals()[name]) for name in before}, before)

    def test_reload_rejects_invalid_references_and_bodies(self):
        for change in ("n.windows[1].reports[key].report=0", "n.windows[1].reports[key].report=2",
                       "n.windows[1].reports[key].report='1'", "n.windows[1].reports[key].report=1.5",
                       "n.reports[key][1]='invalid'", "n.reports[key][1]=17",
                       "n.reports[key][1]=n.reports[key][1]:gsub('Alia','Bram')",
                       "n.reports[key]={[2]=n.reports[key][1]}"):
            c = boot()
            c.run("local r=RelaxRaid.modules.Prep.build('ready'); assert(RelaxRaid.modules.Store.put(r.name,r))")
            c.run("local n=RelaxRaidDB.prep.nights[1]; local key='alia-stone-pvp-eu'; " + change)
            c = boot({"RelaxRaidDB": literal(c.lua.globals().RelaxRaidDB)})
            self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0, change)
            self.assertIsNone(c.run("return next(RelaxRaid.errors)"), change)

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
        c = boot({"RelaxRaidDB": "{version=1,prep={version=2,nights={{day=1,reports={},windows={{openedAt=5,reports={forged={receivedAt=6,report=1}}}}}}}}"})
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
        self.assertEqual(report_at(c)["name"], "Alia-Stone-PvP-EU")

    def test_not_scored_validated_copied_and_reloaded(self):
        c = boot()
        self.assertTrue(c.run("""
            local r=RelaxRaid.modules.Prep.build('ready')
            r.gear={}; r.notScored={1,19}
            local ok=RelaxRaid.modules.Store.put(r.name,r)
            r.notScored[1]=2
            return ok
        """))
        row = report_at(c)
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
        c.run("""
            local n=RelaxRaidDB.prep.nights[1]
            local bodies=n.reports['alia-stone-pvp-eu']
            bodies[1]=bodies[1]:gsub('1,19$', '0')
        """)
        corrupt = literal(c.lua.globals().RelaxRaidDB)
        self.assertNotEqual(corrupt, saved)
        self.assertEqual(boot({"RelaxRaidDB": corrupt}).run("return #RelaxRaidDB.prep.nights"), 0)

    def test_read_only_latch_survives_external_version_reset(self):
        c = boot({"RelaxRaidDB": "{version=2,future='opaque'}"})
        c.run("RelaxRaidDB={version=1,prep={version=2,nights={}}}")
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
            local reports, bodies={}, {}
            for i=1,41 do
                local r=RelaxRaid.modules.Prep.build('join'); r.name='Bram'..i..'-EU'
                reports[string.lower(r.name)]={receivedAt=5,report=1}
                bodies[string.lower(r.name)]={RelaxRaid.modules.Wire.encode(r)}
            end
            RelaxRaidDB.prep.nights={{day=1,reports=bodies,windows={{openedAt=5,reports=reports}}}}
        """)
        saved = literal(c.lua.globals().RelaxRaidDB)
        c = boot({"RelaxRaidDB": saved})
        self.assertEqual(c.run("local n=0; for _ in pairs(RelaxRaidDB.prep.nights[1].windows[1].reports) do n=n+1 end; return n"), 40)

if __name__ == "__main__":
    unittest.main()
