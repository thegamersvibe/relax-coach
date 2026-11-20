"""Synthetic Forever recap tests; native access/rendering is not certified."""
import json
from pathlib import Path
import runpy
import sys
import unittest

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'test/raid/_client'))
from client import Client

ADDON = ROOT / 'addon/RelaxRaid'


class RecapClient(Client):
    def __init__(self):
        super().__init__()
        self.run('''
            events = {}; maxHealth = 1000; reads = 0
            canaccessvalue = function(v) return not issecretvalue(v) end
            canaccesstable = function(v) return not issecretvalue(v) end
            C_DeathRecap = {
                HasRecapEvents=function() return #events > 0 end,
                GetRecapEvents=function() reads=reads+1; return events end,
                GetRecapMaxHealth=function() return maxHealth end
            }
            GetZoneText=function() return 'Test Zone' end
            GetGameTime=function() return 21,14 end
            GetPlayerInfoByGUID=function() return 'Mage' end
            Enum={CombatLogObject={ReactionHostile=64, ControlPlayer=256, TypePlayer=1024}}
        ''')

    def load_shared(self, name='RelaxRaid', version=None):
        path = ADDON / 'DeathRecap.lua'
        assert path.exists(), 'shared recap core is missing'
        src = path.read_text()
        if version is not None:
            src = src.replace('local VERSION = 1', f'local VERSION = {version}')
        self.lua.eval('function(src,name) assert(loadstring(src))(name,{}) end')(src, name)

    def hit(self, fields=''):
        self.run('events={{timestamp=10,event="SPELL_DAMAGE",spellId=123,spellName="Test Hit",'
                 'sourceName="Enemy One",sourceGUID="Player-1",sourceFlags=1344,'
                 'amount=350,overkill=50,currentHP=0,school=4,hideCaster=false'+fields+'}}')


class UIClient(RecapClient):
    def __init__(self):
        super().__init__()
        self.run((Path(__file__).parent / 'ui_mock.lua').read_text())
        create = self._create_frame
        def frame(kind='Frame', name=None, parent=None, template=None):
            obj = create(kind,name,parent,template)
            return self.lua.globals().ExtendFrame(obj,kind,name,parent,template)
        self.lua.globals().CreateFrame = frame

    def load_ui(self):
        path = ADDON / 'DeathRecapUI.lua'
        assert path.exists(), 'shared recap UI is missing'
        self.lua.eval('function(src) assert(loadstring(src))() end')(path.read_text())


class RecapUITests(unittest.TestCase):
    def client(self):
        c = UIClient(); c.load_shared(); c.load_ui()
        return c

    def test_open_in_combat_everywhere_close_escape_and_autoclose(self):
        for kind in ('raid','party','pvp','arena','none'):
            c = self.client(); c.instance_type=kind; c.lockdown=True
            c.hit(); c.fire('PLAYER_DEAD')
            self.assertTrue(c.run('return RelaxDeathRecap.frame:IsShown()'))
            c.run('Escape()')
            self.assertFalse(c.run('return RelaxDeathRecap.frame:IsShown()'))
            for button in ('x','closeButton'):
                c.slash('/recap')
                c.run(f'local b=RelaxDeathRecap.{button}; b.scripts.OnClick(b)')
                self.assertFalse(c.run('return RelaxDeathRecap.frame:IsShown()'))
            for event in ('PLAYER_ALIVE','PLAYER_UNGHOST'):
                c.slash('/recap'); c.fire(event)
                self.assertFalse(c.run('return RelaxDeathRecap.frame:IsShown()'))
            c.slash('/recap autoclose off'); c.slash('/recap'); c.fire('PLAYER_ALIVE')
            self.assertTrue(c.run('return RelaxDeathRecap.frame:IsShown()'))
            self.assertEqual(c.sent,[])

    def test_auto_off_and_link_route_and_no_data(self):
        c=self.client(); c.slash('/recap auto off'); c.fire('PLAYER_DEAD'); c.advance(3)
        self.assertFalse(c.run('return RelaxDeathRecap.frame:IsShown()'))
        c.run("EventRegistry:TriggerEvent('SetItemRef','addon:RelaxDeathRecap:1')")
        self.assertTrue(c.run('return RelaxDeathRecap.frame:IsShown()'))
        self.assertEqual(c.run('return RelaxDeathRecap.empty.text'),'no recap data for this death')

    def test_stale_previous_rows_are_not_a_new_death(self):
        # Reviewer P2/P3: the API still exposes death 1 on every death 2 retry.
        c=self.client(); c.hit(); c.fire('PLAYER_DEAD'); c.advance(3)
        c.fire('PLAYER_ALIVE'); c.fire('PLAYER_UNGHOST'); c.fire('PLAYER_DEAD')
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows'),0)
        self.assertEqual(c.run('return RelaxDeathRecap.empty.text'),'no recap data for this death')
        self.assertTrue(c.run('return RelaxDeathRecap.empty:IsShown()'))
        # Corrections to amounts/names do not make the same timestamps/IDs fresh.
        c.run('events[1].amount=400; events[1].sourceName="Renamed Enemy"')
        c.advance(3)
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows,RelaxDeathRecap.history[1].killer'),(0,'Unknown'))
        self.assertEqual(c.run('return RelaxDeathRecap.history[2].rows[1].amount'),350)
        # A stale/empty death must not erase the last accepted identity.
        c.fire('PLAYER_ALIVE'); c.fire('PLAYER_DEAD'); c.advance(3)
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows'),0)
        # Same source/spell, new timestamp: fresh, not an eternal source ban.
        c.fire('PLAYER_ALIVE'); c.hit(',timestamp=50'); c.fire('PLAYER_DEAD')
        self.assertEqual(c.run('return RelaxDeathRecap.history[1].rows[1].timestamp'),50)

    def test_fresh_rows_after_quick_release_attach_without_reopening(self):
        # Reviewer P12: fresh rows appear at 0.5s, after release at 0.1s.
        for release in ('PLAYER_ALIVE','PLAYER_UNGHOST'):
            c=self.client(); c.hit(); c.fire('PLAYER_DEAD'); c.advance(3); c.fire('PLAYER_ALIVE')
            c.run('calls=0; RelaxDeathRecap.register("Other",{onRecap=function(r) calls=calls+1; notified=r end})')
            c.fire('PLAYER_DEAD'); c.advance(0.1); c.fire(release); c.advance(0.4)
            c.hit(',timestamp=50,sourceName="New Enemy"'); c.advance(1.5)
            self.assertEqual(c.run('return RelaxDeathRecap.history[1].killer,#RelaxDeathRecap.history[1].rows'),('New Enemy',1))
            self.assertEqual(c.run('return calls,notified.id,notified.rows[1].timestamp'),(1,2,50))
            self.assertFalse(c.run('return RelaxDeathRecap.frame:IsShown()'))
            self.assertEqual(c.run('return RelaxDeathRecap.history[2].killer'),'Enemy One')
            # Nothing arriving beyond the 2s window can replace this recap.
            c.hit(',timestamp=80'); c.advance(3)
            self.assertEqual(c.run('return RelaxDeathRecap.history[1].rows[1].timestamp,calls'),(50,1))

    def test_unreadable_whole_rows_mark_partial_and_blank_shares(self):
        # Reviewer P5: inaccessible middle/first rows must not end a complete prefix.
        for position in (1,2):
            for mode in ('secret()', '"wrong type"', 'false', 'throw'):
                c=self.client()
                c.run('''for i=1,3 do events[i]={timestamp=11-i,amount=300,overkill=0,
                    sourceName='Enemy '..i,sourceGUID='Player-'..i,sourceFlags=1344,hideCaster=false} end''')
                if mode == 'throw':
                    c.run(f'events[{position}]=nil; setmetatable(events,{{__index=function(_,k) if k=={position} then error("denied") end end}})')
                else:
                    c.run(f'events[{position}]={mode}')
                c.fire('PLAYER_DEAD'); c.advance(2)
                self.assertTrue(c.run('return RelaxDeathRecap.history[1].incomplete'), (position,mode))
                self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows'),2)
                self.assertEqual(c.run('local n=0; for _,a in ipairs(RelaxDeathRecap.history[1].attackers) do if a.share then n=n+1 end end; return n'),0)
                self.assertEqual(c.run('return RelaxDeathRecap.attackerFrames[1].share.text'),'')
                self.assertIn('some hits unreadable',c.run('return RelaxDeathRecap.footer.text'))
                self.assertEqual(c.run('return RelaxDeathRecap.history[1].killer'),'Unknown')
                self.assertIsNone(c.run('return RelaxDeathRecap.history[1].rows[1].seconds'))

    def test_incomplete_recaps_stay_incomplete_after_saved_roundtrip(self):
        c=self.client(); c.hit(); c.run('events[2]=secret()')
        c.run('settings={version=1,keep=true}; RelaxDeathRecap.register("RelaxRaid",{settings=settings})')
        c.fire('PLAYER_DEAD'); c.advance(2)
        c.run('RelaxDeathRecap.history={}; RelaxDeathRecap.configure(settings); RelaxDeathRecap.open(1)')
        self.assertTrue(c.run('return RelaxDeathRecap.history[1].incomplete'))
        self.assertIsNone(c.run('return RelaxDeathRecap.history[1].attackers[1].share'))
        self.assertIn('some hits unreadable',c.run('return RelaxDeathRecap.footer.text'))

    def assert_invalid_amount_rejected(self, value):
        c=self.client(); c.hit()
        c.run(f'''events[2]={{timestamp=11,amount={value},overkill=0,sourceName='Bad Amount',
            sourceGUID='Player-2',sourceFlags=1344,hideCaster=false}}''')
        c.fire('PLAYER_DEAD'); c.advance(2)
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows'),1,value)
        self.assertEqual(c.run('return RelaxDeathRecap.history[1].rows[1].sourceName'),'Enemy One')
        self.assertTrue(c.run('return RelaxDeathRecap.history[1].incomplete'))
        self.assertIsNone(c.run('return RelaxDeathRecap.history[1].attackers[1].share'))
        self.assertEqual(c.run('return RelaxDeathRecap.attackerFrames[1].share.text'),'')
        self.assertIn('some hits unreadable',c.run('return RelaxDeathRecap.footer.text'))
        self.assertNotIn('--',c.run('return RelaxDeathRecap.rowFrames[1].amount.text'))

    def test_negative_amount_row_is_unreadable_not_zero_damage(self):
        self.assert_invalid_amount_rejected('-500')
        c=self.client(); c.hit(',amount=0,overkill=-1'); c.fire('PLAYER_DEAD')
        self.assertEqual(c.run('return RelaxDeathRecap.history[1].rows[1].effective'),0)
        self.assertFalse(c.run('return RelaxDeathRecap.history[1].incomplete'))

    def test_nonfinite_amount_row_is_unreadable(self):
        for value in ('0/0','math.huge','-math.huge','1e300'):
            self.assert_invalid_amount_rejected(value)

    def test_freshness_compares_identity_multisets_not_order_or_amount(self):
        c=self.client(); c.hit()
        c.run('events[2]={timestamp=10,amount=50,overkill=0,spellId=456,event="SPELL_DAMAGE",sourceGUID="Player-2",hideCaster=false}')
        c.fire('PLAYER_DEAD'); c.advance(2); c.fire('PLAYER_ALIVE')
        c.run('events[1],events[2]=events[2],events[1]; events[1].amount=99')
        c.fire('PLAYER_DEAD'); c.advance(0.3)
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows'),0)
        c.run('events[1].spellId=789'); c.advance(0.7)
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows'),2)
        # An old snapshot returning later must not replace the newly accepted one.
        c.run('events[1].spellId=456'); c.advance(1)
        self.assertEqual(c.run('local n=0; for _,r in ipairs(RelaxDeathRecap.history[1].rows) do if r.spellId==789 then n=n+1 end end; return n'),1)

    def test_saved_previous_identity_blocks_stale_data_after_reload(self):
        c=self.client(); c.hit()
        c.run('settings={version=1,keep=true}; RelaxDeathRecap.register("RelaxRaid",{settings=settings})')
        c.fire('PLAYER_DEAD'); c.advance(2)
        c.run('RelaxDeathRecap.history={}; RelaxDeathRecap.lastRows=nil; RelaxDeathRecap.configure(settings)')
        c.fire('PLAYER_ALIVE'); c.fire('PLAYER_DEAD'); c.advance(2)
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows'),0)

    def test_only_unreadable_rows_still_report_incomplete(self):
        c=self.client(); c.run('events={secret()}'); c.fire('PLAYER_DEAD'); c.advance(2)
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows'),0)
        self.assertTrue(c.run('return RelaxDeathRecap.history[1].incomplete'))
        self.assertIn('some hits unreadable',c.run('return RelaxDeathRecap.footer.text'))

    def test_unreadable_amount_and_overkill_warn_without_inventing_shares(self):
        for key in ('amount','overkill'):
            c=self.client(); c.hit(); c.run(f'events[1].{key}=secret()'); c.fire('PLAYER_DEAD'); c.advance(2)
            self.assertTrue(c.run('return RelaxDeathRecap.history[1].incomplete'))
            self.assertIsNone(c.run('return RelaxDeathRecap.history[1].attackers[1].share'))
            self.assertIn('some hits unreadable',c.run('return RelaxDeathRecap.footer.text'))

    def test_rows_show_health_time_extras_crit_only_if_readable(self):
        c=self.client(); c.hit(',blocked=96,absorbed=4,resisted=2'); c.fire('PLAYER_DEAD')
        self.assertEqual(c.run('return RelaxDeathRecap.rowFrames[1].time.text,RelaxDeathRecap.rowFrames[1].health.value'),('0.0 s',0))
        self.assertNotIn('Critical',c.run('return RelaxDeathRecap.rowFrames[1].spell.text'))
        c.run('events[1].critical=true; events[1].avoidable=true'); c.advance(0.3)
        self.assertIn('Critical',c.run('return RelaxDeathRecap.rowFrames[1].spell.text'))
        c.run('RelaxDeathRecap.rowFrames[1].scripts.OnEnter(RelaxDeathRecap.rowFrames[1])')
        tooltip=c.run('return GameTooltip.text')
        for text in ('Overkill 50','Blocked 96','Absorbed 4','Resisted 2','School 4','flagged by the game'):
            self.assertIn(text,tooltip)
        c.run('events[1].amount=secret(); events[1].currentHP=secret(); events[1].timestamp=secret()'); c.advance(0.7)
        self.assertEqual(c.run('return RelaxDeathRecap.rowFrames[1].amount.text,RelaxDeathRecap.rowFrames[1].time.text'),('',''))
        self.assertFalse(c.run('return RelaxDeathRecap.rowFrames[1].health:IsShown()'))

    def test_tabs_labels_and_twelve_row_scroll(self):
        c=self.client()
        for i,(fields,label) in enumerate([('', 'Enemy One'),(',sourceFlags=2048,sourceName="Boss"','Boss'),(',event="ENVIRONMENTAL_DAMAGE",environmentalType="LAVA",sourceName=nil','Lava'),(',sourceName=secret()','Unknown')]):
            c.hit(fields+f',timestamp={20+i}'); c.fire('PLAYER_DEAD'); c.advance(2)
            self.assertIn(label,c.run('return RelaxDeathRecap.tabs[1].label.text'))
            c.fire('PLAYER_ALIVE')
        c.slash('/recap 2'); self.assertEqual(c.run('return RelaxDeathRecap.selected'),2)
        c.run('for i=1,15 do events[i]={timestamp=100-i,spellName="Hit "..i,amount=i,overkill=0} end')
        c.fire('PLAYER_DEAD'); c.advance(2)
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows'),12)
        c.run('for i=1,20 do RelaxDeathRecap.frame.scripts.OnMouseWheel(RelaxDeathRecap.frame,-1) end')
        self.assertEqual(c.run('return RelaxDeathRecap.rowFrames[5].spell.text'),'Hit 12')
        c.run('RelaxDeathRecap.history[1].zone=string.rep("Z",100); RelaxDeathRecap.render(); RelaxDeathRecap.tabs[1].scripts.OnEnter(RelaxDeathRecap.tabs[1])')
        self.assertIn('...',c.run('return RelaxDeathRecap.tabs[1].label.text'))
        self.assertIn('Z'*100,c.run('return GameTooltip.text'))

    def test_position_saved_clamped_reset_and_popup_never_modified(self):
        c=self.client()
        c.run('settings={version=1,position={x=9000,y=-99}}; RelaxDeathRecap.register("RelaxRaid",{settings=settings})')
        self.assertEqual(c.run('return RelaxDeathRecap.frame:GetLeft()'),670)
        self.assertGreater(c.run('return RelaxDeathRecap.frame:GetTop()'),0)
        c.run('RelaxDeathRecap.frame.left=123; RelaxDeathRecap.frame.top=654; RelaxDeathRecap.titleBar.scripts.OnDragStop(RelaxDeathRecap.titleBar)')
        self.assertEqual(c.run('return settings.position.x,settings.position.y'),(123,654))
        c.slash('/recap reset'); self.assertIsNone(c.run('return settings.position'))
        c.run('''popup={which='DEATH',GetCenter=function() return 300,500 end,GetBottom=function() return 450 end,
            GetRight=function() return 400 end,GetTop=function() return 550 end,GetEffectiveScale=function() return 1 end}
            PopupEventManager:TriggerEvent('PopupOpened',popup)''')
        self.assertTrue(c.run('return RelaxDeathRecap.detailButton:IsShown()'))
        self.assertTrue(c.run('return RelaxDeathRecap.detailButton.parent==UIParent and RelaxDeathRecap.detailButton.point[2]==UIParent'))
        c.hit(); c.fire('PLAYER_DEAD')
        c.run('local b=RelaxDeathRecap.detailButton; b.scripts.OnClick(b)')
        self.assertEqual(c.run('return releaseCalls'),0)
        self.assertEqual(c.run('return RelaxDeathRecap.frame.strata'),'MEDIUM')
        self.assertGreaterEqual(c.run('return RelaxDeathRecap.frame:GetLeft()'),412)
        c.run("PopupEventManager:TriggerEvent('PopupClosed',popup)")
        self.assertFalse(c.run('return RelaxDeathRecap.detailButton:IsShown()'))

    def test_header_identity_and_guild_reason_are_visible(self):
        c=self.client(); c.hit()
        c.run("RelaxDeathRecap.register('Other',{identity=function() return 'Rogue',60,'Synthetic Guild' end,avoidable=function() return true end})")
        c.fire('PLAYER_DEAD')
        label=c.run('return RelaxDeathRecap.killer.text')
        for value in ('Enemy One','Rogue','60','Synthetic Guild'):
            self.assertIn(value,label)
        c.run('RelaxDeathRecap.rowFrames[1].scripts.OnEnter(RelaxDeathRecap.rowFrames[1])')
        self.assertIn("on the guild's avoidable list",c.run('return GameTooltip.text'))

    def test_maximum_player_block_fits_above_footer(self):
        c=self.client()
        c.run('''for i=1,12 do events[i]={timestamp=100-i,amount=10,overkill=0,sourceGUID='Player-'..i,
            sourceName='Enemy '..i,sourceFlags=1344,hideCaster=false} end''')
        c.fire('PLAYER_DEAD')
        self.assertLessEqual(c.run('return RelaxDeathRecap.frame:GetHeight()+38'),c.run('return UIParent:GetHeight()'))
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].attackers'),12)
        self.assertEqual(c.run('return RelaxDeathRecap.rowFrames[1].height'),32)

    def test_maximum_player_block_fits_600px_screen_and_caps_attackers(self):
        # Reviewer height probe; include tabs, content, controls and late growth.
        for late in (False,True):
            c=self.client(); c.run('UIParent:SetSize(1080,600)')
            if late: c.fire('PLAYER_DEAD'); c.advance(0.1)
            c.run('''for i=1,12 do events[i]={timestamp=100-i,amount=10,overkill=0,sourceGUID='Player-'..i,
                sourceName='Enemy '..i,sourceFlags=1344,hideCaster=false} end''')
            if late: c.advance(0.2)
            else: c.fire('PLAYER_DEAD')
            self.assertLessEqual(c.run('return RelaxDeathRecap.frame:GetHeight()+38'),600)
            self.assertGreaterEqual(c.run('return RelaxDeathRecap.frame:GetBottom()'),0)
            self.assertLessEqual(c.run('return RelaxDeathRecap.frame:GetTop()+38'),600)
            self.assertEqual(c.run('return #RelaxDeathRecap.history[1].attackers'),12)
            self.assertEqual(c.run('local n=0; for _,a in ipairs(RelaxDeathRecap.attackerFrames) do if a:IsShown() then n=n+1 end end; return n'),3)
            self.assertEqual(c.run('return RelaxDeathRecap.moreAttackers.text'),'+9 more')
            self.assertTrue(c.run('return RelaxDeathRecap.moreAttackers:IsShown()'))
            c.run('''local R=RelaxDeathRecap
                local _, frameBottom, _, frameTop=UIBounds(R.frame)
                local _, moreBottom, _, moreTop=UIBounds(R.moreAttackers)
                local _, _, _, firstTop=UIBounds(R.rowFrames[1])
                local _, rowBottom=UIBounds(R.rowFrames[5])
                local _, footerBottom, _, footerTop=UIBounds(R.footer)
                local _, closeBottom, _, closeTop=UIBounds(R.closeButton)
                assert(moreBottom>=firstTop, 'attacker overflow overlaps hits')
                assert(rowBottom>=footerTop and footerBottom>=closeTop, 'footer/control overlap')
                assert(closeBottom>=frameBottom and moreTop<=frameTop, 'content outside frame')
                for _,tab in ipairs(R.tabs) do
                    if tab:IsShown() then local _,_,_,top=UIBounds(tab); assert(top<=UIParent:GetHeight()) end
                end''')
            c.run('for i=1,20 do RelaxDeathRecap.scroll(1) end')
            self.assertEqual(c.run('return RelaxDeathRecap.rowFrames[5].row.timestamp'),88)
            # A smaller later recap hides +X rather than leaving stale text visible.
            c.fire('PLAYER_ALIVE'); c.hit(',timestamp=200'); c.fire('PLAYER_DEAD')
            self.assertFalse(c.run('return RelaxDeathRecap.moreAttackers:IsShown()'))

    def test_two_ui_copies_and_popup_manager_absent(self):
        c=self.client(); c.load_shared('Other'); c.load_ui()
        self.assertEqual(c.run("local n=0; for _,f in ipairs(UIFrames) do if f.name=='RelaxDeathRecapFrame' then n=n+1 end end; return n"),1)
        self.assertEqual(c.run('return #UISpecialFrames'),1)
        c=UIClient(); c.run('PopupEventManager=nil'); c.load_shared(); c.load_ui(); c.slash('/recap')
        self.assertTrue(c.run('return RelaxDeathRecap.frame:IsShown()'))

    def test_source_owns_no_restricted_actions_and_static_strings_ascii(self):
        for file in ('DeathRecap.lua','DeathRecapUI.lua'):
            src=(ADDON/file).read_text()
            for word in ('Secure','RepopMe','COMBAT_LOG_EVENT_UNFILTERED','SendAddonMessage','SendChatMessage'):
                self.assertNotIn(word,src)
            self.assertTrue(src.isascii())


class IntegrationTests(unittest.TestCase):
    def test_toc_hook_and_existing_settings_panel(self):
        c=UIClient(); c.load_addon(str(ADDON))
        self.assertIsNotNone(c.run("return RelaxDeathRecap and RelaxDeathRecap.hooks.RelaxRaid"))
        self.assertEqual(c.run('return RelaxDeathRecap.settings.version'),1)
        self.assertIsNone(c.run('return RelaxRaidCharDB'), 'default-only init stays session-local')
        for key, default in (('auto',True),('autoclose',True),('keep',False)):
            variable='RELAXRAID_RECAP_'+key.upper()
            self.assertIn(variable,c.settings)
            self.assertEqual(c.setting_value(variable),default)
            self.assertEqual(c.settings[variable]['category']['name'],'Relax Raid')
            c.click_setting(variable,not default)
            self.assertEqual(c.run(f'return RelaxDeathRecap.settings.{key}'),not default)
        c.run('RelaxRaid.avoidableSpells[123]=true')
        c.hit(); c.fire('PLAYER_DEAD'); c.advance(2)
        self.assertTrue(c.run('return RelaxDeathRecap.history[1].rows[1].guildAvoidable'))
        self.assertEqual(c.run('return #RelaxRaidCharDB.deathRecap.history'),1)
        self.assertEqual(c.run('return #RelaxRaid.errors'),0)

    def test_newer_character_save_is_untouched(self):
        c=UIClient(); c.load_addon(str(ADDON),saved={'RelaxRaidCharDB':'{version=99,marker="future"}'})
        self.assertIsNone(c.run('return RelaxRaidCharDB.deathRecap'))
        self.assertEqual(c.run('return RelaxRaidCharDB.version,RelaxRaidCharDB.marker'),(99,'future'))
        c.hit(); c.fire('PLAYER_DEAD')
        self.assertTrue(c.run('return RelaxDeathRecap.frame:IsShown()'))

    def test_avoidable_generator_single_source_and_empty_seed(self):
        gen=runpy.run_path(str(ROOT/'scripts/build_rules_lua.py'))
        self.assertIn('convert_avoidable',gen)
        convert=gen['convert_avoidable']
        rules=json.loads((ROOT/'rules/raid/encounters-enabled.json').read_text())
        self.assertEqual(convert(rules),(ADDON/'AvoidableSpells.lua').read_text())
        def rule(id, avoid=True):
            return {'spell':{'id':id,'name':'Synthetic','source':{'kind':'real_log','note':'synthetic fixture'}},'avoidable':avoid}
        fixture={'schema':1,'encounters':[{'mechanics':[rule(44),rule(3),rule(44),rule(9,False),{'spell':{'name':'No ID'},'avoidable':True}]}]}
        out=convert(fixture)
        self.assertEqual(out.count('[44]'),1)
        self.assertLess(out.index('[3]'),out.index('[44]'))
        self.assertNotIn('[9]',out)
        for bad in (True,-1,'44',1.5):
            fixture['encounters'][0]['mechanics']=[rule(bad)]
            with self.assertRaises(ValueError): convert(fixture)
        for source in (None, {}, {'kind':'guess','note':'not evidence'}, {'kind':'real_log','note':''}):
            item=rule(44); item['spell']['source']=source
            fixture['encounters'][0]['mechanics']=[item]
            with self.assertRaises(ValueError): convert(fixture)
        fixture['schema']=2
        with self.assertRaises(ValueError): convert(fixture)


class RecapTests(unittest.TestCase):
    def client(self):
        c = RecapClient()
        c.load_shared()
        return c

    def test_read_copies_sorts_and_calculates(self):
        c = self.client()
        c.hit()
        c.run('events[2]={timestamp=8,amount=200,overkill=-1,currentHP=200}; r=RelaxDeathRecap.read()')
        self.assertEqual(c.run('return #r.rows, r.rows[1].effective, r.rows[2].seconds, r.rows[2].healthPercent'),
                         (2, 300, 2, 20))
        self.assertEqual(c.run('return r.killer, r.rows[1].spellName, r.rows[1].overkill'), ('Enemy One', 'Test Hit', 50))
        c.run('events[1].amount=99')
        self.assertEqual(c.run('return r.rows[1].amount'), 350)

    def test_no_events_and_zero_health(self):
        c = self.client()
        self.assertEqual(c.run('return #RelaxDeathRecap.read().rows'), 0)
        c.hit()
        c.run('maxHealth=0; r=RelaxDeathRecap.read()')
        self.assertIsNone(c.run('return r.rows[1].healthPercent'))
        self.assertIsNone(c.run('return r.rows[1].critical'))

    def test_inaccessible_amount_is_not_used(self):
        c=self.client(); c.hit(); c.run('events[1].amount=secret(); r=RelaxDeathRecap.read()')
        self.assertIsNone(c.run('return r.rows[1].amount'))
        self.assertIsNone(c.run('return r.rows[1].effective'))

    def test_each_secret_missing_or_throwing_field_stays_unknown(self):
        fields = {'timestamp':'number','amount':'number','overkill':'number','absorbed':'number',
                  'resisted':'number','blocked':'number','currentHP':'number','school':'number',
                  'spellId':'number','spellName':'string','event':'string','environmentalType':'string',
                  'sourceName':'string','sourceGUID':'string','sourceFlags':'number','hideCaster':'boolean',
                  'deadly':'boolean','avoidable':'boolean','critical':'boolean'}
        for key in fields:
            for mode in ('secret()', 'nil', 'throw'):
                with self.subTest(key=key, mode=mode):
                    c = self.client()
                    c.hit()
                    if mode == 'throw':
                        c.run(f'events[1].{key}=nil; setmetatable(events[1],{{__index=function(_,k) if k=="{key}" then error("hidden") end end}})')
                    else:
                        c.run(f'events[1].{key}={mode}')
                    self.assertIsNone(c.run(f'return RelaxDeathRecap.read().rows[1].{key}'))
        c = self.client()
        c.hit()
        c.run('canaccessvalue=function() error("denied") end')
        self.assertEqual(c.run('return #RelaxDeathRecap.read().rows'), 0)

    def test_avoidable_hooks_and_identity_are_guarded(self):
        c = self.client()
        c.hit()
        c.run('''RelaxDeathRecap.register('Bad', {avoidable=function() error('bad') end})
            RelaxDeathRecap.register('RelaxRaid', {avoidable=function(id) return id==123 end})
            RelaxDeathRecap.register('Other', {identity=function() return 'Rogue',60,'Test Guild' end})
            r=RelaxDeathRecap.read()''')
        self.assertEqual(c.run('return r.rows[1].guildAvoidable, r.attackers[1].class, r.attackers[1].level, r.attackers[1].guild'),
                         (True, 'Rogue', 60, 'Test Guild'))
        c.run('events[1].spellId=999; events[1].avoidable=true; r=RelaxDeathRecap.read()')
        self.assertFalse(c.run('return r.rows[1].guildAvoidable'))
        self.assertTrue(c.run('return r.rows[1].avoidable'))

    def test_pvp_shares_use_all_damage_and_pets_stay_separate(self):
        c = self.client()
        c.hit()
        c.run('''events[2]={timestamp=9,amount=200,overkill=0,sourceFlags=1344,sourceGUID='Player-2',sourceName='Enemy Two',hideCaster=false}
            events[3]={timestamp=8,amount=100,overkill=0,sourceFlags=4416,sourceGUID='Pet-1',sourceName='Pet One',hideCaster=false}
            events[4]={timestamp=7,amount=400,overkill=0,sourceFlags=2048,sourceGUID='Creature-1',sourceName='Creature',hideCaster=false}
            r=RelaxDeathRecap.read()''')
        self.assertEqual(c.run('return #r.attackers,r.attackers[1].share,r.attackers[2].share,r.attackers[3].share'), (3,30,20,10))
        self.assertEqual(c.run('return r.attackers[3].kind,r.attackers[3].ownerGUID'), ('pet', None))
        c.run("UnitTokenFromGUID=function(g) assert(g=='Pet-1'); return 'target' end; UnitOwnerGUID=function(t) assert(t=='target'); return 'Player-1' end; r=RelaxDeathRecap.read()")
        self.assertEqual(c.run('return #r.attackers,r.attackers[3].ownerGUID,r.attackers[1].share'), (3,'Player-1',30))
        c.run('events[4]=nil; events[3]=nil; r=RelaxDeathRecap.read()')
        self.assertEqual(c.run('return r.attackers[1].share+r.attackers[2].share'), 100)
        c.run('events[1].amount=secret(); r=RelaxDeathRecap.read()')
        self.assertIsNone(c.run('return r.attackers[1].share'))

    def test_pvp_hidden_for_environment_creatures_or_unreadable_flags(self):
        c = self.client()
        for flags in ('2048', '4416', 'secret()', 'nil', '1024'):
            c.hit()
            c.run(f'events[1].sourceFlags={flags}; r=RelaxDeathRecap.read()')
            self.assertEqual(c.run('return #r.attackers'), 0)
        c.run("events={{timestamp=1,event='ENVIRONMENTAL_DAMAGE',environmentalType='FALLING',amount=50,overkill=0}}; r=RelaxDeathRecap.read()")
        self.assertEqual(c.run('return r.killer,#r.attackers'), ('Falling',0))

    def test_late_third_retry_and_one_notification(self):
        c = self.client()
        c.run("calls=0; RelaxDeathRecap.register('RelaxRaid',{onRecap=function(r) calls=calls+1; notified=r end})")
        c.fire('PLAYER_DEAD')
        c.advance(1.1)
        self.assertEqual(c.run('return #RelaxDeathRecap.history'), 1)
        c.hit()
        c.advance(1)
        self.assertEqual(c.run('return calls,#notified.rows,reads'), (1,1,1))
        self.assertEqual(sum('[You died: details]' in s for s in c.chat), 1)
        c.fire('PLAYER_DEAD')
        c.advance(3)
        self.assertEqual(c.run('return calls,#RelaxDeathRecap.history'), (1,1))

    def test_both_load_orders_and_higher_version_keep_one_dispatch(self):
        for first, second, version in [('RelaxRaid','Other',1), ('Other','RelaxRaid',1), ('RelaxRaid','Other',2)]:
            c = RecapClient()
            c.load_shared(first)
            c.run(f"one=0; two=0; RelaxDeathRecap.register('{first}',{{onRecap=function() one=one+1 end}})")
            c.load_shared(second, version)
            c.run(f"RelaxDeathRecap.register('{second}',{{onRecap=function() two=two+1 end}})")
            c.hit()
            c.fire('PLAYER_DEAD')
            c.advance(3)
            self.assertEqual(c.run('return one,two,#RelaxDeathRecap.history'), (1,1,1))
            self.assertEqual(len(c.frames), 1)
            self.assertEqual(c.run('return RelaxDeathRecap.owner'), second if version==2 else first)

    def test_four_deaths_keep_three_and_cancel_old_timers(self):
        c = self.client()
        for i in range(4):
            c.hit(f',timestamp={20+i}')
            c.fire('PLAYER_DEAD')
            c.fire('PLAYER_ALIVE')
        c.advance(3)
        self.assertEqual(c.run('return #RelaxDeathRecap.history,RelaxDeathRecap.history[1].id,RelaxDeathRecap.history[3].id'), (3,4,2))
        self.assertEqual(c.run('return RelaxDeathRecap.history[1].rows[1].timestamp,RelaxDeathRecap.history[3].rows[1].timestamp'),(23,21))

    def test_links_reject_foreign_and_expired_ids(self):
        c = self.client()
        c.hit()
        c.fire('PLAYER_DEAD')
        c.run('opened=0; RelaxDeathRecap.open=function(i) opened=i end')
        for link in ('addon:Other:1','addon:RelaxDeathRecap:0','addon:RelaxDeathRecap:1junk','addon:RelaxDeathRecap:99999999999999999999'):
            c.lua.globals().RelaxDeathRecap.link(link)
            self.assertEqual(c.run('return opened'),0)
        c.run("RelaxDeathRecap.link('addon:RelaxDeathRecap:1')")
        self.assertEqual(c.run('return opened'),1)
        c.run("RelaxDeathRecap.link('addon:RelaxDeathRecap:9')")
        self.assertTrue(any('no longer kept' in s for s in c.chat))

    def test_saved_ids_are_bounded_integers_and_new_links_work(self):
        for bad in ('1e12','1.5','0','-1','math.huge','0/0','"1"','secret()'):
            c=self.client()
            c.run(f'''settings={{version=1,keep=true,history={{{{id={bad},clock='21:14',zone='Test',rows={{}}}}}}}};
                RelaxDeathRecap.register('RelaxRaid',{{settings=settings}})''')
            self.assertEqual(c.run('return #RelaxDeathRecap.history'),0,bad)
            self.assertEqual(c.run('return #settings.history'),0,bad)
            c.hit(); c.fire('PLAYER_DEAD')
            self.assertEqual(c.run('return RelaxDeathRecap.history[1].id'),1)
            c.run('opened=0; RelaxDeathRecap.open=function(i) opened=i end; RelaxDeathRecap.link("addon:RelaxDeathRecap:1")')
            self.assertEqual(c.run('return opened'),1)

    def test_largest_valid_id_never_emits_an_unusable_or_reused_link(self):
        c=self.client()
        c.run('''settings={version=1,keep=true,history={{id=999999999998,clock='21:14',zone='Test',rows={}}}};
            RelaxDeathRecap.register('RelaxRaid',{settings=settings})''')
        c.hit(); c.fire('PLAYER_DEAD'); c.advance(2); c.fire('PLAYER_ALIVE')
        self.assertEqual(c.run('return settings.history[1].id'),999999999999)
        c.run('opened=0; RelaxDeathRecap.open=function(i) opened=i end; RelaxDeathRecap.link("addon:RelaxDeathRecap:999999999999")')
        self.assertEqual(c.run('return opened'),1)
        c.hit(',timestamp=50'); c.fire('PLAYER_DEAD')
        self.assertEqual(c.run('return RelaxDeathRecap.serial,#RelaxDeathRecap.history'),(999999999999,2))
        self.assertIn('Recap ID limit reached',c.chat[-1])

    def test_saved_roundtrip_corrupt_drop_caps_and_owner_only(self):
        c = self.client()
        c.run("settings={version=1,keep=true}; other={version=1,keep=true}; RelaxDeathRecap.register('RelaxRaid',{settings=settings}); RelaxDeathRecap.register('Other',{settings=other})")
        for i in range(4):
            c.hit(f',timestamp={20+i}')
            c.fire('PLAYER_DEAD'); c.advance(2); c.fire('PLAYER_ALIVE')
        self.assertEqual(c.run('return #settings.history,other.history'), (3,None))
        c.run('settings.history[2].rows[1].amount="corrupt"; saved=settings; RelaxDeathRecap.history={}; RelaxDeathRecap.configure(saved)')
        self.assertEqual(c.run('return #RelaxDeathRecap.history,RelaxDeathRecap.history[1].id,RelaxDeathRecap.history[2].id'), (2,4,2))
        c.run('for i=2,13 do saved.history[1].rows[i]={} end; RelaxDeathRecap.history={}; RelaxDeathRecap.configure(saved)')
        self.assertEqual(c.run('return #RelaxDeathRecap.history'),1)
        c.run("RelaxDeathRecap.option('keep',false)")
        self.assertIsNone(c.run('return saved.history'))
        c.run('future={version=99,keep=true,history={marker=1}}; RelaxDeathRecap.configure(future); RelaxDeathRecap.option("keep",false)')
        self.assertEqual(c.run('return future.version,future.keep,future.history.marker'),(99,True,1))

    def test_hook_errors_and_revive_cannot_reopen_or_replace(self):
        c = self.client()
        c.run("calls=0; RelaxDeathRecap.register('Bad',{onRecap=function() error('bad') end}); RelaxDeathRecap.register('Other',{onRecap=function() calls=calls+1 end})")
        c.hit(); c.fire('PLAYER_DEAD'); c.advance(2)
        self.assertEqual(c.run('return calls'),1)
        c.fire('PLAYER_ALIVE'); c.run('events={}'); c.advance(3)
        self.assertEqual(c.run('return #RelaxDeathRecap.history[1].rows'),1)

    def test_retry_clears_unreadable_killing_blow(self):
        c=self.client(); c.hit(); c.fire('PLAYER_DEAD')
        self.assertIsNotNone(c.run('return RelaxDeathRecap.history[1].killingBlow'))
        c.run('events[1].timestamp=secret()'); c.advance(0.3)
        self.assertIsNone(c.run('return RelaxDeathRecap.history[1].killingBlow'))
        self.assertEqual(c.run('return RelaxDeathRecap.history[1].killer'),'Unknown')

    def test_api_failures_are_no_data(self):
        c = self.client()
        c.hit()
        for fn in ('HasRecapEvents', 'GetRecapEvents'):
            c.run(f'C_DeathRecap.{fn}=function() error("unavailable") end')
            self.assertEqual(c.run('return #RelaxDeathRecap.read().rows'), 0)


if __name__ == '__main__':
    unittest.main()
