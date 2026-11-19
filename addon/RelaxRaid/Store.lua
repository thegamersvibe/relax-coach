-- Account-wide logger preparation, inside the existing app-approved SavedVariables.
-- Version/layout/caps and API evidence: docs/raid/PREP-PROTOCOL.md.
local _, RR = ...
local Store = RR.module("Store")
Store.VERSION, Store.MAX_NIGHTS, Store.MAX_WINDOWS, Store.MAX_PLAYERS = 1, 3, 80, 40
Store.stats = {}
local newWindow = false
function Store.count(key)
    Store.stats[key] = math.min(999999, (Store.stats[key] or 0) + 1)
end
local readOnly = false
local function newer(t)
    return type(t) == "table" and type(t.version) == "number" and t.version > Store.VERSION
end
function Store.writable()
    local db = RelaxRaidDB
    return not readOnly and not newer(RelaxRaidCharDB) and type(db) == "table" and db.version == 1
        and type(db.prep) == "table" and db.prep.version == Store.VERSION
end
local function cleanNights(source)
    local P, out, days = RR.modules.Prep, {}, {}
    if not P or not P.array(source, Store.MAX_NIGHTS) then return out end
    for _, n in ipairs(source) do
        if type(n) == "table" and P.number(n.day, -1, 999999) and not days[n.day]
            and P.array(n.windows, Store.MAX_WINDOWS) then
            local night = {day=n.day, windows={}}
            days[n.day] = true
            for _, w in ipairs(n.windows) do
                if type(w) == "table" and P.number(w.openedAt, 0, 9999999999) and type(w.reports) == "table" then
                    local window, seen = {openedAt=w.openedAt, reports={}}, 0
                    for key, row in pairs(w.reports) do
                        seen = seen + 1
                        if seen > Store.MAX_PLAYERS then break end
                        if type(row) == "table" and P.number(row.receivedAt, 0, 9999999999) then
                            local r = P.validate(row.report)
                            if r and P.nameKey(key) == key and P.nameKey(r.name) == key then
                                window.reports[key] = {receivedAt=row.receivedAt, report=r}
                            else Store.count("invalidSaved") end
                        end
                    end
                    if next(window.reports) then night.windows[#night.windows+1] = window end
                end
            end
            if #night.windows > 0 then out[#out+1] = night end
        end
    end
    return out
end
function Store.Init()
    local db = RelaxRaidDB
    if newer(db) or (type(db) == "table" and newer(db.prep)) or newer(RelaxRaidCharDB) then
        readOnly = true
        return -- not even normalization: every byte of the newer value is retained
    end
    if type(db) ~= "table" then db = {}; RelaxRaidDB = db end
    db.version = 1
    local nights = type(db.prep) == "table" and db.prep.nights
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
    local now = P.read(GetServerTime)
    if not P.number(now, 0, 9999999999) then return false end
    local nights = RelaxRaidDB.prep.nights
    local day = math.floor((now - 43200)/86400) -- receiver's UTC-noon boundary; not the app's nightId
    local night
    for _, n in ipairs(nights) do if n.day == day then night = n end end
    if not night then
        night = {day=day, windows={}}
        nights[#nights+1] = night
        if #nights > Store.MAX_NIGHTS then table.remove(nights, 1) end
    end
    local windows = night.windows
    if #windows == 0 or newWindow then windows[#windows+1] = {openedAt=now, reports={}} end
    newWindow = false
    if #windows > Store.MAX_WINDOWS then table.remove(windows, 1) end
    local reports, count, oldest, oldestTime = windows[#windows].reports, 0, nil, math.huge
    for k, row in pairs(reports) do
        count = count + 1
        if row.receivedAt < oldestTime or (row.receivedAt == oldestTime and (not oldest or k < oldest)) then
            oldest, oldestTime = k, row.receivedAt
        end
    end
    if not reports[key] and count >= Store.MAX_PLAYERS then reports[oldest] = nil; Store.count("evicted") end
    reports[key] = {receivedAt=now, report=r}
    return true
end
RR.on("ENCOUNTER_END", "Store", function() newWindow = true end)
