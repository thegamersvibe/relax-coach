#!/usr/bin/env python3
"""Executable negative controls: mutate in memory, never edit product files."""
from pathlib import Path
import sys, unittest
from unittest.mock import patch
import lupa.lua51 as lupa
sys.path.insert(0,str(Path(__file__).resolve().parent))
import cleanup_policy_test as policy_tests
from cleanup_harness import ADDON

class CleanupMutationTests(unittest.TestCase):
    def test_candidate_wording_candidate_label_and_absence_logic_are_caught(self):
        source=ADDON/'Core/Cleanup.lua'
        original=source.read_text()
        mutants=[
            ("['no-use-found']='No use found in our data'", "['no-use-found']='Can be deleted'",
             'test_furen_no_use_is_information_not_permission_and_sorted_first'),
            ("table.sort(report.itemRows,function(a,b)",
             "for _,r in ipairs(report.itemRows) do if r.reasonCodes[1]=='no-use-found' then r.disposition='CONSIDER MANUAL CLEANUP' end end\n    table.sort(report.itemRows,function(a,b)",
             'test_furen_no_use_is_information_not_permission_and_sorted_first'),
            ("else uncertain(row,'no-use-found') end", "else keep(row,'known') end",
             'test_shared_all_stacks_and_no_import_absence_lowering'),
            ("finish(row); report.itemRows[#report.itemRows+1]=row",
             "if not any then row.disposition='UNCERTAIN-KEEP'; row.reasonCodes={'no-use-found'}; row.reasons={C.Reasons['no-use-found']} end\n            finish(row); report.itemRows[#report.itemRows+1]=row",
             'test_shared_all_stacks_and_no_import_absence_lowering'),
            ("if fact~=nil then", "if fact~=nil and block~='tool' then",
             'test_furen_no_use_is_information_not_permission_and_sorted_first'),
        ]
        read=Path.read_text
        for old,new,test in mutants:
            with self.subTest(mutant=new):
                self.assertEqual(original.count(old),1)
                mutant=original.replace(old,new,1)
                lupa.LuaRuntime(unpack_returned_tuples=True).execute('local f,e=loadstring(...); assert(f,e)',mutant)
                def changed(path,*args,**kwargs):
                    return mutant if path==source else read(path,*args,**kwargs)
                result=unittest.TestResult()
                with patch.object(Path,'read_text',changed): policy_tests.CleanupPolicyTests(test).run(result)
                self.assertFalse(result.wasSuccessful(),'mutant survived')
                failures='\n'.join(message for _,message in result.errors+result.failures)
                self.assertIn('assertion failed',failures,failures)

if __name__=='__main__': unittest.main(verbosity=2)
