"""R2 preparation and app-1.8.0 release gates. All names/items are invented."""
import os
from pathlib import Path
import re
import shutil
import tempfile
import sys
import unittest

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "test/raid/_client"))
from client import Client
ADDON = ROOT / "addon/RelaxRaid"


def app_contract(folder):
    rows = []
    for line in (folder / "RelaxRaid.toc").read_text().splitlines():
        match = re.match(r"^\s*##\s*([^:]+):\s*(.*)$", line)
        if match:
            rows.append((match[1].strip().lower(), match[2].strip()))
    saves = [(key, val) for key, val in rows if key.startswith("savedvariables")]
    assert sorted(saves) == [("savedvariables", "RelaxRaidDB"),
                             ("savedvariablespercharacter", "RelaxRaidCharDB")], saves
    assert not any(key.startswith("dep") or key.startswith("requireddep") for key, _ in rows), rows
    bad = [p.name for p in folder.rglob("*") if p.is_file() and p.suffix.lower() not in
           {".lua", ".toc", ".md", ".txt"}]
    assert not bad, bad


class PrepTests(unittest.TestCase):
    def test_app_180_contract(self):
        app_contract(Path(os.environ.get("RAID_CONTRACT_FOLDER", ADDON)))

    def test_absent_providers_are_not_checked(self):
        c = Client()
        c.load_addon(str(ADDON))
        self.assertTrue(c.run("return RelaxRaid.modules.Prep ~= nil"), "prep module missing")
        report = c.run("return RelaxRaid.modules.Prep.build('join')")
        self.assertEqual(report["gear"], "not checked")
        self.assertEqual(report["playedLevel"], "unreadable")
        self.assertEqual(report["name"], "Alia-Stone-PvP-EU")
        self.assertEqual(report["class"], "MAGE")
        self.assertEqual(report["level"], 60)
        self.assertEqual(report["addonVersion"], "0.2.0")

    def test_consumables_by_category_and_id(self):
        c = Client()
        c.bags = {0: [{"itemID": 901, "stackCount": 2}, {"itemID": 902, "stackCount": 5}],
                  1: [{"itemID": 901, "stackCount": 3}, None]}
        c.item_types = {901: (0, 3), 902: (0, 1)}
        c.load_addon(str(ADDON))
        report = c.run("return RelaxRaid.modules.Prep.build('ready')")
        items = report["consumables"]
        self.assertNotIsInstance(items, str)
        self.assertEqual([(items[i]["category"], items[i]["itemID"], items[i]["count"])
                          for i in range(1, len(items) + 1)], [("flask", 901, 5), ("potion", 902, 5)])

    def test_bag_unknowns_and_caps_are_not_empty(self):
        cases = ["C_Container.GetContainerNumSlots = function() return secret() end",
                 "C_Container.GetContainerItemInfo = function() error('unavailable') end",
                 "C_Container.GetContainerItemInfo = function() return {itemID=901,stackCount=secret()} end",
                 "C_Item.GetItemInfo = nil",
                 "issecretvalue = nil"]
        for script in cases:
            c = Client()
            c.bags = {0: [{"itemID": 901, "stackCount": 1}]}
            c.item_types = {901: (0, 3)}
            c.load_addon(str(ADDON))
            c.run(script)
            self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('ready').consumables"), "unreadable", script)
        c = Client()
        c.bags = {0: [{"itemID": i, "stackCount": 1} for i in range(901, 934)]}
        c.item_types = {i: (0, 3) for i in range(901, 934)}
        c.load_addon(str(ADDON))
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('ready').consumables"), "unreadable")

    def test_coach_audit_supplies_better_ring(self):
        c = Client()
        c.load_addon(str(ADDON))
        c.run("""
            ForeverCoach = {
                Snapshot=function(self) return {equipped={[11]={id=903}}, baginfo={known=true}} end,
                Gear={Audit=function(self,snap) return {status='ok', owned={{id=904,inv=11}},
                      slots={[11]={id=903,scoreStatus='known'}}} end}
            }
        """)
        report = c.run("return RelaxRaid.modules.Prep.build('ready')")
        self.assertNotIsInstance(report["gear"], str)
        self.assertEqual((report["gear"][1]["slot"], report["gear"][1]["wornID"], report["gear"][1]["bagID"]),
                         (11, 903, 904))
        c.run("ForeverCoach.Gear.Audit = function() error('provider unavailable') end")
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('ready').gear"), "unreadable")
        c.run("ForeverCoach.Gear.Audit = function() return {status='missing'} end")
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('ready').gear"), "not checked")

    def test_played_time_is_label_input_not_points(self):
        c = Client()
        c.load_addon(str(ADDON))
        self.assertEqual(c.played_requests, 1)
        c.fire("TIME_PLAYED_MSG", 9999, 100)
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('join').playedLevel"), 100)
        c.fire("TIME_PLAYED_MSG", 9999, c.secret())
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('join').playedLevel"), "unreadable")
        c.fire("TIME_PLAYED_MSG", 9999, 300)
        c.level = 59
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('join').playedLevel"), "unreadable")
        c.level = 60
        c.fire("PLAYER_LEVEL_UP", 60)
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('join').playedLevel"), "unreadable")
        self.assertIsNone(c.run("return RelaxRaid.modules.Prep.build('join').score"))

    def test_loading_bags_and_partial_gear_are_unknown(self):
        c = Client()
        c.load_addon(str(ADDON))
        c.bags = {}
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('join').consumables"), "unreadable")
        c.run("""
            ForeverCoach={Snapshot=function() return {baginfo={known=false}} end,
                Gear={Audit=function() return {status='ok',owned={},slots={}} end}}
        """)
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('join').gear"), "unreadable")
        c.bags={0: [None]*16}
        c.run("""
            ForeverCoach.Snapshot=function() return {baginfo={known=true}} end
            ForeverCoach.Gear.Audit=function() return {status='ok',owned={},slots={[11]={scoreStatus='unreadable'}}} end
        """)
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('join').gear"), "unreadable")

    def test_contract_rejects_mutated_copies(self):
        with tempfile.TemporaryDirectory() as tmp:
            folder = Path(tmp) / "RelaxRaid"
            shutil.copytree(ADDON, folder)
            toc = folder / "RelaxRaid.toc"
            original = toc.read_text()
            for key in ("SavedVariablesFuture", "Dependencies", "RequiredDeps", "DepOther"):
                toc.write_text(original + "\n## " + key + ": OtherDB\n")
                with self.assertRaises(AssertionError):
                    app_contract(folder)
            toc.write_text(original.replace("## SavedVariables: RelaxRaidDB", "## SavedVariables: RelaxRaidDB, RelaxRaidPrepDB"))
            with self.assertRaises(AssertionError):
                app_contract(folder)
            toc.write_text(original)
            (folder / "unexpected.tga").write_bytes(b"synthetic")
            with self.assertRaises(AssertionError):
                app_contract(folder)

    def test_each_module_loads_with_core_only(self):
        for module in ("Prep", "Wire", "Store"):
            c = Client()
            c.load_addon(str(ADDON), files=["Core.lua", module + ".lua"])
            for event in ("GROUP_ROSTER_UPDATE", "PLAYER_ENTERING_WORLD", "READY_CHECK", "ENCOUNTER_END"):
                c.fire(event)
            c.advance(5)
            self.assertIsNone(c.run("return next(RelaxRaid.errors)"), module)

    def test_client_sources_ascii_and_no_item_actions(self):
        for file in ADDON.glob("*.lua"):
            self.assertTrue(file.read_bytes().isascii(), file.name)
            code="\n".join(line.split("--",1)[0] for line in file.read_text().splitlines())
            self.assertNotRegex(code, r"LoggingCombat\s*\(\s*(false|nil|0)\b")
            self.assertNotRegex(code, r"\b(loadstring|RunScript|DeleteCursorItem|PickupContainerItem|UseContainerItem|EquipItemByName)\b")

    def test_secret_elapsed_clock_is_unreadable(self):
        c=Client()
        c.load_addon(str(ADDON))
        c.fire("TIME_PLAYED_MSG",9999,100)
        c.run("GetTime=function() return secret() end")
        ok, value=c.run("local ok,r=pcall(RelaxRaid.modules.Prep.build,'join'); return ok,ok and r.playedLevel or 'error'")
        self.assertTrue(ok)
        self.assertEqual(value,"unreadable")

    def test_malformed_provider_is_unreadable(self):
        c=Client()
        c.load_addon(str(ADDON))
        c.run("ForeverCoach={Snapshot=function() return {} end,Gear={Audit=function() return {} end}}")
        self.assertEqual(c.run("return RelaxRaid.modules.Prep.build('ready').gear"),"unreadable")


if __name__ == "__main__":
    unittest.main()
