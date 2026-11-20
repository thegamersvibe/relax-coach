"""R2 wire boundary tests: real events and client-send attempts, never real names."""
from pathlib import Path
import sys
import unittest
ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "test/raid/_client"))
from client import Client
from prep_store import report_at
ADDON = ROOT / "addon/RelaxRaid"


def boot(saved=None):
    c = Client()
    c.guild = ("Fake Guild", "Rank F", 5)
    c.load_addon(str(ADDON), saved)
    return c


class WireTests(unittest.TestCase):
    def test_non_guild_sender_and_both_paths_store_canonical_bodies(self):
        sender = boot()
        sender.guild = None
        sender.player_name = "Bram"
        sender.realm = "OwnRealm"
        sender.in_raid = True
        sender.fire("PLAYER_GUILD_UPDATE", "player")
        self.assertFalse(sender.run("return RelaxRaid.modules.Log.logger('raid')"))
        sender.fire("READY_CHECK")
        sender.advance(5)
        self.assertGreater(len(sender.sent), 0)
        self.assertEqual(sender.run("return #RelaxRaidDB.prep.nights"), 0)
        receiver = boot()
        receiver.in_raid = True
        for prefix, body, channel, _ in sender.sent:
            receiver.fire("CHAT_MSG_ADDON", prefix, body, channel, "Bram-OwnRealm")
        self.assertEqual(receiver.run("return RelaxRaid.modules.Wire.stats.received"), 1)
        r = report_at(receiver, "bram-ownrealm")
        body = receiver.lua.globals().RelaxRaid.modules.Wire.encode(r)
        self.assertEqual(receiver.run("return RelaxRaidDB.prep.nights[1].reports['bram-ownrealm'][1]"), body)
        own = boot()
        own.in_raid = True
        own.fire("READY_CHECK")
        own.advance(5)
        self.assertGreater(len(own.sent), 0)
        self.assertEqual(own.run("return RelaxRaidDB.prep.nights[1].reports['alia-stone-pvp-eu'][1]"),
                         own.sent[0][1].split("|", 4)[4])

    def test_protocol_marker_documentation_matches_codec(self):
        text = (ROOT / "docs/raid/PREP-PROTOCOL.md").read_text()
        self.assertNotIn("Unknown scalar is", text)
        c = boot()
        body = c.run("""
            local r=RelaxRaid.modules.Prep.build('ready')
            r.addonVersion,r.class='unreadable','unreadable'
            r.level,r.playedLevel='unreadable','unreadable'
            r.consumables,r.gear='unreadable','unreadable'
            return RelaxRaid.modules.Wire.encode(r)
        """)
        fields = body.split("\t")
        for field, index, marker in (("addonVersion", 1, "unreadable"), ("class", 3, "unreadable"),
                                     ("level", 4, "u"), ("playedLevel", 5, "u"),
                                     ("consumables", 8, "u"), ("gear", 9, "u"), ("notScored", 10, "u")):
            self.assertEqual(fields[index], marker)
            self.assertIn(f"| `{field}` | `{marker}` |", text)
        self.assertIn("| `protocol`, `name`, `reason`, `sentAt` | no marker; invalid/unreadable rejects the report |", text)

    def test_report_round_trip_and_bounds(self):
        c = boot()
        self.assertTrue(c.run("return RelaxRaid.modules.Wire ~= nil"), "wire module missing")
        self.assertTrue(c.run("""
            local W, P = RelaxRaid.modules.Wire, RelaxRaid.modules.Prep
            local r = P.build('ready')
            local body = W.encode(r)
            local copy = W.decode(body)
            return copy.name == r.name and copy.gear == 'not checked' and
                copy.playedLevel == 'unreadable' and copy.protocol == 1 and #body <= W.MAX_BODY
        """))
        for field, value in [("protocol", "2"), ("name", "'x|fake'"), ("level", "61"),
                             ("playedLevel", "-1"), ("reason", "'execute'"), ("sentAt", "0/0")]:
            self.assertIsNone(c.run("local r=RelaxRaid.modules.Prep.build('join'); r.%s=%s; return RelaxRaid.modules.Wire.encode(r)" % (field, value)))
        self.assertIsNone(c.run("return RelaxRaid.modules.Wire.decode(string.rep('x',4097))"))

    def test_join_ready_end_send_own_report_to_raid(self):
        c = boot()
        c.in_raid = True
        c.fire("GROUP_ROSTER_UPDATE")
        c.advance(15)
        self.assertGreater(len(c.sent), 0)
        self.assertEqual(c.prefixes, ["RLXR1"])
        first = len(c.sent)
        c.fire("READY_CHECK", "Bram-Oak-Hill-PvP-EU", 30)
        c.advance(15)
        self.assertGreater(len(c.sent), first)
        first = len(c.sent)
        c.fire("ENCOUNTER_END", 901, "Invented boss", 1, 40, 1)
        c.advance(15)
        self.assertGreater(len(c.sent), first)
        for prefix, body, channel, target in c.sent:
            self.assertEqual((prefix, channel, target), ("RLXR1", "RAID", None))
            self.assertTrue(body.startswith("P1|"))
            self.assertLessEqual(len(body.encode()), 240)
        self.assertTrue(any("Alia-Stone-PvP-EU" in body for _, body, _, _ in c.sent))

    def test_dispatch_rechecks_each_restriction(self):
        for field in ("combat", "lockdown", "chat_lockdown", "restricted"):
            c = boot()
            c.in_raid = True
            c.fire("READY_CHECK")
            setattr(c, field, True)  # restriction becomes active AFTER scheduling
            c.advance(5)
            self.assertEqual(c.send_attempts, [], field)
            setattr(c, field, False)
            c.fire("PLAYER_REGEN_ENABLED")
            c.advance(15)
            self.assertGreater(len(c.sent), 0, field)

    def test_unknown_gate_and_missing_api_fail_closed(self):
        for expr in ("InCombatLockdown", "UnitAffectingCombat", "IsInRaid",
                     "C_ChatInfo.InChatMessagingLockdown", "C_ChatInfo.AreOutgoingAddonChatMessagesRestricted"):
            for result in ("nil", "secret()", "error('unavailable')"):
                c = boot()
                c.in_raid = True
                c.fire("READY_CHECK")
                c.run(expr + " = function() return " + result + " end")
                c.advance(5)
                self.assertEqual(c.send_attempts, [], expr + result)
        c = boot()
        c.in_raid = True
        c.run("C_ChatInfo.InChatMessagingLockdown=nil")
        c.fire("READY_CHECK")
        c.advance(5)
        self.assertEqual(c.send_attempts, [])

    def test_rates_coalesce_and_cancel_on_leave(self):
        c = boot()
        c.in_raid = True
        for _ in range(100):
            c.fire("READY_CHECK")
        c.advance(20)
        self.assertEqual(len(c.send_attempts), 1)
        c.fire("READY_CHECK")
        c.advance(9)
        self.assertEqual(len(c.send_attempts), 1)
        c.advance(3)
        self.assertEqual(len(c.send_attempts), 2)
        c.combat = True
        c.fire("READY_CHECK")
        c.in_raid = False
        c.fire("GROUP_ROSTER_UPDATE")
        c.in_raid = True
        c.combat = False
        c.advance(180)
        self.assertEqual(len(c.send_attempts), 2)
        self.assertLessEqual(len(c._timers), 1)

    def test_multipart_paced_and_each_fragment_guarded(self):
        c = boot()
        c.bags = {0: [{"itemID": i, "stackCount": 999} for i in range(901, 933)]}
        c.item_types = {i: (0, 3) for i in range(901, 933)}
        c.in_raid = True
        c.fire("READY_CHECK")
        c.advance(1)
        self.assertEqual(len(c.send_attempts), 1)
        c.chat_lockdown = True
        c.advance(10)
        self.assertEqual(len(c.send_attempts), 1)
        c.chat_lockdown = False
        c.advance(10)
        self.assertGreater(len(c.send_attempts), 1)
        self.assertTrue(all(b-a >= 0.5 for a,b in zip(c.send_times,c.send_times[1:])))

    def test_receive_forged_name_dropped_and_counted(self):
        c = boot()
        c.in_raid = True
        c.enter("raid", 901)
        body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('ready'))")
        c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|1|1|1|" + body, "RAID", "Bram-Oak-Hill-PvP-EU")
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.forged"), 1)
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
        c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|2|1|1|" + body, "RAID", "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 1)
        self.assertEqual(report_at(c)["name"],
                         "Alia-Stone-PvP-EU")

    def test_multipart_reassembly_replay_expiry_and_version(self):
        sender = boot()
        sender.in_raid = True
        sender.bags = {0: [{"itemID": i, "stackCount": 999} for i in range(901, 933)]}
        sender.item_types = {i: (0, 3) for i in range(901, 933)}
        sender.fire("READY_CHECK")
        sender.advance(15)
        self.assertGreater(len(sender.sent), 1)
        c = boot()
        c.in_raid = True
        for prefix, text, channel, _ in reversed(sender.sent):
            c.fire("CHAT_MSG_ADDON", prefix, text, channel, "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 1)
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"), 1)
        for prefix, text, channel, _ in sender.sent:
            c.fire("CHAT_MSG_ADDON", prefix, text, channel, "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"), 1)
        c = boot()
        c.in_raid = True
        for prefix, text, channel, _ in sender.sent:
            c.fire("CHAT_MSG_ADDON", prefix, text.replace("P1|", "P2|", 1), channel, "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
        prefix, text, channel, _ = sender.sent[0]
        c.fire("CHAT_MSG_ADDON", prefix, text, channel, "Alia-Stone-PvP-EU")
        c.advance(31)
        for prefix, text, channel, _ in sender.sent[1:]:
            c.fire("CHAT_MSG_ADDON", prefix, text, channel, "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.expiredReceive"), 1)

    def test_receive_flood_bounded_and_malformed_never_executes(self):
        c = boot()
        c.in_raid = True
        body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('ready'))")
        for i in range(1, 62):
            c.fire("CHAT_MSG_ADDON", "RLXR1", f"P1|{i}|1|1|" + body, "RAID", "Alia-Stone-PvP-EU")
        self.assertGreater(c.run("return RelaxRaid.modules.Wire.stats.receiveLimited or 0"), 0)
        self.assertLessEqual(c.run("return RelaxRaid.modules.Wire.stats.received or 0"), 8)
        for i in range(41):
            c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|1|1|2|x", "RAID", "Bram-" + chr(65+i//26)+chr(65+i%26)+"-PvP-EU")
        self.assertLessEqual(c.run("return RelaxRaid.modules.Wire.peerCount()"), 40)
        self.assertGreater(c.run("return RelaxRaid.modules.Wire.stats.peerLimited or 0"), 0)
        for text in ("P9|1|1|1|"+body, "P1|01|1|1|"+body, "P1|1|0|1|x", "P1|1|1|22|x",
                     "P1|1|1|1|"+"x"*241, "P1|1|1|1|do HACKED=true end"):
            c.fire("CHAT_MSG_ADDON", "RLXR1", text, "RAID", "Cato-PvP-EU")
        self.assertIsNone(c.lua.globals().HACKED)
        self.assertIsNone(c.run("return next(RelaxRaid.errors)"))

    def test_restriction_states_fail_closed_including_activating(self):
        for state in ("1", "2", "nil", "secret()"):
            c = boot()
            c.in_raid = True
            c.fire("READY_CHECK")
            c.run("C_RestrictedActions={GetAddOnRestrictionState=function() return " + state + " end}")
            c.advance(5)
            self.assertEqual(c.send_attempts, [], state)

    def test_local_logger_keeps_own_report_without_self_echo(self):
        c = boot()
        c.in_raid = True
        c.fire("READY_CHECK")
        c.advance(5)
        self.assertGreater(len(c.sent), 0)
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 1)

    def test_provider_changes_gate_after_collection(self):
        c = boot()
        c.in_raid = True
        c.run("""
            ForeverCoach={Snapshot=function() return {} end, Gear={Audit=function()
                InCombatLockdown=function() return true end; return {status='missing'} end}}
        """)
        c.fire("READY_CHECK")
        c.advance(5)
        self.assertEqual(c.send_attempts, [])

    def test_failed_client_send_spends_budget_not_retried(self):
        c = boot()
        c.in_raid = True
        c.send_result = 3
        c.fire("READY_CHECK")
        c.advance(5)
        self.assertEqual(len(c.send_attempts), 1)
        self.assertEqual(c.sent, [])
        c.fire("READY_CHECK")
        c.advance(24)
        self.assertEqual(len(c.send_attempts), 1)
        c.send_result = 0
        c.advance(3)
        self.assertEqual(len(c.sent), 1)

    def test_queue_expires_and_logout_cancels(self):
        c = boot()
        c.in_raid = True
        c.combat = True
        c.fire("READY_CHECK")
        c.advance(120)
        c.combat = False
        c.advance(5)
        self.assertEqual(c.sent, [])
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.expired"), 1)
        c.fire("READY_CHECK")
        c.fire("PLAYER_LOGOUT")
        c.advance(5)
        self.assertEqual(c.sent, [])

    def test_encounter_discards_partial_snapshot_and_rebuilds(self):
        c = boot()
        c.in_raid = True
        c.bags = {0: [{"itemID": i, "stackCount": 999} for i in range(901, 933)]}
        c.item_types = {i: (0, 3) for i in range(901, 933)}
        c.fire("READY_CHECK")
        c.advance(1)
        self.assertEqual(len(c.sent), 1)
        c.fire("ENCOUNTER_START")
        c.advance(15)
        c.bags = {0: [None]*16}
        c.fire("ENCOUNTER_END")
        c.advance(40)
        # No old fragments (part 2/3 of report id 1) can escape into the next pull window.
        self.assertTrue(all(not text.startswith("P1|1|") for _,text,_,_ in c.sent[1:]))

    def test_accented_own_name_sends_and_sender_is_stored(self):
        sender = boot()
        sender.player_name = "Ali\u00e1-St\u00f3ne"
        sender.realm = "PvP-\u00c9U"
        full_name = sender.player_name + "-" + sender.realm
        sender.in_raid = True
        sender.fire("READY_CHECK")
        sender.advance(5)
        self.assertGreater(len(sender.sent), 0)
        receiver = boot()
        receiver.in_raid = True
        for prefix, body, channel, _ in sender.sent:
            receiver.fire("CHAT_MSG_ADDON", prefix, body, channel, full_name)
        self.assertEqual(receiver.run("return RelaxRaid.modules.Wire.stats.received"), 1)
        rows = receiver.run("return RelaxRaidDB.prep.nights[1].windows[1].reports")
        # Lua folds ASCII only: preserve every UTF-8 byte in the identity key.
        key = full_name.translate(str.maketrans("ABCDEFGHIJKLMNOPQRSTUVWXYZ", "abcdefghijklmnopqrstuvwxyz"))
        self.assertIsInstance(rows[key]["report"], int)
        self.assertEqual(report_at(receiver, key)["name"], full_name)

    def test_accented_sender_report_is_stored(self):
        c = boot()
        c.in_raid = True
        name = "Ali\u00e1-St\u00f3ne-PvP-EU"
        body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('join'))")
        fields = body.split("\t")
        fields[2] = name
        c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|1|1|1|" + "\t".join(fields), "RAID", name)
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"), 1)
        self.assertEqual(report_at(c, name.lower())["name"], name)

    def test_unusable_receive_names_are_counted(self):
        for bad in (None, "", "unreadable", " Bram-EU", "Bram|EU", "Bram,EU", "Bram;EU",
                    "Bram\tEU", "Bram\x01EU", "Bram\x7fEU"):
            for claimed in (False, True):
                with self.subTest(bad=bad, claimed=claimed):
                    c = boot()
                    c.in_raid = True
                    body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('ready'))")
                    sender = "Alia-Stone-PvP-EU"
                    if claimed:
                        # A TAB changes arity rather than only the name grammar.
                        if bad is None or "\t" in bad:
                            continue
                        fields = body.split("\t")
                        fields[2] = bad
                        body = "\t".join(fields)
                    else:
                        sender = bad
                    c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|1|1|1|" + body, "RAID", sender)
                    self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.badName"), 1)
                    self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
                    self.assertIsNone(c.run("return next(RelaxRaid.errors)"))
        c = boot()
        c.in_raid = True
        c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|1|1|1|x", "RAID", c.secret())
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.badName"), 1)

    def test_realm_less_names_normalize_before_forgery_check(self):
        for sender, claimed, forged in (("Bram", "Bram-OwnRealm", False),
                                        ("Bram-OwnRealm", "Bram", False),
                                        ("Bram", "Cato-OwnRealm", True)):
            with self.subTest(sender=sender, claimed=claimed):
                c = boot()
                c.normalized_realm = "OwnRealm"
                c.in_raid = True
                body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('ready'))")
                fields = body.split("\t")
                fields[2] = claimed
                c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|1|1|1|" + "\t".join(fields), "RAID", sender)
                if forged:
                    self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.forged"), 1)
                    self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
                else:
                    self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"), 1)
                    self.assertEqual(report_at(c, "bram-ownrealm")["name"], "Bram-OwnRealm")

    def test_own_nil_realm_uses_normalized_realm(self):
        c = boot()
        c.realm = None
        c.normalized_realm = "OwnRealm"
        c.in_raid = True
        c.fire("READY_CHECK")
        c.advance(5)
        self.assertGreater(len(c.sent), 0)
        self.assertIn("Alia-Stone-OwnRealm", c.sent[0][1])
        self.assertEqual(report_at(c, "alia-stone-ownrealm")["name"], "Alia-Stone-OwnRealm")

    def test_unreadable_normalized_realm_fails_closed_and_counts(self):
        for expr in ("nil", "function() return nil end", "function() return '' end",
                     "function() return secret() end", "function() error('no realm') end"):
            for sender, claimed in (("Bram", "Bram-OwnRealm"), ("Bram-OwnRealm", "Bram")):
                with self.subTest(expr=expr, sender=sender):
                    c = boot()
                    c.in_raid = True
                    body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('ready'))")
                    fields = body.split("\t")
                    fields[2] = claimed
                    c.run("GetNormalizedRealmName=" + expr)
                    c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|1|1|1|" + "\t".join(fields), "RAID", sender)
                    self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.badName"), 1)
                    self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
                    c.realm = None
                    self.assertIsNone(c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('ready'))"))
                    self.assertIsNone(c.run("return next(RelaxRaid.errors)"))
        c = boot()
        c.realm = c.secret()
        self.assertIsNone(c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('ready'))"))

    def test_partial_gear_not_scored_survives_wire_and_store(self):
        c = boot()
        c.in_raid = True
        c.run("""
            ForeverCoach={Snapshot=function() return {baginfo={known=true}} end,
                Gear={Audit=function() return {status='ok',owned={{id=904,inv=11}},slots={
                    [11]={id=903,scoreStatus='known'},[1]={id=905,scoreStatus='unknown'},
                    [19]={id=906,scoreStatus='unknown'}}} end}}
        """)
        r = c.run("return RelaxRaid.modules.Prep.build('ready')")
        self.assertNotIsInstance(r["gear"], str)
        self.assertEqual(list(r["notScored"].values()), [1, 19])
        self.assertEqual(r["gear"][1]["bagID"], 904)
        body = c.lua.globals().RelaxRaid.modules.Wire.encode(r)
        self.assertEqual(len(body.split("\t")), 11)
        self.assertEqual(body.split("\t")[-1], "1,19")
        c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|1|1|1|" + body, "RAID", r["name"])
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"), 1)
        saved = report_at(c)
        self.assertEqual(list(saved["notScored"].values()), [1, 19])
        r["notScored"][1] = 7
        self.assertEqual(saved["notScored"][1], 1)

    def test_not_scored_wire_arity_and_closed_grammar(self):
        c = boot()
        body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('ready'))")
        self.assertEqual(len(body.split("\t")), 11)
        self.assertEqual(body.split("\t")[-1], "n")
        decode = c.lua.globals().RelaxRaid.modules.Wire.decode
        self.assertIsNone(decode("\t".join(body.split("\t")[:-1])))
        self.assertIsNone(decode(body + "\textra"))
        fields = body.split("\t")
        fields[9], fields[10] = "-", "-"
        self.assertEqual(len(decode("\t".join(fields))["notScored"]), 0)
        fields[10] = ",".join(map(str, range(1, 20)))
        self.assertEqual(len(decode("\t".join(fields))["notScored"]), 19)
        for invalid in ("", "0", "20", "1,1", "01", "1.0", "1,", "1;2", "n", "u",
                        ",".join(map(str, range(1, 21)))):
            fields[10] = invalid
            self.assertIsNone(decode("\t".join(fields)), invalid)
        for gear, not_scored in (("n", "1"), ("u", "1"), ("n", "-"), ("u", "n")):
            fields[9], fields[10] = gear, not_scored
            self.assertIsNone(decode("\t".join(fields)))

    def test_instance_type_send_gate_and_each_fragment(self):
        for kind in ("pvp", "arena", "raid", "party", "none"):
            with self.subTest(kind=kind):
                c = boot()
                c.in_raid = True
                c.zone_change(kind)
                c.fire("READY_CHECK")
                c.advance(5)
                if kind in ("pvp", "arena"):
                    self.assertEqual(c.send_attempts, [])
                else:
                    self.assertGreater(len(c.sent), 0)
        for kind in ("pvp", "arena"):
            c = boot()
            c.in_raid = True
            c.bags = {0: [{"itemID": i, "stackCount": 999} for i in range(901, 933)]}
            c.item_types = {i: (0, 3) for i in range(901, 933)}
            c.fire("READY_CHECK")
            c.advance(1)
            self.assertEqual(len(c.sent), 1)
            c.zone_change(kind)
            c.advance(5)
            self.assertEqual(len(c.send_attempts), 1)
            c.zone_change("raid")
            c.advance(5)
            self.assertGreater(len(c.sent), 1)

    def test_unreadable_instance_gate_and_collection_change(self):
        for expr in ("nil", "function() error('unavailable') end", "function() return nil,'raid' end",
                     "function() return secret(),'raid' end", "function() return true,secret() end",
                     "function() return true,'' end", "function() return true,'unknown' end"):
            c = boot()
            c.in_raid = True
            c.fire("READY_CHECK")
            c.run("IsInInstance=" + expr)
            c.advance(5)
            self.assertEqual(c.send_attempts, [], expr)
            self.assertIsNone(c.run("return next(RelaxRaid.errors)"))
        c = boot()
        c.in_raid = True
        c.run("""
            ForeverCoach={Snapshot=function() return {} end, Gear={Audit=function()
                IsInInstance=function() return true,'pvp' end; return {status='missing'} end}}
        """)
        c.fire("READY_CHECK")
        c.advance(5)
        self.assertEqual(c.send_attempts, [])

    def test_observed_encounter_blocks_even_with_inactive_client_flags(self):
        c = boot()
        c.in_raid = True
        c.fire("ENCOUNTER_START")
        c.fire("READY_CHECK")
        c.advance(5)
        self.assertEqual(c.send_attempts, [])
        c.fire("ENCOUNTER_END")
        c.advance(5)
        self.assertGreater(len(c.sent), 0)

    def test_each_restriction_kind_independently_blocks(self):
        for kind in range(6):
            for status in (1, 2):
                c = boot()
                c.in_raid = True
                c.fire("READY_CHECK")
                c.restrictions[kind] = status
                c.advance(5)
                self.assertEqual(c.send_attempts, [], (kind, status))
                c.restrictions[kind] = 0
                c.advance(5)
                self.assertGreater(len(c.sent), 0)

    def test_receive_requires_raid_distribution(self):
        for channel in ("PARTY", "GUILD", "WHISPER", "INSTANCE_CHAT", "raid", ""):
            c = boot()
            c.in_raid = True
            body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('join'))")
            packet = "P1|1|1|1|" + body
            c.fire("CHAT_MSG_ADDON", "RLXR1", packet, channel, "Alia-Stone-PvP-EU")
            self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0, channel)
            self.assertEqual(c.run("return RelaxRaid.modules.Wire.peerCount()"), 0)
            c.fire("CHAT_MSG_ADDON", "RLXR1", packet, "RAID", "Alia-Stone-PvP-EU")
            self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"), 1)

    def test_oversize_packets_do_not_spend_receive_budget(self):
        c = boot()
        c.in_raid = True
        packet = "P1|1|1|1|" + "x" * 232
        self.assertEqual(len(packet), 241)
        for _ in range(2401):
            c.fire("CHAT_MSG_ADDON", "RLXR1", packet, "RAID", "Alia-Stone-PvP-EU")
        body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('join'))")
        c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|2|1|1|" + body, "RAID", "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"), 1)
        self.assertIsNone(c.run("return RelaxRaid.modules.Wire.stats.receiveLimited"))

    def test_per_peer_minute_cap_boundary_and_recovery(self):
        c = boot()
        c.in_raid = True
        body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('join'))")
        for round_id, duplicates in enumerate((59, 60, 120, 59), 1):
            for _ in range(duplicates):
                c.fire("CHAT_MSG_ADDON", "RLXR1", f"P1|{round_id}|1|2|" + body[:20], "RAID", "Alia-Stone-PvP-EU")
            before = c.run("return RelaxRaid.modules.Wire.stats.received or 0")
            c.fire("CHAT_MSG_ADDON", "RLXR1", f"P1|{round_id}|2|2|" + body[20:], "RAID", "Alia-Stone-PvP-EU")
            self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received or 0"), before + (duplicates == 59))
            if duplicates >= 60:
                self.assertGreater(c.run("return RelaxRaid.modules.Wire.stats.receiveLimited or 0"), 0)
            c.advance(60)

    def test_global_minute_cap_precedes_peer_admission(self):
        c = boot()
        c.in_raid = True
        body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('join'))")
        for round_id in range(1, 4):
            before = c.run("return RelaxRaid.modules.Wire.stats.received or 0")
            for _ in range(2400):
                c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|0|1|1|x", "RAID", "Bram-EU")
            c.fire("CHAT_MSG_ADDON", "RLXR1", f"P1|{round_id}|1|1|" + body, "RAID", "Alia-Stone-PvP-EU")
            self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received or 0"), before)
            self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.receiveLimited"), round_id)
            c.advance(60)
        c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|9|1|1|" + body, "RAID", "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"), 1)

    def test_different_reasons_still_wait_ten_seconds(self):
        c = boot()
        c.in_raid = True
        c.fire("READY_CHECK")
        c.advance(1)
        self.assertEqual(len(c.sent), 1)
        c.fire("ENCOUNTER_END")
        c.advance(9.9)
        self.assertEqual(len(c.sent), 1)
        c.advance(0.2)
        self.assertEqual(len(c.sent), 2)
        self.assertGreaterEqual(c.send_times[1] - c.send_times[0], 10)

    def test_fragment_spacing_uses_read_clock_not_timer_assumption(self):
        c = boot()
        c.in_raid = True
        c.bags = {0: [{"itemID": i, "stackCount": 999} for i in range(901, 933)]}
        c.item_types = {i: (0, 3) for i in range(901, 933)}
        c.fire("READY_CHECK")
        c.advance(1)
        self.assertEqual(len(c.sent), 1)
        # The timer can fire while the readable elapsed clock has regressed.
        c.run("GetTime=function() return 1.1 end")
        c.advance(1)
        self.assertEqual(len(c.send_attempts), 1)
        c.lua.globals().GetTime = lambda: c.now
        c.advance(5)
        self.assertGreater(len(c.sent), 1)

    def test_assembled_limit_precedes_decode(self):
        for size in (4096, 4097, 4200):
            c = boot()
            c.in_raid = True
            c.run("""
                DECODE_SIZES={}
                local W=RelaxRaid.modules.Wire
                local decode=W.decode
                W.decode=function(body) DECODE_SIZES[#DECODE_SIZES+1]=#body; return decode(body) end
            """)
            body = "x" * size
            parts = [body[i:i+200] for i in range(0, size, 200)]
            for part, chunk in enumerate(parts, 1):
                c.fire("CHAT_MSG_ADDON", "RLXR1", f"P1|1|{part}|{len(parts)}|" + chunk, "RAID", "Bram-EU")
            self.assertEqual(list(c.lua.globals().DECODE_SIZES.values()), [size] if size == 4096 else [])
            self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.invalid"), 1)
            self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)

    def test_replay_drop_is_counted_without_overwrite(self):
        c = boot()
        c.in_raid = True
        body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('join'))")
        packet = "P1|1|1|1|" + body
        c.fire("CHAT_MSG_ADDON", "RLXR1", packet, "RAID", "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"), 1)
        c.advance(1)
        c.fire("CHAT_MSG_ADDON", "RLXR1", packet, "RAID", "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.replay"), 1)
        self.assertEqual(c.run("return RelaxRaidDB.prep.nights[1].windows[1].reports['alia-stone-pvp-eu'].receivedAt"), c.wall)

    def test_outside_raid_cannot_seed_receive_assembly(self):
        c = boot()
        body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('join'))")
        c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|1|1|2|" + body[:20], "RAID", "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.peerCount()"), 0)
        c.in_raid = True
        c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|1|2|2|" + body[20:], "RAID", "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
        c.fire("CHAT_MSG_ADDON", "RLXR1", "P1|2|1|1|" + body, "RAID", "Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"), 1)

    def test_unknown_body_version(self):
        c = boot()
        body = c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('join'))")
        self.assertIsNone(c.lua.globals().RelaxRaid.modules.Wire.decode("2" + body[1:]))

    def test_bad_prefix_registration_is_silent(self):
        for status in (2, 3, None, True):
            c = Client()
            c.prefix_result = status
            c.load_addon(str(ADDON))
            c.in_raid = True
            c.fire("READY_CHECK")
            c.advance(5)
            self.assertEqual(c.send_attempts, [])

    def test_secret_incoming_fields_and_malformed_packets(self):
        for field in range(4):
            c = boot()
            c.in_raid = True
            values = ["RLXR1", "P1|1|1|1|x", "RAID", "Cato-PvP-EU"]
            values[field] = c.secret()
            c.fire("CHAT_MSG_ADDON", *values)
            self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
            self.assertIsNone(c.run("return next(RelaxRaid.errors)"))
        c=boot()
        c.in_raid=True
        import random
        rng=random.Random(901)
        for _ in range(400):
            payload="".join(chr(rng.randrange(32,127)) for _ in range(rng.randrange(0,241)))
            c.fire("CHAT_MSG_ADDON", "RLXR1", payload, "RAID", "Cato-PvP-EU")
        self.assertEqual(c.run("return #RelaxRaidDB.prep.nights"), 0)
        self.assertIsNone(c.run("return next(RelaxRaid.errors)"))

    def test_send_rolling_minute_budget(self):
        c=boot()
        c.in_raid=True
        c.run("""
            local P=RelaxRaid.modules.Prep
            local build=P.build
            RelaxRaid.modules.Wire.CHUNK=80 -- valid tighter fragmentation exceeds the minute budget without its guard
            P.build=function(reason)
                local r=build(reason)
                r.consumables={}; r.gear={}; r.notScored={}
                for j=1,32 do r.consumables[j]={category='enhancement',itemID=999999999-j,count=999999} end
                for j=1,19 do r.gear[j]={slot=j,wornID=999999998,bagID=999999999} end
                return r
            end
        """)
        for second in range(120):
            if second % 30 == 0: c.fire("READY_CHECK")
            if second % 30 == 10: c.fire("ENCOUNTER_END")
            c.advance(1)
        self.assertLessEqual(max(int(row[1].split("|", 4)[3]) for row in c.sent), 21)
        peak=max(sum(t-60 < s <= t for s in c.send_times) for t in c.send_times)
        self.assertEqual(peak, 60)
        self.assertIsNone(c.run("return next(RelaxRaid.errors)"))

    def test_secret_queue_clock_does_not_poison_next_trigger(self):
        c=boot()
        c.in_raid=True
        c.run("GetTime=function() return secret() end")
        c.fire("READY_CHECK")
        c.lua.globals().GetTime=lambda: c.now
        c.advance(2)
        c.fire("READY_CHECK")
        c.advance(3)
        self.assertGreater(len(c.sent),0)
        self.assertIsNone(c.run("return next(RelaxRaid.errors)"))

    def test_expiry_cannot_overfill_replay_ledger(self):
        c=boot()
        c.in_raid=True
        body=c.run("return RelaxRaid.modules.Wire.encode(RelaxRaid.modules.Prep.build('join'))")
        for i in range(1,8):
            c.fire("CHAT_MSG_ADDON","RLXR1",f"P1|{i}|1|1|"+body,"RAID","Alia-Stone-PvP-EU")
        c.fire("CHAT_MSG_ADDON","RLXR1","P1|8|1|2|"+body[:10],"RAID","Alia-Stone-PvP-EU")
        c.advance(31)
        c.fire("CHAT_MSG_ADDON","RLXR1","P1|9|1|1|"+body,"RAID","Alia-Stone-PvP-EU")
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.received"),7)
        self.assertEqual(c.run("return RelaxRaid.modules.Wire.stats.expiredReceive"),1)


if __name__ == "__main__":
    unittest.main()
