#!/usr/bin/env python3
"""Literal-only data import gate. Set CLEANUP_USES_ZIP to the pinned archive."""
import hashlib, importlib.util, os, sys, unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'tools'))

class CleanupUsesTests(unittest.TestCase):
    def module(self):
        path=ROOT/'tools/gen_cleanup_uses.py'
        self.assertTrue(path.exists(), 'data generator is missing')
        spec=importlib.util.spec_from_file_location('gen_cleanup_uses',path)
        module=importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
        return module

    def test_pinned_archive_regenerates_byte_identical_data_and_counts(self):
        g=self.module()
        self.assertIn('CLEANUP_USES_ZIP',os.environ,'set the pinned archive path (no silent skip)')
        archive=Path(os.environ['CLEANUP_USES_ZIP'])
        text=g.generate(archive)
        self.assertEqual(text,(ROOT/'addon/RelaxQuestPlanner/Core/CleanupUses.lua').read_text())
        blocks=g.validate(text)
        self.assertEqual(blocks['tool'][7667],True)
        self.assertNotIn(6842,{k for n,b in blocks.items() if n!='vanilla' for k in b})
        self.assertEqual(len(blocks['reagent']),594)
        self.assertEqual(len(blocks['teaches']),1978)
        self.assertEqual(len(blocks['known']),53)
        self.assertEqual(len(blocks['starts']),554)
        self.assertEqual(len(blocks['vanilla']),1764)

    def test_literal_grammar_rejects_code_duplicate_keys_and_aliases(self):
        g=self.module()
        for text in ['ns.X = { f() }','ns.X = {1+2}','ns.X = {[1]=1,[1]=2}',
                     'ns.X = {"x" .. "y"}','print("x")','ns.X={}; ns.X={}',
                     'ns.X={unknown}','ns.X=setmetatable({}, {})']:
            with self.subTest(text=text),self.assertRaises((ValueError,g.ParseError)):
                g.parse(text)
        self.assertEqual(g.parse('local _, ns = ...\nlocal L = ns.L\nlocal A=L["x"]\nns.X={nil,3,{kind=A}}'),
                         {'X':{1:None,2:3,3:{'kind':'x'}}})

    def test_generated_gate_rejects_code_missing_header_and_uncredited_data(self):
        g=self.module(); text=(ROOT/'addon/RelaxQuestPlanner/Core/CleanupUses.lua').read_text()
        for bad in [text+'\nprint("oops")',text.replace('GENERATED, do not edit','no provenance',1),
                    text.replace('ns.tool =','ns.other =',1)]:
            with self.subTest(tail=bad[-60:]), self.assertRaises(ValueError): g.validate(bad)
        self.assertIn('Keep or Toss: Forever 1.7.0',(ROOT/'THIRD_PARTY.md').read_text())
        self.assertIn('data tables only, no code',(ROOT/'THIRD_PARTY.md').read_text())

    def test_wrong_zip_hash_refused_before_parsing(self):
        g=self.module()
        with self.assertRaisesRegex(ValueError,'hash'): g.generate(ROOT/'THIRD_PARTY.md')

if __name__=='__main__': unittest.main(verbosity=2)
