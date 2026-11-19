"""Relax Raid core: the toc, guarded event and timer dispatch, the module registry. Core.lua loads alone.

  python -I -B test/raid/core/test_core.py
"""
import os
import re
import runpy
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
sys.path.insert(0, os.path.join(ROOT, "test", "raid", "_client"))
from client import Client  # noqa: E402

ADDON = os.path.join(ROOT, "addon", "RelaxRaid")


def core_alone():
    c = Client()
    c.load_addon(ADDON, files=["Core.lua"])
    return c


class CoreTests(unittest.TestCase):
    def test_toc(self):
        with open(os.path.join(ADDON, "RelaxRaid.toc"), encoding="utf-8") as f:
            toc = f.read()
        self.assertRegex(toc, r"(?m)^## Interface: 16001$")
        self.assertRegex(toc, r"(?m)^## SavedVariables: RelaxRaidDB$")
        self.assertRegex(toc, r"(?m)^## SavedVariablesPerCharacter: RelaxRaidCharDB$")
        files = [ln.strip() for ln in toc.splitlines() if ln.strip() and not ln.startswith("#")]
        self.assertEqual(files[0], "Core.lua", "Core loads first")
        for f in files:
            self.assertTrue(os.path.isfile(os.path.join(ADDON, f)), f)
        self.assertNotRegex(toc, r"(?m)^## (Dependencies|RequiredDeps):", "no dependencies (INCLUDE-PROPOSAL)")

    def test_core_loads_alone(self):
        c = core_alone()
        self.assertTrue(c.run("return RelaxRaid.loaded"))
        self.assertEqual(c.chat, [])

    def test_init_runs_once_for_our_addon_only(self):
        c = Client()
        c.lua.execute("seen = 0")
        ns = c.lua.table()
        loader = c.lua.eval("function(src, name, ns) return assert(loadstring(src, '@Core.lua'))(name, ns) end")
        with open(os.path.join(ADDON, "Core.lua"), encoding="utf-8") as f:
            loader(f.read(), "RelaxRaid", ns)
        c.run("RelaxRaid.module('T').Init = function() seen = seen + 1 end")
        c.fire("ADDON_LOADED", "SomeOtherAddon", False)
        self.assertEqual(c.run("return seen"), 0)
        c.fire("ADDON_LOADED", "RelaxRaid", False)
        c.fire("ADDON_LOADED", "RelaxRaid", False)
        self.assertEqual(c.run("return seen"), 1)

    def test_handler_error_is_said_once_and_others_still_run(self):
        c = core_alone()
        c.run("""ran = 0
                 RelaxRaid.on('TEST_EVENT', 'Bad', function() error('boom') end)
                 RelaxRaid.on('TEST_EVENT', 'Good', function() ran = ran + 1 end)""")
        c.fire("TEST_EVENT")
        c.fire("TEST_EVENT")
        self.assertEqual(c.run("return ran"), 2)
        self.assertEqual(sum("internal error in Bad:TEST_EVENT" in ln for ln in c.chat), 1)
        self.assertEqual(c.run("return RelaxRaid.errors['Bad:TEST_EVENT']"), 2)

    def test_timer_error_is_guarded(self):
        c = core_alone()
        c.run("RelaxRaid.after(2, 'Tick', function() error('boom') end)")
        c.advance(5)
        self.assertEqual(sum("internal error in Tick" in ln for ln in c.chat), 1)

    def test_init_error_does_not_stop_other_modules(self):
        c = Client()
        ns = c.lua.table()
        loader = c.lua.eval("function(src, name, ns) return assert(loadstring(src, '@Core.lua'))(name, ns) end")
        with open(os.path.join(ADDON, "Core.lua"), encoding="utf-8") as f:
            loader(f.read(), "RelaxRaid", ns)
        c.run("""ok = false
                 RelaxRaid.module('A').Init = function() error('boom') end
                 RelaxRaid.module('B').Init = function() ok = true end""")
        c.fire("ADDON_LOADED", "RelaxRaid", False)
        self.assertTrue(c.run("return ok"))
        self.assertEqual(sum("internal error in A.Init" in ln for ln in c.chat), 1)

    def test_one_frame_for_all_events(self):
        c = Client()
        c.load_addon(ADDON)
        self.assertEqual(len(c.frames), 1)

    def test_not_bundled_before_the_app_knows_it(self):
        # R9b replaces the R1 no-bundling placeholder with the app-owned gate.
        # The CLI/archive test in test_guild_bundle.py checks the actual folders.
        builder = runpy.run_path(os.path.join(ROOT, "scripts", "build_guild_bundle.py"))
        with open(os.path.join(ROOT, "desktop", "addon_update.go"), encoding="utf-8") as f:
            match = re.search(r'^const addonRaidMinimumVersion = "([0-9.]+)"$', f.read(), re.M)
        assert match is not None, 'app raid floor missing'
        floor = match.group(1)
        self.assertFalse(builder["raid_release"]("1.7.10"))
        self.assertTrue(builder["raid_release"](floor))

    def test_no_off_call_and_no_send_in_source(self):
        # REQ-LOG-01: nothing turns logging off; R1 sends nothing (sending is R2's Wire.lua)
        for name in os.listdir(ADDON):
            if not name.endswith(".lua"):
                continue
            with open(os.path.join(ADDON, name), encoding="utf-8") as f:
                code = "\n".join(ln.split("--", 1)[0] for ln in f.read().splitlines())
            self.assertNotRegex(code, r"LoggingCombat\s*\(\s*(false|nil|0)\b", name)
            self.assertNotRegex(code, r"advancedCombatLogging\"\s*,\s*\"?0", name)
            self.assertNotIn("SendAddonMessage", code, name)


if __name__ == "__main__":
    unittest.main()
