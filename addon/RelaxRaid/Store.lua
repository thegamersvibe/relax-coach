-- Account-wide logger preparation, inside the existing app-approved SavedVariables.
-- Version/layout/caps and API evidence: docs/raid/PREP-PROTOCOL.md.
local _, RR = ...
local Store = RR.module("Store")
Store.VERSION, Store.MAX_NIGHTS, Store.MAX_WINDOWS, Store.MAX_PLAYERS = 2, 3, 80, 40
Store.MAX_BYTES = 100000 -- canonical wire-body bytes per character, per night
Store.stats = {}
local newWindow = false
function Store.count(key)
    Store.stats[key] = math.min(999999, (Store.stats[key] or 0) + 1)
end
local readOnly = false
local function newer(t, version)
    return type(t) == "table" and type(t.version) == "number" and t.version > (version or 1)
end
function Store.writable()
    local db = RelaxRaidDB
    return not readOnly and not newer(RelaxRaidCharDB) and type(db) == "table" and db.version == 1
        and type(db.prep) == "table" and db.prep.version == Store.VERSION
end
-- Both inputs come from Wire.encode after validation. Compare every canonical
-- field except field 8 (sentAt), without decoding the retained body on each put.
local function content(body)
    local start = 1
    for _ = 1, 7 do start = body:find("\t", start, true) + 1 end
    local stop = body:find("\t", start, true)
    return body:sub(1, start-1) .. body:sub(stop)
end
local function trimReports(night, key)
    local bodies, bytes = night.reports[key], 0
    for _, body in ipairs(bodies) do bytes = bytes + #body end
    local over = bytes > Store.MAX_BYTES
    while bytes > Store.MAX_BYTES do
        bytes = bytes - #table.remove(bodies, 1)
        for _, window in ipairs(night.windows) do
            local row = window.reports[key]
            if row then
                if row.report == 1 then window.reports[key] = nil
                else row.report = row.report - 1 end
            end
        end
        Store.count("budgetEvicted")
    end
    return over
end
local function pruneNight(night)
    local live = {}
    for i = #night.windows, 1, -1 do
        local rows = night.windows[i].reports
        if not next(rows) then table.remove(night.windows, i)
        else for key in pairs(rows) do live[key] = true end end
    end
    for key in pairs(night.reports) do
        if not live[key] then night.reports[key] = nil end
    end
end
-- Bodies use the canonical wire grammar, not expanded Lua field tables.
-- Re-encoding the validated report also gives deterministic budget bytes on reload.
local function cleanReports(source, key)
    local P, W = RR.modules.Prep, RR.modules.Wire
    local bodies, indices, bytes = {}, {}, 0
    if not W or not P.array(source, Store.MAX_BYTES) then return bodies, indices end
    -- Walk newest-first so even an oversized local save cannot expand all bodies.
    for i = #source, 1, -1 do
        local r = W.decode(source[i])
        if r and P.nameKey(r.name) == key then
            local body = W.encode(r)
            if bytes + #body > Store.MAX_BYTES then break end
            bytes = bytes + #body
            bodies[#bodies+1] = body
            indices[i] = #bodies
        else Store.count("invalidSaved") end
    end
    local count = #bodies
    for i = 1, math.floor(count/2) do bodies[i], bodies[count-i+1] = bodies[count-i+1], bodies[i] end
    for i, index in pairs(indices) do indices[i] = count-index+1 end
    return bodies, indices
end
local function cleanNights(source)
    local P, out, days = RR.modules.Prep, {}, {}
    if not P or not P.array(source, Store.MAX_NIGHTS) then return out end
    for _, n in ipairs(source) do
        if type(n) == "table" and P.number(n.day, -1, 999999) and not days[n.day]
            and P.array(n.windows, Store.MAX_WINDOWS) and type(n.reports) == "table" then
            local night, remap = {day=n.day, windows={}, reports={}}, {}
            days[n.day] = true
            for _, w in ipairs(n.windows) do
                if type(w) == "table" and P.number(w.openedAt, 0, 9999999999) and type(w.reports) == "table" then
                    local window, seen = {openedAt=w.openedAt, reports={}}, 0
                    for key, row in pairs(w.reports) do
                        seen = seen + 1
                        if seen > Store.MAX_PLAYERS then break end
                        if P.nameKey(key) == key and type(row) == "table"
                            and P.number(row.receivedAt, 0, 9999999999) and P.number(row.report, 1, 100000) then
                            if not remap[key] then night.reports[key], remap[key] = cleanReports(n.reports[key], key) end
                            local index = remap[key][row.report]
                            if index then
                                window.reports[key] = {receivedAt=row.receivedAt, report=index}
                            else Store.count("invalidSaved") end
                        end
                    end
                    if next(window.reports) then night.windows[#night.windows+1] = window end
                end
            end
            pruneNight(night)
            if #night.windows > 0 then out[#out+1] = night end
        end
    end
    return out
end
function Store.Init()
    local db = RelaxRaidDB
    if newer(db) or (type(db) == "table" and newer(db.prep, Store.VERSION)) or newer(RelaxRaidCharDB) then
        readOnly = true
        return -- not even normalization: every byte of the newer value is retained
    end
    if type(db) ~= "table" then db = {}; RelaxRaidDB = db end
    db.version = 1
    local nights = type(db.prep) == "table" and db.prep.version == Store.VERSION and db.prep.nights
    db.prep = {version=Store.VERSION, nights=cleanNights(nights)}
end
function Store.put(sender, report)
    if not Store.writable() then return false end
    local P, L = RR.modules.Prep, RR.modules.Log
    if not P or not L or L.logger("raid") ~= true or P.read(IsInRaid) ~= true then return false end
    local r = P.validate(report)
    local key = P.nameKey(P.fullName(sender))
    if not r or not key then Store.count("invalid"); return false end
    if P.nameKey(r.name) ~= key then Store.count("forged"); return false end
    local W = RR.modules.Wire
    local body = W and W.encode(r)
    if not body then Store.count("invalid"); return false end
    local now = P.read(GetServerTime)
    if not P.number(now, 0, 9999999999) then return false end
    local nights = RelaxRaidDB.prep.nights
    local day = math.floor((now - 43200)/86400) -- receiver's UTC-noon boundary; not the app's nightId
    local night
    for _, n in ipairs(nights) do if n.day == day then night = n end end
    if not night then
        night = {day=day, windows={}, reports={}}
        nights[#nights+1] = night
        if #nights > Store.MAX_NIGHTS then table.remove(nights, 1) end
    end
    local windows = night.windows
    if #windows == 0 or newWindow then windows[#windows+1] = {openedAt=now, reports={}} end
    newWindow = false
    local pruned = #windows > Store.MAX_WINDOWS
    if #windows > Store.MAX_WINDOWS then table.remove(windows, 1) end
    local reports, count, oldest, oldestTime = windows[#windows].reports, 0, nil, math.huge
    for k, row in pairs(reports) do
        count = count + 1
        if row.receivedAt < oldestTime or (row.receivedAt == oldestTime and (not oldest or k < oldest)) then
            oldest, oldestTime = k, row.receivedAt
        end
    end
    if not reports[key] and count >= Store.MAX_PLAYERS then
        reports[oldest] = nil; pruned = true; Store.count("evicted")
    end
    local bodies = night.reports[key] or {}
    night.reports[key] = bodies
    local latest = bodies[#bodies]
    if not latest or content(latest) ~= content(body) then bodies[#bodies+1] = body end
    reports[key] = {receivedAt=now, report=#bodies}
    if trimReports(night, key) or pruned then pruneNight(night) end
    return true
end
RR.on("ENCOUNTER_END", "Store", function() newWindow = true end)
