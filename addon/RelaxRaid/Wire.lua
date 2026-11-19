-- RLXR1 preparation transport. No executable text and no dependency on other addons.
-- Full grammar, limits and pinned API evidence: docs/raid/PREP-PROTOCOL.md.
local _, RR = ...
local Wire = RR.module("Wire")
Wire.PREFIX, Wire.MAX_BODY, Wire.CHUNK, Wire.MAX_PARTS = "RLXR1", 4096, 200, 21
Wire.stats = {}
function Wire.count(key)
    Wire.stats[key] = math.min(999999, (Wire.stats[key] or 0) + 1)
end

local function split(s, sep, cap)
    local out, start = {}, 1
    while true do
        local at = s:find(sep, start, true)
        if #out >= cap then return nil end
        out[#out + 1] = s:sub(start, at and at - 1 or #s)
        if not at then return out end
        start = at + #sep
    end
end
local function integer(s)
    if s == "u" then return "unreadable" end
    if not s:match("^%d+$") or (#s > 1 and s:sub(1,1) == "0") or #s > 10 then return nil end
    return tonumber(s)
end
local function fmt(n)
    return n == "unreadable" and "u" or string.format("%.0f", n)
end
function Wire.encode(report)
    local P = RR.modules.Prep
    if not P then return nil end
    local r = P.validate(report)
    if not r then return nil end
    local c, g = {}, {}
    if type(r.consumables) == "table" then
        for _, row in ipairs(r.consumables) do c[#c+1] = row.category .. "," .. fmt(row.itemID) .. "," .. fmt(row.count) end
        c = #c == 0 and "-" or table.concat(c, ";")
    else c = "u" end
    if type(r.gear) == "table" then
        for _, row in ipairs(r.gear) do g[#g+1] = fmt(row.slot) .. "," .. fmt(row.wornID) .. "," .. fmt(row.bagID) end
        g = #g == 0 and "-" or table.concat(g, ";")
    else g = r.gear == "not checked" and "n" or "u" end
    local ns = {}
    if r.notScored then
        for i, slot in ipairs(r.notScored) do ns[i] = fmt(slot) end
        ns = #ns == 0 and "-" or table.concat(ns, ",")
    else ns = r.gear == "not checked" and "n" or "u" end
    local body = table.concat({"1", r.addonVersion, r.name, r.class, fmt(r.level), fmt(r.playedLevel),
        r.reason, fmt(r.sentAt), c, g, ns}, "\t")
    if #body > Wire.MAX_BODY then return nil end
    return body
end
local function rows(s, kind)
    if s == "u" then return "unreadable" end
    if kind == "gear" and s == "n" then return "not checked" end
    if s == "-" then return {} end
    local parts = split(s, ";", kind == "gear" and 19 or 32)
    if not parts then return nil end
    local out = {}
    for _, text in ipairs(parts) do
        local f = split(text, ",", 3)
        if not f or #f ~= 3 then return nil end
        if kind == "gear" then out[#out+1] = {slot=integer(f[1]), wornID=integer(f[2]), bagID=integer(f[3])}
        else out[#out+1] = {category=f[1], itemID=integer(f[2]), count=integer(f[3])} end
    end
    return out
end
function Wire.decode(body)
    local P = RR.modules.Prep
    if not P or not P.plain(body) or type(body) ~= "string" or #body > Wire.MAX_BODY then return nil end
    local f = split(body, "\t", 11)
    if not f or #f ~= 11 or f[1] ~= "1" then return nil end
    f[3] = P.fullName(f[3])
    if not f[3] then return nil, "badName" end
    local gear, notScored = rows(f[10], "gear"), nil
    if type(gear) == "table" then
        notScored = f[11] == "-" and {} or split(f[11], ",", 19)
        if not notScored then return nil end
        for i, slot in ipairs(notScored) do
            local n = integer(slot)
            if not P.number(n, 1, 19) then return nil end
            notScored[i] = n
        end
    elseif not ((gear == "not checked" and f[11] == "n") or (gear == "unreadable" and f[11] == "u")) then
        return nil
    end
    return P.validate({protocol=1, addonVersion=f[2], name=f[3], class=f[4], level=integer(f[5]),
        playedLevel=integer(f[6]), reason=f[7], sentAt=integer(f[8]), consumables=rows(f[9], "bags"), gear=gear, notScored=notScored})
end

local registered, joined, nextID = false, false, 0
local encounter = false
function Wire.allowed()
    local P = RR.modules.Prep
    if not P then return false end
    local ok, inside, instanceType = pcall(IsInInstance)
    if not ok or not P.plain(inside) or type(inside) ~= "boolean" or not P.text(instanceType, 16)
        or not (instanceType == "none" or instanceType == "raid" or instanceType == "party") then return false end
    for kind = 0, 5 do -- Combat, Encounter, ChallengeMode, PvPMatch, Map, Chat (pinned enum)
        if P.read(C_RestrictedActions and C_RestrictedActions.GetAddOnRestrictionState, kind) ~= 0 then return false end
    end
    return not encounter and P.read(IsInRaid) == true and
        P.read(InCombatLockdown) == false and P.read(UnitAffectingCombat, "player") == false and
        P.read(C_ChatInfo and C_ChatInfo.InChatMessagingLockdown) == false and
        P.read(C_ChatInfo and C_ChatInfo.AreOutgoingAddonChatMessagesRestricted) == false
end
local pending, active, scheduled, generation = {}, nil, false, 0
local lastReason, lastReport, lastPart, attempts = {}, -math.huge, -math.huge, {}
local ORDER = {"join", "ready", "encounter"}
local pump
local function wake(delay)
    if scheduled then return end
    scheduled = true
    local token = generation
    RR.after(delay, "Wire.pump", function()
        if token ~= generation then return end
        scheduled = false
        pump()
    end)
end
function Wire.cancel()
    pending, active, scheduled = {}, nil, false
    generation = generation + 1
end
pump = function()
    local P, now = RR.modules.Prep, RR.now()
    if not P or not P.plain(now) or type(now) ~= "number" or now < 0 or now > 9999999999 then return end
    for reason, at in pairs(pending) do
        if now < at or now - at >= 120 then pending[reason] = nil; Wire.count("expired") end
    end
    if active and (now < active.at or now - active.at >= 120) then active = nil; Wire.count("expired") end
    if not active and next(pending) == nil then return end
    if not Wire.allowed() then wake(1); return end
    if not active then
        for _, reason in ipairs(ORDER) do
            if pending[reason] and now - (lastReason[reason] or -math.huge) >= 30 and now - lastReport >= 10 then
                local body = Wire.encode(P.build(reason))
                local at = pending[reason]
                pending[reason] = nil
                if body then
                    local S, r = RR.modules.Store, Wire.decode(body)
                    if S and r then S.put(r.name, r) end -- own copy needs no client self-echo
                    nextID = nextID % 999999999 + 1
                    active = {body=body, id=nextID, part=1, total=math.ceil(#body/Wire.CHUNK), at=at, reason=reason}
                else Wire.count("invalidLocal") end
                break
            end
        end
    end
    if not active then if next(pending) then wake(1) end; return end
    while attempts[1] and now - attempts[1] >= 60 do table.remove(attempts, 1) end
    if #attempts >= 60 or now - lastPart < 0.5 then wake(0.5); return end
    local a = active
    local text = "P1|" .. a.id .. "|" .. a.part .. "|" .. a.total .. "|" .. a.body:sub((a.part-1)*Wire.CHUNK+1, a.part*Wire.CHUNK)
    -- Last possible gate: collection/optional providers can change restriction state too.
    if not Wire.allowed() then wake(1); return end
    lastPart = now
    attempts[#attempts+1] = now -- failed client attempts cost the same rate budget
    if a.part == 1 then lastReason[a.reason], lastReport = now, now end
    local ok, result = pcall(C_ChatInfo.SendAddonMessage, Wire.PREFIX, text, "RAID")
    if not ok or not P.plain(result) or result ~= 0 then active = nil; Wire.count("sendFailed")
    else
        Wire.count("sent")
        a.part = a.part + 1
        if a.part > a.total then active = nil end
    end
    if active or next(pending) then wake(0.5) end
end
function Wire.request(reason)
    local P = RR.modules.Prep
    if not RR.loaded or not registered or not P or not P.reasons[reason] or P.read(IsInRaid) ~= true then return end
    if active and active.reason == reason then return end
    local now = P.clock()
    if not now then return end
    pending[reason] = pending[reason] or now -- three reasons, fixed deadline; bursts do not extend it
    wake(1)
end
function Wire.Init()
    local ok, result = pcall(C_ChatInfo and C_ChatInfo.RegisterAddonMessagePrefix, Wire.PREFIX)
    local P = RR.modules.Prep
    registered = ok and P and P.plain(result) and (result == 0 or result == 1) -- Success / DuplicatePrefix
end
local function group()
    local P = RR.modules.Prep
    local inRaid = P and P.read(IsInRaid) == true
    if inRaid and not joined then Wire.request("join") end
    if not inRaid then Wire.cancel(); encounter = false; if Wire.clearReceive then Wire.clearReceive() end end
    joined = inRaid
end
RR.on("GROUP_ROSTER_UPDATE", "Wire", group)
RR.on("PLAYER_ENTERING_WORLD", "Wire", group)
RR.on("READY_CHECK", "Wire", function() Wire.request("ready") end)
local peers, receiveTimes = {}, {}
function Wire.peerCount()
    local count = 0
    for _ in pairs(peers) do count = count + 1 end
    return count
end
function Wire.clearReceive()
    peers, receiveTimes = {}, {}
end
local function charge(times, cap, now)
    while times[1] and (now < times[1] or now-times[1] >= 60) do table.remove(times,1) end
    if #times >= cap then return false end
    times[#times+1] = now
    return true
end
function Wire.receive(prefix, text, channel, sender)
    local P, S = RR.modules.Prep, RR.modules.Store
    if not P or not S or not registered or P.read(IsInRaid) ~= true then return end
    if not P.plain(prefix) or prefix ~= Wire.PREFIX or not P.plain(channel) or channel ~= "RAID"
        or not P.plain(text) or type(text) ~= "string" or #text > 240 then return end
    sender = P.fullName(sender)
    if not sender then Wire.count("badName"); return end
    local now = RR.now()
    if not P.plain(now) or type(now) ~= "number" or now < 0 or now > 9999999999 then return end
    if not charge(receiveTimes, 2400, now) then Wire.count("receiveLimited"); return end
    local id, part, total, chunk = text:match("^P1|(%d+)|(%d+)|(%d+)|(.*)$")
    id, part, total = id and integer(id), part and integer(part), total and integer(total)
    if not P.number(id, 1, 999999999) or not P.number(part, 1, Wire.MAX_PARTS)
        or not P.number(total, part or 1, Wire.MAX_PARTS) or #chunk == 0 or #chunk > Wire.CHUNK
        then Wire.count("invalid"); return end
    local key = P.nameKey(sender)
    for k, p in pairs(peers) do if now < p.at or now-p.at >= 60 then peers[k] = nil end end
    local peer = peers[key]
    if not peer then
        if Wire.peerCount() >= 40 then Wire.count("peerLimited"); return end
        peer = {done={}, times={}, at=now}; peers[key] = peer
    end
    peer.at = now
    if not charge(peer.times, 60, now) then Wire.count("receiveLimited"); return end
    local doneCount = 0
    for old, at in pairs(peer.done) do
        if now < at or now-at >= 60 then peer.done[old] = nil else doneCount = doneCount + 1 end
    end
    if doneCount >= 8 then Wire.count("receiveLimited"); return end
    if peer.done[id] then Wire.count("replay"); return end
    local a = peer.active
    if a and (now < a.at or now-a.at >= 30) then
        peer.active = nil; peer.done[a.id] = now; a = nil; Wire.count("expiredReceive")
        if doneCount + 1 >= 8 then Wire.count("receiveLimited"); return end
    end
    if peer.done[id] then return end
    if not a or a.id ~= id then
        a = {id=id, total=total, at=now, chunks={}, count=0, bytes=0}
        peer.active = a
    end
    if a.total ~= total or (a.chunks[part] and a.chunks[part] ~= chunk) then
        peer.active = nil; peer.done[id] = now; Wire.count("invalid"); return
    end
    if a.chunks[part] then return end
    a.chunks[part], a.count, a.bytes = chunk, a.count+1, a.bytes+#chunk
    if a.bytes > Wire.MAX_BODY then peer.active = nil; peer.done[id] = now; Wire.count("invalid"); return end
    if a.count ~= total then return end
    peer.active = nil
    peer.done[id] = now
    local r, why = Wire.decode(table.concat(a.chunks))
    if not r then Wire.count(why or "invalid"); return end
    if P.nameKey(r.name) ~= key then Wire.count("forged"); return end
    if S.put(sender, r) then Wire.count("received") end
end
RR.on("CHAT_MSG_ADDON", "Wire", function(_, prefix, text, channel, sender) Wire.receive(prefix,text,channel,sender) end)
RR.on("PLAYER_LOGOUT", "Wire", function() Wire.cancel(); Wire.clearReceive() end)
RR.on("ENCOUNTER_START", "Wire", function()
    encounter = true
    if active then
        pending[active.reason] = pending[active.reason] or active.at
        active = nil -- discard partial pre-pull snapshot; collect again only after the gates reopen
    end
end)
RR.on("ENCOUNTER_END", "Wire", function() encounter = false; Wire.request("encounter") end)
