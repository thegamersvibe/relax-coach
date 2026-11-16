#!/usr/bin/env python3
import sys,unittest
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parent))
from cleanup_harness import client,POLICY,ADDON,ROOT

UI=('UI/Common.lua','UI/Steps.lua','UI/Window.lua','UI/Picker.lua','UI/Guides.lua','UI/Controller.lua','UI/Cleanup.lua')

def runtime(before=''):
    lua=client(POLICY+UI,before)
    lua.execute('''
      RelaxQuestPlanner.UI.Refresh=function() ROUTE_REFRESHES=(ROUTE_REFRESHES or 0)+1 end
      QUEST(1701,{sourceItemId=6842}); ITEM(6842,{name="Furen's Instructions"}); BAG(6842); DONE={1701}
    ''')
    return lua

class CleanupUITests(unittest.TestCase):
    def test_review_slash_and_cleanup_alias_open_same_window(self):
        runtime().execute('''
          local frame
          for _,command in ipairs({'review','cleanup','REVIEW'}) do
            SlashCmdList.RELAXQUESTPLANNER(command); DRAIN()
            local U=RelaxQuestPlanner.CleanupUI
            assert(U.frame and U.frame:IsShown(),command..' did not open review')
            assert(U.title:GetText()=='Quest and item review')
            assert(not frame or frame==U.frame,'aliases opened different windows')
            frame=U.frame; U.close:Click()
          end
          assert(RelaxQuestPlanner.UI.frame==nil,'review must not open navigation')
          assert(RelaxQuestPlannerDB.lastError==nil,RelaxQuestPlannerDB.lastError)
        ''')

    def test_review_name_in_header_and_help(self):
        checks=('''
          RelaxQuestPlanner.UI.BuildWindow()
          local button=RelaxQuestPlanner.UI.cleanupButton
          assert(button:GetText()=='Review','header uses the wrong feature name')
          button.scripts.OnEnter(button)
          assert(GameTooltip:GetText():find('Quest and item review',1,true))
          assert(GameTooltip:GetText():find('/rqp review',1,true))
        ''', '''
          SlashCmdList.RELAXQUESTPLANNER('help')
          local help=table.concat(OUT,' | ')
          assert(help:find('Quest and item review',1,true),'help omits the feature name')
          assert(help:find('/rqp review',1,true),'help omits the review command')
        ''')
        for surface,check in zip(('header','help'),checks):
            with self.subTest(surface=surface): runtime().execute(check)

    def test_slash_works_without_route_journey_or_coach_and_explains_limits(self):
        lua=runtime()
        self.assertIsNotNone(lua.globals().RelaxQuestPlanner.Cleanup.Show,'review UI is missing')
        lua.execute('''
          SlashCmdList.RELAXQUESTPLANNER('cleanup'); DRAIN()
          local U=RelaxQuestPlanner.CleanupUI
          assert(U.frame:IsShown() and U.title:GetText()=='Quest and item review')
          assert(RelaxQuestPlanner.UI.frame==nil,'must not open or resize the route window')
          assert(U.status:GetText():find('1.1.1') and U.status:GetText():find('70888'))
          assert(U.status:GetText():find('Player%-fixture') and U.status:GetText():find('70938'))
          assert(U.coverage:GetText():find('not assessed'))
          assert(U.rows[1]:GetText():find('6842') and U.rows[1]:GetText():find('No use found in our data'))
          assert(U.rows[1].label:GetHeight()>0 and U.rows[1].label:GetHeight()<=U.rows[1]:GetHeight())
          U.rows[1]:Click()
          assert(U.details:GetText():find('destroy side effects') and U.details:GetText():find('1701'))
          assert(U.details:GetText():find('UNCERTAIN%-KEEP'))
          assert(not U.counts:GetText():find('suggestions'))
          U.close:Click(); assert(not U.frame:IsShown() and not RelaxQuestPlanner.Cleanup.visible)
          assert(RelaxQuestPlannerDB.lastError==nil,RelaxQuestPlannerDB.lastError)
        ''')

    def test_discovery_header_geometry_route_dimensions_and_paging(self):
        lua=runtime()
        lua.execute('RelaxQuestPlanner.UI.BuildWindow()')
        self.assertIsNotNone(lua.globals().RelaxQuestPlanner.UI.cleanupButton,'header discovery is missing')
        lua.execute('''
          local U=RelaxQuestPlanner.UI
          assert(U.frame:GetWidth()==470 and U.frame:GetHeight()==272)
          local _,_,_,x,y=U.cleanupButton:GetPoint(); assert(y==-6 and x>=336)
          local _,_,_,cx=U.closeButton:GetPoint(); assert(x+U.cleanupButton:GetWidth()<cx)
          U.cleanupButton:Click(); DRAIN()
          local V=RelaxQuestPlanner.CleanupUI
          for id=8000,8008 do ITEM(id); BAG(id) end
          V.refresh:Click(); DRAIN()
          assert(#V.rows==4 and V.next:IsEnabled())
          local first=V.rows[1].record.exactID
          V.next:Click(); assert(V.rows[1].record.exactID~=first)
          for _,scale in ipairs({0.64,0.8,1,1.2}) do
            assert(V.frame:GetWidth()*scale<=1024 and V.frame:GetHeight()*scale<=768)
            for _,button in ipairs({V.questTab,V.itemTab,V.refresh,V.prev,V.next,V.pin}) do
              local _,_,_,bx,by=button:GetPoint()
              assert(bx>=0 and bx+button:GetWidth()<=V.frame:GetWidth())
              assert(by<0 and -by+button:GetHeight()<=V.frame:GetHeight())
            end
          end
          assert(U.frame:GetWidth()==470 and U.frame:GetHeight()==272)
          assert(RelaxQuestPlannerDB.lastError==nil,RelaxQuestPlannerDB.lastError)
        ''')

    def test_stable_id_pins_locations_refresh_and_no_external_mutations(self):
        lua=runtime('''
          function forbidden() error('forbidden mutation') end
          for _,key in ipairs({'DeleteCursorItem','PickupContainerItem','UseContainerItem','SplitContainerItem',
            'SellCursorItem','AbandonQuest','SetAbandonQuest','SendChatMessage','SendAddonMessage','PickupItem',
            'EquipItemByName','RunMacroText','AcceptQuest','CompleteQuest','GetQuestReward'}) do _G[key]=forbidden end
          C_ChatInfo={SendAddonMessage=forbidden,SendAddonMessageLogged=forbidden}
          RelaxJourneyAPI={RegisterLocalGuide=forbidden,Goto=forbidden}
          RelaxQuestPlannerDB={mode='strict',selection={kind='goal',q=512},sentinel='same'}
        ''')
        self.assertIsNotNone(lua.globals().RelaxQuestPlanner.Cleanup.Show,'review UI is missing')
        lua.execute('''
          SlashCmdList.RELAXQUESTPLANNER('cleanup'); DRAIN()
          local U=RelaxQuestPlanner.CleanupUI
          U.rows[1]:Click(); U.pin:Click(); DRAIN()
          assert(RelaxQuestPlannerDB.cleanup.pins.item[6842])
          BAGS[1][1]=BAGS[0][1]; BAGS[0]={}; FIRE('BAG_UPDATE_DELAYED'); DRAIN()
          U.rows[1]:Click(); assert(U.details:GetText():find('bag 1, slot 1'))
          U.pin:Click(); DRAIN()
          assert(not RelaxQuestPlannerDB.cleanup.pins.item[6842])
          U.frame.scripts.OnDragStop(U.frame)
          assert(RelaxQuestPlannerDB.cleanup.position)
          assert(RelaxQuestPlannerDB.mode=='strict' and RelaxQuestPlannerDB.selection.q==512 and RelaxQuestPlannerDB.sentinel=='same')
          for k in pairs(RelaxQuestPlannerDB.cleanup) do assert(k=='schema' or k=='pins' or k=='position') end
          assert(RelaxQuestPlannerDB.lastError==nil,RelaxQuestPlannerDB.lastError)
        ''')

    def test_missing_provider_and_profession_names_fail_closed(self):
        lua=runtime('LibQuestieDB=nil')
        self.assertIsNotNone(lua.globals().RelaxQuestPlanner.Cleanup.Show,'review UI is missing')
        lua.execute('''
          SlashCmdList.RELAXQUESTPLANNER('cleanup'); DRAIN()
          local U=RelaxQuestPlanner.CleanupUI
          assert(U.status:GetText():find('Limited data'))
          assert(U.rows[1]:GetText():find('Quest state could not be resolved'))
          local id=next(RelaxQuestPlanner.CleanupUses.reagent); BAG(id)
          C_TradeSkillUI={GetTradeSkillDisplayName=function() return 'Client profession' end}
          local row={professions={164},reasons={'Used in recipes'}}
          assert(U.ProfessionText(row):find('Client profession'))
          C_TradeSkillUI.GetTradeSkillDisplayName=function() error('name') end
          assert(U.ProfessionText(row):find('164'))
        ''')

    def test_fontstrings_are_not_frames_and_status_hover_uses_a_frame(self):
        lua=runtime()
        lua.execute('''
          local create=CreateFrame
          CreateFrame=function(...)
            local f=create(...); local font=f.CreateFontString
            f.CreateFontString=function(self,...)
              local fs=font(self,...)
              fs.SetScript=function() error('FontString has no SetScript') end
              return fs
            end
            return f
          end
          SlashCmdList.RELAXQUESTPLANNER('cleanup'); DRAIN()
          assert(RelaxQuestPlannerDB.lastError==nil,RelaxQuestPlannerDB.lastError)
          local U=RelaxQuestPlanner.CleanupUI
          assert(U.statusHover and U.statusHover.kind=='Frame')
          U.statusHover.scripts.OnEnter(U.statusHover)
          assert(GameTooltip:GetText():find('1.1.1'))
        ''')

    def test_saved_window_anchor_replaces_center_instead_of_stretching(self):
        lua=runtime()
        lua.execute('''
          RelaxQuestPlannerDB.cleanup={schema=1,pins={},position={point='TOPLEFT',relativePoint='TOPLEFT',x=10,y=-10}}
          local create=CreateFrame
          CreateFrame=function(kind,name,...)
            local f=create(kind,name,...)
            if name=='RelaxQuestReviewFrame' then
              local point,clear=f.SetPoint,f.ClearAllPoints
              local points={}
              f.SetPoint=function(self,anchor,...)
                points[anchor]=true
                local n=0 for _ in pairs(points) do n=n+1 end
                assert(n==1,'multiple anchors would stretch the window')
                return point(self,anchor,...)
              end
              f.ClearAllPoints=function(self) points={}; return clear(self) end
            end
            return f
          end
          SlashCmdList.RELAXQUESTPLANNER('cleanup'); DRAIN()
          assert(RelaxQuestPlannerDB.lastError==nil,RelaxQuestPlannerDB.lastError)
          local point=RelaxQuestPlanner.CleanupUI.frame:GetPoint()
          assert(point=='TOPLEFT','saved position was not applied')
        ''')

    def test_coach_hint_uses_runtime_presence_and_does_not_scan(self):
        lua=client()
        lua.execute('''
          FC={Rules=function() return {thresholds={}} end,Session={Current=function() return nil end}}
          CHAR={snapshot={level=3,class='WARRIOR',baginfo={known=true,slots=16,free=0}}}
        ''')
        lua.execute('return function(...)\n'+(ROOT/'addon/ForeverCoach/Audit.lua').read_text()+'\nend')('ForeverCoach',lua.globals().FC)
        lua.execute('''
          local function hints()
            local n=0 for _,row in ipairs(FC.Audit:Run(CHAR)) do
              if row.text:find('Quest and item review',1,true) then n=n+1; assert(row.fix=='/rqp review (manual only)') end
            end return n
          end
          assert(hints()==0)
          RelaxQuestPlanner.Cleanup={Show=function() error('hint must not scan') end}
          assert(hints()==1,'planner-present hint missing')
          RelaxQuestPlanner=nil; assert(hints()==0)
        ''')

if __name__=='__main__': unittest.main(verbosity=2)
