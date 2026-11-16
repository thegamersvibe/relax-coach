#!/usr/bin/env python3
"""Source safety gate: identifiers are rejected even when aliased or quoted."""
from pathlib import Path
import re,sys,unittest
ROOT=Path(__file__).resolve().parents[2]
ADDON=ROOT/'addon/RelaxQuestPlanner'
FEATURE=[ADDON/('Core/'+n+'.lua') for n in ('CleanupData','CleanupSnapshot','CleanupUses','Cleanup')]+[ADDON/'UI/Cleanup.lua']
INTEGRATION=[ADDON/p for p in ('Init.lua','Core/Data.lua','UI/Window.lua','UI/Controller.lua')]+[ROOT/'addon/ForeverCoach/Audit.lua']
DENY=re.compile(r'\b(?:DeleteCursorItem|DestroyItem|DeleteItem|AbandonQuest|SetAbandonQuest|'
                r'Pickup\w*|SplitContainerItem|UseContainerItem|UseItem\w*|SellCursorItem|'
                r'UseAction|UseInventoryItem|BNSend\w*|Send\w*|ClickSendMailButton|TakeInbox\w*|AutoLootMailItem|'
                r'PutItemIn\w*|EquipItemByName|AutoEquipCursorItem|ClearCursor|'
                r'AcceptQuest|CompleteQuest|GetQuestReward|RunMacro\w*|SecureActionButtonTemplate|SetAttribute)\b')

def violations(text):
    return sorted(set(DENY.findall(text)))

class CleanupStaticTests(unittest.TestCase):
    def test_no_delete_abandon_pickup_use_sell_move_or_send_api_anywhere_in_feature(self):
        for path in FEATURE+INTEGRATION:
            with self.subTest(file=path.relative_to(ROOT)):
                self.assertTrue(path.is_file())
                self.assertEqual(violations(path.read_text()),[],str(path))
        for path in FEATURE:
            text=path.read_text()
            self.assertNotRegex(text,r'_G\s*\[')  # no computed global dispatch
            self.assertNotRegex(text,r'\b(?:loadstring|loadfile|dofile)\s*\(')
            self.assertNotRegex(text,r'(?:RelaxJourneyAPI|Journey)\s*[.:]')

    def test_gate_rejects_qualified_calls_aliases_and_string_dispatch(self):
        for text in ('C_Container.PickupContainerItem (0,1)', 'AbandonQuest\n()',
                     'local action = DeleteCursorItem', '_G["SendChatMessage"]("x")',
                     'C_ChatInfo.SendAddonMessage("x","y","GUILD")','C_Item.UseItemByName(1)',
                     'BNSendWhisper(1,"x")','C_FriendList.SendWho("x")',
                     'frame:SetAttribute("type","macro")'):
            self.assertTrue(violations(text),text)

    def test_toc_has_exact_feature_order_and_no_profiles_or_extra_dependencies(self):
        toc=(ADDON/'RelaxQuestPlanner.toc').read_text()
        names=[p.relative_to(ADDON).as_posix() for p in FEATURE]
        offsets=[toc.index(n) for n in names]
        self.assertEqual(offsets,sorted(offsets))
        self.assertFalse((ADDON/'Core/CleanupProfiles.lua').exists())
        self.assertNotIn('KeepOrToss',toc)
        self.assertNotRegex('\n'.join(p.read_text() for p in FEATURE),r'CONSIDER MANUAL CLEANUP|reviewed-obsolete')

if __name__=='__main__': unittest.main(verbosity=2)
