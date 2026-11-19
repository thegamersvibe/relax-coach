"""Character-shaped public fixtures. All quest/entity IDs and text are invented.

The shapes cover T1a/T1b/T1c/T2/T3/T4 without storing any donor records or saves.
"""
import pathlib as _p, sys as _s; _s.path.insert(0, str(_p.Path(__file__).resolve().parent))
import unittest
from journal_test import Harness, ROOT, A, B, C, T, OUTSIDE, INSIDE, OUT_NPC, IN_NPC, q, to_lua, array, action


def quest(name, inside=False, pickup=OUT_NPC, turnin=OUT_NPC, **kw):
    # Instance-work fixtures need an objective, not only a dungeon category.
    kw.setdefault('objectives', {1: [[IN_NPC]]} if inside else {})
    kw.setdefault('zoneOrSort', INSIDE if inside else OUTSIDE)
    return q(name, questLevel=30,
             startedBy={1: [pickup]} if pickup else {}, finishedBy={1: [turnin]} if turnin else {}, **kw)


def entries(rows, **overrides):
    return [dict(id=id, name=row['name'], **overrides.get(str(id), {})) for id,row in rows.items()]


class DungeonRouteTests(unittest.TestCase):
    def check_route(self, h):
        before = h.states()
        route = h.route()
        self.assertEqual(h.states(), before, 'tailoring and hints must not alter any quest state')
        self.assertFalse(any(i[1] == 'actionCycle' for i in array(route.issues)))
        blocks = [s.block for s in array(route.steps)]
        self.assertTrue(all(b in (1,2,3,4) for b in blocks), 'every action needs a route block')
        self.assertEqual(blocks, sorted(blocks))
        return route

    def test_outside_talk_leads_do_not_force_a_second_run(self):
        # Dungeon-tagged delivery breadcrumbs, but both NPCs are in cities.
        # Follow-ups have real instance objectives, not just a dungeon category.
        rows = {
            A: quest('Copper introduction', True, nextQuestInChain=B, objectives={}),
            B: quest('Tin components', True, preQuestGroup=[A], objectives={1: [[IN_NPC]]}),
            C: quest('Zinc introduction', True, nextQuestInChain=T, objectives={1: [], 3: []}),
            T: quest('Alloy records', True, preQuestGroup=[C], objectives={1: [[IN_NPC]]}),
        }
        js = entries({B: rows[B], T: rows[T]})
        for log in ([], [dict(id=A, complete=False)], [dict(id=A, complete=True)]):
            with self.subTest(log=log):
                h = Harness(rows, js, level=31, log=log)
                route = self.check_route(h)
                for lead, target in ((A, B), (C, T)):
                    self.assertEqual(action(route, 'turnin', lead).block, 1)
                    for s in array(route.steps):
                        if s.q == lead:
                            self.assertEqual((s.where, s.block), ('outside', 1))
                    self.assertEqual(action(route, 'accept', target).block, 1)
                    self.assertEqual(action(route, 'complete', target).block, 2)
                    self.assertEqual(action(route, 'turnin', target).block, 3)
                self.assertFalse(any(s.block == 4 for s in array(route.steps)))

    # M2: a quest with no objective rows is not always a talk quest. Order: objective rows, then
    # QuestieDB's end trigger zone, then the planner's inside list, then the talk-quest fallback.
    def m2(self, trigger=None, listed=None):
        kw = dict(objectives={})
        if trigger is not None: kw['triggerEnd'] = ['Escort the copper keeper out', {trigger: [[50, 50]]}]
        rows = {A: quest('Copper escort', True, pickup=IN_NPC, turnin=OUT_NPC, **kw)}
        h = Harness(rows, entries(rows), level=31)
        if listed is not None:
            h.lua.execute('RelaxQuestPlanner.PlaceInside={[%d]=%s}' % (A, listed))
            h.r.Data.Reset()
        return self.check_route(h)

    def test_m2_inside_trigger_means_inside_work(self):
        route = self.m2(trigger=INSIDE)
        s = action(route, 'complete', A)
        self.assertEqual((s.where, s.block), ('inside', 2))
        self.assertEqual(action(route, 'accept', A).block, 2)
        self.assertEqual(action(route, 'turnin', A).block, 3)

    def test_m2_outside_trigger_means_outside_work(self):
        s = action(self.m2(trigger=OUTSIDE), 'complete', A)
        self.assertEqual(s.where, 'outside')

    def test_m2_inside_list_for_quests_without_rows_or_trigger(self):
        s = action(self.m2(listed='TEST_DUNGEON'), 'complete', A)
        self.assertEqual((s.where, s.block), ('inside', 2))
        # Listed for another dungeon: not inside this one; the talk fallback places it by its turn-in.
        s = action(self.m2(listed='"Another Keep"'), 'complete', A)
        self.assertEqual(s.where, 'outside')
        # A trigger wins over the list.
        h_route = self.m2(trigger=OUTSIDE, listed='TEST_DUNGEON')
        self.assertEqual(action(h_route, 'complete', A).where, 'outside')

    def test_m2_talk_quest_fallback_unchanged(self):
        s = action(self.m2(), 'complete', A)
        self.assertEqual(s.where, 'outside', 'no rows, no trigger, not listed: by its turn-in')

    def test_inside_list_is_data_only(self):
        src = (ROOT / 'addon/RelaxQuestPlanner/Core/PlaceData.lua').read_text()
        code = [l for l in src.splitlines() if l.strip() and not l.lstrip().startswith('--')]
        self.assertEqual(code[:2], ['RelaxQuestPlanner = RelaxQuestPlanner or {}', 'RelaxQuestPlanner.PlaceInside = {'])
        self.assertEqual(code[-1], '}')
        import re
        for line in code[2:-1]:
            self.assertRegex(line, r'^    \[\d+\] = "[^"|\\]+",$')
        toc = (ROOT / 'addon/RelaxQuestPlanner/RelaxQuestPlanner.toc').read_text().splitlines()
        self.assertEqual(toc.index('Core/PlaceData.lua'), toc.index('Core/Data.lua') + 1)

    def test_holiday_repeatable_in_the_log_stays_out(self):
        rows = {A: quest('Copper festival', True, zoneOrSort=-22, specialFlags=1)}
        h = Harness(rows, entries(rows), level=31, log=[dict(id=A, complete=False)])
        route = h.route()
        self.assertFalse(any(s.q == A for s in array(route.steps)), 'a holiday quest never enters the plan')
        # The same repeatable outside a holiday stays when it is in the log (S1).
        rows = {A: quest('Copper chore', True, specialFlags=1)}
        h = Harness(rows, entries(rows), level=31, log=[dict(id=A, complete=False)])
        self.assertTrue(any(s.q == A for s in array(h.route().steps)))

    # An unknown-location step takes the block of what it depends on and never raises it, so it
    # cannot push the rest of the route into "Next run" by itself.
    def test_unknown_location_never_raises_the_block(self):
        rows = {
            A: quest('Copper whisper', True, pickup=None, turnin=None, objectives={}, nextQuestInChain=B),
            B: quest('Tin vault', True, preQuestGroup=[A], objectives={1: [[IN_NPC]]}),
        }
        h = Harness(rows, entries(rows), level=31)
        route = self.check_route(h)
        for kind in ('accept', 'turnin'):
            s = action(route, kind, A)
            self.assertEqual((s.where, s.block), ('unknown', 1), kind)
            self.assertIn('location not confirmed', s.note)
        self.assertEqual(action(route, 'accept', B).block, 1)
        self.assertEqual(action(route, 'complete', B).block, 2, 'the inside work stays in this run')
        self.assertEqual(action(route, 'turnin', B).block, 3)
        self.assertFalse(any(s.block == 4 for s in array(route.steps)))

    def test_unknown_location_inherits_an_inside_parent(self):
        # Picked up inside, turn-in place unknown: the turn-in stays with the run (block 2), not after it.
        rows = {A: quest('Copper relic', True, pickup=IN_NPC, turnin=None)}
        route = self.check_route(Harness(rows, entries(rows), level=31))
        self.assertEqual(action(route, 'complete', A).block, 2)
        s = action(route, 'turnin', A)
        self.assertEqual((s.where, s.block), ('unknown', 2))

    def test_t1a_active_inside_turnin_and_two_missing_starters(self):
        rows = {
            A: quest('Copper search', True, turnin=None),
            B: quest('Tin rescue', True, pickup=None, preQuestGroup=[A]),
            C: quest('Zinc search', True, turnin=IN_NPC),
            T: quest('Alloy rescue', True, pickup=None),
            A+4: quest('Bronze preparation'),
            A+5: quest('Silver prism', True, preQuestGroup=[A+4]),
        }
        js = entries(rows, **{str(A): dict(turnin='Inside the copper vault'),
            str(B): dict(pickup='Inside the copper vault'), str(T): dict(pickup='Inside the copper vault'),
            str(A+4): dict(hideFromMainList=True), str(C): dict(hideFromMainList=True)})
        h = Harness(rows, js, {T: [dict(id=C, name='Zinc search')]}, level=31,
                    log=[dict(id=A, complete=True), dict(id=C, complete=False)])
        route = self.check_route(h)
        self.assertEqual(action(route, 'turnin', A).block, 2)
        self.assertEqual(action(route, 'accept', B).block, 2)
        self.assertEqual(action(route, 'accept', T).block, 2)
        self.assertEqual(action(route, 'accept', A+5).block, 1)
        self.assertTrue(all(s.block == 1 for s in array(route.steps) if s.q == A+4))
        order = [(s.kind,s.q) for s in array(route.steps)]
        self.assertLess(order.index(('turnin', A)), order.index(('accept', B)))
        self.assertLess(order.index(('turnin', C)), order.index(('accept', T)))
        for id in (B,T,A+5): self.assertEqual(action(route,'turnin',id).block,3)

    def test_t1b_no_area_two_hidden_leads_and_three_later_targets(self):
        rows = {A: quest('Copper introduction'), B: quest('Tin introduction', preQuestGroup=[A]),
                C: quest('Zinc introduction', preQuestGroup=[A]),
                T: q('Alloy expedition', preQuestGroup=[B]), A+4: q('Bronze expedition', preQuestGroup=[C]),
                A+5: q('Silver expedition', requiredLevel=34), A+6: q('Gold expedition', requiredLevel=34),
                A+7: q('Iron expedition', requiredLevel=34), A+8: q('Nickel expedition')}
        js = entries(rows, **{str(A):dict(hideFromMainList=True,pickup='At Copper City',turnin='At Copper City'),
            str(B):dict(hideFromMainList=True,pickup='At Copper City',turnin='At Copper City'),
            str(C):dict(hideFromMainList=True,pickup='At Copper City',turnin='At Copper City'), str(A+8):dict(faction='Horde')})
        h = Harness(rows, js, level=31)
        h.lua.execute('LibQuestieDB.Support=nil'); h.r.Data.Reset()
        route = self.check_route(h)
        self.assertEqual(route.targets, 5)
        self.assertEqual({s.q for s in array(route.later)}, {A+5,A+6,A+7})
        self.assertNotIn(A+8, array(route.plan))
        self.assertEqual(array(route.plan)[:3], [A,B,T])
        self.assertTrue(all(s.block==1 for s in array(route.steps) if s.q in (A,B,C)))

    def test_t1c_ten_targets_city_flag_conflict_and_extras(self):
        rows = {A+i: quest('Metal assignment '+str(i), True) for i in range(15)}
        rows[A] = quest('Copper ring', True, pickup=IN_NPC)
        rows[B] = quest('Tin appraisal', preQuestGroup=[A])
        rows[C] = quest('Zinc upgrade', True, preQuestGroup=[B])
        rows[T] = quest('Alloy mishap', True, pickup=IN_NPC)
        rows[A+4] = quest('Bronze machine', True, pickup=IN_NPC)
        js = entries({id:r for id,r in rows.items() if id<A+10}, **{str(C):dict(startsInDungeon=True,pickup='At Copper City')})
        h = Harness(rows, js, level=31, log=[dict(id=A+5,complete=False),dict(id=A+6,complete=False)])
        route = self.check_route(h)
        self.assertEqual(route.targets, 10); self.assertEqual(route.extraCount, 5)
        self.assertEqual(action(route,'accept',C).where,'outside')
        self.assertEqual(action(route,'accept',C).block,3)
        for id in (T,A+4): self.assertEqual(action(route,'accept',id).block,2)
        self.assertEqual(action(route,'complete',C).block,4)

    def test_t2_level_one_hunter_seven_later_no_paladin(self):
        rows = {A+i: quest('Metal warrant '+str(i),True,requiredLevel=15) for i in range(9)}
        for id in (A+7,A+8): rows[id]['requiredClasses']=2
        rows[A]['startedBy']={3:[882001]}
        h = Harness(rows, entries(rows), level=1, race='Human', raceID=1, **{'class':'HUNTER'})
        h.g.TEST_ITEMS[882001]=to_lua(h.lua,dict(name='Copper letter',npcDrops=[IN_NPC])); h.r.Data.Reset()
        route = self.check_route(h)
        self.assertEqual(route.targets,7); self.assertEqual(len(route.steps),0); self.assertEqual(len(route.later),7)
        self.assertEqual(next(s for s in array(route.later) if s.q==A).where,'inside')
        h.g.TEST_SNAPSHOT.level=35
        self.assertEqual(action(self.check_route(h),'turnin',A).block,3)

    def test_t3_mage_filters_and_inside_discs_to_city_chain(self):
        rows = {
            A: quest('Copper plates',True,pickup=IN_NPC,turnin=IN_NPC,requiredLevel=20),
            B: quest('Tin plates',True,pickup=IN_NPC,preQuestGroup=[A],requiredLevel=20),
            C: quest('Zinc report',preQuestGroup=[B],requiredLevel=20),
            T: quest('Alloy signal',requiredLevel=20),
            A+4: quest('Bronze guard',True,requiredLevel=20,requiredClasses=1),
            A+5: quest('Silver guard',True,requiredLevel=20,requiredClasses=2),
            A+6: quest('Gold guard',True,requiredLevel=20,requiredRaces=128),
        }
        h=Harness(rows,entries(rows,**{str(T):dict(turnin='Inside the copper vault')}),
                  level=1,race='Gnome',raceID=7,**{'class':'MAGE'})
        route=self.check_route(h)
        self.assertEqual(route.targets,4); self.assertEqual(len(route.steps),0)
        h.g.TEST_SNAPSHOT.level=35; route=self.check_route(h)
        self.assertEqual(action(route,'turnin',A).block,2)
        self.assertEqual(action(route,'accept',B).block,2)
        self.assertEqual(action(route,'turnin',B).block,3)
        self.assertEqual(action(route,'accept',C).block,3)
        self.assertEqual(action(route,'turnin',T).block,1)
        self.assertIn('sources disagree',action(route,'turnin',T).note)

    def test_t4_horde_lead_three_targets_and_two_neutral(self):
        rows={A+i:q('Metal expedition '+str(i)) for i in range(8)}
        rows[A]['nextQuestInChain']=B
        js=entries(rows,**{str(A):dict(faction='Horde',hideFromMainList=True),
            str(B):dict(faction='Horde'),str(C):dict(faction='Horde'),str(T):dict(faction='Horde'),
            str(A+6):dict(faction='Alliance'),str(A+7):dict(faction='Alliance')})
        h=Harness(rows,js,level=35,race='Troll',raceID=8,faction='Horde')
        h.lua.execute('LibQuestieDB.Support=nil'); h.r.Data.Reset()
        route=self.check_route(h)
        self.assertEqual(route.targets,5)
        self.assertEqual(set(array(route.plan)),{A,B,C,T,A+4,A+5})

    def test_character_fixtures_are_in_the_ci_loop(self):
        workflow=(ROOT/'.github/workflows/ci.yml').read_text()
        planner=workflow.split('- name: Relax Quest Planner tests',1)[1].split('- name:',1)[0]
        self.assertIn('dungeon_route_test',planner)
        self.assertIn('journal_test',planner)


if __name__=='__main__':
    unittest.main(verbosity=2)
