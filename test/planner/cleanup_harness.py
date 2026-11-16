"""Authored fixtures only; Forever bebcb6bf / 1.60.1.70938 API shapes.
No provider/donor modules are executed. Uses the existing strict UI mock.
"""
from pathlib import Path
import sys
import lupa.lua51 as lupa
sys.path.insert(0, str(Path(__file__).resolve().parent))
from ui_shell_test import WOW
ROOT = Path(__file__).resolve().parents[2]
ADDON = ROOT / 'addon/RelaxQuestPlanner'

FIXTURE = r'''
RelaxQuestPlannerDB={}
Q,I={},{}
READS=0
function API(rows)
  return {Exists=function(id) return rows[id]~=nil end,
    GetAllIds=function() local t={} for id in pairs(rows) do t[#t+1]=id end table.sort(t) return t end,
    Get=function(id,key) READS=READS+1; return rows[id] and rows[id][key] end}
end
LibQuestieDB={Quest=API(Q),Item=API(I),contractVersion=4,flavor={expansion='Forever'},
  addonName='QuestieDB',readMode='baked',RequireContract=function(n) return n<=4 end}
C_AddOns={GetAddOnMetadata=function(_,key) if key=='Version' then return '1.1.1' end end}
function GetBuildInfo() return '1.60.1','70938','date',16001 end
function UnitGUID() return 'Player-fixture' end
function UnitLevel() return 40 end
function UnitClass() return 'Warrior','WARRIOR',1 end
function UnitFactionGroup() return 'Alliance' end
function time() return 123456 end
function debugprofilestop() return NOW*1000 end
LOG,DONE,BAGS={},{},{}
for b=0,4 do BAGS[b]={} end
C_QuestLog.GetNumQuestLogEntries=function() return #LOG,#LOG end
C_QuestLog.GetInfo=function(i) return LOG[i] end
C_QuestLog.GetAllCompletedQuestIDs=function() return DONE end
C_QuestLog.IsQuestTrivial=function() return true end
C_QuestLog.ReadyForTurnIn=function() return false end
C_QuestLog.IsRepeatableQuest=function() return false end
C_Container={
  GetContainerNumSlots=function(b) assert(b>=0 and b<=4); return #BAGS[b] end,
  GetContainerNumFreeSlots=function(b) return 0,0 end,
  GetContainerItemInfo=function(b,s) return BAGS[b][s] end,
  GetContainerItemID=function(b,s) return BAGS[b][s] and BAGS[b][s].itemID end,
  GetContainerItemQuestInfo=function(b,s) return {isQuestItem=true,isActive=false} end,
}
C_Item={GetItemInfoInstant=function(id) return id,'Quest','Quest','',0,12,0 end}
function QUEST(id,extra)
  local q={name='Quest '..id,questLevel=20,requiredRaces=0,requiredClasses=0,
    specialFlags=0,questFlags=0,type=0}
  for k,v in pairs(extra or {}) do q[k]=v end
  Q[id]=q; return q
end
function ITEM(id,extra)
  local r={name='Item '..id}; for k,v in pairs(extra or {}) do r[k]=v end
  I[id]=r; return r
end
function BAG(id,count) local t=BAGS[0]; t[#t+1]={itemID=id,stackCount=count or 1,isLocked=false} end
function CURRENT(id) LOG[#LOG+1]={questID=id,title='Quest '..id,isHeader=false} end
'''

def client(modules=('Core/Data.lua', 'Core/CleanupData.lua'), before=''):
    lua = lupa.LuaRuntime(unpack_returned_tuples=True)
    lua.execute(WOW)
    lua.execute(FIXTURE)
    lua.execute((ADDON/'Init.lua').read_text())
    lua.execute(before)
    for name in modules:
        path = ADDON/name
        if path.exists():
            if name == 'Core/CleanupUses.lua':
                lua.execute('return function(...)\n'+path.read_text()+'\nend')('RelaxQuestPlanner', lua.globals().RelaxQuestPlanner.CleanupUses)
            else:
                lua.execute(path.read_text(), name=str(path))
    return lua

POLICY = ('Core/Data.lua','Core/CleanupData.lua','Core/CleanupSnapshot.lua','Core/CleanupUses.lua','Core/Cleanup.lua')

def policy(before=''):
    lua=client(POLICY, before)
    lua.execute('''
      function SCAN()
        local C=RelaxQuestPlanner.Cleanup
        local snapshot=C.Capture()
        C.BuildIndexAsync(snapshot.provider,function(index) INDEX=index end)
        DRAIN(3000)
        REPORT=C.Evaluate(snapshot,INDEX)
        return REPORT
      end
      function ROW(id,kind)
        for _,r in ipairs((kind=='quest' and REPORT.questRows or REPORT.itemRows)) do
          if r.exactID==id then return r end
        end
      end
    ''')
    return lua
