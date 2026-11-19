-- Preparation only; no score or equipment actions. API evidence: docs/raid/PREP-PROTOCOL.md.
local _, RR = ...
local Prep = RR.module("Prep")

-- A missing/throwing secret detector cannot certify that a value is readable.
function Prep.plain(v)
    local ok, secret = pcall(issecretvalue, v)
    return ok and secret == false
end
function Prep.clock()
    local ok, v = pcall(GetTime)
    if ok and Prep.plain(v) and type(v) == "number" and v >= 0 and v <= 9999999999 then return v end
end
function Prep.number(v, low, high)
    return Prep.plain(v) and type(v) == "number" and v >= low and v <= high and v % 1 == 0
end
function Prep.text(v, max)
    return Prep.plain(v) and type(v) == "string" and #v > 0 and #v <= max and not v:find("[%c|]")
end
function Prep.read(fn, ...)
    local ok, v = pcall(fn, ...)
    if ok and Prep.plain(v) then return v end
end
function Prep.identity()
    local ok, name, realm = pcall(UnitFullName, "player")
    if not ok or not Prep.plain(realm) then return "unreadable" end
    if realm == nil then realm = Prep.read(GetNormalizedRealmName) end
    if Prep.text(name, 64) and Prep.text(realm, 32) and Prep.nameKey(realm) then
        local full = name .. "-" .. realm
        if Prep.nameKey(full) then return full end
    end
    return "unreadable"
end
-- Class/subclass IDs are client enums, not a guessed item catalogue.
-- ItemConstantsDocumentation.lua:276,300-323. No stacking or scoring inference.
Prep.categories = {[0]="generic", [1]="potion", [2]="elixir", [3]="flask", [4]="scroll",
    [5]="food", [6]="enhancement", [7]="bandage", [8]="other", [9]="vantus",
    [10]="utility", [11]="combat", [12]="relic", [13]="draught", [14]="device", [15]="temporary"}
function Prep.consumables()
    local out, byID = {}, {}
    for bag = 0, 4 do -- equipped bags only; no bank (same range as Coach:Bags)
        local n = Prep.read(C_Container and C_Container.GetContainerNumSlots, bag)
        if not Prep.number(n, bag == 0 and 1 or 0, 100) then return "unreadable" end
        for slot = 1, n do
            local ok, info = pcall(C_Container.GetContainerItemInfo, bag, slot)
            if not ok or not Prep.plain(info) then return "unreadable" end
            if info ~= nil then
                if type(info) ~= "table" or not Prep.number(info.itemID, 1, 999999999)
                    or not Prep.number(info.stackCount, 1, 999999) then return "unreadable" end
                local good, _, _, _, _, _, _, _, _, _, _, _, cls, sub = pcall(C_Item and C_Item.GetItemInfo, info.itemID)
                if not good or not Prep.number(cls, 0, 99) or not Prep.number(sub, 0, 99) then return "unreadable" end
                if cls == 0 then
                    local category = Prep.categories[sub]
                    if not category then return "unreadable" end
                    local row = byID[info.itemID]
                    if not row then
                        if #out >= 32 then return "unreadable" end
                        row = {category=category, itemID=info.itemID, count=0}
                        out[#out + 1], byID[info.itemID] = row, row
                    end
                    row.count = row.count + info.stackCount
                    if row.count > 999999 then return "unreadable" end
                end
            end
        end
    end
    table.sort(out, function(a, b) return a.itemID < b.itemID end)
    return out
end
function Prep.gear()
    local coach = _G.ForeverCoach
    if type(coach) ~= "table" or type(coach.Gear) ~= "table"
        or type(coach.Snapshot) ~= "function" or type(coach.Gear.Audit) ~= "function" then return "not checked" end
    -- Read Coach's public snapshot/audit at use time, so load order is irrelevant.
    -- Never read GearQuest, Journey or the shared item database directly.
    local ok, result, notScored = pcall(function()
        local snap = coach:Snapshot()
        local audit = coach.Gear:Audit(snap)
        if not Prep.plain(audit) or type(audit) ~= "table" then return "unreadable" end
        if not Prep.text(audit.status, 16) then return "unreadable" end
        if audit.status == "missing" or audit.status == "layout" or audit.status == "classdata" or audit.status == "nodata" then return "not checked" end
        if audit.status ~= "ok" then return "unreadable" end
        if type(snap) ~= "table" or type(snap.baginfo) ~= "table" or not Prep.plain(snap.baginfo.known)
            or snap.baginfo.known ~= true then return "unreadable" end
        if not Prep.plain(audit.owned) or type(audit.owned) ~= "table" or #audit.owned > 19
            or type(audit.slots) ~= "table" then return "unreadable" end
        local unscored = {}
        for inv, slot in pairs(audit.slots) do
            if not Prep.number(inv, 1, 19) or not Prep.plain(slot) or type(slot) ~= "table" or not Prep.plain(slot.scoreStatus)
                or slot.scoreStatus == "unreadable" then return "unreadable" end
            if slot.scoreStatus == "unknown" then unscored[#unscored+1] = inv end
        end
        table.sort(unscored)
        local out = {}
        for i = 1, #audit.owned do
            local row = audit.owned[i]
            if not Prep.plain(row) or type(row) ~= "table" or not Prep.number(row.inv, 1, 19)
                or not Prep.number(row.id, 1, 999999999) then return "unreadable" end
            local slot = audit.slots[row.inv]
            if not Prep.plain(slot) or type(slot) ~= "table" then return "unreadable" end
            local worn = slot.id
            if Prep.plain(slot.scoreStatus) and slot.scoreStatus == "empty" then worn = 0 end
            if not Prep.number(worn, 0, 999999999) then return "unreadable" end
            out[#out + 1] = {slot=row.inv, wornID=worn, bagID=row.id}
        end
        return out, unscored
    end)
    if not ok then return "unreadable" end
    return result, notScored
end
-- One validator for wire input, saved input and optional-provider output.
Prep.reasons = {join=true, ready=true, encounter=true}
local classes = {WARRIOR=true, PALADIN=true, HUNTER=true, ROGUE=true, PRIEST=true,
    SHAMAN=true, MAGE=true, WARLOCK=true, DRUID=true, unreadable=true}
function Prep.nameKey(name)
    if not Prep.text(name, 96) or name == "unreadable" or name:find("[^%a%d '%-\128-\255]")
        or name:find("^ ") or name:find(" $") or name:find("  ") then return nil end
    return string.lower(name) -- retain the WHOLE name; never split surnames/realm labels
end
-- Only a bare name gets the local normalized realm; never strip an explicit suffix.
function Prep.fullName(name)
    if not Prep.nameKey(name) then return nil end
    if not name:find("-", 1, true) then
        local realm = Prep.read(GetNormalizedRealmName)
        if not Prep.text(realm, 32) or not Prep.nameKey(realm) then return nil end
        name = name .. "-" .. realm
    end
    if Prep.nameKey(name) then return name end
end
function Prep.array(t, cap)
    if not Prep.plain(t) or type(t) ~= "table" then return nil end
    local n = 0
    for k in pairs(t) do
        n = n + 1
        if n > cap or not Prep.number(k, 1, cap) then return nil end
    end
    for i = 1, n do if t[i] == nil then return nil end end
    return n
end
function Prep.validate(r)
    if not Prep.plain(r) or type(r) ~= "table" or not Prep.number(r.protocol, 1, 1)
        or not Prep.text(r.addonVersion, 24) or not Prep.nameKey(r.name)
        or not Prep.text(r.class, 16) or not classes[r.class]
        or not Prep.plain(r.level) or not (r.level == "unreadable" or Prep.number(r.level, 1, 60))
        or not Prep.plain(r.playedLevel) or not (r.playedLevel == "unreadable" or Prep.number(r.playedLevel, 0, 999999999))
        or not Prep.text(r.reason, 16) or not Prep.reasons[r.reason]
        or not Prep.number(r.sentAt, 0, 9999999999) then return nil end
    local name = Prep.fullName(r.name)
    if not name then return nil end
    local copy = {protocol=1, addonVersion=r.addonVersion, name=name, class=r.class,
        level=r.level, playedLevel=r.playedLevel, reason=r.reason, sentAt=r.sentAt}
    if not Prep.plain(r.consumables) or not Prep.plain(r.gear) then return nil end
    if r.consumables == "unreadable" then copy.consumables = "unreadable"
    else
        local n = Prep.array(r.consumables, 32)
        if not n then return nil end
        local seen, cats = {}, {}
        for _, category in pairs(Prep.categories) do cats[category] = true end
        copy.consumables = {}
        for i = 1, n do
            local row = r.consumables[i]
            if not Prep.plain(row) or type(row) ~= "table" or not Prep.text(row.category, 16)
                or not cats[row.category] or not Prep.number(row.itemID, 1, 999999999)
                or not Prep.number(row.count, 1, 999999) or seen[row.itemID] then return nil end
            seen[row.itemID] = true
            copy.consumables[i] = {category=row.category, itemID=row.itemID, count=row.count}
        end
    end
    if not Prep.plain(r.notScored) then return nil end
    if r.gear == "not checked" or r.gear == "unreadable" then
        if r.notScored ~= nil then return nil end
        copy.gear = r.gear
    else
        local count = Prep.array(r.notScored, 19)
        if not count then return nil end
        local seen = {}
        copy.notScored = {}
        for i = 1, count do
            local slot = r.notScored[i]
            if not Prep.number(slot, 1, 19) or seen[slot] then return nil end
            seen[slot], copy.notScored[i] = true, slot
        end
        local n = Prep.array(r.gear, 19)
        if not n then return nil end
        copy.gear = {}
        for i = 1, n do
            local row = r.gear[i]
            if not Prep.plain(row) or type(row) ~= "table" or not Prep.number(row.slot, 1, 19)
                or not Prep.number(row.wornID, 0, 999999999) or not Prep.number(row.bagID, 1, 999999999)
                then return nil end
            copy.gear[i] = {slot=row.slot, wornID=row.wornID, bagID=row.bagID}
        end
    end
    return copy
end
local played, playedAt, playedLevel
function Prep.requestPlayed()
    played, playedAt, playedLevel = nil, nil, nil
    pcall(RequestTimePlayed)
end
local playedJoined = false
local function playedGroup()
    if not RR.loaded then return end
    local inRaid = Prep.read(IsInRaid)
    if inRaid == true and not playedJoined then Prep.requestPlayed() end
    -- Unreadable membership is not evidence of leaving and must not rearm /played.
    if type(inRaid) == "boolean" then playedJoined = inRaid end
end
RR.on("GROUP_ROSTER_UPDATE", "Prep", playedGroup)
RR.on("PLAYER_ENTERING_WORLD", "Prep", playedGroup)
RR.on("TIME_PLAYED_MSG", "Prep", function(_, _, seconds)
    local level = Prep.read(UnitLevel, "player")
    played, playedAt, playedLevel = nil, nil, nil
    if Prep.number(seconds, 0, 999999999) and Prep.number(level, 1, 60) then
        played, playedAt, playedLevel = seconds, Prep.clock(), level
    end
end)
RR.on("PLAYER_LEVEL_UP", "Prep", function() played, playedAt, playedLevel = nil, nil, nil end)
function Prep.build(reason)
    local _, _, class = pcall(UnitClass, "player")
    local level = Prep.read(UnitLevel, "player")
    local version = Prep.read(C_AddOns and C_AddOns.GetAddOnMetadata, RR.name, "Version")
    local clock, elapsedClock = Prep.read(GetServerTime), Prep.clock()
    local gear, notScored = Prep.gear()
    return {protocol = 1, addonVersion = Prep.text(version, 24) and version or "unreadable",
        name = Prep.identity(), class = Prep.text(class, 16) and class or "unreadable",
        level = Prep.number(level, 1, 60) and level or "unreadable",
        playedLevel = (level == 60 and playedLevel == 60 and played and playedAt and elapsedClock and elapsedClock >= playedAt)
            and math.min(999999999, played + math.floor(elapsedClock - playedAt)) or "unreadable", consumables = Prep.consumables(), gear = gear, notScored = notScored,
        reason = reason, sentAt = Prep.number(clock, 0, 9999999999) and clock or "unreadable"}
end
