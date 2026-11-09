"""Opt-in/private regression: real decoded catalog + real Journey Lua 5.1.
RQP_PRIVATE_DATA points at quest-db-sources; RQP_PICKER_REFERENCE points at the
private research prototype.py. No decoded data, keys or account tags are saved.
"""
import pathlib as _p, sys as _s; _s.path.insert(0, str(_p.Path(__file__).resolve().parent))
import importlib.util
import json
import os
from pathlib import Path
import re
import sys
import time
import unittest
import zlib
from journey_harness import runtime, load_rqp, table, ROOT, JOURNEY

sys.dont_write_bytecode = True
PRIVATE = Path(os.environ.get('RQP_PRIVATE_DATA') or '/nonexistent-rqp-private-data')
REFERENCE = Path(os.environ.get('RQP_PICKER_REFERENCE') or '/nonexistent-rqp-picker-reference')
CONTENT = PRIVATE / 'installed-2026-11-09/Content/Journey-Content'
AVAILABLE = (CONTENT / 'guides.html').is_file() and REFERENCE.is_file() and (PRIVATE / 'snapshot/snapshot.json').is_file()


def private_fixture(with_engine=False):
    spec = importlib.util.spec_from_file_location('private_picker_reader', REFERENCE)
    assert spec is not None and spec.loader is not None
    parser = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(parser)
    catalog = parser.parse(parser.decode(CONTENT / 'guides.html'), signed=True)
    key = ''.join(sorted(s.lower() for s in parser.array(catalog.get('battleTags', {})))).encode()
    infos = [g for g in parser.array(catalog) if isinstance(g, dict) and 'guideID' in g]
    payloads = {}
    for g in infos:
        if g.get('stub') or not g.get('slot'):
            continue
        raw = parser.decode(CONTENT / f'{g["slot"]:04d}.html', key if g.get('encrypted') else None)
        actual = f'{zlib.adler32(raw.encode()) & 0xffffffff:08x}'
        assert not g.get('contentHash') or actual == g['contentHash'], 'Private guide hash mismatch'
        payloads[g['guideID']] = parser.parse(raw)
    lua, env = runtime()
    env.DEBUG_VALIDATOR = False
    env.INFOS = table(lua, infos)
    # Signed admission/transport is tested separately. This hook supplies only
    # authenticated-offline payloads to the real, patched normalization path.
    lua_payloads = {gid: table(lua, data) for gid, data in payloads.items()}
    env.BUILD = lambda template: lua_payloads.get(template.removeprefix('template:'))
    snap = json.loads((PRIVATE / 'snapshot/snapshot.json').read_text())
    for q in snap['completed']:
        lua.globals().STATES[q] = 3
    for entry in snap['log']:
        lua.globals().STATES[entry['id']] = 2 if entry['complete'] else 1
    player = {'level': snap['level'], 'faction': snap['faction'], 'raceMask': 4, 'classMask': 1,
              'class': snap['class'], 'done': {q: True for q in snap['completed']},
              'log': {q['id']: True for q in snap['log']},
              'ready': {q['id']: True for q in snap['log'] if q['complete']}, 'trivial': snap['trivialRange']}
    lua.globals().PLAYER_FIXTURE = table(lua, player)
    lua.execute("RelaxQuestPlanner.Player={Snapshot=function()return PLAYER_FIXTURE end}")
    if with_engine:
        source = (PRIVATE / 'QuestieDB/src/corrections/Forever/combined/foreverQuestDB.lua').read_text()
        fields = {key: int(index) for index, key in re.findall(r'^-- \[(\d+)\] = (\w+)', source, re.M)}
        lua.globals().QUEST_ROWS = lua.execute(source)
        lua.globals().QUEST_FIELDS = table(lua, fields)
        lua.execute('''
            LibQuestieDB={RequireContract=function()return true end,Enum={},Quest={}}
            function LibQuestieDB.Quest.Get(q,key)
                local row=QUEST_ROWS[q]
                return row and row[QUEST_FIELDS[key]]
            end
            function LibQuestieDB.Quest.Exists(q)return QUEST_ROWS[q]~=nil end
            function LibQuestieDB.Quest.GetAllIds()
                local ids={} for id in pairs(QUEST_ROWS) do ids[#ids+1]=id end table.sort(ids) return ids
            end
        ''')
        for name in ('Data', 'Engine'):
            lua.execute((ROOT / f'addon/RelaxQuestPlanner/Core/{name}.lua').read_text())
    load_rqp(lua, 'Bridge.lua')
    load_rqp(lua, 'Picker.lua')
    lua.execute('function PickCallback(r,e) PICK_RESULT=r; PICK_ERROR=e end')
    return lua, env, infos, snap


def run_pick(lua):
    lua.globals().RelaxQuestPlanner.Journey.PickGuide(lua.globals().PickCallback)
    frames = 0
    max_ms = 0
    while lua.globals().PICK_RESULT is None and frames < 10000:
        before = time.perf_counter()
        lua.globals().Tick()
        max_ms = max(max_ms, (time.perf_counter() - before) * 1000)
        frames += 1
    assert lua.globals().PICK_RESULT is not None, 'async callback missing'
    assert lua.globals().PICK_ERROR is None, lua.globals().PICK_ERROR
    return lua.globals().PICK_RESULT, frames, max_ms


@unittest.skipUnless(AVAILABLE, 'Private Journey content/reference/snapshot not installed')
class JourneyPrivate(unittest.TestCase):
    def test_snapshot_real_guides_strict_first_undone_fallback(self):
        lua, env, infos, snap = private_fixture()
        result, frames, max_ms = run_pick(lua)
        self.assertEqual(len(result), 3)
        info_by_id = {g['guideID']: g for g in infos}
        done = set(snap['completed'])
        log = {q['id'] for q in snap['log']}
        ready = {q['id'] for q in snap['log'] if q['complete']}
        report = []
        for choice in result.values():
            info = info_by_id[choice.guideID]
            self.assertGreaterEqual(int(info['levels'].split('-')[1]), 25)
            self.assertEqual(info['faction'], 'Alliance')
            self.assertTrue(choice.provisional)
            scan = lua.globals().RelaxJourneyAPI.ScanGuideQuests(choice.guideID)
            first = None
            for step in scan.steps.values():
                for a in step.actions.values():
                    if a.active is not True or a.completed or a.optional or a.questID in done:
                        continue
                    if a.alternatives and any(q in done | log for q in a.alternatives.values()):
                        continue
                    if ((a.kind == 'accept' and a.questID not in log) or
                        (a.kind == 'complete' and a.questID in log - ready) or
                        (a.kind == 'turnin' and a.questID in ready)):
                        first = step.id
                        break
                if first:
                    break
            self.assertEqual(choice.startStepID, first)
            report.append((choice.guideID, info['levels'], choice.score, first))
        # Without prerequisite/work-level data, only the ~27-35 route-band fit
        # is supportable; do not pretend a specific zone is verified available.
        self.assertTrue(all(27 <= int(info_by_id[c.guideID]['levels'].split('-')[1]) <= 35 for c in result.values()))
        print('PRIVATE FALLBACK', report, 'frames', frames, 'max-frame-ms', round(max_ms, 3))
        self.assertIsNone(env.GuideNavigationService.GetGuide())

    def test_snapshot_with_real_lua_engine_and_questdb(self):
        path = PRIVATE / 'QuestieDB/src/corrections/Forever/combined/foreverQuestDB.lua'
        if not path.is_file():
            self.skipTest('Private QuestieDB not installed')
        lua, env, infos, _ = private_fixture(with_engine=True)
        result, frames, max_ms = run_pick(lua)
        self.assertEqual(len(result), 3)
        info_by_id = {g['guideID']: g for g in infos}
        report = []
        P = lua.globals().PLAYER_FIXTURE
        rqp = lua.globals().RelaxQuestPlanner
        for choice in result.values():
            info = info_by_id[choice.guideID]
            self.assertGreaterEqual(int(info['levels'].split('-')[1]), 25)
            self.assertFalse(choice.provisional)
            scan = lua.globals().RelaxJourneyAPI.ScanGuideQuests(choice.guideID)
            first = None
            for step in scan.steps.values():
                for a in step.actions.values():
                    q = a.questID
                    row = rqp.Data.Quest(q)
                    if not row or row.qLevel < 25 or a.active is not True or a.completed or a.optional or P.done[q]:
                        continue
                    state, _ = rqp.Engine.State(q, P)
                    if a.alternatives and any(P.done[x] or P.log[x] for x in a.alternatives.values()):
                        continue
                    if ((a.kind == 'accept' and state == 'available' and not P.log[q]) or
                        (a.kind == 'complete' and P.log[q] and not P.ready[q]) or
                        (a.kind == 'turnin' and P.log[q] and P.ready[q])):
                        first = step.id
                        break
                if first:
                    break
            self.assertEqual(choice.startStepID, first)
            report.append((choice.guideID, info['levels'], choice.score, first))
        self.assertTrue(any(any(zone in info_by_id[c.guideID]['description'] for zone in ('Stranglethorn', 'Arathi', 'Redridge')) for c in result.values()))
        print('PRIVATE ENGINE', report, 'frames', frames, 'max-frame-ms', round(max_ms, 3))
        self.assertIsNone(env.GuideNavigationService.GetGuide())

    def test_horde_30_snapshot_never_loads_stub_content(self):
        lua, env, infos, _ = private_fixture()
        lua.execute("STATES={}; PLAYER_FIXTURE={level=30,faction='Horde',done={},log={},ready={},trivial=7}")
        env.ConditionContext.ALLIANCE = None
        env.ConditionContext.DWARF = None
        env.ConditionContext.HORDE = True
        env.ConditionContext.ORC = True
        result, frames, _ = run_pick(lua)
        stubs = {g['guideID'] for g in infos if g.get('stub') and g.get('faction') == 'Horde'}
        self.assertGreater(len(stubs), 0)
        self.assertFalse({c.guideID for c in result.values()} & stubs)
        self.assertFalse({c.template.removeprefix('template:') for c in lua.globals().LOAD_CALLS.values()} & stubs)
        print('PRIVATE HORDE', [(c.guideID, c.startStepID) for c in result.values()], 'stub-only-30+ excluded', len(stubs), 'frames', frames)


if __name__ == '__main__':
    unittest.main(verbosity=2)
