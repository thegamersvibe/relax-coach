RelaxQuestPlanner = RelaxQuestPlanner or {}
local RQP=RelaxQuestPlanner
RQP.Player=RQP.Player or {}
local Player=RQP.Player
local function secret(v) return issecretvalue and issecretvalue(v) end
function Player.Number(v,default)
    if secret(v) then return default end
    local n=tonumber(v); if n and n==n and n~=math.huge and n~=-math.huge then return n end
    return default
end
local function safe(fn,...)
    if type(fn)~="function" then return nil end
    local ok,a,b,c,d,e,f,g,h=pcall(fn,...)
    if ok then return a,b,c,d,e,f,g,h end
    if RQP.Data then RQP.Data.Error(a) end
end
function Player.Snapshot()
    local n=Player.Number
    local _,race,raceID=safe(UnitRace,"player")
    local _,class=safe(UnitClass,"player")
    local enum=RQP.Data.Enums()
    local p={level=n(safe(UnitLevel,"player"),0),raceMask=0,classMask=0,
        faction=safe(UnitFactionGroup,"player"),race=race,class=class,
        done={},log={},ready={},professions={},reputation={},trivial=7}
    raceID=n(raceID)
    if raceID and enum.raceMaskById then p.raceMask=n(enum.raceMaskById[raceID],0) end
    if not secret(class) and enum.classKeys then p.classMask=n(enum.classKeys[class],0) end
    local q=C_QuestLog or {}
    p.trivial=n(safe(q.GetTrivialRange),7)
    local done=safe(q.GetAllCompletedQuestIDs)
    if type(done)=="table" then for _,value in ipairs(done) do local id=n(value); if id then p.done[id]=true end end end
    local count=n(safe(q.GetNumQuestLogEntries),0)
    for i=1,count do
        local entry=safe(q.GetInfo,i)
        if type(entry)=="table" and not secret(entry.isHeader) and not entry.isHeader then
            local id=n(entry.questID)
            if id and id>0 then
                p.log[id]=true
                local ready=safe(q.ReadyForTurnIn,id)
                if not secret(ready) and ready then p.ready[id]=true end
            end
        end
    end
    local skills=C_SkillInfo or {}
    for i=1,n(safe(skills.GetNumSkillLines),0) do
        local name,header,_,rank=safe(skills.GetSkillLineInfo,i)
        -- Accept both documented tuple data and structured client returns.
        if type(name)=="table" and not secret(name) then
            local entry=name; name=entry.skillName or entry.name; header=entry.isHeader; rank=entry.skillRank or entry.rank
        end
        if not secret(name) and type(name)=="string" and not secret(header) and not header then p.professions[name]=n(rank,0) end
    end
    local rep=C_Reputation or {}
    for i=1,n(safe(rep.GetNumFactions),0) do
        local entry=safe(rep.GetFactionDataByIndex,i)
        if type(entry)=="table" and not secret(entry.name) and type(entry.name)=="string" then
            p.reputation[entry.name]={id=n(entry.factionID),standing=n(entry.reaction),value=n(entry.currentStanding)}
        end
    end
    return p
end
