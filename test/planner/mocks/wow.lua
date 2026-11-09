-- Offline WoW surface, intentionally only the modern APIs in the contract.
TEST_FRAMES, TEST_CLOCK, TEST_TICKS, TEST_MAX_FRAME = {}, 0, 0, 0
TEST_CLOCK_STEP = 0.05
function debugprofilestop() TEST_CLOCK=TEST_CLOCK+TEST_CLOCK_STEP; return TEST_CLOCK end
function GetTime() return TEST_CLOCK/1000 end
function issecretvalue(v) return type(v)=='table' and rawget(v,'secret')==true end
function CreateFrame()
    local f={scripts={},events={}}
    function f:SetScript(k,v) self.scripts[k]=v end
    function f:GetScript(k) return self.scripts[k] end
    function f:RegisterEvent(e) self.events[e]=true end
    function f:UnregisterEvent(e) self.events[e]=nil end
    function f:Show() end
    function f:Hide() end
    TEST_FRAMES[#TEST_FRAMES+1]=f
    return f
end
function TEST_DriveFrames(limit)
    for i=1,limit do
        local active=false; local start=TEST_CLOCK
        for _,f in ipairs(TEST_FRAMES) do
            if f.scripts.OnUpdate then active=true; f.scripts.OnUpdate(f,0.016) end
        end
        if not active then return end
        TEST_TICKS=TEST_TICKS+1
        TEST_MAX_FRAME=math.max(TEST_MAX_FRAME,TEST_CLOCK-start)
    end
    error('frame driver did not drain')
end
function TEST_Event(event,...)
    for _,f in ipairs(TEST_FRAMES) do
        if f.events[event] and f.scripts.OnEvent then f.scripts.OnEvent(f,event,...) end
    end
end
function UnitLevel() return TEST_SNAPSHOT.level end
function UnitRace() return TEST_SNAPSHOT.race,TEST_SNAPSHOT.race,TEST_SNAPSHOT.raceID or 3 end
function UnitClass() return TEST_SNAPSHOT.class,TEST_SNAPSHOT.class,1 end
function UnitFactionGroup() return TEST_SNAPSHOT.faction end
function UnitGUID(unit) if unit=='npc' then return TEST_NPC_GUID end; return 'Player-test' end
function GetQuestID() return TEST_QUEST_ID end
C_QuestLog={}
function C_QuestLog.GetAllCompletedQuestIDs() return TEST_SNAPSHOT.completed end
function C_QuestLog.IsQuestFlaggedCompleted(q)
    for _,id in ipairs(TEST_SNAPSHOT.completed) do if q==id then return true end end
    return false
end
function C_QuestLog.IsOnQuest(q)
    for _,e in ipairs(TEST_SNAPSHOT.log) do if q==e.id then return true end end
    return false
end
function C_QuestLog.ReadyForTurnIn(q)
    for _,e in ipairs(TEST_SNAPSHOT.log) do if q==e.id then return e.complete end end
    return false
end
C_QuestLog.IsComplete=C_QuestLog.ReadyForTurnIn
function C_QuestLog.GetNumQuestLogEntries() return #TEST_SNAPSHOT.log,#TEST_SNAPSHOT.log end
function C_QuestLog.GetInfo(i)
    local e=TEST_SNAPSHOT.log[i]
    if e then return {questID=e.id,title=e.title,level=e.level,isHeader=false} end
end
function C_QuestLog.GetTrivialRange() return TEST_SNAPSHOT.trivialRange or 7 end
C_SkillInfo={}
function C_SkillInfo.GetNumSkillLines() return #(TEST_SNAPSHOT.skillLines or {}) end
function C_SkillInfo.GetSkillLineInfo(i)
    local s=TEST_SNAPSHOT.skillLines[i]
    return s.name,false,false,s.rank,0,0,300
end
C_Reputation={
    GetNumFactions=function() return #(TEST_SNAPSHOT.reputationLines or {}) end,
    GetFactionDataByIndex=function(i) return TEST_SNAPSHOT.reputationLines[i] end
}
C_Map={GetMapInfo=function(id) return {name='Map '..id} end}
function GetAreaInfo(id) return 'Area '..id end
RelaxQuestPlannerDB={}
