#!/usr/bin/env python3
"""Planner UI contract tests. No private game data and no live engine required.

Run: uv run -q --with lupa python test/planner/ui_shell_test.py
The strict frame mock deliberately rejects unimplemented APIs/protected calls.
"""
from pathlib import Path
import unittest

import lupa.lua51 as lupa

ROOT = Path(__file__).resolve().parents[2]
ADDON = ROOT / "addon" / "RelaxQuestPlanner"

WOW = r'''
FRAMES, OUT, TIMERS, HOOKS = {}, {}, {}, {}
NOW = 0
STATE = {done={}, log={}, ready={}, map=101, zone="Home", level=30, combat=false}
SECRET = setmetatable({}, {__tostring=function() error("secret tostring") end})
function issecretvalue(v) return rawequal(v, SECRET) end
function GetTime() return NOW end
function InCombatLockdown() return STATE.combat end
function UnitHealth() error("UI must never query health") end
function UnitGUID() return STATE.guid or "Creature-0-1-2-3-901-123456" end
function GetRealZoneText() return STATE.zone end
function strsplit(sep, text)
  local result = {}
  for part in (text .. sep):gmatch("(.-)" .. sep) do result[#result+1] = part end
  return unpack(result)
end
function print(...)
  local t = {}; for i=1,select('#',...) do t[#t+1]=tostring((select(i,...))) end
  OUT[#OUT+1] = table.concat(t, " ")
end
DEFAULT_CHAT_FRAME = {AddMessage=function(_, text) OUT[#OUT+1]=text end}
UISpecialFrames, SlashCmdList = {}, {}
local methods = {}
for _, key in ipairs({"SetBackdrop", "SetBackdropColor", "SetBackdropBorderColor", "SetMovable",
  "SetClampedToScreen", "EnableMouse", "EnableMouseWheel", "RegisterForDrag", "SetFrameStrata",
  "SetJustifyH", "SetJustifyV", "SetFontObject", "SetWordWrap", "SetTextColor", "SetAutoFocus",
  "SetMultiLine", "SetMaxLetters", "SetTextInsets", "SetNumeric", "SetNormalFontObject",
  "SetHighlightFontObject", "SetDisabledFontObject", "SetHighlightTexture", "SetObeyStepOnDrag"}) do
  methods[key] = function() end
end
function methods:SetSize(w,h) self.width,self.height=w,h end
function methods:SetWidth(w) self.width=w end
function methods:SetHeight(h) self.height=h end
function methods:SetJustifyH() end
function methods:GetWidth() return self.width or 400 end
function methods:GetHeight() return self.height or 300 end
function methods:SetPoint(...) self.point={...} end
function methods:GetPoint() return unpack(self.point or {"CENTER", UIParent, "CENTER", 0,0}) end
function methods:ClearAllPoints() self.point=nil end
function methods:SetScript(key, fn) self.scripts[key]=fn end
function methods:GetScript(key) return self.scripts[key] end
function methods:HookScript(key, fn)
  local old=self.scripts[key]
  self.scripts[key]=function(...) if old then old(...) end; fn(...) end
end
function methods:RegisterEvent(event) self.events[event]=true end
function methods:UnregisterEvent(event) self.events[event]=nil end
function methods:Show()
  local changed=not self.shown; self.shown=true
  if changed and self.scripts.OnShow then self.scripts.OnShow(self) end
end
function methods:Hide()
  local changed=self.shown; self.shown=false
  if changed and self.scripts.OnHide then self.scripts.OnHide(self) end
end
function methods:IsShown() return self.shown end
function methods:IsVisible() return self.shown and (not self.parent or self.parent:IsVisible()) end
function methods:GetName() return self.name end
function methods:SetText(text) self.text=tostring(text or "") end
function methods:GetText() return self.text or "" end
function methods:Enable() self.enabled=true end
function methods:Disable() self.enabled=false end
function methods:IsEnabled() return self.enabled end
function methods:EnableMouseMotion(v) self.mouseMotion=v end
function methods:SetFocus() self.focus=true end
function methods:ClearFocus() self.focus=false end
function methods:HasFocus() return self.focus==true end
function methods:HighlightText() end
function methods:Insert(text) self:SetText(self:GetText() .. text) end
function methods:StartMoving() self.moving=true end
function methods:StopMovingOrSizing() self.moving=false end
function methods:SetScrollChild(f) self.child=f end
function methods:GetVerticalScroll() return self.scroll or 0 end
function methods:GetVerticalScrollRange() return math.max(0,(self.child.height or 0)-(self.height or 220)) end
function methods:SetVerticalScroll(v) self.scroll=v end
function methods:Click()
  if self.enabled and self.scripts.OnClick then self.scripts.OnClick(self,"LeftButton") end
end
local function newFrame(kind,name,parent,template)
  assert(not template or not template:find("Secure"), "protected template forbidden")
  local f=setmetatable({kind=kind,name=name,parent=parent,template=template,scripts={},events={},
    shown=true,enabled=true}, {__index=methods})
  FRAMES[#FRAMES+1]=f; if name then _G[name]=f end
  return f
end
function methods:CreateFontString(name,layer,font) return newFrame("FontString",name,self,font) end
function methods:CreateTexture(name,layer) return newFrame("Texture",name,self) end
function methods:SetColorTexture(...) self.color={...} end
function methods:SetTexture(t) self.texture=t end
function methods:SetBackdrop(b) self.backdrop=b end
function methods:SetBackdropColor(...) self.bgColor={...} end
function methods:SetBackdropBorderColor(...) self.borderColor={...} end
function methods:SetTextColor(...) self.textColor={...} end
function methods:SetFontString(fs) self.fontString=fs end
function methods:SetFontObject(f) self.fontObject=f end
function methods:SetTextInsets() end
function methods:HookScript(name,fn) local old=self.scripts[name]; self.scripts[name]=function(...) if old then old(...) end; return fn(...) end end
function methods:GetWidth() return self.width or 0 end
CreateFrame=newFrame
UIParent=newFrame("Frame","UIParent")
GameTooltip=newFrame("GameTooltip","GameTooltip")
function GameTooltip:SetOwner(owner) self.owner=owner end
function GameTooltip:AddLine(text) self.text=(self.text or "") .. "\n" .. text end
C_QuestLog = {
  IsQuestFlaggedCompleted=function(q) return STATE.done[q]==true end,
  IsOnQuest=function(q) return STATE.log[q]==true end,
  ReadyForTurnIn=function(q) return STATE.ready[q]==true end,
  IsComplete=function(q) return STATE.ready[q]==true end,
}
C_Map = {GetBestMapForUnit=function() return STATE.map end, SetUserWaypoint=function(p) WP=p end}
UiMapPoint = {CreateFromCoordinates=function(m,x,y) return {map=m,x=x,y=y} end}
C_SuperTrack = {SetSuperTrackedUserWaypoint=function(v) SUPERTRACK=v end}
C_Timer = {After=function(delay,fn) TIMERS[#TIMERS+1]={due=NOW+delay,fn=fn} end}
function TICK(dt)
  NOW=NOW+(dt or 0.25)
  local pending=TIMERS; TIMERS={}
  for _,t in ipairs(pending) do
    if t.due<=NOW then t.fn() else TIMERS[#TIMERS+1]=t end
  end
  for _,f in ipairs(FRAMES) do if f.scripts.OnUpdate then f.scripts.OnUpdate(f,dt or 0.25) end end
end
function DRAIN(n) for i=1,(n or 20) do TICK(0.25) end end
function FIRE(event,...)
  for _,f in ipairs(FRAMES) do if f.events[event] and f.scripts.OnEvent then f.scripts.OnEvent(f,event,...) end end
end
function ESC()
  for _,name in ipairs(UISpecialFrames) do if _G[name]:IsShown() then _G[name]:Hide() end end
end
function hooksecurefunc(name, fn)
  HOOKS[name]=HOOKS[name] or {}; table.insert(HOOKS[name], fn)
end
function ChatEdit_InsertLink(link)
  for _,fn in ipairs(HOOKS.ChatEdit_InsertLink or {}) do fn(link) end
end
function SetItemRef(link, text, button, frame)
  for _,fn in ipairs(HOOKS.SetItemRef or {}) do fn(link,text,button,frame) end
end
function IsShiftKeyDown() return STATE.shift==true end
function IsModifiedClick() return STATE.shift==true end
'''

ENGINE = r'''
RelaxQuestPlanner = RelaxQuestPlanner or {}
local R = RelaxQuestPlanner
BUILD_CALLS, LEARN_CALLS, PENDING = {}, {}, {}
DATA_SOURCE = "QuestieDB"
DUNGEONS = {{name="Alpha Keep",areaIds={1}},{name="Beta Hall",areaIds={2}}}
ROUTES = {}
R.Data = {
  Ready=function() return true end,
  Source=function() return DATA_SOURCE end,
  Dungeons=function() return DUNGEONS end,
  Quest=function(q) return {id=q,name="Quest "..q} end,
}
R.Player = {Snapshot=function()
  return {level=STATE.level,done=STATE.done,log=STATE.log,ready=STATE.ready,trivial=7,
    raceMask=4,classMask=1,faction="Alliance",professions={}}
end}
local function route(title)
  return {title=title,level=30,floor=23,steps={
    {kind="accept",q=11,name="Far errand",zone="Away",map=102,x=20,y=30,needs={},source="QuestieDB",confidence="db"},
    {kind="accept",q=12,name="Local errand",who="Keeper",zone="Home",map=101,x=40,y=50,needs={},source="FDJ",confidence="observed",below=true,note="unlocks 1 quest"},
    {kind="complete",q=12,name="Local errand",zone="Home",map=101,x=45,y=55,needs={},source="FDJ",confidence="addon"},
    {kind="turnin",q=12,name="Local errand",zone="Home",map=101,x=40,y=50,needs={},source="FDJ",confidence="addon"},
    {kind="accept",q=13,name="Follow-up",zone="Home",needs={12},source="QuestieDB",confidence="db"},
  },later={},issues={}}
end
R.Engine = {
  State=function(q) if STATE.done[q] then return "done",{} end
    if STATE.log[q] then return "inlog",{} end
    return STATE.blocked==q and "blocked" or "available", {} end,
  BuildPathAsync=function(name,P,opts,cb)
    BUILD_CALLS[#BUILD_CALLS+1]={kind="dungeon",name=name,mode=opts.mode,level=P.level}
    local result=ROUTES[name] or route(name)
    if HOLD then PENDING[#PENDING+1]={cb=cb,route=result} else C_Timer.After(0,function() cb(result) end) end
  end,
  BuildGoalAsync=function(q,P,opts,cb)
    BUILD_CALLS[#BUILD_CALLS+1]={kind="goal",q=q,mode=opts.mode}
    C_Timer.After(0,function() cb(route("Goal "..q)) end)
  end,
}
R.Learn={Record=function(kind,q,npc) LEARN_CALLS[#LEARN_CALLS+1]={kind=kind,q=q,npc=npc} end}
'''


def client(before=""):
    lua = lupa.LuaRuntime(unpack_returned_tuples=True)
    lua.execute(WOW)
    lua.execute(ENGINE)
    if before:
        lua.execute(before)
    toc = ADDON / "RelaxQuestPlanner.toc"
    if toc.exists():
        for raw in toc.read_text().splitlines():
            name = raw.strip().replace("\\", "/")
            # Engine and Journey are parallel lanes: their APIs are mocked here.
            if name.endswith(".lua") and not name.startswith(("Core/", "Journey/")):
                lua.execute((ADDON / name).read_text())
    lua.execute('FIRE("ADDON_LOADED", "RelaxQuestPlanner"); FIRE("PLAYER_LOGIN"); DRAIN()')
    return lua


class UIShellTests(unittest.TestCase):
    def test_shell_loads_and_handles_help_hide_show_escape_and_errors(self):
        lua = client()
        self.assertIsNotNone(lua.globals().SlashCmdList.RELAXQUESTPLANNER,
                             "the /rqp handler must be registered")
        lua.execute(r'''
          assert(SLASH_RELAXQUESTPLANNER1=="/rqp")
          assert(RelaxQuestPlannerDB.mode=="game")
          assert(RelaxQuestPlannerFrame:IsShown())
          SlashCmdList.RELAXQUESTPLANNER("help")
          assert(table.concat(OUT,"\n"):find("goal"))
          SlashCmdList.RELAXQUESTPLANNER("hide")
          assert(not RelaxQuestPlannerFrame:IsShown() and RelaxQuestPlannerDB.hidden)
          SlashCmdList.RELAXQUESTPLANNER("")
          assert(RelaxQuestPlannerFrame:IsShown() and not RelaxQuestPlannerDB.hidden)
          ESC(); assert(not RelaxQuestPlannerFrame:IsShown() and RelaxQuestPlannerDB.hidden)
          SlashCmdList.RELAXQUESTPLANNER("show")
          RelaxQuestPlanner.UI.Refresh=function() error("handler sentinel") end
          FIRE("QUEST_LOG_UPDATE"); DRAIN()
          assert(RelaxQuestPlannerDB.lastError:find("handler sentinel"))
        ''')

    def test_dungeon_route_prefers_local_step_and_advances_with_waypoint(self):
        lua = client()
        lua.execute(r'''
          SlashCmdList.RELAXQUESTPLANNER("dungeon alpha keep"); DRAIN()
          local U=RelaxQuestPlanner.UI
          assert(U.current and U.current.q==12, "pick first actionable step in this zone")
          assert(U.current.kind=="accept")
          assert(U.title:GetText():find("Alpha Keep"))
          assert(U.tags:GetText():find("FDJ") and U.tags:GetText():find("observed"))
          assert(U.note:GetText():find("Your choice") and U.note:GetText():find("unlocks 1"))
          assert(U.stepStripe and U.stepStripe.color and U.stepStripe.color[1]==1, "accept stripe is gold")
          SlashCmdList.RELAXQUESTPLANNER("map")
          assert(WP.map==101 and WP.x==0.4 and WP.y==0.5 and SUPERTRACK==true)
          STATE.log[12]=true; FIRE("QUEST_ACCEPTED",1,12); DRAIN()
          assert(U.current.kind=="complete")
          STATE.ready[12]=true; FIRE("QUEST_LOG_UPDATE"); DRAIN()
          assert(U.current.kind=="turnin")
          STATE.done[12]=true; STATE.log[12]=nil; FIRE("QUEST_TURNED_IN",12); DRAIN()
          assert(U.current.q==13 and U.current.kind=="accept")
          STATE.done[13]=true; FIRE("QUEST_LOG_UPDATE"); DRAIN()
          assert(U.current.q==11)
          STATE.done[11]=true; FIRE("QUEST_LOG_UPDATE"); DRAIN()
          assert(U.current==nil and U.line1:GetText():find("All done"))
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_goal_id_link_entry_and_mode_rebuilds(self):
        lua = client()
        lua.execute(r'''
          SlashCmdList.RELAXQUESTPLANNER("goal 4321"); DRAIN()
          assert(BUILD_CALLS[#BUILD_CALLS].kind=="goal", "goal must use BuildGoalAsync")
          assert(BUILD_CALLS[#BUILD_CALLS].q==4321 and BUILD_CALLS[#BUILD_CALLS].mode=="game")
          local U=RelaxQuestPlanner.UI
          assert(U.title:GetText():find("Goal 4321"))
          SlashCmdList.RELAXQUESTPLANNER("mode strict"); DRAIN()
          assert(RelaxQuestPlannerDB.mode=="strict" and BUILD_CALLS[#BUILD_CALLS].mode=="strict")
          assert(U.modeButton:GetText():find("Strict %-5"))
          U.modeButton:Click(); DRAIN()
          assert(RelaxQuestPlannerDB.mode=="all" and BUILD_CALLS[#BUILD_CALLS].mode=="all")
          U.modeButton:Click(); DRAIN(); assert(RelaxQuestPlannerDB.mode=="game")
          U.goalBox:SetFocus()
          STATE.shift=true
          ChatEdit_InsertLink("|cffffff00|Hquest:1234:30|h[A test quest]|h|r")
          U.goalBox.scripts.OnEnterPressed(U.goalBox); DRAIN()
          assert(BUILD_CALLS[#BUILD_CALLS].q==1234)
          U.goalBox:SetFocus(); SetItemRef("quest:9876:30", "Another", "LeftButton")
          U.goalButton:Click(); DRAIN(); assert(BUILD_CALLS[#BUILD_CALLS].q==9876)
          local count=#BUILD_CALLS
          for _,bad in ipairs({"0","-1","1.5","junk","1e4","12 extra"}) do SlashCmdList.RELAXQUESTPLANNER("goal "..bad) end
          SlashCmdList.RELAXQUESTPLANNER("mode rubbish"); DRAIN()
          assert(#BUILD_CALLS==count and RelaxQuestPlannerDB.mode=="game")
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_dungeon_picker_counts_async_scrolls_and_selects(self):
        lua = client('for i=1,14 do DUNGEONS[#DUNGEONS+1]={name="Tower "..i,areaIds={i+2}} end')
        lua.execute(r'''
          SlashCmdList.RELAXQUESTPLANNER("dungeon")
          local U=RelaxQuestPlanner.UI
          assert(U.picker and U.picker:IsShown(), "dungeon command must open the picker")
          assert(#U.dungeonRows==16 and U.dungeonCounts["Alpha Keep"]==nil)
          assert(#BUILD_CALLS<=1, "never build all dungeon counts synchronously")
          DRAIN(80)
          assert(U.dungeonCounts["Alpha Keep"]==3, "count unique quests, not route steps")
          assert(U.dungeonRows[1]:GetText():find("3 quests"))
          U.dungeonScroll.scripts.OnMouseWheel(U.dungeonScroll,-3)
          assert(U.dungeonScroll:GetVerticalScroll()>0)
          U.dungeonRows[2]:Click(); DRAIN()
          assert(RelaxQuestPlannerDB.selection.name=="Beta Hall" and not U.picker:IsShown())
          assert(U.title:GetText():find("Beta Hall"))
          U.dungeonButton:Click(); SlashCmdList.RELAXQUESTPLANNER("mode strict"); DRAIN(90)
          assert(U.dungeonCounts["Alpha Keep"]==3 and BUILD_CALLS[#BUILD_CALLS].mode=="strict")
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_missing_questiedb_status_survives_a_fallback_route(self):
        lua = client('DATA_SOURCE="FDJ-only"')
        lua.execute(r'''
          local U=RelaxQuestPlanner.UI
          assert(U.status:GetText():find("QuestieDB missing: limited data"), "show limited-data status")
          SlashCmdList.RELAXQUESTPLANNER("dungeon Alpha Keep")
          assert(U.status:GetText():find("QuestieDB missing: limited data"))
          DRAIN(); assert(U.status:GetText():find("QuestieDB missing: limited data"))
          assert(U.current.q==12)
          RelaxQuestPlanner.Data.Ready=function() return false,"No usable data addons" end
          DUNGEONS={}; SlashCmdList.RELAXQUESTPLANNER("dungeon"); DRAIN()
          assert(U.pickerStatus:GetText():find("No dungeon data"))
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_quest_events_record_npc_observations_and_throttle_replans(self):
        lua = client()
        lua.execute(r'''
          SlashCmdList.RELAXQUESTPLANNER("dungeon Alpha Keep"); DRAIN()
          local before=#BUILD_CALLS
          FIRE("QUEST_ACCEPTED",4,12); FIRE("QUEST_TURNED_IN",12,500,0)
          assert(#LEARN_CALLS==2, "forward accept and turn-in observations")
          assert(LEARN_CALLS[1].kind=="accept" and LEARN_CALLS[1].q==12 and LEARN_CALLS[1].npc==901)
          assert(LEARN_CALLS[2].kind=="turnin" and LEARN_CALLS[2].q==12 and LEARN_CALLS[2].npc==901)
          for i=1,30 do FIRE("QUEST_LOG_UPDATE") end
          FIRE("QUEST_REMOVED",12); FIRE("ZONE_CHANGED"); FIRE("ZONE_CHANGED_INDOORS"); FIRE("ZONE_CHANGED_NEW_AREA")
          assert(#BUILD_CALLS==before); TICK(0.19); assert(#BUILD_CALLS==before)
          TICK(0.02); assert(#BUILD_CALLS==before+1); DRAIN()
          FIRE("QUEST_ACCEPTED",13); assert(LEARN_CALLS[3].q==13)
          STATE.guid=SECRET; FIRE("QUEST_ACCEPTED",14); assert(LEARN_CALLS[4].npc==nil)
          FIRE("QUEST_ACCEPTED",SECRET); assert(#LEARN_CALLS==4)
          RelaxQuestPlanner.Learn.Record=function() error("learning sentinel") end
          before=#BUILD_CALLS; FIRE("QUEST_TURNED_IN",12); DRAIN()
          assert(RelaxQuestPlannerDB.lastError:find("learning sentinel"))
          assert(#BUILD_CALLS==before+1, "an observation error must not stop refresh")
        ''')

    def test_position_persistence_escape_and_combat_safe_controls(self):
        lua = client('RelaxQuestPlannerDB={mode="all",hidden=true,position={point="TOPLEFT",relativePoint="TOPLEFT",x=83.5,y=-94.5}}')
        lua.execute(r'''
          local U=RelaxQuestPlanner.UI
          local point,relative,relativePoint,x,y=U.frame:GetPoint()
          assert(point=="TOPLEFT" and x==83.5 and y==-94.5, "restore saved anchor")
          assert(not U.frame:IsShown() and U.modeButton:GetText()=="Show all")
          SlashCmdList.RELAXQUESTPLANNER("show")
          U.frame.scripts.OnDragStart(U.frame); assert(U.frame.moving)
          U.frame:SetPoint("BOTTOM",UIParent,"BOTTOM",112.5,93.5)
          U.frame.scripts.OnDragStop(U.frame)
          assert(not U.frame.moving and RelaxQuestPlannerDB.position.x==112.5)
          assert(RelaxQuestPlannerDB.position.relativePoint=="BOTTOM")
          STATE.combat=true
          SlashCmdList.RELAXQUESTPLANNER("goal 101"); DRAIN()
          U.modeButton:Click(); DRAIN(); U.mapButton:Click()
          U.dungeonButton:Click(); DRAIN(); ESC()
          assert(not U.frame:IsShown() and not U.picker:IsShown())
          SlashCmdList.RELAXQUESTPLANNER("show"); U.goalBox:SetFocus()
          U.goalBox.scripts.OnEscapePressed(U.goalBox)
          assert(not U.frame:IsShown() and not U.goalBox:HasFocus())
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_journey_picker_optional_button_top_three_and_route_handoff(self):
        lua = client()
        lua.execute(r'''
          local U=RelaxQuestPlanner.UI
          assert(U.guideButton and not U.guideButton:IsEnabled(), "disable optional guide picker")
          U.guideButton.scripts.OnEnter(U.guideButton)
          assert(GameTooltip:GetText():find("Journey"))
          SlashCmdList.RELAXQUESTPLANNER("guide")
          assert(table.concat(OUT,"\n"):find("Journey"))
          RelaxQuestPlanner.Journey={
            PickGuide=function(cb)
              PICK_CALLS=(PICK_CALLS or 0)+1
              C_Timer.After(0,function() cb({
                {guideID="one",description="First guide",startStepID="q1",reasons={"3 remaining quests"}},
                {guideID="two",description="Second guide",startStepID="q2",reasons={"2 remaining quests"}},
                {guideID="three",description="Third guide",startStepID="q3",reasons={"1 remaining quest"}}}) end)
              return true
            end,
            GotoBest=function(choice) WENT=choice; return true end,
            LoadRoute=function(route) LOADED=route; return true end,
          }
          RelaxJourneyAPI={RegisterLocalGuide=function() end,Goto=function() end}
          U.Refresh(); assert(U.guideButton:IsEnabled())
          U.guideButton:Click(); DRAIN()
          assert(PICK_CALLS==1 and #U.guideRows==3 and U.guidePicker:IsShown())
          assert(U.guideRows[1]:GetText():find("First guide"))
          U.guideRows[2]:Click(); assert(WENT.guideID=="two" and WENT.startStepID=="q2")
          assert(not U.guidePicker:IsShown())
          SlashCmdList.RELAXQUESTPLANNER("guide"); DRAIN(); assert(PICK_CALLS==2)
          SlashCmdList.RELAXQUESTPLANNER("dungeon Alpha Keep"); DRAIN()
          U.journeyButton:Click(); assert(LOADED==U.route)
          STATE.combat=true; LOADED=nil; U.journeyButton:Click(); assert(LOADED==nil)
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_async_route_replans_coalesce_ignore_stale_and_stay_hidden(self):
        lua = client()
        lua.execute(r'''
          HOLD=true
          SlashCmdList.RELAXQUESTPLANNER("dungeon Alpha Keep")
          local U=RelaxQuestPlanner.UI
          for i=1,10 do FIRE("QUEST_LOG_UPDATE"); TICK(0.25) end
          assert(#BUILD_CALLS==1, "coalesce events while a build is in flight")
          SlashCmdList.RELAXQUESTPLANNER("dungeon Beta Hall")
          assert(#PENDING==2)
          PENDING[2].cb(PENDING[2].route)
          SlashCmdList.RELAXQUESTPLANNER("hide")
          PENDING[1].cb(PENDING[1].route)
          assert(U.route.title=="Beta Hall" and not U.frame:IsShown())
          assert(not U.busy)
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_empty_failed_routes_do_not_claim_done_and_errors_are_caught(self):
        lua = client()
        lua.execute(r'''
          ROUTES["Alpha Keep"]={title="Alpha Keep",steps={},later={},issues={{"error","test data unavailable"}}}
          SlashCmdList.RELAXQUESTPLANNER("dungeon Alpha Keep"); DRAIN()
          local U=RelaxQuestPlanner.UI
          assert(not U.line1:GetText():find("All done"), "failed empty plans are not completion")
          assert(U.status:GetText():find("test data unavailable"))
          RelaxQuestPlanner.Engine.BuildPathAsync=function() error("planner sentinel") end
          SlashCmdList.RELAXQUESTPLANNER("dungeon Beta Hall")
          assert(not U.busy and RelaxQuestPlannerDB.lastError:find("planner sentinel"))
          assert(U.status:GetText():find("Planning failed"))
          RelaxQuestPlanner.Player.Snapshot=function() error("snapshot sentinel") end
          SlashCmdList.RELAXQUESTPLANNER("dungeon Alpha Keep")
          assert(not U.busy and RelaxQuestPlannerDB.lastError:find("snapshot sentinel"))
        ''')

    def test_secret_quest_id_is_not_replaced_with_a_log_index(self):
        lua = client()
        lua.execute(r'''
          FIRE("QUEST_ACCEPTED",4,SECRET)
          assert(#LEARN_CALLS==0, "a restricted quest ID must not teach a false quest 4")
          FIRE("QUEST_ACCEPTED",4,nil)
          assert(#LEARN_CALLS==0)
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_no_data_status_reports_dependency_warning_once(self):
        lua = client('DATA_SOURCE="FDJ-only"; RelaxQuestPlanner.Data.Ready=function() return false,"QuestieDB missing: limited data" end')
        lua.execute(r'''
          ROUTES["Alpha Keep"]={title="Alpha Keep",steps={},later={},issues={{"missingData","QuestieDB missing: limited data"}}}
          SlashCmdList.RELAXQUESTPLANNER("dungeon Alpha Keep"); DRAIN()
          local text=RelaxQuestPlanner.UI.status:GetText()
          local _,count=text:gsub("QuestieDB missing: limited data","")
          assert(count==1, "do not repeat the dependency warning")
          assert(text:find("No usable quest data addons"))
        ''')

    def test_failed_async_count_is_unknown_not_zero(self):
        lua = client()
        lua.execute(r'''
          ROUTES["Alpha Keep"]={title="Alpha Keep",steps={},later={},issues={{"error","count sentinel"}}}
          SlashCmdList.RELAXQUESTPLANNER("dungeon"); DRAIN()
          local U=RelaxQuestPlanner.UI
          assert(U.dungeonCounts["Alpha Keep"]==nil, "a failed count is not zero quests")
          assert(U.dungeonRows[1]:GetText():find("unavailable"))
          assert(U.dungeonCounts["Beta Hall"]==3, "continue after one failed dungeon")
        ''')

    def test_mode_change_invalidates_guide_results_and_routes_close_picker(self):
        lua = client('GUIDE_PENDING={}; RelaxQuestPlanner.Journey={PickGuide=function(cb) GUIDE_PENDING[#GUIDE_PENDING+1]=cb; return true end}')
        lua.execute(r'''
          SlashCmdList.RELAXQUESTPLANNER("guide")
          SlashCmdList.RELAXQUESTPLANNER("mode strict")
          assert(#GUIDE_PENDING==2, "rescan displayed guide choices after a mode change")
          local U=RelaxQuestPlanner.UI
          GUIDE_PENDING[1]({{guideID="old",description="Old mode",reasons={}}})
          assert(not U.guideRows[1]:IsShown(), "ignore stale scan results")
          GUIDE_PENDING[2]({{guideID="new",description="Current mode",reasons={}}})
          assert(U.guideRows[1]:GetText():find("Current mode"))
          SlashCmdList.RELAXQUESTPLANNER("dungeon Alpha Keep"); DRAIN()
          assert(not U.guidePicker:IsShown())
          SlashCmdList.RELAXQUESTPLANNER("guide"); SlashCmdList.RELAXQUESTPLANNER("goal 123"); DRAIN()
          assert(not U.guidePicker:IsShown())
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_waypoint_missing_secret_and_handler_error_paths(self):
        lua = client()
        lua.execute(r'''
          local U=RelaxQuestPlanner.UI
          SlashCmdList.RELAXQUESTPLANNER("map")
          assert(OUT[#OUT]:find("No map pin"))
          U.current={map=101,x=40}; U.mapButton:Click(); assert(WP==nil)
          U.current={map=SECRET,x=40,y=50}; U.mapButton:Click(); assert(WP==nil)
          U.current={map=101,x=40,y=50}
          C_Map.SetUserWaypoint=function() error("map handler sentinel") end
          U.mapButton:Click(); assert(RelaxQuestPlannerDB.lastError:find("map handler sentinel"))
          C_Map.SetUserWaypoint=nil; U.mapButton:Click()
          assert(OUT[#OUT]:find("unavailable"))
        ''')

    def test_actionability_gates_and_zone_changes_repick(self):
        lua = client()
        lua.execute(r'''
          STATE.blocked=12
          SlashCmdList.RELAXQUESTPLANNER("dungeon Alpha Keep"); DRAIN()
          local U=RelaxQuestPlanner.UI
          assert(U.current.q==11, "do not prefer a local blocked acceptance")
          STATE.blocked=nil; STATE.map=102; STATE.zone="Away"; FIRE("ZONE_CHANGED_NEW_AREA"); DRAIN()
          assert(U.current.q==11)
          STATE.map=SECRET; STATE.zone="Home"; FIRE("ZONE_CHANGED_INDOORS"); DRAIN()
          assert(U.current.q==12)
          STATE.log[12]=true; STATE.ready[12]=nil; FIRE("QUEST_LOG_UPDATE"); DRAIN()
          assert(U.current.kind=="complete", "do not offer a turn-in before it is ready")
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_timer_fallback_uses_guarded_onupdate(self):
        lua = client(r'''
          C_Timer=nil
          RelaxQuestPlanner.Engine.BuildPathAsync=function(name,P,opts,cb)
            BUILD_CALLS[#BUILD_CALLS+1]={name=name}
            RelaxQuestPlanner.After(0,function() cb({title=name,steps={},issues={},later={}}) end)
          end
        ''')
        lua.execute(r'''
          SlashCmdList.RELAXQUESTPLANNER("dungeon Alpha Keep"); DRAIN()
          assert(RelaxQuestPlanner.UI.route.title=="Alpha Keep")
          FIRE("QUEST_REMOVED",123); DRAIN()
          assert(#BUILD_CALLS==2)
          RelaxQuestPlanner.After(0,function() error("fallback timer sentinel") end); DRAIN()
          assert(RelaxQuestPlannerDB.lastError:find("fallback timer sentinel"))
        ''')

    def test_saved_selection_restores_without_unhiding_window(self):
        lua = client('RelaxQuestPlannerDB={mode="strict",hidden=true,selection={kind="dungeon",name="Alpha Keep"}}')
        lua.execute(r'''
          assert(RelaxQuestPlanner.UI.route.title=="Alpha Keep")
          assert(BUILD_CALLS[1].mode=="strict")
          assert(not RelaxQuestPlannerFrame:IsShown() and RelaxQuestPlannerDB.hidden)
          SlashCmdList.RELAXQUESTPLANNER("nonsense")
          SlashCmdList.RELAXQUESTPLANNER("dungeon not a real dungeon")
          assert(OUT[#OUT]:find("Unknown dungeon"))
          assert(RelaxQuestPlannerDB.lastError==nil, RelaxQuestPlannerDB.lastError)
        ''')

    def test_toc_contract_and_no_protected_quest_actions(self):
        toc = (ADDON / "RelaxQuestPlanner.toc").read_text().splitlines()
        metadata = dict(line[3:].split(": ", 1) for line in toc if line.startswith("## "))
        self.assertEqual(metadata["Interface"], "16001")
        self.assertEqual(metadata["Title"], "Relax Quest Planner")
        self.assertEqual(metadata["Author"], "TheGamersVibe")
        self.assertEqual(metadata["Version"], "0.1.0-test")
        self.assertEqual(metadata["SavedVariablesPerCharacter"], "RelaxQuestPlannerDB")
        self.assertEqual(metadata["OptionalDeps"].split(", "), [
            "QuestieDB", "Questie", "ForeverDungeonJournal", "DungeonGuideForever", "Journey"])
        scripts = [line.replace("\\", "/") for line in toc if line.endswith(".lua")]
        self.assertEqual(scripts[:7], ["Init.lua", "Core/Data.lua", "Core/Player.lua", "Core/Engine.lua",
                                     "Core/Learn.lua", "Journey/Bridge.lua", "Journey/Picker.lua"])
        self.assertTrue(all(name.startswith("UI/") for name in scripts[7:]))
        owned = [ADDON / "Init.lua", *sorted((ADDON / "UI").glob("*.lua"))]
        for path in owned:
            text = path.read_text()
            for forbidden in ("AcceptQuest(", "CompleteQuest(", "GetQuestReward(", "AbandonQuest(",
                              "UnitHealth(", "SecureActionButtonTemplate", "SetAttribute("):
                self.assertNotIn(forbidden, text, str(path))
            self.assertTrue(all(line.rstrip() == line for line in text.splitlines()), str(path))


if __name__ == "__main__":
    unittest.main(verbosity=2)
