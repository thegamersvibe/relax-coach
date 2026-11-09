"""Lua 5.1 harness: real Journey facade, loader normalization and navigation.
Only the game APIs, catalog transport, text lookup and UI are mocked. Private
content never becomes a repository fixture. Run tests with uv --with lupa.
"""
from pathlib import Path
from typing import Any
from lupa.lua51 import LuaRuntime

ROOT = Path(__file__).resolve().parents[2]
JOURNEY = ROOT / "addon/Journey"
RQP = ROOT / "addon/RelaxQuestPlanner/Journey"


def runtime() -> tuple[Any, Any]:
    # Lua tables intentionally expose runtime-defined attributes.
    lua: Any = LuaRuntime(unpack_returned_tuples=True)
    lua.execute('''
        FACADE = {}
        C_AddOns = {GetAddOnMetadata=function(_, key)
            if key == 'Title' then return 'Journey' end
            if key == 'Version' then return '4.00.06' end
        end}
        FRAMES = {}
        function CreateFrame()
            local f={scripts={}}
            function f:SetScript(name, fn) self.scripts[name]=fn end
            function f:RegisterEvent() end
            function f:UnregisterEvent() end
            function f:Hide() end
            function f:Show() end
            FRAMES[#FRAMES+1]=f
            return f
        end
        function Tick()
            for _, f in ipairs(FRAMES) do
                if f.scripts.OnUpdate then f.scripts.OnUpdate(f, 0.016) end
            end
        end
        function debugprofilestop() return os.clock()*1000 end
        function issecretvalue(v) return v == SECRET end
        SECRET = {}
        function UnitLevel() return LEVEL or 30 end
        function UnitFactionGroup() return FACTION or 'Alliance' end
        function UnitRace() return 'Dwarf', 'Dwarf', 3 end
        function UnitClass() return 'Warrior', 'WARRIOR', 1 end
        function InCombatLockdown() return false end
        function Mixin(dst, ...)
            for i=1,select('#',...) do
                for k,v in pairs(select(i,...)) do dst[k]=v end
            end
            return dst
        end
        STATES, MESSAGES, LOAD_CALLS = {}, {}, {}
        C_QuestLog = {
            IsQuestFlaggedCompleted=function(q) return STATES[q] == 3 end,
            IsOnQuest=function(q) return STATES[q] == 1 or STATES[q] == 2 end,
            ReadyForTurnIn=function(q) return STATES[q] == 2 end,
            IsComplete=function(q) return STATES[q] == 2 end,
            GetAllCompletedQuestIDs=function()
                local t={} for q,s in pairs(STATES) do if s==3 then t[#t+1]=q end end return t
            end,
            GetNumQuestLogEntries=function() return 0 end,
            GetInfo=function() end,
            GetTrivialRange=function() return 7 end,
        }
        C_Map={
            GetWorldPosFromMapPos=function(map,pos)return map,{x=pos.x,y=pos.y} end,
            GetMapPosFromWorldPos=function(map,pos)return map,{x=pos.x,y=pos.y} end,
        }
        function strsplit(sep, s)
            local result={}; local start=1
            while true do
                local first,last=string.find(s,sep,start,true)
                if not first then result[#result+1]=string.sub(s,start); break end
                result[#result+1]=string.sub(s,start,first-1); start=last+1
            end
            return unpack(result)
        end
        DEFAULT_CHAT_FRAME={AddMessage=function(_,s)MESSAGES[#MESSAGES+1]=s end}
        RelaxQuestPlanner = {}
        RelaxQuestPlannerDB = {mode='strict'}
    ''')
    lua.execute((JOURNEY / 'lib/TomCats/GlobalFacade.lua').read_text(), 'Journey', lua.globals().FACADE)
    env = lua.execute('''
        select(2,...).SetupGlobalFacade()
        I, Brand, L = {}, {}, setmetatable({}, {__index=function(_,k)return k end})
        Color={QUEST_DO='ffffff'}
        IconService={GetIconInfo=function(k)return k end}
        AuraService={PlayerHasBuff=function()return false end,PlayerHasDebuff=function()return false end}
        QuestStatus={status={NONE=0,ACCEPTED=1,COMPLETED=2,TURNEDIN=3},
            GetQuestStatus=function(q)return STATES[q] or 0 end}
        BOOKMARKS={}
        State={IsDebugEnabled=function()return DEBUG_VALIDATOR ~= false end,
            GetBookmark=function(id) BOOKMARKS[id]=BOOKMARKS[id] or {} return BOOKMARKS[id] end,
            SetGuide=function(id) LAST_GUIDE=id end,
            SetGuideShown=function(v) GUIDE_SHOWN=v end}
        UI={MarkDirty=function()end}
        function MarkAllDirty() end
        function ReadHTMLTemplate() end
        function GetWowVersion()return 16001 end
        function C_QuestLog_GetQuestObjectives() return {} end
        LocationMixin={}
        LOCATION_DEFAULT_RADIUS=15
        QuestTextService={TryCreateTextTask=function()return nil end}
        MULTITEXT_IGNORED_KEYS={condition=true,completed=true,optional=true}
        Flags={HasFlag=function(obj,flag)return type(obj.flags)=='string' and string.find(obj.flags,flag,1,true)~=nil end}
        TriggerService={OnComplete=function()end, OnLoad=function()end}
        LocationsService={SetGuide=function()end,SetStep=function()end}
        Debug={AnnounceStep=function()end}
        MERCHANT_TYPE={}
        GUIDE_TYPE={NONE=0,LEVELING=1,RUNE=2}
        RunesService={IsRuneGuide=function()return false end}
        INFOS={{guideID='A_TEST',moduleID='test',flags='B',condition='ALLIANCE',
            description='Test 27-32',levels='27-32'}}
        CatalogService={
            CheckCompatibility=function()return INCOMPATIBLE end,
            GetGuideInfos=function()return INFOS end,
            GetGuideTemplate=function(id)return 'template:'..id end,
            IsInstalled=function(id) for _,i in ipairs(INFOS) do if i.guideID==id then return true end end return false end,
            IsExpired=function()return EXPIRED end,
            GetDecryptKey=function()return 'test-key' end,
            GetContentHash=function()return 'test-hash' end,
            GetSlotInfo=function()return {prefix='External'} end,
            Reload=function() RELOADS=(RELOADS or 0)+1 end,
        }
        LoadHTMLData=function(template, merchant, key, hash)
            LOAD_CALLS[#LOAD_CALLS+1]={template=template,key=key,hash=hash}
            if HASH_FAIL then return nil,'hash_mismatch' end
            return BUILD(template)
        end
        BUILD=function()return {{id='one',{{accept=101}}},{id='end',completed='false',{{text='End'}}}} end
        return getfenv(1)
    ''', 'Journey', lua.globals().FACADE)
    sources = ['Brand-Ext.lua','Tools.lua','ui/TaskType.lua','ui/tasks/DoQuestMixin.lua',
               'ui/tasks/Accept.lua','ui/tasks/AcceptOne.lua','ui/tasks/Complete.lua',
               'ui/tasks/StartWork.lua','ui/tasks/Turnin.lua',
               'Condition.lua','Validator.lua','GuideModules.lua',
               'services/LocationsInit.lua','services/ButtonGeneration.lua',
               'services/Guide.lua','services/Evaluation.lua','services/GuideNavigation.lua']
    for relative in sources:
        lua.execute((JOURNEY / relative).read_text(), 'Journey', lua.globals().FACADE)
    lua.execute('''
        select(2,...).SetupGlobalFacade()
        RegisterTaskType(Mixin({type='text'},TaskTypeMixin))
        ConditionContext.ALLIANCE=true
        ConditionContext.DWARF=true
        ConditionContext.WARRIOR=true
        ConditionContext.ERA=true
        ConditionContext.FOREVER=true
        ConditionContext.QUEST=setmetatable({}, {__index=function(_,q)return STATES[q] or 0 end})
        ConditionContext.OBJECTIVE=setmetatable({}, {__index=function()return {} end})
        ConditionContext.TGLOCATION_ENABLED=setmetatable({}, {__index=function()return true end})
        for k,v in pairs(QuestStatus.status) do ConditionContext[k]=v end
        GuideService.Init()
        GuideModules.Reload()
    ''', 'Journey', lua.globals().FACADE)
    return lua, env


def planner_runtime():
    """Real Journey state/services plus planner UI; only game and route builder mocked."""
    from ui_shell_test import WOW, ENGINE
    lua, env = runtime()
    lua.execute((JOURNEY / 'State.lua').read_text(), 'Journey', lua.globals().FACADE)
    lua.execute("""
        select(2,...).SetupGlobalFacade()
        SavedVariables={}; SavedVariablesPerCharacter={bookmarks={}}; PLAYER_FACTION='ALLIANCE'
    """, 'Journey', lua.globals().FACADE)
    lua.execute('SAVED_MAP=C_Map')
    lua.execute(WOW)
    lua.execute('''
        for k,v in pairs(SAVED_MAP) do C_Map[k]=v end
        setmetatable(STATES,{__index=function(_,q)
            if STATE.done[q] then return 3 end
            if STATE.log[q] then return STATE.ready[q] and 2 or 1 end
            return 0
        end})
    ''')
    lua.execute(ENGINE)
    lua.execute((RQP.parent / 'Init.lua').read_text())
    load_rqp(lua, 'Bridge.lua')
    for name in ('Common', 'Steps', 'Window', 'Picker', 'Guides', 'Controller'):
        lua.execute((RQP.parent / 'UI' / (name + '.lua')).read_text())
    lua.execute("FIRE('ADDON_LOADED','RelaxQuestPlanner'); FIRE('PLAYER_LOGIN'); DRAIN()")
    return lua, env


def load_rqp(lua, name):
    path = RQP / name
    assert path.is_file(), f'{name} must exist'
    lua.execute(path.read_text(), 'RelaxQuestPlanner', lua.globals().RelaxQuestPlanner)


def table(lua, value):
    if isinstance(value, dict):
        return lua.table_from({k: table(lua, v) for k, v in value.items()})
    if isinstance(value, (list, tuple)):
        return lua.table_from([table(lua, v) for v in value])
    return value


def drain(lua, limit=10000):
    for count in range(limit):
        pending = lua.eval("(function() for _,f in ipairs(FRAMES) do if f.scripts.OnUpdate then return true end end return false end)()")
        if not pending:
            return count
        lua.globals().Tick()
    raise AssertionError('worker did not terminate')
