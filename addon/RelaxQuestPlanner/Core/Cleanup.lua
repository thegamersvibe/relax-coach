-- Read-only quest/item review. Two labels; no removal verdict or profile mechanism.
local RQP=RelaxQuestPlanner
local C,D=RQP.Cleanup,RQP.CleanupData
local value=D.Value
C.Reasons={
    ['state-unresolved']='Quest state could not be resolved',
    ['new-forever']='New in Forever, no use data',
    ['no-use-found']='No use found in our data',
    pinned='Kept pinned', ['active-use']='Used by a quest in the current log',
    ['future-use']='Used by a quest not recorded turned in', starter='Starts a quest',
    ['starter-missing']='Starts a quest (not in quest data)',
    tool='A spell or recipe needs this in your bags', spellReagent='A class spell uses this up',
    openable='Can be opened', crate='Waylaid crate', filling='Fills a waylaid crate',
    petFood='Pet food', food='Food', bonus='Gear with a situational bonus',
    known='Has a known use', secret='Part of a secret', ['other-class-use']='Has a known use for another class',
    reagent='Used in recipes', teaches='Teaches a recipe', reward='Quest reward; other utility is not ruled out',
    ['spell-use']='Teaches a spell', ['item-utility']='Equipment or key utility',
    ['ready-turnin']='Ready to turn in', repeatable='Repeatable or event-gated use',
    ['protected-chain']='Possible path to a dungeon or higher-level quest',
    ['active-parent']='Parent or shared dependency of an active quest',
    goal='Selected goal or current route step', dungeon='Known dungeon-related quest',
}
function C.DispositionForReason(code)
    if code=='no-use-found' or code=='new-forever' or code=='state-unresolved' or not C.Reasons[code] then
        return 'UNCERTAIN-KEEP'
    end
    return 'KEEP'
end
local function sortedKeys(t)
    local out={}; for k in pairs(t or {}) do out[#out+1]=k end; table.sort(out); return out
end
local function list(t,fn,bad)
    if t==nil then return end
    if type(t)~='table' then bad(); return end
    local count=0
    for k,v in pairs(t) do
        if not D.ID(k) then bad() else count=count+1; fn(v) end
    end
    for i=1,count do if t[i]==nil then bad() end end
end
local function clock()
    local v,s=D.Call(debugprofilestop)
    if s=='known' and D.Number(v) then return v end
end
function C.Combat()
    local v,s=D.Call(InCombatLockdown)
    if s~='known' or type(v)~='boolean' then return nil end
    return v
end

local function buildGraph(index,p,checkpoint)
    index.edges,index.forward,index.unknown,index.dungeon={},{},{},{}
    local function unknown(id) if D.ID(id) then index.unknown[id]=true end end
    local function edge(from,to,kind,source)
        if not D.ID(from) or not D.ID(to) then unknown(from); unknown(to); return end
        if from==to then unknown(from) end
        local e={from=from,to=to,kind=kind,source=source or 'QuestieDB',version=p.version}
        index.edges[#index.edges+1]=e
        local out=index.forward[from] or {}; index.forward[from]=out; out[#out+1]=e
    end
    for id,r in pairs(index.quests) do
        local function invalid() unknown(id) end
        list(value(r,'preQuestSingle'),function(q) edge(q,id,'or-completed') end,invalid)
        list(value(r,'preQuestGroup'),function(q)
            if D.Number(q) and q<0 then edge(-q,id,'and-completed-no-substitution')
            else
                edge(q,id,'and-completed')
                list(value(index.quests[q],'exclusiveTo'),function(alt) edge(alt,id,'exclusive-substitute') end,invalid)
            end
        end,invalid)
        for field,kind in pairs({parentQuest='active-parent',availableStartingWith='active-or-completed',
            availableUntilCompleted='until-completed',disabledByQuest='temporary-block'}) do
            local q=value(r,field); if q and q~=0 then edge(q,id,kind) end
        end
        for field,kind in pairs({nextQuestInChain='successor',breadcrumbForQuestId='breadcrumb'}) do
            local q=value(r,field); if q and q~=0 then edge(id,q,kind) end
        end
        for field,kind in pairs({childQuests='child',inGroupWith='shared',exclusiveTo='exclusive-alternative'}) do
            list(value(r,field),function(q) edge(id,q,kind) end,invalid)
        end
        list(value(r,'breadcrumbs'),function(q) edge(q,id,'breadcrumb') end,invalid)
        checkpoint(true)
    end
    -- Optional supplements add protections only, without navigation's ID filtering
    -- or guessed chain normalization. Never reinterpret a negative FDJ placeholder.
    local fdj=_G.ForeverDungeonJournal_NS
    if type(fdj)=='table' then
        local db,state=D.Call(function() return fdj.DB end)
        if state~='known' then index.complete=false; index.failures['dungeon supplement']=true end
        for _,dungeon in pairs(type(db)=='table' and db or {}) do
            list(type(dungeon)=='table' and dungeon.quests or nil,function(q)
                if type(q)=='table' and D.ID(q.id) then index.dungeon[q.id]=true end
            end,function() index.complete=false end)
            checkpoint(true)
        end
        local chains,cs=D.Call(function() return fdj.QUEST_PREREQ_CHAINS end)
        if cs~='known' then index.complete=false; index.failures['chain supplement']=true end
        for target,steps in pairs(type(chains)=='table' and chains or {}) do
            list(steps,function(step)
                if type(step)=='table' and D.ID(step.id) then edge(step.id,target,'supplemental-chain','FDJ')
                else unknown(target) end
            end,function() unknown(target) end)
            checkpoint(true)
        end
    end
    local dg=_G.DungeonGuideForever_NS or _G.DungeonGuideForever
    if type(dg)=='table' then
        local rows,state=D.Call(function() return dg.quests end)
        if state~='known' then index.complete=false; index.failures['dungeon guide supplement']=true end
        list(rows,function(q)
            if type(q)~='table' or not D.ID(q.id) then return end
            if type(q.dungeons)=='table' and next(q.dungeons) then index.dungeon[q.id]=true end
            list(q.prereqs,function(id) edge(id,q.id,'supplemental-chain','DungeonGuideForever') end,function() unknown(q.id) end)
            checkpoint(true)
        end,function() index.complete=false end)
    end
    local areas,state=D.Call(function()
        if not (p.handle and p.handle.Support and p.handle.Support.Get) then return nil end
        local support=p.handle.Support.Get('ZoneDB')
        return support and support.private and support.private.dungeons
    end)
    if state~='known' then index.complete=false; index.failures['dungeon areas']=true end
    if type(areas)=='table' then
        local zones={}
        for id,row in pairs(areas) do
            zones[id]=true
            if type(row)=='table' then list(row.altIds or row[2],function(alt) zones[alt]=true end,function() end) end
        end
        for id,r in pairs(index.quests) do
            if zones[value(r,'zoneOrSort')] then index.dungeon[id]=true end
            checkpoint(true)
        end
    end
end

function C.BuildIndexAsync(provider,callback)
    if C.indexJob then C.indexJob:Cancel() end
    local p=type(provider)=='table' and provider or D.Provider()
    local index={provider=p,quests={},items={},uses={},complete=p.available and p.version~=nil,failures={}}
    local job={index=index,records=0,frames=0,cancelled=false}
    local frame=CreateFrame('Frame')
    function job:Cancel() self.cancelled=true; frame:SetScript('OnUpdate',nil) end
    C.indexJob=job
    local function bad(label) index.complete=false; index.failures[label]=true end
    local start,count
    local function checkpoint(record)
        if record then count=count+1; job.records=job.records+1 end
        local now=clock()
        if count>=100 or (now and start and now-start>=2) then coroutine.yield() end
    end
    local function use(item,q,role,source)
        if not D.ID(item) or not D.ID(q) then bad('item relation'); return end
        local rows=index.uses[item] or {}; index.uses[item]=rows
        rows[#rows+1]={questID=q,role=role,source=source or p.source,version=p.version}
    end
    local function questUses(id,r)
        local function invalid() bad('quest '..id) end
        local started=value(r,'startedBy')
        if started then list(started[3],function(i) use(i,id,'starter') end,invalid) end
        local source=value(r,'sourceItemId'); if source and source~=0 then use(source,id,'source') end
        list(value(r,'requiredSourceItems'),function(i) use(i,id,'required-source') end,invalid)
        local obj=value(r,'objectives')
        if obj then
            list(obj[3],function(t) if type(t)=='table' then use(t[1],id,'objective') else invalid() end end,invalid)
            list(obj[6],function(t)
                if type(t)~='table' then invalid() elseif t[3] and t[3]~=0 then use(t[3],id,'spell-objective') end
            end,invalid)
        end
        list(value(r,'extraObjectives'),function(t)
            if type(t)~='table' or type(t[5])~='table' then invalid(); return end
            list(t[5],function(ref)
                if type(ref)~='table' then invalid()
                elseif ref[1]=='item' then use(ref[2],id,'extra-objective') end
            end,invalid)
        end,invalid)
    end
    local worker=coroutine.create(function()
        if not p.available then bad('provider'); return end
        local ids=D.Ids(p.quest)
        if not ids then bad('quest enumeration') else
            for _,id in ipairs(ids) do
                local r=D.ReadQuest(id,p,checkpoint); index.quests[id]=r
                if not r.complete then bad('quest '..id) end
                questUses(id,r); checkpoint(true)
            end
        end
        ids=D.Ids(p.item)
        if not ids then bad('item enumeration') else
            for _,id in ipairs(ids) do
                local r=D.ReadItem(id,p,checkpoint); index.items[id]=r
                if not r.complete then bad('item '..id) end
                local q=value(r,'startQuest'); if q and q~=0 then use(id,q,'starter') end
                list(value(r,'relatedQuests'),function(qid) use(id,qid,'reverse') end,function() bad('item '..id) end)
                checkpoint(true)
            end
        end
        buildGraph(index,p,checkpoint)
    end)
    frame:SetScript('OnUpdate',function()
        if job.cancelled then return end
        local combat=C.Combat()
        if combat==true then return end
        start,count=clock(),0; job.frames=job.frames+1
        if combat==nil then
            bad('combat read'); frame:SetScript('OnUpdate',nil); callback(index); return
        end
        local ok=coroutine.resume(worker)
        if not ok then bad('scan failed') end
        if not ok or coroutine.status(worker)=='dead' then
            frame:SetScript('OnUpdate',nil)
            if not D.SameProvider(p,D.Provider()) then bad('provider changed') end
            if not job.cancelled then callback(index) end
        end
    end)
    return job
end

local function newRow(kind,id,name)
    return {kind=kind,exactID=id,name=name or (kind..' '..id),disposition='UNCERTAIN-KEEP',
        reasons={},reasonCodes={},evidence={},relatedIDs={},slots={}}
end
local function keep(row,code,detail)
    row.disposition='KEEP'
    for _,c in ipairs(row.reasonCodes) do if c==code then return end end
    row.reasonCodes[#row.reasonCodes+1]=code; row.reasons[#row.reasons+1]=detail or C.Reasons[code]
end
local function uncertain(row,code)
    if row.disposition=='KEEP' then return end
    row.reasonCodes={code}; row.reasons={C.Reasons[code]}
end
local function vanilla(id,uses)
    local ranges=uses.vanilla
    if type(ranges)~='table' or #ranges~=1764 then return nil end
    for i=1,#ranges,2 do if id>=ranges[i] and id<=ranges[i+1] then return true end end
    return false
end
local blocks={'reagent','teaches','tool','spellReagent','openable','crate','petFood','food','bonus','known','starts'}
local function facts(row,s,index,uses)
    local any,complete=false,true
    for _,block in ipairs(blocks) do
        local data=uses[block]
        if type(data)~='table' then complete=false else
            local fact=data[row.exactID]
            if fact~=nil then
                any=true
                local code=block
                if block=='starts' then
                    row.relatedIDs[fact]=true
                    code=index.quests[fact] and 'starter' or 'starter-missing'
                elseif block=='petFood' and s.classID~=3 then code='other-class-use'
                elseif block=='crate' then code=fact==1 and 'crate' or 'filling'
                elseif block=='known' then
                    code=fact[2] and 'secret' or 'known'
                    if fact[1]~=0 and fact[1]~=s.classID then code='other-class-use' end
                end
                keep(row,code)
                if block=='reagent' or block=='teaches' then
                    row.professions=type(fact)=='table' and fact or {fact}
                end
                row.evidence[#row.evidence+1]={role=block,source='Keep or Toss',version='1.7.0'}
            end
        end
    end
    return any,complete
end
local function repeatable(q)
    local special,flags=value(q,'specialFlags'),value(q,'questFlags')
    return (D.Number(special) and special>0) or (D.Number(flags) and
        (flags%8192>=4096 or flags%65536>=32768 or flags%131072>=65536))
end
-- Iterative DFS: cycles/self-relations remain uncertain, without recursive overflow.
local function protection(row,root,s,index)
    local seen,onpath,stack={},{},{{id=root,pos=0}}
    local unknown=false
    while #stack>0 do
        local frame=stack[#stack]; local id=frame.id; local q=index.quests[id]
        if frame.pos==0 then
            seen[id]=true; onpath[id]=true
            if not q or not q.complete or (index.unknown and index.unknown[id]) then unknown=true end
            if repeatable(q) then keep(row,'repeatable'); row.relatedIDs[id]=true end
            local level=value(q,'questLevel')
            if not D.Number(level) or level<=0 or not s.level then unknown=true end
            if (index.dungeon and index.dungeon[id]) or (id~=root and D.Number(level) and s.level
                and level>s.level and not (s.done and s.done[id])) then
                keep(row,id==root and 'dungeon' or 'protected-chain'); row.relatedIDs[id]=true
            end
        end
        frame.pos=frame.pos+1
        local e=(index.forward and index.forward[id] or {})[frame.pos]
        if not e then onpath[id]=nil; stack[#stack]=nil else
            if s.log and s.log[e.to] and (e.kind=='active-parent' or e.kind=='shared' or e.kind=='child') then
                keep(row,'active-parent'); row.relatedIDs[e.to]=true
            end
            if onpath[e.to] then unknown=true
            elseif not seen[e.to] then
                row.evidence[#row.evidence+1]=e
                stack[#stack+1]={id=e.to,pos=0}
            end
        end
    end
    return not unknown
end
function C.Evaluate(s,index)
    index=index or {quests={},items={},uses={},failures={},complete=false}
    local uses=RQP.CleanupUses or {}
    local report={generation=s.generation,providerRevision=index.provider and index.provider.revision,
        provider=s.provider,complete=s.complete and index.complete,questRows={},itemRows={},
        capturedAt=s.capturedAt,identity=s.identity,counts={KEEP=0,['UNCERTAIN-KEEP']=0}}
    local function finish(row)
        row.relatedIDs=sortedKeys(row.relatedIDs)
        row.source='QuestieDB '..(s.provider.version or 'unknown')..'; Keep or Toss 1.7.0'
        row.capturedAt=s.capturedAt
        row.detail='Checked: '..row.source..'. Not checked: destroy side effects, whether the item can be had again, uses added after build 70888. Known uses only, not proof of no use.'
        report.counts[row.disposition]=report.counts[row.disposition]+1
    end
    for _,id in ipairs(sortedKeys(s.log)) do
        local logged=s.log[id]
        if logged.trivial~=false then
            local row=newRow('quest',id,logged.name or value(index.quests[id],'name'))
            if logged.ready==true then keep(row,'ready-turnin') end
            if s.pins.quest[id] then keep(row,'pinned') end
            if s.goals[id] then keep(row,'goal') end
            if logged.repeatable then keep(row,'repeatable') end
            protection(row,id,s,index)
            uncertain(row,'state-unresolved')
            finish(row); report.questRows[#report.questRows+1]=row
        end
    end
    local grouped={}
    for _,slot in ipairs(s.slots) do
        if slot.itemID then
            local g=grouped[slot.itemID] or {slots={},flag=false,valid=true}
            grouped[slot.itemID]=g; g.slots[#g.slots+1]=slot
            g.flag=g.flag or slot.questFlag or slot.classID==12 or slot.questID~=nil
            g.valid=g.valid and slot.valid
        end
    end
    for _,id in ipairs(sortedKeys(grouped)) do
        local g=grouped[id]; local item=index.items[id]; local edges=index.uses[id] or {}
        local start=uses.starts and uses.starts[id]
        if g.flag or #edges>0 or start then
            local row=newRow('item',id,value(item,'name')); row.slots=g.slots
            if s.pins.item[id] then keep(row,'pinned') end
            local any,importComplete=facts(row,s,index,uses)
            local resolved=s.complete and index.complete and g.valid and item and item.complete and importComplete
            for _,e in ipairs(edges) do
                row.evidence[#row.evidence+1]=e; row.relatedIDs[e.questID]=true
                local q=index.quests[e.questID]
                if not protection(row,e.questID,s,index) then resolved=false end
                if e.role=='starter' then keep(row,q and 'starter' or 'starter-missing') end
                if s.log and s.log[e.questID] then keep(row,'active-use')
                elseif not q or not q.complete or not s.done then resolved=false
                elseif not s.done[e.questID] then keep(row,'future-use') end
            end
            for _,slot in ipairs(g.slots) do
                if slot.questID then
                    row.relatedIDs[slot.questID]=true
                    if slot.questActive or (s.log and s.log[slot.questID]) then keep(row,'active-use')
                    else resolved=false end
                end
            end
            if value(item,'questRewards') and next(value(item,'questRewards')) then keep(row,'reward') end
            if D.ID(value(item,'teachesSpell')) then keep(row,'spell-use') end
            local cls=value(item,'class')
            if cls==2 or cls==4 or cls==13 then keep(row,'item-utility') end
            local old=vanilla(id,uses)
            if not resolved or old==nil then uncertain(row,'state-unresolved')
            elseif not old and not any then uncertain(row,'new-forever')
            else uncertain(row,'no-use-found') end
            finish(row); report.itemRows[#report.itemRows+1]=row
        end
    end
    table.sort(report.itemRows,function(a,b)
        local af,bf=a.reasonCodes[1]=='no-use-found',b.reasonCodes[1]=='no-use-found'
        if af~=bf then return af end
        return a.exactID<b.exactID
    end)
    local failed=sortedKeys(s.failures)
    for _,name in ipairs(sortedKeys(index.failures)) do failed[#failed+1]=name end
    report.status=(report.complete and 'Reads succeeded' or 'Limited data')..': log '..(s.reads.log or 'unknown')
        ..', history '..(s.reads.history or 'unknown')..', bags '..(s.reads.bags or 'unknown')
        ..'; QuestieDB '..(s.provider.version or 'unknown')..'; Keep or Toss 1.7.0 data build 70888 (known uses 70790; vanilla 38704).'
    if #failed>0 then report.status=report.status..' Failed: '..table.concat(failed,', ') end
    return report
end
