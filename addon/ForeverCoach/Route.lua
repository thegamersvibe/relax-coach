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

-- Whole-word, plain-text find (Lua patterns escaped): "Lord Kri" never matches inside "Lord Kristoff", and an
-- apostrophe is part of a word, so "Hakkar" never matches inside "Atal'Hakkar".
local function esc(s) return (s:gsub("[%^%$%(%)%%%.%[%]%*%+%-%?]", "%%%0")) end
local function hasWord(text, name)
    if type(text) ~= "string" or type(name) ~= "string" or name == "" then return false end
    return text:find("%f[%w']" .. esc(name) .. "%f[^%w']") ~= nil
end
R.HasWord = hasWord

-- A zone is a dungeon when the Dungeon Journal knows it, or it is (or contains as whole words) one of the
-- dungeon base names in rules (GearQuest says "Blackrock Spire", the journal "Upper Blackrock Spire").
local function isDungeon(zone)
    if type(zone) ~= "string" or zone == "" then return false end
    local fdj = _G.ForeverDungeonJournal_NS
    if fdj and type(fdj.DB) == "table" and fdj.DB[zone] then return true end
    for name in pairs(FC:Rules().factionDungeons or {}) do if name == zone then return true end end
    for _, name in ipairs((FC:Rules().dungeons or {}).names or {}) do
        if zone == name or hasWord(zone, name) then return true end
    end
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
-- NPCs that drop loot inside a raid zone in the GearQuest data itself: raid mobs the rules don't list (a recipe
-- whose pattern "drops from" one of them is raid loot too).
local learned = setmetatable({}, { __mode = "k" })
function R:LearnRaidNpcs(factTables)
    local rl, out = raidList(), {}
    local zones = {}
    for _, z in ipairs(rl.zones or {}) do zones[z] = true end
    for _, t in ipairs(factTables or {}) do
        if type(t) == "table" then
            local cached = learned[t]
            if not cached then
                cached = {}
                for _, f in pairs(t) do
                    if type(f) == "table" and zones[f.zone] and type(f.npc) == "string" and f.npc ~= "" then cached[f.npc] = f.zone end
                end
                learned[t] = cached
            end
            for npc, z in pairs(cached) do out[npc] = z end
        end
    end
    self._raidNpcs = out
end
-- The names a source text says something drops from, lower case: "Drops from A." and "The pattern drops from A in
-- Uldaman, from B, C, and D in Zul'Farrak." give A, B, C and D (places and brackets removed).
local function dropNames(text)
    local out = {}
    for seg in text:lower():gmatch("drops from ([^%.|]*)") do
        seg = seg:gsub("%b()", ""):gsub(";", ","):gsub(" and ", ","):gsub(" or ", ",")
        for part in (seg .. ","):gmatch("([^,]*),") do
            part = part:gsub("^%s*from%s+", ""):gsub("%s+in%s+.*$", ""):gsub("^%s+", ""):gsub("%s+$", "")
            if part ~= "" then out[part] = true end
        end
    end
    return out
end
R.DropNames = dropNames
-- The raid (or "World bosses") an item comes from, or nil. Fails closed on places: a raid zone or raid mob named
-- anywhere in the record's zone, NPC or text (second sources included), a raid quest it rewards, or a sourceType of
-- "raid". A boss, world boss or raid NPC counts only where it is the source: the record's NPC, or a name in a
-- "drops from" list (so "Avatar of Hakkar" and the quest "The God Hakkar" are not Hakkar).
function R:Raid(zone, f)
    local rl = raidList()
    for _, z in ipairs(rl.zones or {}) do if z == zone then return zone end end
    if type(f) ~= "table" then return nil end
    local text = table.concat({ type(f.instructions) == "string" and f.instructions or "",
        type(f.npc) == "string" and f.npc or "", type(f.zone) == "string" and f.zone or "" }, " | ")
    for _, z in ipairs(rl.zones or {}) do if hasWord(text, z) then return z end end
    local drops = dropNames(text)
    local npc = type(f.npc) == "string" and f.npc:lower() or nil
    local function source(name) name = name:lower(); return drops[name] or npc == name end
    for _, boss in ipairs(rl.worldBosses or {}) do
        if source(boss) then return rl.worldBossName or "World bosses" end
    end
    for boss, raid in pairs(rl.bosses or {}) do if source(boss) then return raid end end
    for mob, raid in pairs(rl.mobs or {}) do if hasWord(text, mob) then return raid end end
    for name, raid in pairs(self._raidNpcs or {}) do if source(name) then return raid end end
    for quest, raid in pairs(rl.quests or {}) do
        if text:find("quest '" .. quest .. "'", 1, true) then return raid end
    end
    if f.sourceType == "raid" then return (type(zone) == "string" and zone ~= "" and zone) or "unknown raid" end
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
    -- A record with no zone (and not crafted or a random world drop) is never a pick, whatever its seller:
    -- nobody can travel to "location unknown".
    if not zone or zone == "" then
        return "unknown", 0, "location unknown", "no zone: nowhere to travel"
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
