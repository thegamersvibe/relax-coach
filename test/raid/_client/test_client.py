"""Tests of the Relax Raid mock client: one per modelled behaviour.

  python -I -B test/raid/_client/test_client.py
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from client import Client  # noqa: E402


class ClientTests(unittest.TestCase):
    def test_logging_read_lags(self):
        c = Client(log_lag=2)
        self.assertTrue(c.run("return LoggingCombat(true)"))
        self.assertFalse(c.run("return LoggingCombat()"), "the read still shows the old state")
        c.advance(1.9)
        self.assertFalse(c.run("return LoggingCombat()"))
        c.advance(0.2)
        self.assertTrue(c.run("return LoggingCombat()"), "after the lag the read shows the start")

    def test_start_ignored_after_zone_change(self):
        c = Client(start_ignored_after_zone=5)
        c.advance(10)
        c.zone_change("raid", 409)
        c.advance(4)
        self.assertFalse(c.run("return LoggingCombat(true)"))
        self.assertEqual(c.ignored_starts, 1)
        c.advance(1)
        self.assertTrue(c.run("return LoggingCombat(true)"), "allowed once the window has passed")
        self.assertTrue(c.run("return LoggingCombat()"))

    def test_advanced_logging_write_refused(self):
        c = Client()
        self.assertEqual(c.run('return C_CVar.GetCVar("advancedCombatLogging")'), "0")
        self.assertTrue(c.run('return C_CVar.SetCVar("advancedCombatLogging", "1")'))
        self.assertEqual(c.run('return C_CVar.GetCVar("advancedCombatLogging")'), "1")
        c.refuse_cvar_write = True
        self.assertFalse(c.run('return C_CVar.SetCVar("advancedCombatLogging", "0")'))
        self.assertEqual(c.run('return C_CVar.GetCVar("advancedCombatLogging")'), "1", "a refused write changes nothing")

    def test_restricted_messages_dropped_and_counted(self):
        c = Client()
        self.assertEqual(c.run('return C_ChatInfo.SendAddonMessage("RLXR1", "a", "RAID")'), 0)
        c.restricted = True
        self.assertEqual(c.run('return C_ChatInfo.SendAddonMessage("RLXR1", "b", "RAID")'), 11)
        self.assertEqual(c.run('return C_ChatInfo.SendAddonMessage("RLXR1", "c", "RAID")'), 11)
        self.assertEqual(c.dropped, 2)
        self.assertEqual([s[1] for s in c.sent], ["a"])

    def test_instance_type_and_map(self):
        c = Client()
        self.assertEqual(c.run("return IsInInstance()"), (False, "none"))
        c.zone_change("raid", 409)
        self.assertEqual(c.run("return IsInInstance()"), (True, "raid"))
        info = c.run("return GetInstanceInfo()")
        self.assertEqual((info[1], info[7]), ("raid", 409))
        c.zone_change("party", 329)
        self.assertEqual(c.run("local _, t = IsInInstance(); return t"), "party")

    def test_clock_and_timer_order(self):
        c = Client()
        c.run("""
            order = {}
            C_Timer.After(3, function() order[#order+1] = "c@" .. GetTime() end)
            C_Timer.After(1, function() order[#order+1] = "a@" .. GetTime() end)
            C_Timer.After(1, function() order[#order+1] = "b@" .. GetTime()
                C_Timer.After(0.5, function() order[#order+1] = "nested@" .. GetTime() end) end)
        """)
        c.advance(0.5)
        self.assertEqual(c.run("return #order"), 0)
        c.advance(2.6)
        got = [c.run("return order[%d]" % i) for i in range(1, c.run("return #order") + 1)]
        self.assertEqual(got, ["a@1", "b@1", "nested@1.5", "c@3"])
        self.assertAlmostEqual(c.run("return GetTime()"), 3.1)

    def test_secret_values_raise(self):
        c = Client()
        # Lua 5.1 calls __lt/__le only when both sides are tables; a mixed comparison raises Lua's own error.
        for expr, ours in (("s + 1", True), ("s .. 'x'", True), ("-s", True), ("s < secret()", True),
                           ("s == secret()", True), ("s < 1", False), ("1 <= s", False)):
            with self.subTest(expr=expr):
                ok, msg = c.run("local s = secret(); return pcall(function() return %s end)" % expr)
                self.assertFalse(ok, expr)
                self.assertIn("secret value used" if ours else "attempt to compare", str(msg))

    # --- added by R1 ----------------------------------------------------------------------------------------
    def test_events_reach_registered_frames(self):
        c = Client()
        c.run("got = {}; f = CreateFrame('Frame'); f:RegisterEvent('A'); "
              "f:SetScript('OnEvent', function(_, e, x) got[#got + 1] = e .. tostring(x) end)")
        c.fire("A", 1)
        c.fire("B", 2)
        c.run("f:UnregisterEvent('A')")
        c.fire("A", 3)
        self.assertEqual(list(c.run("return got").values()), ["A1"])

    def test_enter_is_a_zone_change_and_a_loading_screen(self):
        c = Client()
        c.run("seen = nil; f = CreateFrame('Frame'); f:RegisterEvent('PLAYER_ENTERING_WORLD'); "
              "f:SetScript('OnEvent', function(_, e, a, b) seen = tostring(a) .. tostring(b) end)")
        c.advance(3)
        c.enter("party", 33, reload=True)
        self.assertEqual(c.run("return seen"), "falsetrue")
        self.assertEqual(c.zone_changed_at, 3)
        self.assertEqual(c.run("return select(2, IsInInstance())"), "party")

    def test_guild_info(self):
        c = Client()
        self.assertFalse(c.run("return IsInGuild()"))
        self.assertIsNone(c.run("return GetGuildInfo('player')"))
        c.guild = ("G", "R", 2)
        self.assertEqual(c.run("return GetGuildInfo('player')"), ("G", "R", 2))
        c.guild_loaded = False
        self.assertTrue(c.run("return IsInGuild()"))
        self.assertIsNone(c.run("return GetGuildInfo('player')"), "not loaded yet")

    def test_issecretvalue(self):
        c = Client()
        c.run("s = secret()")
        self.assertTrue(c.run("return issecretvalue(s)"))
        self.assertFalse(c.run("return issecretvalue({})"))
        self.assertFalse(c.run("return issecretvalue(1)"))

    def test_chat_slash_and_settings(self):
        c = Client()
        c.run("DEFAULT_CHAT_FRAME:AddMessage('hi')")
        self.assertEqual(c.chat, ["hi"])
        c.run("SLASH_T1 = '/tt'; SLASH_T2 = '/t2'; SlashCmdList.T = function(m) last = m end")
        c.slash("/T2 a b")
        self.assertEqual(c.run("return last"), "a b")
        with self.assertRaises(KeyError):
            c.slash("/none")
        c.run("""v = false
                 local cat = Settings.RegisterVerticalLayoutCategory('X')
                 local s = Settings.RegisterProxySetting(cat, 'X_V', Settings.VarType.Boolean, 'V', false,
                     function() return v end, function(x) v = x end)
                 Settings.CreateCheckbox(cat, s, 'tip')
                 Settings.RegisterAddOnCategory(cat)""")
        self.assertTrue(c.categories[0]["registered"])
        c.click_setting("X_V", True)
        self.assertTrue(c.run("return v"))
        self.assertTrue(c.setting_value("X_V"))

    def test_calls_are_recorded(self):
        c = Client()
        c.run("LoggingCombat(); LoggingCombat(true); LoggingCombat(nil); C_CVar.SetCVar('advancedCombatLogging', '1')")
        c.refuse_cvar_write = True
        c.run("C_CVar.SetCVar('advancedCombatLogging', '0')")
        self.assertEqual(c.log_calls, [None, True, "nil-arg"])
        self.assertEqual(c.cvar_writes, [("advancedCombatLogging", "1"), ("advancedCombatLogging", "0")])
        self.assertEqual(c.cvars["advancedCombatLogging"], "1")

    def test_load_addon(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            d = os.path.join(tmp, "Fake")
            os.makedirs(d)
            with open(os.path.join(d, "Fake.toc"), "w") as f:
                f.write("## Interface: 16001\n## SavedVariables: FakeDB\n\nA.lua\nB.lua\n")
            with open(os.path.join(d, "A.lua"), "w") as f:
                f.write("local name, ns = ...; ns.order = name .. ':A'; seenDB = FakeDB\n")
            with open(os.path.join(d, "B.lua"), "w") as f:
                f.write("local _, ns = ...; ns.order = ns.order .. ':B'; local fr = CreateFrame('Frame'); "
                        "fr:RegisterEvent('ADDON_LOADED'); fr:SetScript('OnEvent', function(_, _, n) loaded = n; db = FakeDB end)\n")
            c = Client()
            ns = c.load_addon(d, saved={"FakeDB": "{x = 1}"})
            self.assertEqual(ns["order"], "Fake:A:B")
            self.assertIsNone(c.run("return seenDB"), "saved variables arrive after the files run")
            self.assertEqual(c.run("return loaded"), "Fake")
            self.assertEqual(c.run("return db.x"), 1)


if __name__ == "__main__":
    unittest.main()
