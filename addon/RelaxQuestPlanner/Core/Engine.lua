RelaxQuestPlanner = RelaxQuestPlanner or {}
local RQP=RelaxQuestPlanner
RQP.Engine=RQP.Engine or {}
local E,D=RQP.Engine,RQP.Data
local function tick() if E.Checkpoint then E.Checkpoint() end end
local function copy(t) local n={}; for k,v in pairs(t or {}) do n[k]=v end; return n end
local function nonempty(v) if type(v)=="table" then return next(v)~=nil end; return D.Present(v) end
local function firstGate(a,b) if nonempty(a) then return a end; if nonempty(b) then return b end end
local function member(mask,bit) return bit and bit>0 and math.floor((mask or 0)/bit)%2==1 end
E.MaskContains=member
local holidays={[-366]=true,[-22]=true,[-284]=true,[-369]=true,[-364]=true,[-1002]=true,
    [-1003]=true,[-370]=true,[-41]=true,[-375]=true,[-376]=true,[-1006]=true,[-1005]=true}
local never={race=true,class=true,blacklisted=true,repeatable=true,holiday=true,superseded=true,
    exclusive=true,breadcrumbObsolete=true,expired=true,overLevel=true}
local function static(r,p)
    if D.Blacklisted(r.id) then return "blacklisted" end
    if r.races~=0 and not member(r.races,p.raceMask) then return "race" end
    if r.classes~=0 and not member(r.classes,p.classMask) then return "class" end
    -- Without QuestieDB, FDJ's explicit faction/class metadata still applies.
    if (r.faction=="Alliance" or r.faction=="Horde") and r.faction~=p.faction then return "race" end
    if r.classOnly and p.class and string.upper(r.classOnly)~=p.class then return "class" end
    if member(r.special,1) then return "repeatable" end
    if holidays[r.zone] then return "holiday" end
end
local graphs=setmetatable({}, {__mode="k"})
function E.Invalidate() graphs=setmetatable({}, {__mode="k"}) end
local function refuted(q,c)
    if RQP.Learn and RQP.Learn.Refuted then return RQP.Learn.Refuted(q,c) end
    return false
end
local function baseReaches(a,b,seen)
    if seen[b] then return false end; seen[b]=true
    local r=D.Quest(b); if not r then return false end
    for _,list in ipairs({r.grp,r.single}) do for _,id in ipairs(list) do
        local x=math.abs(id)
        if x==a or baseReaches(a,x,seen) then return true end
        tick()
    end end
    return false
end
local function graph(p)
    if graphs[p] then return graphs[p] end
    local g={extra={},kids={},leads={},dropped={}}
    local function edge(parent,q)
        g.kids[parent]=g.kids[parent] or {}; g.kids[parent][q]=true
    end
    local function claim(q,c,source)
        local why; local r=D.Quest(c)
        if c==q then why="self"
        elseif c>=1000000 then why="pseudo item step"
        elseif not r then why="unknown id"
        else
            local st=static(r,p)
            if st=="race" or st=="class" or st=="blacklisted" then why="prereq not obtainable by this character"
            elseif ((p.done[q] or p.log[q]) and not p.done[c]) or refuted(q,c) then why="refuted by player history"
            elseif baseReaches(q,c,{}) then why="would create cycle" end
        end
        if why then g.dropped[#g.dropped+1]={quest=q,prereq=c,source=source,why=why}; return end
        g.extra[q]=g.extra[q] or {}; g.extra[q][c]=g.extra[q][c] or {}; g.extra[q][c][source]=true
        edge(c,q)
    end
    for _,q in ipairs(D.AllQuestIds()) do
        local r=D.Quest(q)
        for _,list in ipairs({r.grp,r.single}) do for _,x in ipairs(list) do edge(math.abs(x),q) end end
        if r.startingWith then edge(r.startingWith,q) end
        if r.parent then edge(r.parent,q) end
        if r.next then g.leads[r.next]=g.leads[r.next] or {}; table.insert(g.leads[r.next],q) end
        tick()
    end
    local fdj=D.FDJ()
    if fdj and type(fdj.QUEST_PREREQ_CHAINS)=="table" then
        for _,q in ipairs(D.Keys(fdj.QUEST_PREREQ_CHAINS)) do
            for _,s in ipairs(fdj.QUEST_PREREQ_CHAINS[q]) do if type(s.id)=="number" then claim(q,s.id,"FDJ") end; tick() end
        end
    end
    local dg=D.DungeonQuestie()
    if dg then for _,q in ipairs(dg.quests) do
        for _,c in ipairs(q.prereqs or {}) do claim(q.id,c,"DungeonQuestie"); tick() end
    end end
    graphs[p]=g
    return g
end
local function gates(q,p,done,log,level,g)
    local r=D.Quest(q)
    if not r then return {{"unknownQuest",q}} end
    if done[q] then return {"done"} end
    if log[q] then return {"inlog"} end
    local st=static(r,p); if st then return {st} end
    local why={}
    local function add(k,v) why[#why+1]={k,v} end
    if #r.single>0 then
        local satisfied=false
        for _,x in ipairs(r.single) do if done[x] or refuted(q,x) then satisfied=true end end
        if not satisfied then add("prereqAny",r.single) end
    elseif #r.grp>0 then
        local missing={}
        for _,x in ipairs(r.grp) do
            local id=math.abs(x); local satisfied=done[id] or refuted(q,id)
            if not satisfied and x>0 then
                local previous=D.Quest(x)
                for _,e in ipairs(previous and previous.excl or {}) do if done[e] then satisfied=true; break end end
            end
            if not satisfied then missing[#missing+1]=id end
        end
        if #missing>0 then add("prereqAll",missing) end
    end
    if r.parent and not log[r.parent] and not refuted(q,r.parent) then add("parentActive",r.parent) end
    if r.next and (done[r.next] or log[r.next]) then return {"superseded"} end
    for _,e in ipairs(r.excl) do if done[e] or log[e] then return {"exclusive"} end end
    if r.bcFor and (done[r.bcFor] or log[r.bcFor]) then return {"breadcrumbObsolete"} end
    if r.untilDone and done[r.untilDone] then return {"expired"} end
    if r.startingWith and not done[r.startingWith] and not log[r.startingWith] and not refuted(q,r.startingWith) then add("startingWith",r.startingWith) end
    if r.disabledBy and log[r.disabledBy] then add("disabledBy",r.disabledBy) end
    -- Lookahead relaxes only the minimum level, never the character's upper cap.
    if r.maxLevel and p.level>r.maxLevel then return {"overLevel"} end
    if r.reqLevel>level then add("level",r.reqLevel) end
    if nonempty(r.skill) or nonempty(r.ranks) or nonempty(r.spec) then add("profession",firstGate(r.skill,r.ranks)) end
    if nonempty(r.minRep) or nonempty(r.maxRep) then add("reputation",firstGate(r.minRep,r.maxRep)) end
    if nonempty(r.spell) then add("spell",r.spell) end
    local missing={}
    for _,c in ipairs(D.Keys(g.extra[q] or {})) do if not done[c] and not refuted(q,c) then missing[#missing+1]=c end end
    if #missing>0 then add("prereqExtra",missing) end
    return why
end
local function state(q,p,done,log,level,g)
    local why=gates(q,p,done,log,level,g)
    if #why==0 then return "available",why end
    if type(why[1])=="string" then return why[1],why end
    local soft={profession=true,reputation=true,spell=true}
    for _,w in ipairs(why) do if not soft[w[1]] then return "blocked",why end end
    return "unknownGate",why
end
function E.State(q,p,opts)
    p=p or RQP.Player.Snapshot(); opts=opts or {}
    return state(q,p,opts.done or p.done,opts.log or p.log,opts.level or p.level,graph(p))
end

function E.Unlocks(q,p)
    p=p or RQP.Player.Snapshot()
    local g=graph(p); local done,log,seen=copy(p.done),copy(p.log),{[q]=true}
    done[q]=true; log[q]=nil
    local chain,frontier={},{q}
    while #frontier>0 do
        local following={}
        for _,parent in ipairs(frontier) do
            for _,child in ipairs(D.Keys(g.kids[parent] or {})) do
                if not seen[child] and not p.done[child] and not p.log[child] then
                    local before=state(child,p,p.done,p.log,60,g)
                    local active=copy(log); active[parent]=true
                    local after=state(child,p,done,active,60,g)
                    if before~="available" and after=="available" then
                        seen[child]=true; chain[#chain+1]=child; following[#following+1]=child
                    end
                end
                tick()
            end
        end
        for _,child in ipairs(following) do done[child]=true end
        frontier=following
    end
    return chain
end

local function itemSources(id,depth,seen)
    depth=depth or 0; seen=seen or {}
    if seen[id] or depth>4 then return {} end
    seen[id]=true
    local item=D.Item(id); if not item then return {} end
    local out={}
    for _,entry in ipairs({{"npc",item.npcDrops},{"object",item.objectDrops},{"vendor",item.vendors}}) do
        for _,source in ipairs(entry[2] or {}) do
            if type(source)=="number" and source>0 then
                local r=entry[1]=="object" and D.Object(source) or D.Npc(source)
                out[#out+1]={kind=entry[1],id=source,name=r and r.name,zone=r and r.zone}
            end
            tick()
        end
    end
    for _,container in ipairs(item.itemDrops or {}) do
        if type(container)=="number" and container>0 then
            for _,src in ipairs(itemSources(container,depth+1,seen)) do out[#out+1]=src end
        end
        tick()
    end
    return out
end
local function starters(r,finish)
    local out={}; local lists=finish and r.finishedBy or r.startedBy
    for index,kind in ipairs({"npc","object","item"}) do
        for _,id in ipairs(lists[index] or {}) do
            if type(id)=="number" and id>0 then
                local entity=(kind=="npc" and D.Npc(id)) or (kind=="object" and D.Object(id)) or (kind=="item" and D.Item(id))
                out[#out+1]={kind=kind,id=id,name=entity and entity.name,zone=entity and entity.zone,
                    sources=kind=="item" and itemSources(id) or nil}
            end
            tick()
        end
    end
    return out
end
local function singleSourceZone(sources,includeVendors)
    local seen,z,count={},nil,0
    for _,s in ipairs(sources) do
        local zone
        if s.kind=="npc" or s.kind=="object" then zone=s.zone or "unknown"
        elseif includeVendors and s.kind=="vendor" then zone="vendor" end
        if zone and not seen[zone] then seen[zone]=true; z=zone; count=count+1 end
        tick()
    end
    if count==1 then return z end
end
local function norm(s) return s:lower():gsub("the ",""):gsub("'",""):gsub("stormwind ",""):gsub(":",""):match("^%s*(.-)%s*$") end
local function dungeonMatch(a,b)
    local x,y=norm(D.CanonicalDungeon(a)),norm(D.CanonicalDungeon(b))
    return x~="" and y~="" and (x:find(y,1,true)~=nil or y:find(x,1,true)~=nil)
end
local classifications,addonLists={},nil
local function classify(r)
    if classifications[r.id] then return classifications[r.id] end
    local out={}
    local function addName(name) if name then out[D.CanonicalDungeon(name)]=true end end
    local function add(z) addName(D.DungeonForArea(z)) end
    for _,s in ipairs(starters(r)) do
        if s.kind=="item" then add(singleSourceZone(s.sources,false)) else add(s.zone) end
    end
    for index,kind in ipairs({"npc","object","item"}) do
        for _,objective in ipairs(r.objectives[index] or {}) do
            if type(objective)=="table" and type(objective[1])=="number" then
                local id=objective[1]
                if kind=="item" then add(singleSourceZone(itemSources(id),true))
                else local row=kind=="npc" and D.Npc(id) or D.Object(id); if row then add(row.zone) end end
            end
            tick()
        end
    end
    for _,objective in ipairs(r.objectives[5] or {}) do
        if type(objective)=="table" and type(objective[1])=="table" then
            for _,id in ipairs(objective[1]) do local n=D.Npc(id); if n then add(n.zone) end; tick() end
        end
    end
    add(r.zone)
    if r.fdj then addName(r.fdj.dungeon) end
    classifications[r.id]=out; return out
end
local oldInvalidate=E.Invalidate
function E.Invalidate() oldInvalidate(); classifications={}; addonLists=nil end
local function dungeonTargets(name,p,g)
    name=D.CanonicalDungeon(name)
    if not addonLists then
        local out={}; local dg=D.DungeonQuestie()
        if dg then for _,q in ipairs(dg.quests) do
            out[q.id]={}
            for _,dn in ipairs(q.dungeons or {}) do table.insert(out[q.id],D.CanonicalDungeon(dn)) end
            tick()
        end end
        addonLists=out
    end
    -- Invalidate() can clear addonLists while this coroutine is yielded.
    local lists=addonLists or {}
    local floor=g and g.floor or 0
    local targets={}
    for _,q in ipairs(D.AllQuestIds()) do
        local r=D.Quest(q)
        if not D.Blacklisted(q) then
            local matches=classify(r)[name]
            for _,dn in ipairs(lists[q] or {}) do if dungeonMatch(name,dn) then matches=true end end
            if matches then
                local st=state(q,p,p.done,p.log,60,g)
                local grey=floor>0 and (r.qLevel or 0)>0 and r.qLevel<floor and not p.log[q]
                if not never[st] and st~="done" and st~="unknownGate" and not grey then targets[#targets+1]=q end
            end
        end
        tick()
    end
    table.sort(targets,function(a,b)
        local al,bl=D.Quest(a).qLevel,D.Quest(b).qLevel
        if al~=bl then return al<bl end; return a<b
    end)
    return targets
end

local professions={[164]="Blacksmithing",[165]="Leatherworking",[171]="Alchemy",[182]="Herbalism",
    [186]="Mining",[197]="Tailoring",[202]="Engineering",[333]="Enchanting",[393]="Skinning",
    [129]="First Aid",[185]="Cooking",[356]="Fishing"}
local function hasProfession(r,p)
    local sk=r.skill; if not nonempty(sk) then return false end
    local id=type(sk)=="table" and sk[1] or sk
    local need=type(sk)=="table" and sk[2] or 1
    if not need or need==0 then need=1 end
    return ((p.professions or {})[professions[id] or tostring(id)] or 0)>=need
end
local function planSet(plan,done)
    local out=copy(done); for _,q in ipairs(plan) do out[q]=true end; return out
end
local function resolve(q,p,g,plan,stack,issues)
    local done=planSet(plan,p.done)
    if done[q] then return true end
    if stack[q] then issues[#issues+1]={"cycle",q}; return false end
    local r=D.Quest(q)
    if not r then issues[#issues+1]={"unknownQuest",q}; return false end
    local st,why=state(q,p,done,p.log,60,g)
    if never[st] then issues[#issues+1]={st,q,r.name}; return false end
    for _,w in ipairs(why) do
        if type(w)=="table" and w[1]=="profession" and not hasProfession(r,p) then
            issues[#issues+1]={"needsProfession",q,r.name,firstGate(r.skill,r.ranks)}; return false
        end
    end
    stack[q]=true; local ok=true
    for _,lead in ipairs(g.leads[q] or {}) do
        if not done[lead] and state(lead,p,done,p.log,60,g)=="available" then
            plan[#plan+1]=lead; done[lead]=true
        end
        tick()
    end
    why=gates(q,p,done,p.log,60,g)
    for _,w in ipairs(why) do
        if type(w)=="table" then
            local kind,v=w[1],w[2]
            if kind=="prereqAll" or kind=="prereqExtra" then
                for _,id in ipairs(v) do if not resolve(math.abs(id),p,g,plan,stack,issues) then ok=false end; tick() end
            elseif kind=="prereqAny" then
                local options={}; done=planSet(plan,p.done)
                for i,id in ipairs(v) do
                    local row=D.Quest(id)
                    options[#options+1]={q=id,available=state(id,p,done,p.log,60,g)=="available",
                        level=row and row.qLevel~=0 and row.qLevel or 99,order=i}
                end
                table.sort(options,function(a,b)
                    if a.available~=b.available then return a.available end
                    if a.level~=b.level then return a.level<b.level end
                    return a.order<b.order
                end)
                local found=false
                for _,candidate in ipairs(options) do
                    local trial=copy(plan)
                    if resolve(candidate.q,p,g,trial,copy(stack),{}) then
                        for i=#plan,1,-1 do plan[i]=nil end
                        for i,id in ipairs(trial) do plan[i]=id end
                        found=true; break
                    end
                end
                if not found then issues[#issues+1]={"noOptionFor",q,v}; ok=false end
            elseif kind=="startingWith" or kind=="parentActive" then
                if not resolve(v,p,g,plan,stack,issues) then ok=false end
            end
        end
        tick()
    end
    stack[q]=nil
    if ok then plan[#plan+1]=q end
    return ok
end

local function dependencies(q,p,g,planned)
    local needs,active,parentActive={},{},{}
    -- An observed active quest is already accepted: do not invent missing gates.
    if p.log[q] then return {},active,parentActive end
    local r=D.Quest(q)
    -- Unlike startingWith, this constraint survives while the parent is in the
    -- log and must still be enforced if it is turned in after this plan is built.
    if r.parent and not refuted(q,r.parent) then
        needs[r.parent],active[r.parent],parentActive[r.parent]=true,true,true
    end
    for _,w in ipairs(gates(q,p,p.done,p.log,60,g)) do
        if type(w)=="table" then
            local k,v=w[1],w[2]
            if k=="prereqAll" or k=="prereqExtra" then
                for _,id in ipairs(v) do
                    id=math.abs(id)
                    if planned[id] and id~=q then needs[id]=true
                    elseif k=="prereqAll" then
                        -- Positive group prereqs allow an exclusive alternative;
                        -- preserve the particular alternative selected in the plan.
                        local positive=false
                        for _,raw in ipairs(D.Quest(q).grp) do if raw==id then positive=true end end
                        local r=positive and D.Quest(id)
                        for _,alt in ipairs(r and r.excl or {}) do
                            if planned[alt] and alt~=q then needs[alt]=true; break end
                        end
                    end
                end
            elseif k=="prereqAny" then
                for _,id in ipairs(v) do if planned[id] and id~=q then needs[id]=true; break end end
            elseif k=="startingWith" then
                if planned[v] and v~=q then needs[v]=true; active[v]=true end
            end
        end
    end
    -- Preserve optional lead-ins selected by resolve: accepting their target
    -- first would make the lead-in superseded before the player could take it.
    for _,lead in ipairs(g.leads[q] or {}) do
        if planned[lead] and not p.done[lead] and lead~=q then needs[lead]=true end
    end
    for _,id in ipairs(D.Quest(q).bcs) do
        if planned[id] and not p.done[id] and id~=q then needs[id]=true end
    end
    return D.Keys(needs),active,parentActive
end
local function step(kind,r,p,floor,needs,active,parentActive)
    local s={kind=kind,q=r.id,name=r.name,needs=needs,activeNeeds=active,parentActive=parentActive,
        below=r.qLevel<floor,levelOK=r.reqLevel<=p.level,source=r.source or "QuestieDB",
        confidence=r.source=="QuestieDB" and "db" or "addon",level=r.qLevel,reqLevel=r.reqLevel}
    if RQP.Learn and next(RQP.Learn.Observed(r.id)) then s.confidence="observed" end
    local pin
    if kind=="accept" or kind=="turnin" then
        local source=starters(r,kind=="turnin")[1]
        if source then
            s.who=source.name; s.zone=D.ZoneName(source.zone)
            if source.kind=="item" then
                local z=singleSourceZone(source.sources,false)
                s.note="Starts from item "..(source.name or tostring(source.id))..(z and " (known drop source)" or " (world drop / source unknown)")
            else pin=D.Pin(source.kind,source.id) end
        elseif r.fdj then
            s.note=(kind=="accept" and r.fdj.pickup or r.fdj.turnin) or "Location unknown (addon supplement)"
            if kind=="accept" then pin=D.Pin("quest",r.id) end
        else s.note="Location unknown; check your quest log" end
    else
        s.note=p.log[r.id] and "In your quest log - finish the objectives" or "Complete the quest objectives"
    end
    if pin then for k,v in pairs(pin) do s[k]=v end end
    if s.below then
        local count=#E.Unlocks(r.id,p)
        local why=count>0 and ("unlocks "..tostring(count).." quests") or "quest reward"
        s.note=(s.note and s.note..". " or "").."Below floor: your choice — "..why
    end
    return s
end
local function actions(route,p,g,targets)
    local planned=planSet(route.plan,{})
    -- Evaluate acceptance during the async build, never while repainting the UI.
    route.availability={}
    local nodes,byQuest,blocked,metadata={},{},{},{}
    for index,q in ipairs(route.plan) do
        local r=D.Quest(q); local needs,active,parentActive=dependencies(q,p,g,planned)
        route.availability[q]=state(q,p,p.done,p.log,p.level,g)
        metadata[q]={needs=needs,active=active,parentActive=parentActive,index=index}
        if r.reqLevel>p.level then blocked[q]=true end
        tick()
    end
    -- A lower-level child cannot be presented now if its required parent is later.
    local changed=true
    while changed do
        changed=false
        for _,q in ipairs(route.plan) do
            for _,id in ipairs(metadata[q].needs) do
                if blocked[id] and not blocked[q] then blocked[q]=true; changed=true end
            end
            tick()
        end
    end
    local isPrerequisite={}
    for _,q in ipairs(route.plan) do for _,id in ipairs(metadata[q].needs) do isPrerequisite[id]=true end end
    local function node(kind,r,m,priority)
        local s=step(kind,r,p,route.floor,m.needs,m.active,m.parentActive)
        local n={step=s,deps={},priority=priority,index=m.index,key=kind..":"..r.id}
        nodes[#nodes+1]=n; byQuest[r.id]=byQuest[r.id] or {}; byQuest[r.id][kind]=n
        return n
    end
    for _,q in ipairs(route.plan) do
        local r,m=D.Quest(q),metadata[q]
        if blocked[q] then
            local s=step("accept",r,p,route.floor,m.needs,m.active,m.parentActive)
            if s.levelOK then s.note=(s.note or "").."; prerequisite is level-locked" end
            route.later[#route.later+1]=s
        else
            local starter=starters(r)[1]
            local inside=starter and (starter.kind=="item" or D.DungeonForArea(starter.zone)==route.title)
            local accept
            if not p.log[q] then accept=node("accept",r,m,inside and 2 or 0) end
            local complete
            if not p.ready[q] then
                complete=node("complete",r,m,targets[q] and 3 or 1)
                if targets[q] then complete.step.zone=route.title end
                if accept then complete.deps[accept.key]=true end
            end
            local turn=node("turnin",r,m,isPrerequisite[q] and 1 or 4)
            if complete then turn.deps[complete.key]=true elseif accept then turn.deps[accept.key]=true end
        end
        tick()
    end
    for _,n in ipairs(nodes) do
        if n.step.kind=="accept" then
            for _,id in ipairs(n.step.needs) do
                local parent=byQuest[id]
                local required
                if parent then
                    if n.step.activeNeeds[id] then required=parent.accept else required=parent.turnin end
                end
                if required then n.deps[required.key]=true end
                if parent and parent.turnin and n.step.parentActive[id] then
                    -- parentActive means still in the log at child acceptance.
                    parent.turnin.deps[n.key]=true
                end
            end
        end
        tick()
    end
    local emitted={}
    local function less(a,b)
        if a.priority~=b.priority then return a.priority<b.priority end
        local az,bz=a.step.zone or "~",b.step.zone or "~"
        if az~=bz then return az<bz end
        if a.index~=b.index then return a.index<b.index end
        return a.key<b.key
    end
    for _=1,#nodes do
        local best
        for _,n in ipairs(nodes) do
            if not emitted[n.key] then
                local ready=true
                for dep in pairs(n.deps) do if not emitted[dep] then ready=false; break end end
                if ready and (not best or less(n,best)) then best=n end
            end
            tick()
        end
        if not best then route.issues[#route.issues+1]={"actionCycle","Unable to order remaining actions safely"}; break end
        route.steps[#route.steps+1]=best.step; emitted[best.key]=true
    end
end
local function build(title,goal,p,opts)
    p=p or RQP.Player.Snapshot(); opts=opts or {}
    local floor=0
    if opts.mode=="strict" then floor=p.level-5
    elseif opts.mode~="all" then floor=p.level-(opts.trivial or p.trivial or 7) end
    local route={title=title,level=p.level,floor=floor,steps={},later={},issues={},plan={}}
    local ok,reason=D.Ready()
    if not ok then route.issues[1]={"missingData",reason}; return route end
    if D.Source()~="QuestieDB" then route.issues[#route.issues+1]={"limitedData",reason} end
    local g=graph(p)
    g.floor=floor
    local targets=goal and {goal} or dungeonTargets(title,p,g)
    local targetSet={}
    for _,q in ipairs(targets) do
        targetSet[q]=true; resolve(q,p,g,route.plan,{},route.issues); tick()
    end
    route.targets=#targets
    actions(route,p,g,targetSet)
    return route
end
-- Synchronous forms are useful for offline checks. UI consumers must use Async.
function E.BuildPath(name,p,opts) return build(name,nil,p,opts) end
function E.BuildGoal(id,p,opts)
    local r=D.Quest(id); return build(r and r.name or ("Quest "..tostring(id)),id,p,opts)
end

-- Lua 5.1 cannot yield across pcall. Resume the coroutine inside the protected
-- OnUpdate handler; checkpoint/yield executes only inside that coroutine.
local queue,frame,current={},nil,nil
local function clock()
    if debugprofilestop then return debugprofilestop() end
    return (GetTime and GetTime() or 0)*1000
end
function E.Checkpoint()
    if not current or coroutine.running()~=current.co then return end
    current.ops=current.ops+1
    if current.ops>=400 or clock()>=current.deadline then coroutine.yield() end
end
local function callback(job,route)
    if type(job.cb)=="function" then local ok,err=pcall(job.cb,route); if not ok then D.Error(err) end end
end
local function pump()
    local job=queue[1]
    if not job then frame:SetScript("OnUpdate",nil); return end
    if job.cancelled then table.remove(queue,1); return end
    job.ops=0; job.deadline=clock()+job.budget-0.5; current=job
    local ok,result=coroutine.resume(job.co)
    current=nil
    if not ok then
        table.remove(queue,1); D.Error(result)
        callback(job,{title=job.title,level=job.level,floor=0,steps={},later={},plan={},issues={{"error",tostring(result)}}})
    elseif coroutine.status(job.co)=="dead" then table.remove(queue,1); callback(job,result) end
    if #queue==0 then frame:SetScript("OnUpdate",nil) end
end
local function async(title,p,opts,cb,fn)
    opts=opts or {}; p=p or RQP.Player.Snapshot()
    local budget=tonumber(opts.sliceMs) or 8; budget=math.min(8,math.max(1,budget))
    local job={title=title,level=p.level,cb=cb,budget=budget}
    job.co=coroutine.create(function() return fn(p,opts) end)
    function job:Cancel() self.cancelled=true end
    queue[#queue+1]=job
    if not frame then frame=CreateFrame("Frame") end
    frame:SetScript("OnUpdate",function()
        local ok,err=pcall(pump)
        if not ok then current=nil; D.Error(err); frame:SetScript("OnUpdate",nil) end
    end)
    return job
end
function E.BuildPathAsync(name,p,opts,cb)
    return async(name,p,opts,cb,function(player,options) return E.BuildPath(name,player,options) end)
end
function E.BuildGoalAsync(id,p,opts,cb)
    return async("Quest "..tostring(id),p,opts,cb,function(player,options) return E.BuildGoal(id,player,options) end)
end
