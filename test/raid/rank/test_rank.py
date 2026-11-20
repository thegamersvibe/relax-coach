"""Account-local officer-lock evidence, through the real addon lifecycle.
Mutation targets use ordinary loops: raidtest requires a named test FAIL,
which unittest emits differently for a failing subTest.
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
sys.path.insert(0, os.path.join(ROOT, "test", "raid", "_client"))
from client import Client
from serialize import literal

ADDON = os.path.join(ROOT, "addon", "RelaxRaid")
NAME = "Alia-Stone-PvP-EU"
GUILD = "Invented Guild"
BAD_CLOCKS = ('GetServerTime=nil',) + tuple(
    'GetServerTime=function() ' + code + ' end' for code in
    ('return nil', 'return secret()', 'error("unavailable")', 'return "123"',
     'return -1', 'return 1.5', 'return 10000000000', 'return 0/0', 'return math.huge', 'return false'))


def boot(saved=None, guild=(GUILD, "Rank B", 1), loaded=True):
    c = Client()
    c.guild, c.guild_loaded = guild, loaded
    c.load_addon(ADDON, saved)
    return c


def rank_save(guild=GUILD, clockless=False):
    """Exercise actual addon writes, including prep that older readers must retain."""
    c = boot({"RelaxRaidDB": '{version=1,other={keep="unchanged"}}'}, guild=(guild, "Rank B", 1))
    c.in_raid = True
    assert c.run('return RelaxRaid.modules.Store.put("' + NAME + '", RelaxRaid.modules.Prep.build("join"))')
    if clockless:
        c.run("GetServerTime=nil")
    for name, guild in (("Bram-Oak", (GUILD, "Rank F", 5)), ("Cato", None),
                        ("Dara-Fern", (GUILD, "Rank Unknown", None))):
        c.player_name, c.guild = name, guild
        c.now += 1
        c.fire("GUILD_ROSTER_UPDATE", True)
    return "-- Synthetic output from test/raid/rank/test_rank.py:rank_save\nRelaxRaidDB = " + literal(c.lua.globals().RelaxRaidDB) + "\n"


class RankTests(unittest.TestCase):
    def test_rank_reader_checks_secrets_before_nil_comparisons(self):
        with open(os.path.join(ADDON, "Log.lua"), encoding="utf-8") as f:
            source = f.read()
        # Lua 5.1's synthetic secret cannot trap equality with nil; pin this
        # native-secret ordering separately from the behavioral secret tests.
        self.assertIn('local indexOk = not isSecret(index) and index ~= nil', source)
        self.assertIn('local nameOk = not isSecret(rankName) and rankName ~= nil', source)

    def test_rank_capture_without_optional_modules_is_inert(self):
        for files in (("Core.lua", "Store.lua"), ("Core.lua", "Prep.lua", "Store.lua")):
            c = Client()
            c.load_addon(ADDON, files=files)
            c.fire("PLAYER_GUILD_UPDATE", "player")
            c.fire("GUILD_ROSTER_UPDATE", True)
            c.enter(initial=True)
            self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})
            self.assertIsNone(c.lua.globals().RelaxRaidDB["ranks"])

    def test_compatibility_fixture_is_real_writer_output(self):
        with open(os.path.join(HERE, "save.lua"), encoding="utf-8") as f:
            self.assertEqual(f.read(), rank_save())

    def test_omitted_guild_fixture_is_real_writer_output(self):
        with open(os.path.join(HERE, "save-omitted-guild.lua"), encoding="utf-8") as f:
            expected = f.read()
        for guild in ("G" * 4096, "G" * 4097, "Invented\nGuild"):
            self.assertEqual(expected, rank_save(guild, clockless=True))

    def row(self, c, name=NAME):
        ranks = c.lua.globals().RelaxRaidDB["ranks"]
        self.assertIsNotNone(ranks, "account rank table missing")
        row = ranks[name]
        self.assertIsNotNone(row, "full-name rank entry missing")
        return dict(row.items())

    def test_login_saves_officer_guild_and_server_time(self):
        c = boot()
        self.assertEqual(self.row(c), {"result": "officer", "guild": GUILD, "readAt": c.wall})
        self.assertEqual(c.lua.globals().RelaxRaidDB["version"], 1)
        self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})

    def test_pending_preserves_previous_entry_even_after_logger_timeout(self):
        saved = {"RelaxRaidDB": '{version=1,ranks={["' + NAME + '"]={result="officer",guild="Earlier Guild",readAt=123}}}'}
        c = boot(saved, loaded=False)
        old = {"result": "officer", "guild": "Earlier Guild", "readAt": 123}
        self.assertEqual(self.row(c), old)
        c.advance(20)  # Log settles its own pending state to unreadable; never save that substitute.
        c.fire("GUILD_ROSTER_UPDATE")
        c.fire("PLAYER_GUILD_UPDATE", "player")
        c.enter()
        self.assertEqual(self.row(c), old)
        fresh = boot(loaded=False)
        self.assertIsNone(fresh.lua.globals().RelaxRaidDB["ranks"])

    def test_events_refresh_rank_and_time_without_requiring_logger(self):
        for event, args in (("PLAYER_ENTERING_WORLD", (True, False)),
                            ("GUILD_ROSTER_UPDATE", ()), ("PLAYER_GUILD_UPDATE", ("player",))):
            c = boot({"RelaxRaidCharDB": "{version=1,logger=false}"})
            c.guild = (GUILD, "Rank F", 5)
            c.advance(8)
            c.fire(event, *args)
            self.assertEqual(self.row(c), {"result": "member", "guild": GUILD, "readAt": c.wall + 8})
            c.guild = None
            c.advance(1)
            c.fire(event, *args)
            self.assertEqual(self.row(c), {"result": "noguild", "readAt": c.wall + 9})
            c.guild = (GUILD, "Rank Unknown", None)
            c.advance(1)
            c.fire(event, *args)
            self.assertEqual(self.row(c), {"result": "unreadable", "guild": GUILD, "readAt": c.wall + 10})
            c.guild = (GUILD, "Rank A", 0)
            c.fire(event, *args)
            self.assertEqual(self.row(c)["result"], "officer")
            self.assertFalse(c.lua.globals().RelaxRaidCharDB["logger"])

    def test_each_settled_result_at_login(self):
        for guild, result in (((GUILD, "Rank A", 0), "officer"), ((GUILD, "Rank F", 5), "member"),
                              (None, "noguild"), ((GUILD, "Rank Unknown", None), "unreadable")):
            c = boot(guild=guild)
            expected = {"result": result, "readAt": c.wall}
            if guild is not None:
                expected["guild"] = GUILD
            self.assertEqual(self.row(c), expected)

    def test_full_names_and_other_characters_survive_reload(self):
        c = boot()
        first = self.row(c)
        saved = {"RelaxRaidDB": literal(c.lua.globals().RelaxRaidDB)}
        other = Client()
        other.guild = (GUILD, "Rank F", 5)
        other.realm = "Other-Realm"
        other.load_addon(ADDON, saved)
        self.assertEqual(self.row(other), first)
        self.assertEqual(self.row(other, "Alia-Stone-Other-Realm")["result"], "member")
        self.assertEqual(set(other.lua.globals().RelaxRaidDB["ranks"].keys()), {NAME, "Alia-Stone-Other-Realm"})
        reloaded = boot({"RelaxRaidDB": literal(other.lua.globals().RelaxRaidDB)}, loaded=False)
        self.assertEqual(literal(reloaded.lua.globals().RelaxRaidDB), literal(other.lua.globals().RelaxRaidDB))

    def test_accounts_are_independent(self):
        officer, member = boot(), boot(guild=(GUILD, "Rank F", 5))
        self.assertEqual(self.row(officer)["result"], "officer")
        self.assertEqual(self.row(member)["result"], "member")
        member.guild = None
        member.fire("GUILD_ROSTER_UPDATE")
        self.assertEqual(self.row(member)["result"], "noguild")
        self.assertEqual(self.row(officer)["result"], "officer")

    def test_existing_account_data_and_prep_are_untouched(self):
        c = boot({"RelaxRaidDB": '{version=1,other={keep="unchanged"}}',
                  "RelaxRaidCharDB": '{version=1,logger=true,other="unchanged"}'})
        c.in_raid = True
        self.assertTrue(c.run('return RelaxRaid.modules.Store.put("' + NAME + '", RelaxRaid.modules.Prep.build("join"))'))
        c.run('savedDB=RelaxRaidDB; savedPrep=RelaxRaidDB.prep; savedRanks=RelaxRaidDB.ranks')
        before = literal(c.lua.globals().RelaxRaidDB["prep"])
        c.guild = ("Another Invented Guild", "Rank F", 5)
        c.advance(3)
        c.fire("GUILD_ROSTER_UPDATE")
        self.assertEqual(literal(c.lua.globals().RelaxRaidDB["prep"]), before)
        self.assertTrue(c.run('return savedDB==RelaxRaidDB and savedPrep==RelaxRaidDB.prep and savedRanks==RelaxRaidDB.ranks'))
        self.assertEqual(c.lua.globals().RelaxRaidDB["other"]["keep"], "unchanged")
        self.assertEqual(dict(c.lua.globals().RelaxRaidCharDB.items()), {"version": 1, "logger": True, "other": "unchanged"})

    def test_guild_name_kept_exactly_as_read(self):
        for guild in (" MiXeD Invented Guild ", "", "Invented \u00c9toile"):
            c = boot(guild=(guild, "Rank B", 1))
            self.assertEqual(self.row(c)["guild"], guild)

    def test_guild_byte_bound_omits_oversize_without_changing_rank(self):
        for guild in ("G" * 96, "É" * 48):
            self.assertEqual(self.row(boot(guild=(guild, "Rank B", 1)))["guild"], guild)
        for guild in ("G" * 97, "É" * 49, "G" * 4096, "G" * 4097):
            c = boot()
            c.guild = (guild, "Rank B", 1)
            c.fire("GUILD_ROSTER_UPDATE")
            self.assertEqual(self.row(c), {"result": "officer", "readAt": c.wall}, len(guild.encode()))

    def test_guild_control_bytes_are_omitted(self):
        for byte in (*range(32), 127):
            c = boot(guild=("Invented" + chr(byte) + "Guild", "Rank B", 1))
            self.assertEqual(self.row(c), {"result": "officer", "readAt": c.wall}, byte)
        for guild in ('Invented | Guild', 'Invented "Guild" \\'):
            self.assertEqual(self.row(boot(guild=(guild, "Rank B", 1)))["guild"], guild)

    def test_large_guild_save_remains_readable_by_prep(self):
        # Feed complete real-writer saves to the unchanged bounded prep reader.
        with tempfile.TemporaryDirectory(prefix="rank-prep-") as tmp:
            tmp = Path(tmp)
            for size in (96, 4096, 4097):
                (tmp / (str(size) + ".lua")).write_text(rank_save("G" * size, clockless=True), encoding="utf-8")
            overlay = tmp / "overlay.json"
            target = Path(ROOT) / "desktop/internal/raid/prepread/rank_save_probe_test.go"
            overlay.write_text(json.dumps({"Replace": {str(target): str(Path(HERE) / "prepread_probe_test.go")}}))
            result = subprocess.run(["go", "-C", "desktop", "test", "-count=1", "-v", "-overlay", str(overlay),
                                     "-run", "^TestRankPrepRetained$", "./internal/raid/prepread"],
                                    cwd=ROOT, env=dict(os.environ, RANK_SAVES=str(tmp)),
                                    text=True, capture_output=True, timeout=60)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn("--- PASS: TestRankPrepRetained ", result.stdout)

    def test_unreadable_guild_is_omitted_not_carried_forward(self):
        for value in ('nil', 'false', '42', '{}', 'secret()'):
            c = boot()
            c.run('GetGuildInfo=function() return ' + value + ', "Rank B", 1 end')
            c.fire("GUILD_ROSTER_UPDATE")
            self.assertEqual(self.row(c), {"result": "officer", "readAt": c.wall})
        c = boot()
        c.run('GetGuildInfo=function() error("unavailable") end')
        c.fire("GUILD_ROSTER_UPDATE")
        self.assertEqual(self.row(c), {"result": "unreadable", "readAt": c.wall})
        self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})

    def test_secret_rank_reads_are_saved_as_unreadable(self):
        for code in ('IsInGuild=function() return secret() end',
                     'GetGuildInfo=function() return "Invented Guild", secret(), secret() end',
                     'IsInGuild=function() error("unavailable") end',
                     'RelaxRaid.modules.Log.readRank=function() error("unavailable") end'):
            c = boot()
            c.run(code)
            c.fire("GUILD_ROSTER_UPDATE")
            self.assertEqual(self.row(c), {"result": "unreadable", "guild": GUILD, "readAt": c.wall})
            self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})

    def test_bad_clock_officer_leaves_previous_entry(self):
        for code in BAD_CLOCKS:
            c = boot()
            old = self.row(c)
            c.guild = ("Another Invented Guild", "Rank B", 1)
            c.run(code)
            c.fire("GUILD_ROSTER_UPDATE")
            self.assertEqual(self.row(c), old)
            self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})
            fresh = Client()
            fresh.guild = (GUILD, "Rank B", 1)
            fresh.run(code)
            fresh.load_addon(ADDON)
            self.assertIsNone(fresh.lua.globals().RelaxRaidDB["ranks"])

    def test_bad_clock_settled_non_officer_replaces_officer_without_read_at(self):
        for code in BAD_CLOCKS:
            for guild, result in (((GUILD, "Rank F", 5), "member"), (None, "noguild"),
                                  ((GUILD, "Rank Unknown", None), "unreadable")):
                for event, args in (("GUILD_ROSTER_UPDATE", ()), ("PLAYER_GUILD_UPDATE", ("player",)),
                                    ("PLAYER_ENTERING_WORLD", (False, True)), ("login", ())):
                    c = boot()
                    old = {"RelaxRaidDB": literal(c.lua.globals().RelaxRaidDB)}
                    if event == "login":
                        c = Client()
                    c.guild = guild
                    c.run(code)
                    if event == "login":
                        c.load_addon(ADDON, old)
                    else:
                        c.fire(event, *args)
                    expected = {"result": result}
                    if guild is not None:
                        expected["guild"] = GUILD
                    self.assertEqual(self.row(c), expected, (code, result, event))
                    self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})
                    # The clockless negative survives a pending reload; no old
                    # timestamp or officer evidence is resurrected.
                    reloaded = boot({"RelaxRaidDB": literal(c.lua.globals().RelaxRaidDB)}, loaded=False)
                    self.assertEqual(self.row(reloaded), expected)

    def test_unreadable_identity_cannot_overwrite_or_create_an_entry(self):
        for code in ('UnitFullName=function() return secret(), "PvP-EU" end',
                     'UnitFullName=function() return "Alia-Stone", secret() end',
                     'UnitFullName=function() error("unavailable") end',
                     'UnitFullName=function() return nil, nil end',
                     'issecretvalue=function() error("unavailable") end', 'issecretvalue=nil'):
            c = boot()
            old = literal(c.lua.globals().RelaxRaidDB)
            c.guild = (GUILD, "Rank F", 5)
            c.run(code)
            c.fire("GUILD_ROSTER_UPDATE")
            self.assertEqual(literal(c.lua.globals().RelaxRaidDB), old)
            self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})

    def test_late_identity_and_realm_fallback(self):
        c = Client()
        c.guild = (GUILD, "Rank B", 1)
        c.player_name = None
        c.load_addon(ADDON)
        self.assertIsNone(c.lua.globals().RelaxRaidDB["ranks"])
        c.player_name, c.realm = "Alia-Stone", None
        c.enter(initial=True)
        self.assertEqual(self.row(c)["result"], "officer")

    def test_foreign_or_secret_unit_and_preload_event_do_not_refresh(self):
        c = boot()
        old = self.row(c)
        c.guild = (GUILD, "Rank F", 5)
        c.advance(3)
        for unit in ("party1", c.secret()):
            c.fire("PLAYER_GUILD_UPDATE", unit)
            self.assertEqual(self.row(c), old)
        c.run('RelaxRaid.loaded=false')
        c.fire("GUILD_ROSTER_UPDATE")
        self.assertEqual(self.row(c), old)
        c.run('RelaxRaid.loaded=true')
        c.fire("PLAYER_GUILD_UPDATE")
        self.assertEqual(self.row(c)["result"], "member")

    def test_rank_cap_keeps_50_and_evicts_oldest_on_51st_write(self):
        c = boot()
        for i in range(1, 50):
            c.player_name = "Player" + str(i)
            c.now = i
            c.fire("GUILD_ROSTER_UPDATE")
        ranks = c.lua.globals().RelaxRaidDB["ranks"]
        self.assertEqual(len(list(ranks.keys())), 50)
        # Updating an existing row at capacity must not evict any character.
        c.player_name, c.now = "Alia-Stone", 50
        c.fire("GUILD_ROSTER_UPDATE")
        self.assertEqual(len(list(ranks.keys())), 50)
        self.assertEqual(self.row(c)["readAt"], c.wall + 50)
        c.player_name, c.now = "Player50", 51
        c.fire("GUILD_ROSTER_UPDATE")
        self.assertEqual(len(list(ranks.keys())), 50)
        self.assertIsNone(ranks["Player1-PvP-EU"])
        self.assertIsNotNone(ranks["Player50-PvP-EU"])
        self.assertIsNotNone(ranks[NAME])
        # Clockless demotion at capacity still replaces its own positive row.
        c.player_name, c.guild = "Alia-Stone", (GUILD, "Rank F", 5)
        c.run("GetServerTime=nil")
        c.fire("GUILD_ROSTER_UPDATE")
        self.assertEqual(self.row(c), {"result": "member", "guild": GUILD})
        self.assertEqual(len(list(ranks.keys())), 50)

    def test_rank_cap_on_pending_reload_drops_untimed_then_oldest_then_name(self):
        rows = {"Player%02d-PvP-EU" % i: {"result": "member", "readAt": i} for i in range(50)}
        def load(source):
            c = Client()
            return boot({"RelaxRaidDB": literal(c.lua.table_from({"version": 1, "ranks": source}, recursive=True))}, loaded=False)
        at_cap = load(rows)
        self.assertEqual(set(at_cap.lua.globals().RelaxRaidDB["ranks"].keys()), set(rows))
        for extra, dropped in (({"Untimed-PvP-EU": {"result": "noguild"}}, {"Untimed-PvP-EU"}),
                               ({"Zed-PvP-EU": {"result": "member", "readAt": 0}}, {"Player00-PvP-EU"}),
                               ({"Aaa-PvP-EU": {"result": "member", "readAt": 0}}, {"Aaa-PvP-EU"}),
                               ({"New-PvP-EU": {"result": "officer", "readAt": 999}}, {"Player00-PvP-EU"}),
                               ({"Aaa-PvP-EU": {"result": "member"}, "Zed-PvP-EU": {"result": "unreadable"}},
                                {"Aaa-PvP-EU", "Zed-PvP-EU"})):
            for source in (dict(rows, **extra), dict(reversed(list(dict(rows, **extra).items())))):
                c = load(source)
                self.assertEqual(set(c.lua.globals().RelaxRaidDB["ranks"].keys()), set(source) - dropped)
                self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})
        # Equal missing times also use the name, rather than pairs() order.
        untimed = {"Player%02d-PvP-EU" % i: {"result": "member"} for i in range(51)}
        c = load(untimed)
        self.assertEqual(set(c.lua.globals().RelaxRaidDB["ranks"].keys()), set(untimed) - {"Player00-PvP-EU"})

    def test_invalid_saved_rank_rows_are_removed_before_use(self):
        invalid = ('false', '42', '"officer"', '{}', '{result="pending"}', '{result="owner"}',
                   '{result=true}', '{result={}}', '{result=secret()}',
                   '{result="officer",readAt="123"}', '{result="officer",readAt=-1}',
                   '{result="officer",readAt=1.5}', '{result="officer",readAt=10000000000}',
                   '{result="officer",readAt=0/0}', '{result="officer",readAt=math.huge}',
                   '{result="officer",readAt=false}', '{result="officer",readAt=secret()}',
                   '{result="officer",guild=false}', '{result="officer",guild={}}',
                   '{result="officer",guild=secret()}', '{result="officer",guild=string.rep("G",97)}',
                   '{result="officer",guild=string.char(10)}',
                   '{result="officer",extra={nested={}}}')
        valid = '["Valid-PvP-EU"]={result="member",guild="",readAt=0}'
        for row in invalid:
            saved = '{version=1,ranks={' + valid + ',["Broken-PvP-EU"]=' + row + '}}'
            c = boot({"RelaxRaidDB": saved}, loaded=False)
            self.assertEqual(set(c.lua.globals().RelaxRaidDB["ranks"].keys()), {"Valid-PvP-EU"}, row)
            self.assertEqual(self.row(c, "Valid-PvP-EU"), {"result": "member", "guild": "", "readAt": 0})
            self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})
        for key in ('42', 'true', '{}', '""', '"unreadable"', '" Bad-PvP-EU"',
                    '"Bad|PvP-EU"', 'string.rep("A",97)', 'secret()'):
            saved = '{version=1,ranks={' + valid + ',[' + key + ']={result="officer",readAt=999}}}'
            c = boot({"RelaxRaidDB": saved}, loaded=False)
            self.assertEqual(set(c.lua.globals().RelaxRaidDB["ranks"].keys()), {"Valid-PvP-EU"}, key)
            self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})
        # Validation runs before sorting; invalid high-time entries do not
        # consume slots or evict older valid evidence.
        c = boot({"RelaxRaidDB": '''(function()
            local ranks = {}
            for i=1,50 do ranks["Player"..i.."-PvP-EU"]={result="member",readAt=i} end
            ranks["Bad-PvP-EU"]={result="pending",readAt=999}
            return {version=1,ranks=ranks}
        end)()'''}, loaded=False)
        self.assertEqual(len(list(c.lua.globals().RelaxRaidDB["ranks"].keys())), 50)
        self.assertIsNone(c.lua.globals().RelaxRaidDB["ranks"]["Bad-PvP-EU"])
        self.assertIsNotNone(c.lua.globals().RelaxRaidDB["ranks"]["Player1-PvP-EU"])

    def test_invalid_saved_rank_table_is_ignored_on_pending_login(self):
        for ranks in ('false', '42', '"officer"', 'secret()'):
            c = boot({"RelaxRaidDB": '{version=1,ranks=' + ranks + '}'}, loaded=False)
            self.assertIsNone(c.lua.globals().RelaxRaidDB["ranks"])
            self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})

    def test_newer_saves_are_never_written(self):
        for saved in ({"RelaxRaidDB": '{version=2,keep="newer"}'},
                      {"RelaxRaidDB": '{version=1,prep={version=3},keep="newer"}'},
                      {"RelaxRaidCharDB": '{version=2,keep="newer"}',
                       "RelaxRaidDB": '{version=1,keep="account"}'}):
            c = boot(saved)
            before = literal(c.lua.globals().RelaxRaidDB)
            c.fire("GUILD_ROSTER_UPDATE")
            c.fire("PLAYER_GUILD_UPDATE", "player")
            c.enter()
            self.assertEqual(literal(c.lua.globals().RelaxRaidDB), before)
            self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})


if __name__ == "__main__":
    unittest.main(verbosity=2)
