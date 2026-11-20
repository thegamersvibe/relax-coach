"""Account-local officer-lock evidence, through the real addon lifecycle.
Mutation targets use ordinary loops: raidtest requires a named test FAIL,
which unittest emits differently for a failing subTest.
"""
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
sys.path.insert(0, os.path.join(ROOT, "test", "raid", "_client"))
from client import Client
from serialize import literal

ADDON = os.path.join(ROOT, "addon", "RelaxRaid")
NAME = "Alia-Stone-PvP-EU"
GUILD = "Invented Guild"


def boot(saved=None, guild=(GUILD, "Rank B", 1), loaded=True):
    c = Client()
    c.guild, c.guild_loaded = guild, loaded
    c.load_addon(ADDON, saved)
    return c


def rank_save():
    """Exercise actual addon writes, including prep that older readers must retain."""
    c = boot({"RelaxRaidDB": '{version=1,other={keep="unchanged"}}'})
    c.in_raid = True
    assert c.run('return RelaxRaid.modules.Store.put("' + NAME + '", RelaxRaid.modules.Prep.build("join"))')
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

    def test_bad_clock_leaves_previous_entry(self):
        for code in ('return nil', 'return secret()', 'error("unavailable")', 'return "123"',
                     'return -1', 'return 1.5', 'return 10000000000', 'return 0/0'):
            c = boot()
            old = self.row(c)
            c.guild = (GUILD, "Rank F", 5)
            c.run('GetServerTime=function() ' + code + ' end')
            c.fire("GUILD_ROSTER_UPDATE")
            self.assertEqual(self.row(c), old)
            self.assertEqual(dict(c.lua.globals().RelaxRaid.errors.items()), {})

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
