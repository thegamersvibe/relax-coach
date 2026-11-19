"""R2 wire boundary tests: real events and client-send attempts, never real names."""
from pathlib import Path
import sys
import unittest
ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "test/raid/_client"))
from client import Client
ADDON = ROOT / "addon/RelaxRaid"


def boot(saved=None):
    c = Client()
    c.guild = ("Fake Guild", "Rank F", 5)
    c.load_addon(str(ADDON), saved)
    return c


class WireTests(unittest.TestCase):
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
        self.assertEqual(c.run("return RelaxRaidDB.prep.nights[1].windows[1].reports['alia-stone-pvp-eu'].report.name"),
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
                r.consumables={}; r.gear={}
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
