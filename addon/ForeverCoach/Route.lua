-- Forever Coach: where things are relative to the player's leveling route (Journey guide index in rules.routes).
local _, FC = ...
local R = {}
FC.Route = R

local function upper(s) return type(s) == "string" and s:upper():gsub("%s", "") or "" end

-- Route zones for this character: zone -> {from, to} (first time the route visits it)
function R:ForPlayer(snap)
    local rules = FC:Rules()
    local race = upper(snap.race):gsub("UNDEAD", "SCOURGE")
    local out = {}
    for _, r in ipairs(rules.routes or {}) do
        local facOk = r.faction == snap.faction or r.faction == "Both"
        local raceOk = true
        if r.races then
            raceOk = false
            for _, x in ipairs(r.races) do if x == race then raceOk = true end end
        end
        if facOk and raceOk then
            for _, z in ipairs(r.zones or {}) do
                local e = out[z]
                if not e then out[z] = { from = r.from, to = r.to }
                else e.from = math.min(e.from, r.from); e.to = math.max(e.to, r.to) end
            end
        end
    end
    return out
end

function R:Parent(zone)
    if not zone then return nil end
    local P = FC:Rules().zoneParents or {}
    return P[zone] or zone
end

function R:Denied(zone, faction)
    local deny = (FC:Rules().factionZoneDeny or {})[faction or ""]
    if not deny or not zone then return false end
    for _, z in ipairs(deny) do if z == zone then return true end end
    return false
end

local function isDungeon(zone)
    local fdj = _G.ForeverDungeonJournal_NS
    if fdj and type(fdj.DB) == "table" and fdj.DB[zone] then return true end
    for name in pairs(FC:Rules().factionDungeons or {}) do if name == zone then return true end end
    return false
end
R.IsDungeon = isDungeon

-- quest name -> dungeon name, built once from the player's DungeonJournal
function R:QuestDungeon(q)
    if not self._qd then
        self._qd = {}
        local fdj = _G.ForeverDungeonJournal_NS
        if fdj and type(fdj.DB) == "table" then
            for dname, d in pairs(fdj.DB) do
                for _, dq in ipairs(type(d) == "table" and type(d.quests) == "table" and d.quests or {}) do
                    if type(dq) == "table" and type(dq.name) == "string" then self._qd[dq.name] = dname end
                end
            end
        end
    end
    return self._qd[q]
end

local function raidList() return (FC:Rules().raids or {}) end
-- The raid an item comes from (instance zone, raid boss, or raid quest), or nil.
function R:Raid(zone, f)
    local rl = raidList()
    for _, z in ipairs(rl.zones or {}) do if z == zone then return zone end end
    if type(f) ~= "table" then return nil end
    local ins = type(f.instructions) == "string" and f.instructions or ""
    for boss, raid in pairs(rl.bosses or {}) do
        if f.npc == boss or ins:find("Drops from " .. boss, 1, true) then return raid end
    end
    for quest, raid in pairs(rl.quests or {}) do
        if ins:find("quest '" .. quest .. "'", 1, true) then return raid end
    end
    return nil
end
function R:RaidOpen(raid)
    for _, z in ipairs(raidList().open or {}) do if z == raid then return true end end
    return false
end

-- Classify a GearQuest item fact for this player.
-- Returns kind, factor (0..1 practicality multiplier), label (human text), blockedReason (or nil)
function R:Place(f, snap, route)
    local P = FC:Rules().practicality or {}
    local zone = f and f.zone
    local generic = f and f.sourceType == "world_drop" and (not f.npc or f.npc == "")
        and type(f.instructions) == "string" and f.instructions:sub(1, 10) == "World drop"
    if f and f.sourceType == "profession" and (not zone or zone == "") then
        local sk = f.profession and snap.skills and snap.skills[f.profession]
        if sk then return "craft", 1, "you can craft it", nil end
        return "craft", P.craftNoProfession or 0.8, "crafted (buy it or ask a crafter)", nil
    end
    if generic then return "world", P.worldDrop or 0.55, "random world drop", nil end
    if not zone or zone == "" then
        return "unknown", P.unknownZone or 0.7, "location unknown", nil
    end
    local parent = self:Parent(zone)
    if not (f.zoneOpen or generic or f.sourceType == "rare_npc") and (self:Denied(zone, snap.faction) or self:Denied(parent, snap.faction)) then
        return "denied", 0, zone, "in a " .. (snap.faction == "Alliance" and "Horde" or "Alliance") .. " area"
    end
    local here = snap.zone and snap.zone.parent
    local GA = FC:Rules().gearAdvice or {}
    local maxLevel = snap.level >= (GA.maxLevel or 60)
    local raid = self:Raid(zone, f)
    if raid then return "raid", P.dungeon or 0.88, "raid: " .. raid, nil end
    local dungeon = isDungeon(zone)
    if not dungeon and type(f.instructions) == "string" then
        -- quest rewards from dungeon quests name the dungeon only in the text
        local q = f.instructions:match("quest '([^']+)'")
        local dname = q and self:QuestDungeon(q)
        if dname then zone, dungeon, parent = dname, true, self:Parent(dname) end
    end
    if here and (parent == here or zone == here) and not dungeon then
        return "here", P.here or 1, "here (" .. zone .. ")", nil
    end
    local rz = route[parent] or route[zone]
    if dungeon then
        local where = parent ~= zone and (" in " .. parent) or ""
        local fdj = _G.ForeverDungeonJournal_NS
        local d = fdj and type(fdj.DB) == "table" and fdj.DB[zone]
        local lo = d and tonumber(tostring(d.level or ""):match("(%d+)"))
        if lo and snap.level < lo - 3 then
            return "denied", 0, zone, "dungeon is for level " .. lo .. "+"
        end
        if route[parent] or maxLevel then return "dungeon", P.dungeon or 0.88, "dungeon: " .. zone .. where, nil end
        return "offroute", P.offRoute or 0.65, "dungeon: " .. zone .. where .. " (off your route)", nil
    end
    -- At max level route wording and route factors mean nothing: a zone is just a place.
    if maxLevel then return "zone", P.here or 1, zone, nil end
    if rz then
        if rz.to < snap.level - (GA.behindLevels or 6) then
            return "behind", P.behind or 0.5, zone .. " (levels " .. rz.from .. "-" .. rz.to .. ", behind you)", nil
        end
        if rz.from > snap.level then
            return "route", P.routeSoon or 0.97, zone .. " (your route, level " .. rz.from .. ")", nil
        end
        return "routePast", P.routePast or 0.9, zone .. " (your route, levels " .. rz.from .. "-" .. rz.to .. ")", nil
    end
    return "offroute", P.offRoute or 0.65, zone .. " (off your route)", nil
end
