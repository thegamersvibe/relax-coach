-- Runtime adapters only. No quest, NPC, item or map records are bundled here.
RelaxQuestPlanner = RelaxQuestPlanner or {}
local RQP = RelaxQuestPlanner
RQP.Data = RQP.Data or {}
local D = RQP.Data
local lib, questAPI, fdj, dg, checked, reason, supplements, ids, dungeons, areaMap
local quests, npcs, objects, items, dungeonByArea = {}, {}, {}, {}, {}
local blacklist = {}
local function tick() if RQP.Engine and RQP.Engine.Checkpoint then RQP.Engine.Checkpoint() end end
local function tbl(v) return type(v)=="table" and v or {} end
local function scalar(v) if type(v)=="string" or type(v)=="number" then return v end end
local function present(v) return v~=nil and v~=false and v~=0 end
local function keys(t)
    local out={}; for k in pairs(t or {}) do out[#out+1]=k; tick() end
    local allNum=true
    for i=1,#out do if type(out[i])~="number" then allNum=false break end end
    if allNum then table.sort(out) else
        table.sort(out, function(a,b) if type(a)==type(b) then return a<b end; return tostring(a)<tostring(b) end)
    end
    return out
end
D.Keys = keys
D.Present = present
local function copyList(v, selfID)
    local out={}
    for _,x in ipairs(tbl(v)) do if x and (not selfID or type(x)~="number" or math.abs(x)~=selfID) then out[#out+1]=x end end
    return out
end
local function read(api,id,key)
    if not api then return nil end
    local value
    if api.Get then value=api.Get(id,key)
    elseif type(api[key])=="function" then value=api[key](id) end
    -- Cold provider decodes can be much more expensive than warmed reads.
    -- Check between fields, not just once after all fields of a quest.
    tick()
    return value
end
local function errorText(err)
    if RelaxQuestPlanner and RelaxQuestPlanner.SetError then RelaxQuestPlanner.SetError(err)
    else RelaxQuestPlannerDB=RelaxQuestPlannerDB or {}; RelaxQuestPlannerDB.lastError=tostring(err) end
end
D.Error = errorText

function D.Reset()
    lib,fdj,dg,checked,reason,supplements,ids,dungeons,areaMap=nil,nil,nil,nil,nil,nil,nil,nil,nil
    questAPI=nil
    quests,npcs,objects,items,dungeonByArea,blacklist={},{},{},{},{},{}
    if RQP.Engine and RQP.Engine.Invalidate then RQP.Engine.Invalidate() end
end

function D.Ready()
    if checked then return lib~=nil or fdj~=nil or dg~=nil,reason end
    checked=true
    local candidate=_G.LibQuestieDB
    if candidate and type(candidate.RequireContract)=="function" then
        local ok, supported, why=pcall(candidate.RequireContract,4)
        if ok and supported and (candidate.Quest or _G.QuestDB) then
            lib=candidate; questAPI=candidate.Quest or _G.QuestDB
        else reason="QuestieDB contract 4 unavailable: "..tostring(why or supported) end
    end
    if type(_G.ForeverDungeonJournal_NS)=="table" then fdj=_G.ForeverDungeonJournal_NS end
    -- DungeonGuideForever normally keeps its namespace private. Never reach into
    -- another addon's locals; use it only if that addon explicitly exports a global.
    local global=_G.DungeonGuideForever_NS or _G.DungeonGuideForever
    if type(global)=="table" and type(global.quests)=="table" then dg=global end
    if not lib then reason=reason or "QuestieDB missing: limited data" end
    if _G.QuestieLoader and type(QuestieLoader.ImportModule)=="function" then
        local ok,mod=pcall(QuestieLoader.ImportModule,QuestieLoader,"QuestieCorrections")
        if ok and type(mod)=="table" then blacklist=tbl(mod.hiddenQuests) end
    end
    return lib~=nil or fdj~=nil or dg~=nil,reason
end
function D.Source() D.Ready(); return lib and "QuestieDB" or "FDJ-only" end
function D.Enums() D.Ready(); return lib and lib.Enum or {} end
function D.Blacklisted(id) D.Ready(); return blacklist[id]==true end
function D.FDJ() D.Ready(); return fdj end
function D.DungeonQuestie() D.Ready(); return dg end

local fieldMap={name="name",reqLevel="requiredLevel",qLevel="questLevel",races="requiredRaces",
    classes="requiredClasses",grp="preQuestGroup",single="preQuestSingle",excl="exclusiveTo",
    zone="zoneOrSort",next="nextQuestInChain",special="specialFlags",parent="parentQuest",
    bcFor="breadcrumbForQuestId",bcs="breadcrumbs",untilDone="availableUntilCompleted",
    startingWith="availableStartingWith",disabledBy="disabledByQuest",maxLevel="requiredMaxLevel",
    skill="requiredSkill",ranks="requiredRanks",spec="requiredSpecialization",minRep="requiredMinRep",
    maxRep="requiredMaxRep",spell="requiredSpell",startedBy="startedBy",finishedBy="finishedBy",
    objectives="objectives",children="childQuests",inGroupWith="inGroupWith"}
local relationLists={"grp","single","excl","bcs","children","inGroupWith"}
local tableFields={"startedBy","finishedBy","objectives"}
local gateFields={"skill","ranks","minRep","maxRep"}
local zeroIsAbsent={"next","parent","bcFor","untilDone","startingWith","disabledBy","maxLevel","spec","spell"}
local function normalize(r,id)
    r.id=id; r.name=r.name or ("quest "..tostring(id))
    for _,k in ipairs(relationLists) do r[k]=copyList(r[k],id) end
    for _,k in ipairs(tableFields) do r[k]=tbl(r[k]) end
    for _,k in ipairs(gateFields) do if r[k]==nil then r[k]={} end end
    -- QuestieDB normalizes absent numerics to 0. Lua treats 0 as true.
    for _,k in ipairs(zeroIsAbsent) do
        if not present(r[k]) then r[k]=nil end
    end
    r.reqLevel=r.reqLevel or 0; r.qLevel=r.qLevel or 0
    r.races=r.races or 0; r.classes=r.classes or 0
    return r
end
local function baseQuest(id)
    if not lib or not questAPI then return nil end
    local api=questAPI
    if api.Exists and not api.Exists(id) then return nil end
    local name=read(api,id,"name"); if not name then return nil end
    local r={source="QuestieDB"}
    for k,key in pairs(fieldMap) do r[k]=read(api,id,key) end
    return normalize(r,id)
end
local function loadSupplements()
    if supplements then return end
    D.Ready()
    local out, records, names={},{},{}
    local chains=fdj and tbl(fdj.QUEST_PREREQ_CHAINS) or {}
    local chainTargets={} -- sorted once; was re-collected and re-sorted for every supplement record
    if fdj then
        for _,dn in ipairs(keys(tbl(fdj.DB))) do
            local dungeon=fdj.DB[dn]
            if type(dungeon)=="table" then
                for _,v in ipairs(tbl(dungeon.quests)) do
                    if type(v)=="table" and type(v.id)=="number" and type(v.name)=="string" then
                        records[v.id]={name=v.name,level=scalar(v.level),requires=scalar(v.requires),
                            faction=scalar(v.faction),pickup=scalar(v.pickup),turnin=scalar(v.turnin),
                            dungeon=dn,classOnly=scalar(v.classOnly),source="FDJ"}
                    end
                    tick()
                end
            end
        end
        for q,v in pairs(tbl(fdj.QUEST_PREREQ_DETAILS)) do
            if type(q)=="number" and type(v)=="table" and not records[q] then
                records[q]={level=scalar(v.level),requires=scalar(v.requires),pickup=scalar(v.pickup),turnin=scalar(v.turnin),source="FDJ"}
            end
            tick()
        end
        chainTargets=keys(chains)
        for _,target in ipairs(chainTargets) do
            for _,step in ipairs(tbl(chains[target])) do
                if type(step)=="table" and step.id and type(step.name)=="string" and not names[step.id] then names[step.id]=step.name end
            end
        end
    end
    if dg and not lib then
        for _,v in ipairs(dg.quests) do
            if type(v.id)=="number" and not records[v.id] then
                records[v.id]={name=v.name,level=v.level,requires=v.minLevel,faction=v.faction,
                    source="DungeonQuestie",dungeons=v.dungeons,grp=v.prereqs}
            end
            tick()
        end
    end
    for _,id in ipairs(keys(records)) do
        local v=records[id]
        if id>0 and id<1000000 and not (lib and questAPI.Exists(id)) then
            local masks=D.Enums().factionRaceMasks or {}
            local r=normalize({name=v.name or names[id],reqLevel=v.requires,qLevel=v.level,
                races=masks[v.faction] or 0,classes=0,grp=v.grp,
                faction=v.faction,classOnly=v.classOnly,source=v.source,
                fdj={pickup=v.pickup,turnin=v.turnin,dungeon=v.dungeon},addonDungeons=v.dungeons},id)
            for _,target in ipairs(chainTargets) do
                local previous
                for _,s in ipairs(tbl(chains[target])) do
                    if s.id==id and previous then r.grp={previous}; break end
                    previous=s.id
                end
                if #r.grp>0 then break end
                if target==id and previous then r.grp={previous}; break end
            end
            out[id]=r
        end
        tick()
    end
    supplements=out -- Publish only complete data, including across coroutine yields.
end
function D.Quest(id)
    if type(id)~="number" or id<=0 then return nil end
    D.Ready()
    if quests[id]~=nil then return quests[id] or nil end
    local r=baseQuest(id)
    if not r then loadSupplements(); r=supplements[id] end
    quests[id]=r or false
    return r
end
function D.AllQuestIds()
    if ids then return ids end
    D.Ready(); loadSupplements()
    local all={}
    if lib then for _,id in ipairs(questAPI.GetAllIds()) do all[id]=true; tick() end end
    for id in pairs(supplements) do all[id]=true end
    ids=keys(all); return ids
end
local function entity(kind,id,cache,fields)
    D.Ready(); if cache[id]~=nil then return cache[id] or nil end
    local api=lib and (lib[kind] or _G[kind.."DB"])
    if not api or (api.Exists and not api.Exists(id)) then return nil end
    local name=read(api,id,"name"); if not name then return nil end
    local r={id=id,name=name}
    for _,k in ipairs(fields) do r[k]=read(api,id,k) end
    r.zone=r.zoneID
    if not present(r.zone) then r.zone=nil; for z in pairs(tbl(r.spawns)) do r.zone=z; break end end
    cache[id]=r; return r
end
function D.Npc(id) return entity("Npc",id,npcs,{"spawns","zoneID","questStarts","questEnds"}) end
function D.Object(id) return entity("Object",id,objects,{"spawns","zoneID","questStarts","questEnds"}) end
function D.Item(id) return entity("Item",id,items,{"npcDrops","objectDrops","itemDrops","vendors","questRewards","startQuest"}) end

local battleground={['Warsong Gulch']=true,['Arathi Basin']=true,['Alterac Valley']=true}
-- Forever 1-60 five-player dungeons only (no raids, battlegrounds or later-expansion instances).
-- name = shown in the picker; aliases = names used by QuestieDB / FDJ / Dungeon Questie; faction = entry restriction.
local FOREVER_DUNGEONS={
    {name="Ragefire Chasm",minLevel=13,maxLevel=18,faction="Horde"},
    {name="Wailing Caverns",minLevel=15,maxLevel=24},
    {name="The Deadmines",minLevel=17,maxLevel=26,aliases={"Deadmines"}},
    {name="Shadowfang Keep",minLevel=22,maxLevel=30},
    {name="The Hall of Thanes",minLevel=13,maxLevel=20,aliases={"Hall of Thanes"}},
    {name="Blackfathom Deeps",minLevel=24,maxLevel=32},
    {name="The Stockade",minLevel=24,maxLevel=32,faction="Alliance",aliases={"Stormwind Stockade"}},
    {name="Ruins of Lordaeron",minLevel=15,maxLevel=22},
    {name="Gnomeregan",minLevel=29,maxLevel=38},
    {name="Razorfen Kraul",minLevel=29,maxLevel=38},
    {name="Excavation Site: Wetlands",minLevel=26,maxLevel=33},
    {name="City of Dalaran",minLevel=28,maxLevel=33},
    {name="Scarlet Monastery",minLevel=30,maxLevel=45,aliases={"Scarlet Monastery: Graveyard","Scarlet Monastery: Library","Scarlet Monastery: Armory","Scarlet Monastery: Cathedral"}},
    {name="The Drowned City",minLevel=35,maxLevel=40,aliases={"Drowned City"}},
    {name="Razorfen Downs",minLevel=37,maxLevel=46},
    {name="Uldaman",minLevel=38,maxLevel=48},
    {name="Krol'dok Stronghold",minLevel=40,maxLevel=45,aliases={"Krol'Dok","Krol'dok"}},
    {name="Zul'Farrak",minLevel=43,maxLevel=50},
    {name="Maraudon",minLevel=45,maxLevel=52},
    {name="Sunken Temple",minLevel=49,maxLevel=56,aliases={"The Temple of Atal'Hakkar","Temple of Atal'Hakkar"}},
    {name="Alcaz Prison",minLevel=48,maxLevel=53},
    {name="Blackrock Depths",minLevel=52,maxLevel=60},
    {name="Blackrock Spire",minLevel=55,maxLevel=60,aliases={"Lower Blackrock Spire","Upper Blackrock Spire"}},
    {name="Blackmaw Hold",minLevel=55,maxLevel=60},
    {name="Dire Maul",minLevel=55,maxLevel=60,aliases={"Dire Maul: East","Dire Maul: West","Dire Maul: North"}},
    {name="Scholomance",minLevel=58,maxLevel=60},
    {name="Stratholme",minLevel=58,maxLevel=60,aliases={"Stratholme: Living","Stratholme: Undead"}},
    {name="The Shaper's Terrace",minLevel=58,maxLevel=60,aliases={"Shaper's Terrace"}},
}
local canonicalOf={}
for _,d in ipairs(FOREVER_DUNGEONS) do
    canonicalOf[d.name]=d
    for _,a in ipairs(d.aliases or {}) do canonicalOf[a]=d end
end
D.FOREVER_DUNGEONS=FOREVER_DUNGEONS
function D.CanonicalDungeon(name)
    local dungeon=canonicalOf[name]
    return dungeon and dungeon.name or name
end
local function zones()
    if dungeons then return end
    D.Ready(); local byName,byArea={},{}
    local support
    if lib and lib.Support and lib.Support.Get then support=lib.Support.Get("ZoneDB") end
    local private=support and support.private or {}
    for area,v in pairs(tbl(private.dungeons)) do
        local name=v.name or v[1]
        if name and not battleground[name] then
            local d=byName[name] or {name=name,areaIds={}}
            byName[name]=d; d.areaIds[#d.areaIds+1]=area; byArea[area]=name
            for _,alt in ipairs(tbl(v.altIds or v[2])) do d.areaIds[#d.areaIds+1]=alt; byArea[alt]=name end
        end
        tick()
    end
    if fdj then for name in pairs(tbl(fdj.DB)) do
        if not battleground[name] then byName[name]=byName[name] or {name=name,areaIds={}} end
    end end
    if dg then for _,q in ipairs(dg.quests) do for _,name in ipairs(tbl(q.dungeons)) do
        if not battleground[name] then byName[name]=byName[name] or {name=name,areaIds={}} end
    end end end
    local map=private.areaIdToUiMapId
    if type(map)=="string" and loadstring then
        local fn,err=loadstring(map,"RQP area map")
        if fn then local ok,res=pcall(fn); if ok then map=res else errorText(res) end
        else errorText(err) end
    end
    areaMap=tbl(map)
    -- Collapse every source name onto the Forever allowlist; drop anything not in Forever.
    local canonArea={}
    for area,name in pairs(byArea) do if canonicalOf[name] then canonArea[area]=D.CanonicalDungeon(name) end end
    dungeonByArea=canonArea
    local myFaction=UnitFactionGroup and UnitFactionGroup("player")
    local list={}
    for _,d in ipairs(FOREVER_DUNGEONS) do
        if not d.faction or not myFaction or d.faction==myFaction then
            local entry={name=d.name,areaIds={},faction=d.faction,minLevel=d.minLevel,maxLevel=d.maxLevel}
            for srcName,src in pairs(byName) do
                if canonicalOf[srcName]==d then for _,a in ipairs(src.areaIds) do entry.areaIds[#entry.areaIds+1]=a end end
            end
            list[#list+1]=entry
        end
    end
    dungeons=list
end
function D.Dungeons() zones(); return dungeons end
function D.DungeonForArea(id) zones(); return dungeonByArea[id] end
function D.ZoneName(id)
    zones(); if dungeonByArea[id] then return dungeonByArea[id] end
    if id and C_Map and C_Map.GetAreaInfo then local ok,name=pcall(C_Map.GetAreaInfo,id); if ok and type(name)=="string" and name~="" then return name end end
    if id and GetAreaInfo then local ok,name=pcall(GetAreaInfo,id); if ok and type(name)=="string" and name~="" then return name end end
    local ui=id and areaMap and areaMap[id]
    if ui and C_Map and C_Map.GetMapInfo then local ok,info=pcall(C_Map.GetMapInfo,ui); if ok and type(info)=="table" and info.name then return info.name end end
    return id and tostring(id) or "Unknown"
end
function D.Pin(kind,id)
    zones()
    if kind=="quest" or kind=="fdj" then
        local v=fdj and tbl(fdj.QUEST_START_MAPS)[id]
        if type(v)~="table" then v=fdj and tbl(fdj.QUEST_PREREQ_DETAILS)[id]; v=type(v)=="table" and v.map end
        if type(v)=="table" and type(v.mapID)=="number" and type(v.x)=="number" and type(v.y)=="number" then
            local info=C_Map and C_Map.GetMapInfo and C_Map.GetMapInfo(v.mapID)
            return {map=v.mapID,x=v.x*100,y=v.y*100,who=v.targetName or v.label,zone=info and info.name}
        end
        return nil
    end
    local r=(kind=="npc" and D.Npc(id)) or (kind=="object" and D.Object(id))
    if not r then return nil end
    for _,z in ipairs(keys(tbl(r.spawns))) do
        local points=r.spawns[z]; local pt=type(points)=="table" and points[1]
        if type(pt)=="table" and type(pt[1])=="number" and type(pt[2])=="number" and pt[1]>=0 and pt[1]<=100 and pt[2]>=0 and pt[2]<=100 and present(areaMap[z]) then
            return {map=areaMap[z],x=pt[1],y=pt[2],who=r.name,zone=D.ZoneName(z)}
        end
    end
    return nil
end

-- Optional addons may finish loading after an early readiness probe.
local events=CreateFrame("Frame")
events:RegisterEvent("ADDON_LOADED")
events:SetScript("OnEvent",function(_,event,name)
    local ok,err=pcall(function()
        if event=="ADDON_LOADED" and (name=="QuestieDB" or name=="Questie" or name=="ForeverDungeonJournal" or name=="DungeonGuideForever") then D.Reset() end
    end)
    if not ok then errorText(err) end
end)
