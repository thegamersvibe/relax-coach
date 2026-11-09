-- Character-scoped observations, not a replacement quest database. A temporal
-- correlation is shown as observed evidence; it is never made a hard prerequisite.
RelaxQuestPlanner = RelaxQuestPlanner or {}
local RQP=RelaxQuestPlanner
RQP.Learn=RQP.Learn or {}
local L=RQP.Learn
local npc,turned,lastTime=nil,{},0
local function number(v) return RQP.Player.Number(v) end
local function store(create)
    local guid=UnitGUID and UnitGUID("player")
    if (issecretvalue and issecretvalue(guid)) or type(guid)~="string" then return nil end
    if create then
        RelaxQuestPlannerDB=RelaxQuestPlannerDB or {}
        RelaxQuestPlannerDB.learn=RelaxQuestPlannerDB.learn or {}
        RelaxQuestPlannerDB.learn[guid]=RelaxQuestPlannerDB.learn[guid] or {}
    end
    return RelaxQuestPlannerDB and RelaxQuestPlannerDB.learn and RelaxQuestPlannerDB.learn[guid]
end
function L.Observed(q)
    local s=store(false); local out={}
    for id in pairs(s and s[q] and s[q].observed or {}) do out[id]=true end
    return out
end
function L.Refuted(q,c)
    local s=store(false); return s and s[q] and s[q].refuted and s[q].refuted[c]==true or false
end
function L.ResetSession() npc,turned,lastTime=nil,{},0 end
function L.Record(kind,q,npcID)
    q=number(q); npcID=number(npcID)
    if not q or q<=0 or (kind~="accept" and kind~="turnin") then return end
    local now=number(GetTime and GetTime()) or 0
    if npcID~=npc or now-lastTime>120 then turned={}; npc=npcID end
    lastTime=now
    if kind=="turnin" then if npcID then turned[q]=true end; return end
    local records=store(true); if not records then return end
    local row=records[q] or {observed={},refuted={}}
    row.observed=row.observed or {}; row.refuted=row.refuted or {}
    row.accepted=true; row.npc=npcID
    if npcID then for id in pairs(turned) do if id~=q then row.observed[id]=true end end end
    local p=RQP.Player.Snapshot()
    local r=RQP.Data.Quest(q)
    local function done(id) return p.done[id] or turned[id] end
    local function refute(id) if type(id)=="number" and id~=q and not done(id) then row.refuted[id]=true end end
    if r then
        if #r.single>0 then
            local any=false; for _,id in ipairs(r.single) do if done(id) then any=true end end
            if not any then for _,id in ipairs(r.single) do refute(id) end end
        else
            for _,id in ipairs(r.grp) do
                local abs=math.abs(id); local satisfied=done(abs)
                if id>0 and not satisfied then
                    local previous=RQP.Data.Quest(id)
                    for _,alt in ipairs(previous and previous.excl or {}) do if done(alt) then satisfied=true end end
                end
                if not satisfied then refute(abs) end
            end
        end
        if r.parent and not p.log[r.parent] then refute(r.parent) end
        if r.startingWith and not p.log[r.startingWith] then refute(r.startingWith) end
    end
    local f=RQP.Data.FDJ()
    if f and type(f.QUEST_PREREQ_CHAINS)=="table" then
        for _,step in ipairs(f.QUEST_PREREQ_CHAINS[q] or {}) do refute(step.id) end
    end
    local d=RQP.Data.DungeonQuestie()
    if d then for _,quest in ipairs(d.quests) do
        if quest.id==q then for _,id in ipairs(quest.prereqs or {}) do refute(id) end end
    end end
    records[q]=row
    -- Each acceptance consumes only the immediately preceding turn-ins.
    turned={}
    if RQP.Engine then RQP.Engine.Invalidate() end
end
local function npcID()
    local guid=UnitGUID and UnitGUID("npc")
    if (issecretvalue and issecretvalue(guid)) or type(guid)~="string" then return nil end
    local parts={}; for part in guid:gmatch("[^-]+") do parts[#parts+1]=part end
    if parts[1]=="Creature" or parts[1]=="Vehicle" then return number(parts[6]) end
end
local function onEvent(_,event,a,b)
    if event=="GOSSIP_CLOSED" or event=="PLAYER_ENTERING_WORLD" then L.ResetSession()
    elseif event=="QUEST_ACCEPTED" then L.Record("accept",number(b) or number(a),npcID())
    elseif event=="QUEST_TURNED_IN" then L.Record("turnin",a,npcID()) end
end
local frame=CreateFrame("Frame")
-- accept/turn-in are forwarded by Init.lua (avoid double Record)
for _,event in ipairs({"GOSSIP_CLOSED","PLAYER_ENTERING_WORLD"}) do frame:RegisterEvent(event) end
frame:SetScript("OnEvent",function(...)
    local ok,err=pcall(onEvent,...); if not ok then RQP.Data.Error(err) end
end)
