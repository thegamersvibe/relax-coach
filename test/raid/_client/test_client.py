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
        self.assertTrue(c.run('return C_ChatInfo.SendAddonMessage("RLXR1", "a", "RAID")'))
        c.restricted = True
        self.assertFalse(c.run('return C_ChatInfo.SendAddonMessage("RLXR1", "b", "RAID")'))
        self.assertFalse(c.run('return C_ChatInfo.SendAddonMessage("RLXR1", "c", "RAID")'))
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


if __name__ == "__main__":
    unittest.main()
