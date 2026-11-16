-- Native read-only snapshot; Forever source bebcb6bf, build 1.60.1.70938.
-- Scoped to backpack + four equipped bags. A nil slot alone never proves empty.
local RQP=RelaxQuestPlanner
local D=RQP.CleanupData
local C={generation=0}
RQP.Cleanup=C
local function integer(v,min,max) return D.Number(v) and v%1==0 and v>=min and v<=max end
local function boolean(v) return type(v)=="boolean" end
local function stringValue(v) return type(v)=="string" and v~="" end

function C.Capture()
    local s={generation=C.generation,provider=D.Provider(),reads={},failures={},complete=true,slots={}}
    local function fail(label,state)
        s.complete=false; s.reads[label]=state or "unknown"; s.failures[label]=true
    end
    local function read(label,fn,valid,...)
        local v,state,results=D.Call(fn,...)
        if state=="known" and (v==nil or (valid and not valid(v))) then state="unknown"; v=nil end
        s.reads[label]=state
        if state~="known" then fail(label,state) end
        return v,state,results
    end
    s.identity=read("identity",UnitGUID,stringValue,"player")
    s.level=read("level",UnitLevel,function(v) return integer(v,1,200) end,"player")
    s.faction=read("faction",UnitFactionGroup,stringValue,"player")
    local _,cs,cr=read("class",UnitClass,stringValue,"player")
    if cs=="known" and cr and D.Plain(cr[4]) and integer(cr[4],1,20) then s.classID=cr[4]
    else fail("class") end
    local _,bs,br=read("build",GetBuildInfo,stringValue)
    if bs=="known" and br and D.Plain(br[3]) and stringValue(br[3]) then s.build=br[3] else fail("build") end
    s.capturedAt=read("time",time,D.Number)
    local q=C_QuestLog or {}
    local done=read("history",q.GetAllCompletedQuestIDs,function(v) return type(v)=="table" end)
    if done then
        s.done={}
        local count=0
        for k,id in pairs(done) do
            if not D.ID(k) or not D.ID(id) then fail("history") else s.done[id]=true end
            count=count+1
        end
        for i=1,count do if not done[i] then fail("history") end end
        if s.reads.history~="known" then s.done=nil end
    end
    local n=read("log",q.GetNumQuestLogEntries,function(v) return integer(v,0,1000) end)
    if n then
        s.log={}
        for i=1,n do
            local entry=read("log entry "..i,q.GetInfo,function(v) return type(v)=="table" and boolean(v.isHeader) end,i)
            if entry and entry.isHeader==false then
                if D.ID(entry.questID) then
                    local id=entry.questID
                    local row={id=id,name=type(entry.title)=="string" and entry.title or nil}
                    row.trivial,row.trivialState=read("trivial "..id,q.IsQuestTrivial,boolean,id)
                    row.ready,row.readyState=read("readiness "..id,q.ReadyForTurnIn,boolean,id)
                    row.repeatable,row.repeatableState=read("repeatable "..id,q.IsRepeatableQuest,boolean,id)
                    if s.log[id] then fail("log") end
                    s.log[id]=row
                else fail("log entry "..i) end
            end
            if not entry or s.failures["log entry "..i] then fail("log") end
        end
    end
    local bags=C_Container or {}
    local bagOK=true
    local function bad(label) bagOK=false; fail(label) end
    for bag=0,4 do
        local prefix="bag "..bag
        local slots=read(prefix.." slots",bags.GetContainerNumSlots,function(v) return integer(v,0,100) end,bag)
        local free=read(prefix.." free",bags.GetContainerNumFreeSlots,function(v) return integer(v,0,100) end,bag)
        if not slots or not free then bagOK=false end
        local empty=0
        for slot=1,slots or 0 do
            local label=prefix.." slot "..slot
            -- Nil is permitted here only for the cross-checked empty-slot path.
            local info,state=D.Call(bags.GetContainerItemInfo,bag,slot)
            local id,idState=D.Call(bags.GetContainerItemID,bag,slot)
            if state~="known" or idState~="known" then bad(label.." info") end
            if info==nil and id==nil and state=="known" and idState=="known" then empty=empty+1
            else
                local r={bag=bag,slot=slot,valid=false}
                if type(info)=="table" then
                    r.itemID=D.ID(info.itemID) and info.itemID or (D.ID(id) and id or nil)
                    r.count=integer(info.stackCount,1,1000000) and info.stackCount or nil
                    r.locked=boolean(info.isLocked) and info.isLocked or nil
                    -- Keep false distinct from nil.
                    if info.isLocked==false then r.locked=false end
                    r.valid=state=="known" and idState=="known" and D.ID(id) and id==r.itemID
                        and r.count~=nil and r.locked==false
                elseif D.ID(id) then r.itemID=id end
                local qi,qs=read(label.." quest info",bags.GetContainerItemQuestInfo,function(v)
                    return type(v)=="table" and boolean(v.isQuestItem) and boolean(v.isActive)
                        and (v.questID==nil or D.ID(v.questID))
                end,bag,slot)
                if qi then r.questFlag=qi.isQuestItem; r.questID=qi.questID; r.questActive=qi.isActive end
                if r.itemID then
                    local _,st,all=read(label.." item class",C_Item and C_Item.GetItemInfoInstant,D.ID,r.itemID)
                    if st=="known" and all and D.Plain(all[7]) and integer(all[7],0,30) then r.classID=all[7]
                    else bad(label.." item class") end
                end
                local again,as=D.Call(bags.GetContainerItemInfo,bag,slot)
                if as~="known" or type(again)~="table" or again.itemID~=r.itemID
                    or again.stackCount~=r.count or again.isLocked~=r.locked then r.valid=false end
                if qs~="known" or not r.classID then r.valid=false end
                if not r.valid then bad(label.." info") end
                s.slots[#s.slots+1]=r
            end
        end
        if slots and free and (free>slots or empty~=free) then bad(prefix.." enumeration") end
    end
    s.reads.bags=bagOK and "known" or "unknown"
    if not bagOK then fail("bags") end
    local db=type(RelaxQuestPlannerDB)=="table" and RelaxQuestPlannerDB or {}
    s.pins={quest={},item={}}
    if type(db.cleanup)=="table" and db.cleanup.schema==1 and type(db.cleanup.pins)=="table" then
        for _,kind in ipairs({"quest","item"}) do
            for id,v in pairs(type(db.cleanup.pins[kind])=="table" and db.cleanup.pins[kind] or {}) do
                if D.ID(id) and v==true then s.pins[kind][id]=true end
            end
        end
    end
    s.goals={}
    if type(db.selection)=="table" and D.ID(db.selection.q) then s.goals[db.selection.q]=true end
    local current=RQP.UI and RQP.UI.current
    if type(current)=="table" and D.ID(current.q) then s.goals[current.q]=true end
    if not s.provider.available or not s.provider.version then fail("provider") end
    return s
end
