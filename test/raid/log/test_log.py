"""Relax Raid logging switch: REQ-LOG-01, REQ-LOG-02 and PLAN.md section 7's R1 tests, on the mock client.

  python -I -B test/raid/log/test_log.py
"""
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
sys.path.insert(0, os.path.join(ROOT, "test", "raid", "_client"))
from client import Client  # noqa: E402

ADDON = os.path.join(ROOT, "addon", "RelaxRaid")
OFFICER = ("Fake Guild", "Rank B", 1)
MEMBER = ("Fake Guild", "Rank F", 5)
ON_LINE = "combat logging is on"
REMIND_LOG = "could not turn combat logging on"
REMIND_ADV = "could not turn Advanced Combat Logging on"
HINT = "could not read your guild rank"
SIZE_HINT = "Combat logs are large"


def boot(guild=OFFICER, loaded=True, lag=0.0, ignore=0.0, saved=None, refuse=False, player_logging=False,
         advanced="0"):
    c = Client(log_lag=lag, start_ignored_after_zone=ignore)
    c.guild, c.guild_loaded, c.refuse_cvar_write = guild, loaded, refuse
    c.cvars["advancedCombatLogging"] = advanced
    if player_logging:  # the player typed /combatlog before
        c._log.append((float("-inf"), True))
    c.load_addon(ADDON, saved)
    c.enter("none", 0, initial=True)
    return c


def truth(c):
    """Is the client really logging (not the lagging read)?"""
    return c._log[-1][1]


def says(c, text):
    return sum(1 for line in c.chat if text in line)


def starts(c):
    return sum(1 for a in c.log_calls if a is True)


class LogTests(unittest.TestCase):
    # --- REQ-LOG-01 -----------------------------------------------------------------------------------------
    def test_raid_entry_turns_both_on(self):
        c = boot()
        c.advance(5)
        c.enter("raid", 409)
        c.advance(60)
        self.assertTrue(truth(c))
        self.assertEqual(c.cvars["advancedCombatLogging"], "1")
        self.assertEqual(says(c, ON_LINE), 1)
        self.assertEqual(says(c, "with Advanced Combat Logging"), 1)
        self.assertEqual(says(c, REMIND_LOG) + says(c, REMIND_ADV), 0)

    def test_dungeon_entry_turns_both_on(self):
        c = boot()
        c.enter("party", 329)
        c.advance(60)
        self.assertTrue(truth(c))
        self.assertEqual(c.cvars["advancedCombatLogging"], "1")
        self.assertEqual(says(c, ON_LINE), 1)

    def test_open_world_and_other_instances_do_nothing(self):
        c = boot()
        for kind in ("none", "pvp", "arena", "scenario"):
            c.enter(kind, 1)
            c.advance(60)
        self.assertEqual(starts(c), 0)
        self.assertEqual(c.cvar_writes, [])
        self.assertEqual(says(c, ON_LINE), 0)

    def test_outdoor_raid_map_from_data(self):
        c = boot()
        c.run("RelaxRaid.modules.Log.outdoorRaidMaps = {9001}")
        c.enter("none", 9001)
        c.advance(60)
        self.assertTrue(truth(c))
        self.assertEqual(says(c, ON_LINE), 1)

    def test_start_ignored_10s_read_lags_5s(self):
        # PLAN.md 7: logging is on by the last retry and the chat line is said once
        c = boot(lag=5, ignore=10)
        c.advance(100)
        c.enter("raid", 409)
        c.advance(30)
        self.assertTrue(truth(c), "on by the last retry")
        c.advance(60)
        self.assertEqual(c.ignored_starts, 3, "tries at 0, 3 and 6 s fall in the ignored window")
        self.assertEqual(starts(c), 4, "the read confirms at 15 s, so no start after the one at 10 s")
        self.assertEqual(says(c, ON_LINE), 1)
        self.assertEqual(says(c, REMIND_LOG), 0)

    def test_call_taken_without_error_and_ignored_is_retried(self):
        # PLAN.md 7.1 point 5, required: the 0 s call returns without error (and even says "on") but is
        # ignored, the read stays off, and logging must be on after a later retry
        c = Client(log_lag=5, start_ignored_after_zone=10, start_result="true")
        c.guild = OFFICER
        c.load_addon(ADDON)
        c.enter("none", 0, initial=True)
        c.enter("raid", 409)
        c.advance(0)
        self.assertEqual(c.ignored_starts, 1)
        self.assertFalse(truth(c))
        self.assertEqual(says(c, ON_LINE), 0, "no chat line at the call")
        c.advance(30)
        self.assertTrue(truth(c), "a later retry started it")
        c.advance(60)
        self.assertEqual(c.ignored_starts, 3)
        self.assertEqual(says(c, ON_LINE), 1)
        self.assertEqual(says(c, REMIND_LOG), 0)

    def test_required_matrix_whatever_a_start_returns(self):
        # PLAN.md 7.1 point 5 and review round 1: ignore windows 2/10/25 s x read lag 0/5 s x a start that
        # returns true while ignored, nil, or the state: logging is on by the end in every case
        for result in ("true", "nil", "state"):
            for ignore in (2, 10, 25):
                for lag in (0, 5):
                    with self.subTest(result=result, ignore=ignore, lag=lag):
                        c = Client(log_lag=lag, start_ignored_after_zone=ignore, start_result=result)
                        c.guild = OFFICER
                        c.load_addon(ADDON)
                        c.enter("none", 0, initial=True)
                        c.enter("raid", 409)
                        c.advance(31)
                        self.assertTrue(truth(c), "on by the last retry")
                        c.advance(60)
                        self.assertEqual(says(c, ON_LINE), 1)
                        self.assertEqual(says(c, REMIND_LOG), 0)
                        self.assertEqual(says(c, "internal error"), 0)

    def test_secret_read_is_unknown_and_retries_go_on(self):
        c = boot()
        c.run("LoggingCombat = function(on) if on == nil then return secret() end return true end")
        c.enter("raid", 409)
        c.advance(60)
        self.assertEqual(c.run("local n = 0 for _ in pairs(RelaxRaid.errors) do n = n + 1 end return n"), 0)
        self.assertEqual(says(c, ON_LINE), 0, "a secret read never confirms")
        self.assertEqual(says(c, REMIND_LOG), 1)
        self.assertEqual(says(c, "internal error"), 0)

    def test_setting_api_missing_does_not_skip_the_start(self):
        c = boot()
        c.run("C_CVar = nil")
        c.enter("raid", 409)
        c.advance(60)
        self.assertTrue(truth(c), "the start still ran")
        self.assertEqual(says(c, REMIND_ADV), 1)
        self.assertEqual(says(c, ON_LINE), 1)
        self.assertEqual(says(c, "internal error"), 0)

    def test_a_throw_in_one_step_skips_nothing_else(self):
        c = boot()
        c.run("RelaxRaid.say = (function(say) return function(t) if t:find('Advanced Combat Logging') then error('x') end say(t) end end)(RelaxRaid.say)")
        c.refuse_cvar_write = True
        c.enter("raid", 409)
        c.advance(0)
        self.assertTrue(truth(c), "the advanced step threw; the start in the same try still ran")
        c.advance(60)
        self.assertEqual(says(c, ON_LINE), 1)

    def test_rank_arrives_late_without_an_event(self):
        c = boot(loaded=False)
        c.advance(20)                          # the wait ran out: unreadable, hint said
        self.assertEqual(says(c, HINT), 1)
        c.guild_loaded = True                  # data now readable, but no PLAYER_GUILD_UPDATE
        c.enter("party", 329)
        c.advance(60)
        self.assertTrue(truth(c), "read again at the loading screen")
        self.assertEqual(says(c, ON_LINE), 1)

    def test_not_in_a_guild_at_load_then_in_one(self):
        c = boot(guild=None)
        c.guild = OFFICER                      # joined (or the guild flag came late), no event
        c.enter("raid", 409)
        c.advance(60)
        self.assertTrue(truth(c))

    def test_secret_guild_values_are_unknown_not_missing(self):
        c = Client()
        c.guild = (c.secret(), c.secret(), c.secret())
        c.load_addon(ADDON)
        c.enter("none", 0, initial=True)
        c.advance(20)
        self.assertEqual(says(c, HINT), 1, "unreadable, not waiting for guild data forever")
        self.assertEqual(says(c, "internal error"), 0)
        self.assertIs(c.run("return RelaxRaid.modules.Log.logger(\'party\')"), False, "the dungeon default carries the officer rule")

    def test_chat_line_reads_the_advanced_setting_when_it_speaks(self):
        c = boot(lag=5, ignore=10, advanced="1")
        c.enter("raid", 409)
        c.advance(1)
        c.cvars["advancedCombatLogging"] = "0"   # switched off by the player after the first try
        c.advance(60)
        self.assertEqual(says(c, ON_LINE), 1)
        self.assertEqual(says(c, "with Advanced"), 0, "the line says what is true when it is said")

    def test_toggling_the_logger_inside_one_entry_keeps_once_per_entry(self):
        c = boot(refuse=True, ignore=10_000)
        c.enter("raid", 409)
        c.advance(45)                            # write refused once, reminder said once
        for _ in range(2):
            c.slash("/relaxraid logger off")
            c.slash("/relaxraid logger on")
            c.advance(45)
        self.assertEqual(len(c.cvar_writes), 1, "one setting write per entry")
        self.assertEqual(says(c, REMIND_ADV), 1)
        self.assertEqual(says(c, REMIND_LOG), 1)
        c2 = boot()
        c2.enter("party", 329)
        c2.advance(45)
        c2.slash("/relaxraid logger off")
        c2.slash("/relaxraid logger on")
        c2.advance(45)
        self.assertEqual(says(c2, ON_LINE), 1, "one confirmation per entry")
        c2.enter("none", 0)
        c2.enter("party", 329)
        c2.advance(45)
        self.assertEqual(says(c2, ON_LINE), 2, "a new entry says it again")

    # --- DEC-40 (PLAN.md 6B.4): raids log for every guild member, dungeons for the officer ranks -------------
    def test_dec40_member_logs_raids_not_dungeons(self):
        c = boot(guild=MEMBER)
        c.enter("raid", 409)
        c.advance(60)
        self.assertTrue(truth(c), "a non-officer guild member logs a raid")
        self.assertEqual(says(c, ON_LINE), 1)
        self.assertEqual(says(c, SIZE_HINT), 1, "the size hint for a default non-officer logger")
        c2 = boot(guild=MEMBER)
        c2.enter("party", 329)
        c2.advance(60)
        self.assertEqual(c2.log_calls, [], "the same character in a dungeon does not")
        self.assertEqual(says(c2, SIZE_HINT), 0)

    def test_dec40_officer_logs_both_without_the_size_hint(self):
        for kind, mapid in (("raid", 409), ("party", 329)):
            c = boot(guild=OFFICER)
            c.enter(kind, mapid)
            c.advance(60)
            self.assertTrue(truth(c), kind)
            self.assertEqual(says(c, SIZE_HINT), 0, "officers are not told")

    def test_dec40_stored_off_stays_off_in_both(self):
        for guild in (MEMBER, OFFICER):
            for kind, mapid in (("raid", 409), ("party", 329)):
                c = boot(guild=guild, saved={"RelaxRaidCharDB": "{version = 1, logger = false}"})
                c.enter(kind, mapid)
                c.advance(60)
                self.assertEqual(c.log_calls, [], (guild, kind))

    def test_dec40_not_in_a_guild_stays_off(self):
        c = boot(guild=None)
        c.enter("raid", 409)
        c.advance(60)
        self.assertEqual(c.log_calls, [])
        self.assertEqual(c.chat, [])

    def test_dec40_unreadable_rank_raid_on_dungeon_off_with_hint(self):
        def unreadable():
            c = Client()
            c.guild = ("Fake Guild", "Rank B", c.secret())
            c.load_addon(ADDON)
            c.enter("none", 0, initial=True)
            return c
        c = unreadable()
        c.enter("raid", 409)
        c.advance(60)
        self.assertTrue(truth(c), "in the guild is enough for a raid")
        c2 = unreadable()
        c2.enter("party", 329)
        c2.advance(60)
        self.assertEqual(c2.log_calls, [], "a dungeon needs the officer rule")
        self.assertEqual(says(c2, HINT), 1)

    def test_dec40_size_hint_once_per_session(self):
        c = boot(guild=MEMBER)
        for _ in range(3):
            c.enter("raid", 409)
            c.advance(60)
            c.enter("none", 0)
        self.assertEqual(says(c, SIZE_HINT), 1)
        self.assertTrue(all(ord(ch) < 128 for line in c.chat for ch in line), "ASCII only")

    def test_dec40_default_rule_table(self):
        c = boot()
        rule = c.run("return RelaxRaid.modules.Log.default")
        cases = {("officer", "raid"): True, ("member", "raid"): True, ("unreadable", "raid"): True,
                 ("pending", "raid"): True, ("noguild", "raid"): False,
                 ("officer", "party"): True, ("member", "party"): False, ("unreadable", "party"): False,
                 ("pending", "party"): None, ("noguild", "party"): False}
        for (rank, kind), want in cases.items():
            self.assertEqual(rule(rank, kind), want, (rank, kind))

    def test_retries_until_the_read_confirms(self):
        # the read lags 8 s, longer than the gap between tries: only the read stops the retries, so the start
        # at 10 s is repeated once at 15 s (one extra header line, which the parser accepts: PROBES P2 item 3)
        c = boot(lag=8, ignore=10)
        c.enter("raid", 409)
        c.advance(60)
        self.assertEqual(starts(c), 5, "0, 3, 6 ignored; 10 taken; 15 repeats it; 20 reads on")
        self.assertTrue(truth(c))
        self.assertEqual(says(c, ON_LINE), 1)

    def test_status_uses_our_call_only_for_its_answer(self):
        c = boot(lag=8)
        c.advance(20)
        c.enter("raid", 409)
        c.advance(1)
        c.slash("/relaxraid status")
        self.assertIn("combat logging: started 1 s ago, waiting for the game to confirm.", c.chat[-1])
        c.advance(10)
        c.slash("/relaxraid status")
        self.assertIn("combat logging: on.", c.chat[-1])
        c2 = boot(guild=MEMBER)
        c2.slash("/relaxraid status")
        self.assertIn("combat logging: off.", c2.chat[-1])

    def test_start_accepted_only_at_the_last_try(self):
        c = boot(lag=5, ignore=25)
        c.enter("raid", 409)
        c.advance(29)
        self.assertFalse(truth(c))
        c.advance(1)
        self.assertTrue(truth(c), "the 30 s try is accepted")
        c.advance(60)
        self.assertEqual(says(c, ON_LINE), 1, "the 40 s read confirms it")
        self.assertEqual(says(c, REMIND_LOG), 0)

    def test_never_confirmed_gives_one_reminder_and_stops(self):
        c = boot(ignore=10_000)
        c.enter("raid", 409)
        c.advance(60)
        self.assertEqual(says(c, REMIND_LOG), 1)
        self.assertIn("/combatlog", c.chat[-1])
        self.assertEqual(starts(c), 7, "one try per scheduled time, no more")
        calls = len(c.log_calls)
        c.advance(3600)
        self.assertEqual(len(c.log_calls), calls, "no retry loop")
        self.assertEqual(says(c, REMIND_LOG), 1)
        self.assertEqual(says(c, ON_LINE), 0)

    def test_already_on_with_advanced_off_turns_advanced_on_only(self):
        c = boot(player_logging=True)
        c.enter("raid", 409)
        c.advance(60)
        self.assertEqual(starts(c), 0, "logging was on: no start, so no extra header line")
        self.assertEqual(c.cvars["advancedCombatLogging"], "1")
        self.assertEqual(says(c, ON_LINE), 1)

    def test_never_off(self):
        # PLAN.md 7: logging already on by the player: we never call the "off" form, on any path
        c = boot(player_logging=True, advanced="1")
        c.enter("raid", 409)
        c.advance(60)
        c.fire("ZONE_CHANGED_NEW_AREA")
        c.enter("none", 0)                     # leaving the raid
        c.advance(60)
        c.enter("party", 329)
        c.advance(10)
        c.slash("/relaxraid logger off")       # the setting turned off inside a dungeon
        c.advance(60)
        c.click_setting("RELAXRAID_LOGGER", False)
        c.slash("/rraid logger default")
        c.enter("party", 329, reload=True)     # /reload inside
        c.advance(60)
        c.fire("PLAYER_LOGOUT")
        c.advance(60)
        self.assertNotIn(False, c.log_calls)
        self.assertNotIn(0, c.log_calls)
        self.assertNotIn("nil-arg", c.log_calls, "LoggingCombat(nil) is not a read")
        self.assertTrue(all(v == "1" for _, v in c.cvar_writes), c.cvar_writes)
        self.assertTrue(truth(c))
        # and when we started it ourselves
        c2 = boot()
        c2.enter("raid", 409)
        c2.advance(60)
        c2.enter("none", 0)
        c2.slash("/relaxraid logger off")
        c2.fire("PLAYER_LOGOUT")
        c2.advance(60)
        self.assertNotIn(False, c2.log_calls)
        self.assertTrue(truth(c2))

    def test_setting_write_refused(self):
        # PLAN.md 7: reminder shown with the manual step, no error, no retry loop
        c = boot(refuse=True)
        c.enter("raid", 409)
        c.advance(600)
        self.assertEqual(says(c, REMIND_ADV), 1)
        self.assertIn("Options > Network", [ln for ln in c.chat if REMIND_ADV in ln][0])
        self.assertEqual(len(c.cvar_writes), 1, "the write is tried once")
        self.assertEqual(says(c, "internal error"), 0)
        self.assertEqual(dict(c.run("return RelaxRaid.errors").items()), {})
        self.assertTrue(truth(c), "combat logging itself still goes on")
        self.assertEqual(says(c, ON_LINE), 1)
        self.assertEqual(says(c, "with Advanced"), 0)

    def test_setting_api_throws(self):
        c = boot()
        c.run("C_CVar.SetCVar = function() error('blocked') end")
        c.enter("raid", 409)
        c.advance(60)
        self.assertEqual(says(c, REMIND_ADV), 1)
        self.assertEqual(says(c, "internal error"), 0)
        self.assertTrue(truth(c))

    def test_logging_api_throws(self):
        c = boot()
        c.run("LoggingCombat = function() error('blocked') end")
        c.enter("raid", 409)
        c.advance(60)
        self.assertEqual(says(c, REMIND_LOG), 1)
        self.assertEqual(says(c, "internal error"), 0)

    def test_chat_line_once_per_entry(self):
        c = boot()
        c.enter("raid", 409)
        c.advance(60)
        c.fire("ZONE_CHANGED_NEW_AREA")
        c.enter("raid", 409)                   # a loading screen inside the same raid
        c.advance(60)
        self.assertEqual(says(c, ON_LINE), 1)
        c.enter("none", 0)
        c.advance(5)
        c.enter("raid", 409)                   # a new entry
        c.advance(60)
        self.assertEqual(says(c, ON_LINE), 2)

    def test_leaving_early_stops_the_tries(self):
        c = boot(ignore=10_000)
        c.enter("raid", 409)
        c.advance(4)
        c.enter("none", 0)
        c.advance(120)
        self.assertEqual(starts(c), 2, "tries at 0 and 3 s only")
        self.assertEqual(says(c, REMIND_LOG), 0, "no reminder outside")

    # --- REQ-LOG-02 -----------------------------------------------------------------------------------------
    def test_member_rank_is_not_a_logger(self):
        c = boot(guild=MEMBER)
        c.enter("party", 329)
        c.advance(60)
        self.assertEqual(c.log_calls, [])
        self.assertEqual(c.cvar_writes, [])
        self.assertEqual(c.chat, [])
        self.assertIs(c.run("return RelaxRaid.modules.Log.logger(\'party\')"), False, "the dungeon default carries the officer rule")

    def test_officer_rank_is_a_logger(self):
        c = boot(guild=("Fake Guild", "Rank A", 0))
        self.assertTrue(c.setting_value("RELAXRAID_LOGGER"))
        c.slash("/relaxraid status")
        self.assertIn("logger on for this character (default: officer rank)", c.chat[-2])

    def test_officer_rank_unreadable_gives_logger_off_and_hint(self):
        # PLAN.md 7: officer rank unreadable: logger off, hint shown
        c = Client()
        c.guild = ("Fake Guild", "Rank B", c.secret())
        c.load_addon(ADDON)
        c.enter("none", 0, initial=True)
        self.assertEqual(says(c, HINT), 1)
        self.assertIn("/relaxraid logger on", c.chat[-1])
        c.fire("PLAYER_GUILD_UPDATE", "player")
        c.fire("PLAYER_GUILD_UPDATE", "player")
        c.enter("party", 329)
        c.advance(60)
        self.assertEqual(says(c, HINT), 1, "once")
        self.assertEqual(c.log_calls, [])
        self.assertEqual(says(c, "internal error"), 0)
        self.assertIs(c.run("return RelaxRaid.modules.Log.logger(\'party\')"), False, "the dungeon default carries the officer rule")

    def test_officer_rank_names_case_insensitive(self):
        saved = {"RelaxRaid.modules.Log.officerRankNames": '{"Raid Lead"}'}
        c = boot(guild=("Fake Guild", "raid LEAD", 6), saved=saved)
        self.assertTrue(c.setting_value("RELAXRAID_LOGGER"), "the name matches though the index does not")
        c2 = Client()
        c2.guild = ("Fake Guild", "RAID lead", c2.secret())
        c2.load_addon(ADDON, saved)
        self.assertTrue(c2.setting_value("RELAXRAID_LOGGER"), "a listed name decides even with the index secret")
        self.assertEqual(says(c2, HINT), 0)
        c3 = boot(guild=("Fake Guild", "Raider", 6), saved=saved)
        self.assertIs(c3.run("return RelaxRaid.modules.Log.logger(\'party\')"), False, "the dungeon default carries the officer rule")

    def test_shipped_officer_data(self):
        c = boot()
        self.assertEqual(list(c.run("return RelaxRaid.modules.Log.officerRankIndexes").values()), [0, 1])
        self.assertEqual(len(c.run("return RelaxRaid.modules.Log.officerRankNames")), 0)
        self.assertEqual(len(c.run("return RelaxRaid.modules.Log.outdoorRaidMaps")), 0)

    def test_panel_failure_leaves_the_slash_command(self):
        c = Client()
        c.guild = MEMBER
        c.run("Settings.RegisterVerticalLayoutCategory = function() error('no panel') end")
        c.load_addon(ADDON)
        c.enter("none", 0, initial=True)
        c.slash("/relaxraid logger on")
        self.assertIs(c.run("return RelaxRaidCharDB.logger"), True)
        c.enter("party", 329)
        c.advance(60)
        self.assertTrue(truth(c))

    def test_rank_index_not_a_number_is_unreadable(self):
        c = boot(guild=("Fake Guild", "Rank B", "1"))
        self.assertEqual(says(c, HINT), 1)
        self.assertIs(c.run("return RelaxRaid.modules.Log.logger(\'party\')"), False, "the dungeon default carries the officer rule")

    def test_guild_data_late_then_officer(self):
        # guild data arrives after the last try (the wait for it is set longer here), so only a restart helps
        c = boot(loaded=False, saved={"RelaxRaid.modules.Log.RANK_WAIT": "600"})
        c.enter("party", 329)                   # logged in straight into a raid
        c.advance(35)
        self.assertEqual(c.log_calls, [], "nothing until the rank is known")
        c.guild_loaded = True
        c.fire("PLAYER_GUILD_UPDATE", "player")
        c.advance(60)
        self.assertTrue(truth(c))
        self.assertEqual(says(c, ON_LINE), 1)
        self.assertEqual(says(c, HINT), 0)

    def test_guild_data_never_comes(self):
        c = boot(loaded=False)
        c.enter("party", 329)
        c.advance(60)
        self.assertEqual(says(c, HINT), 1, "unreadable after the wait")
        self.assertEqual(c.log_calls, [])
        c.slash("/relaxraid logger on")        # the hint's own step works
        c.advance(60)
        self.assertTrue(truth(c))

    def test_no_guild_is_off_without_hint(self):
        c = boot(guild=None)
        c.enter("raid", 409)
        c.advance(60)
        self.assertEqual(c.chat, [])
        self.assertEqual(c.log_calls, [])

    def test_choice_persists_and_wins_over_rank(self):
        c = boot(guild=MEMBER)
        c.slash("/relaxraid logger on")
        self.assertIs(c.run("return RelaxRaidCharDB.logger"), True)
        c2 = boot(guild=MEMBER, saved={"RelaxRaidCharDB": "{version = 1, logger = true}"})
        c2.enter("party", 329)
        c2.advance(60)
        self.assertTrue(truth(c2))
        c3 = boot(guild=OFFICER, saved={"RelaxRaidCharDB": "{version = 1, logger = false}"})
        c3.enter("party", 329)
        c3.advance(60)
        self.assertEqual(c3.log_calls, [])
        c3.slash("/relaxraid logger default")
        self.assertIsNone(c3.run("return RelaxRaidCharDB.logger"))
        c3.advance(60)
        self.assertTrue(truth(c3), "back on the rank default, inside the dungeon")

    def test_default_is_not_saved_as_a_choice(self):
        c = boot(guild=OFFICER)
        c.enter("raid", 409)
        c.advance(60)
        self.assertIsNone(c.run("return RelaxRaidCharDB"), "nothing written until the player chooses")

    def test_panel_checkbox(self):
        c = boot(guild=MEMBER)
        cat = [x for x in c.categories if x["name"] == "Relax Raid"]
        self.assertEqual(len(cat), 1)
        self.assertTrue(cat[0]["registered"])
        self.assertIsNotNone(c.settings["RELAXRAID_LOGGER"]["checkbox"])
        self.assertTrue(c.setting_value("RELAXRAID_LOGGER"), "a member's box shows the raid default: on")
        c.click_setting("RELAXRAID_LOGGER", False)
        self.assertIs(c.run("return RelaxRaidCharDB.logger"), False, "an explicit off is stored")
        self.assertFalse(c.setting_value("RELAXRAID_LOGGER"))
        c.click_setting("RELAXRAID_LOGGER", True)
        self.assertIsNone(c.run("return RelaxRaidCharDB.logger"), "ticking back returns to the default, not 'on'")
        # not in a guild: the default is off; ticking stores on and starts now inside a dungeon
        c2 = boot(guild=None)
        self.assertFalse(c2.setting_value("RELAXRAID_LOGGER"))
        c2.enter("party", 329)
        c2.advance(35)                         # after the last try of this entry
        c2.click_setting("RELAXRAID_LOGGER", True)
        self.assertIs(c2.run("return RelaxRaidCharDB.logger"), True)
        c2.advance(60)
        self.assertTrue(truth(c2), "turning it on inside a dungeon starts now")
        c2.click_setting("RELAXRAID_LOGGER", False)
        self.assertIsNone(c2.run("return RelaxRaidCharDB.logger"), "unticking returns to the default")

    def test_status_shows_the_default_per_instance_type(self):
        c = boot(guild=MEMBER)
        c.slash("/relaxraid status")
        self.assertIn("logger for this character: default, on in raids, off in dungeons (not an officer rank).", c.chat[-2])
        c.slash("/relaxraid logger off")
        c.slash("/relaxraid status")
        self.assertIn("logger off for this character (your choice).", c.chat[-2])

    def test_size_hint_waits_for_a_settled_rank(self):
        # an officer whose guild data loads after he is inside a raid is not told the non-officer line
        c = boot(guild=OFFICER, loaded=False, saved={"RelaxRaid.modules.Log.RANK_WAIT": "600"})
        c.enter("raid", 409)
        c.advance(5)
        c.guild_loaded = True
        c.fire("PLAYER_GUILD_UPDATE", "player")
        c.advance(60)
        self.assertTrue(truth(c))
        self.assertEqual(says(c, SIZE_HINT), 0)
        # a member in the same position is told once the rank says member
        c2 = boot(guild=MEMBER, loaded=False, saved={"RelaxRaid.modules.Log.RANK_WAIT": "600"})
        c2.enter("raid", 409)
        c2.advance(5)
        self.assertEqual(says(c2, SIZE_HINT), 0, "not while the rank is pending")
        c2.guild_loaded = True
        c2.fire("PLAYER_GUILD_UPDATE", "player")
        self.assertEqual(says(c2, SIZE_HINT), 1)

    def test_newer_save_left_untouched(self):
        saved = '{version = 2, logger = false, future = {a = 1}}'
        c = boot(guild=OFFICER, saved={"RelaxRaidCharDB": saved})
        dump = "local t = RelaxRaidCharDB; return t.version, t.logger, t.future.a, (function() local n = 0 " \
               "for _ in pairs(t) do n = n + 1 end return n end)()"
        before = c.run(dump)
        c.slash("/relaxraid logger on")
        c.click_setting("RELAXRAID_LOGGER", True)
        self.assertEqual(c.run(dump), before)
        self.assertEqual(before, (2, False, 1, 3))
        self.assertEqual(says(c, "newer Relax Raid"), 2)

    def test_slash_help_and_unknown(self):
        c = boot()
        c.slash("/relaxraid")
        c.slash("/rraid what")
        self.assertEqual(says(c, "/relaxraid logger on | off | default | status"), 2)


if __name__ == "__main__":
    unittest.main()
