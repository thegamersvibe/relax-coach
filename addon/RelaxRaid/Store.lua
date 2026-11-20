-- Account-wide logger preparation, inside the existing app-approved SavedVariables.
-- Version/layout/caps and API evidence: docs/raid/PREP-PROTOCOL.md.
local _, RR = ...
local Store = RR.module("Store")
Store.VERSION, Store.MAX_NIGHTS, Store.MAX_WINDOWS, Store.MAX_PLAYERS = 2, 3, 80, 40
Store.MAX_BYTES = 100000 -- canonical wire-body bytes per character, per night
Store.MAX_NIGHT_BYTES = 4000000 -- sum of all characters' retained body bytes
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
-- Both inputs come from Wire.encode after validation. Equality excludes field 6
-- (playedLevel, the only played-time field) and field 8 (sentAt).
local function content(body)
    local played = 1
    for _ = 1, 5 do played = body:find("\t", played, true) + 1 end
    local reason = body:find("\t", played, true) + 1
    local sent = body:find("\t", reason, true) + 1
    local rest = body:find("\t", sent, true)
    return body:sub(1, played-1) .. body:sub(reason, sent-1) .. body:sub(rest)
end
-- Session-only accounting; never persisted or trusted from a save. Weak keys
-- release evicted nights. All body-list mutations below update this ledger.
local usage = setmetatable({}, {__mode="k"})
local function nightUsage(night)
    local current = usage[night]
    if not current then
        current = {bytes=0, held={}}
        for key, bodies in pairs(night.reports) do
            local own = 0
            for _, body in ipairs(bodies) do own = own + #body end
            current.held[key], current.bytes = own, current.bytes + own
        end
        usage[night] = current
    end
    return current
end
local function forgetReports(night, key)
    local current = usage[night]
    if current then
        current.bytes = current.bytes - (current.held[key] or 0)
        current.held[key] = nil
    end
    night.reports[key] = nil
end
local function dropOldest(night, key)
    local current = nightUsage(night)
    local bodies = night.reports[key]
    local bytes = #table.remove(bodies, 1)
    current.bytes, current.held[key] = current.bytes - bytes, current.held[key] - bytes
    for _, window in ipairs(night.windows) do
        local row = window.reports[key]
        if row then
            if row.report == 1 then window.reports[key] = nil
            else row.report = row.report - 1 end
        end
    end
    return bytes
end
local function trimReports(night, key)
    local bodies, bytes = night.reports[key], 0
    for _, body in ipairs(bodies) do bytes = bytes + #body end
    local over = bytes > Store.MAX_BYTES
    while bytes > Store.MAX_BYTES do
        bytes = bytes - dropOldest(night, key)
        Store.count("budgetEvicted")
    end
    return over
end
-- Compact only live indices, preserving order and the identity of every row.
local function compactReports(night, key, live)
    if not live then forgetReports(night, key); return end
    local bodies, remap, kept = night.reports[key], {}, {}
    local current, removed = nightUsage(night), 0
    for i, body in ipairs(bodies) do
        if live[i] then kept[#kept+1] = body; remap[i] = #kept
        else removed = removed + #body end
    end
    if removed == 0 then return end
    night.reports[key] = kept
    current.bytes, current.held[key] = current.bytes - removed, current.held[key] - removed
    for _, window in ipairs(night.windows) do
        local row = window.reports[key]
        if row then row.report = remap[row.report] end
    end
end
local function pruneNight(night, only)
    local live = {}
    for i = #night.windows, 1, -1 do
        local rows = night.windows[i].reports
        if not next(rows) then table.remove(night.windows, i)
        elseif only then
            if rows[only] then
                live[only] = live[only] or {}; live[only][rows[only].report] = true
            end
        else
            for key, row in pairs(rows) do
                live[key] = live[key] or {}; live[key][row.report] = true
            end
        end
    end
    if only then
        compactReports(night, only, live[only])
    else
        for key in pairs(night.reports) do compactReports(night, key, live[key]) end
    end
end
local function trimNight(night)
    local current = nightUsage(night)
    while current.bytes > Store.MAX_NIGHT_BYTES do
        local largest, most = nil, -1
        for key, own in pairs(current.held) do
            if own > most or (own == most and (not largest or key < largest)) then
                largest, most = key, own
            end
        end
        dropOldest(night, largest)
        -- Only this holder lost references; do not rescan every window row.
        pruneNight(night, largest)
        Store.count("nightBudgetEvicted")
    end
end
-- Bodies use the canonical wire grammar, not expanded Lua field tables.
-- Re-encoding the validated report also gives deterministic budget bytes on reload.
local function cleanReports(source, key, live)
    local P, W = RR.modules.Prep, RR.modules.Wire
    local bodies, indices, bytes = {}, {}, 0
    if not W or not P.array(source, Store.MAX_BYTES) then return bodies, indices end
    -- Only admitted window references spend bytes. Walk newest-first, but never
    -- let an unreferenced suffix evict older live history on reload.
    for i = #source, 1, -1 do
        if live[i] then
            local r = W.decode(source[i])
            if r and P.nameKey(r.name) == key then
                local body = W.encode(r)
                if bytes + #body > Store.MAX_BYTES then break end
                bytes = bytes + #body
                bodies[#bodies+1] = body
                indices[i] = #bodies
            else Store.count("invalidSaved") end
        end
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
            local night, live, remap = {day=n.day, windows={}, reports={}}, {}, {}
            days[n.day] = true
            for _, w in ipairs(n.windows) do
                if type(w) == "table" and P.number(w.openedAt, 0, 9999999999) and type(w.reports) == "table" then
                    local window, seen = {openedAt=w.openedAt, reports={}}, 0
                    for key, row in pairs(w.reports) do
                        seen = seen + 1
                        if seen > Store.MAX_PLAYERS then break end
                        if P.nameKey(key) == key and type(row) == "table"
                            and P.number(row.receivedAt, 0, 9999999999) and P.number(row.report, 1, 100000) then
                            live[key] = live[key] or {}; live[key][row.report] = true
                            window.reports[key] = {receivedAt=row.receivedAt, report=row.report}
                        end
                    end
                    if next(window.reports) then night.windows[#night.windows+1] = window end
                end
            end
            for key, indices in pairs(live) do
                night.reports[key], remap[key] = cleanReports(n.reports[key], key, indices)
            end
            for _, window in ipairs(night.windows) do
                for key, row in pairs(window.reports) do
                    local index = remap[key][row.report]
                    if index then row.report = index
                    else window.reports[key] = nil; Store.count("invalidSaved") end
                end
            end
            pruneNight(night)
            trimNight(night)
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
    local current = nightUsage(night)
    local bodies = night.reports[key] or {}
    night.reports[key] = bodies
    local latest = bodies[#bodies]
    if not latest or content(latest) ~= content(body) then
        bodies[#bodies+1] = body
        current.bytes = current.bytes + #body
        current.held[key] = (current.held[key] or 0) + #body
    elseif latest ~= body then
        local previous = W.decode(latest)
        if previous.playedLevel ~= r.playedLevel then
            r.sentAt = previous.sentAt -- keep the first content timestamp
            body = W.encode(r)
            bodies[#bodies] = body -- all existing references see latest played time
            local delta = #body - #latest
            current.bytes, current.held[key] = current.bytes + delta, current.held[key] + delta
        end
    end
    reports[key] = {receivedAt=now, report=#bodies}
    -- Replaced/cap-evicted references release their bodies BEFORE budgets run.
    if pruned then pruneNight(night) else pruneNight(night, key) end
    if trimReports(night, key) then pruneNight(night, key) end
    trimNight(night)
    return true
end
RR.on("ENCOUNTER_END", "Store", function() newWindow = true end)
