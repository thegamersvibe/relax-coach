-- First-party evidence adapter. Corrected getters only; nil is never a read failure.
local RQP=RelaxQuestPlanner
local D={}
RQP.CleanupData=D
RQP.CleanupUses={}

function D.Plain(v)
    if issecretvalue then
        local ok,secret=pcall(issecretvalue,v)
        if not ok or secret then return false end
    end
    return true
end
function D.Number(v)
    return D.Plain(v) and type(v)=="number" and v==v and v~=math.huge and v~=-math.huge
end
function D.ID(v) return D.Number(v) and v>0 and v%1==0 and v<1000000 end

-- Copy foreign results, preserving sparse positions and rejecting cycles/secrets.
local function copy(v,seen,depth,budget)
    if not D.Plain(v) then error("restricted") end
    if type(v)~="table" then
        if v==nil or type(v)=="string" or type(v)=="boolean" or D.Number(v) then return v end
        error("unsupported value")
    end
    if seen[v] or depth>12 or getmetatable(v) then error("unreadable table") end
    seen[v]=true
    local out={}
    for k,x in pairs(v) do
        budget.n=budget.n+1; if budget.n>100000 then error("oversize read") end
        if not D.Plain(k) or not (type(k)=="string" or D.Number(k)) then error("invalid key") end
        out[k]=copy(x,seen,depth+1,budget)
    end
    seen[v]=nil
    return out
end
function D.Call(fn,...)
    if type(fn)~="function" then return nil,"unknown" end
    local results={pcall(fn,...)}
    if not results[1] then return nil,"error" end
    local ok,v=pcall(copy,results[2],{},0,{n=0})
    if not ok then return nil,"unknown" end
    return v,"known",results
end
function D.Provider()
    local ok,p=pcall(RQP.Data.CleanupProvider)
    if not ok or type(p)~="table" then return {available=false,source="QuestieDB",revisionKnown=false} end
    return p
end
function D.SameProvider(a,b)
    return a and b and a.handle==b.handle and a.available==b.available and a.version==b.version and a.flavor==b.flavor
end

local questFields={name="string",questLevel="number",requiredLevel="number",requiredRaces="number",
    requiredClasses="number",startedBy="table",sourceItemId="number",requiredSourceItems="table",
    objectives="table",extraObjectives="table",preQuestSingle="table",preQuestGroup="table",
    exclusiveTo="table",parentQuest="number",childQuests="table",inGroupWith="table",
    nextQuestInChain="number",breadcrumbForQuestId="number",breadcrumbs="table",
    availableStartingWith="number",availableUntilCompleted="number",disabledByQuest="number",
    questFlags="number",specialFlags="number",zoneOrSort="number",type="number",
    requiredSkill="table",requiredRanks="table",requiredMinRep="table",requiredMaxRep="table",
    requiredSpell="number",requiredSpecialization="number",requiredMaxLevel="number"}
local itemFields={name="string",startQuest="number",relatedQuests="table",questRewards="table",
    itemClass="number",flags="number",teachesSpell="number"}
local function read(kind,id,p,checkpoint)
    p=p or D.Provider()
    local r={id=id,fields={},complete=false,source=p.source,version=p.version}
    if not D.ID(id) or not p.available then return r end
    local api=kind=="Quest" and p.quest or p.item
    local exists,state=D.Call(api.Exists,id)
    if checkpoint then checkpoint() end
    if state~="known" or exists~=true then return r end
    r.complete=true
    for key,want in pairs(kind=="Quest" and questFields or itemFields) do
        local fn=api.Get
        local v,s
        if type(fn)=="function" then v,s=D.Call(fn,id,key) else v,s=D.Call(api[key],id) end
        if s=="known" and v~=nil and type(v)~=want then s="unknown"; v=nil end
        r.fields[key]={value=v,state=s,source=p.source,version=p.version}
        if s~="known" then r.complete=false end
        if checkpoint then checkpoint() end
    end
    if type(r.fields.name.value)~="string" or r.fields.name.value=="" then r.complete=false end
    return r
end
function D.ReadQuest(id,p,checkpoint) return read("Quest",id,p,checkpoint) end
function D.ReadItem(id,p,checkpoint) return read("Item",id,p,checkpoint) end
function D.Value(r,key) return r and r.fields[key] and r.fields[key].value end
function D.Ids(api)
    local ids,state=D.Call(api and api.GetAllIds)
    if state~="known" or type(ids)~="table" then return nil,"unknown" end
    local seen,count={},0
    for k,id in pairs(ids) do
        if not D.ID(k) or not D.ID(id) or seen[id] then return nil,"unknown" end
        seen[id]=true; count=count+1
    end
    for i=1,count do if not ids[i] then return nil,"unknown" end end
    return ids,"known"
end
