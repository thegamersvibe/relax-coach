-- Forever Coach: gear audit. Reads GearQuest Forever's data from the player's own install at runtime,
-- then ranks upgrades by how practical they are for this character (route, faction, reputation, profession, race).
local _, FC = ...
local safe, num, plain = FC.safe, FC.num, FC.plain
local G = {}
FC.Gear = G

G.SLOTS = { -- inventory slot id -> GearQuest slot name
    [1] = "Head", [2] = "Neck", [3] = "Shoulder", [15] = "Back", [5] = "Chest", [9] = "Wrist", [10] = "Hands",
    [6] = "Waist", [7] = "Legs", [8] = "Feet", [11] = "Finger", [12] = "Finger", [13] = "Trinket", [14] = "Trinket",
    [16] = "MainHand", [17] = "SecondaryHand", [18] = "Ranged",
}
G.LABELS = { [1] = "Head", [2] = "Neck", [3] = "Shoulder", [15] = "Back", [5] = "Chest", [9] = "Wrist", [10] = "Hands",
    [6] = "Waist", [7] = "Legs", [8] = "Feet", [11] = "Ring 1", [12] = "Ring 2", [13] = "Trinket 1", [14] = "Trinket 2",
    [16] = "Main Hand", [17] = "Off Hand", [18] = "Ranged" }
local JEWELRY = { [2] = true, [11] = true, [12] = true, [13] = true, [14] = true }
G.LOC_SLOTS = {
    INVTYPE_HEAD = { 1 }, INVTYPE_NECK = { 2 }, INVTYPE_SHOULDER = { 3 }, INVTYPE_CLOAK = { 15 }, INVTYPE_CHEST = { 5 },
    INVTYPE_ROBE = { 5 }, INVTYPE_WRIST = { 9 }, INVTYPE_HAND = { 10 }, INVTYPE_WAIST = { 6 }, INVTYPE_LEGS = { 7 },
    INVTYPE_FEET = { 8 }, INVTYPE_FINGER = { 11, 12 }, INVTYPE_TRINKET = { 13, 14 }, INVTYPE_WEAPON = { 16 },
    INVTYPE_2HWEAPON = { 16 }, INVTYPE_WEAPONMAINHAND = { 16 }, INVTYPE_SHIELD = { 17 }, INVTYPE_HOLDABLE = { 17 },
    INVTYPE_WEAPONOFFHAND = { 17 }, INVTYPE_RANGED = { 18 }, INVTYPE_RANGEDRIGHT = { 18 }, INVTYPE_THROWN = { 18 },
    INVTYPE_RELIC = { 18 },
}
local TAB_SPEC = {
    WARRIOR = { "arms", "fury", "protection" }, PALADIN = { "holy", "protection", "retribution" },
    HUNTER = { "beast_mastery", "marksmanship", "survival" }, ROGUE = { "assassination", "combat", "subtlety" },
    PRIEST = { "discipline", "holy", "shadow" }, SHAMAN = { "elemental", "enhancement", "restoration" },
    MAGE = { "arcane", "fire", "frost" }, WARLOCK = { "affliction", "demonology", "destruction" },
    DRUID = { "balance", "feral", "restoration" },
}
local PREFIX = { WARRIOR = "warrior", PALADIN = "paladin", HUNTER = "hunter", ROGUE = "rogue", PRIEST = "priest",
    SHAMAN = "shaman", MAGE = "mage", WARLOCK = "warlock", DRUID = "druid" }

function G:Status()
    local GQ = _G.GearQuest
    if type(GQ) ~= "table" or type(GQ.Data) ~= "table" then return "missing" end
    return "ok"
end

function G:SpecOptions(class)
    local GQ = _G.GearQuest
    local ids = {}
    if GQ and GQ.Spec and GQ.Spec.GetOptions then
        for _, o in ipairs(safe(GQ.Spec.GetOptions, GQ.Spec, class) or {}) do if type(o) == "table" and type(o.id) == "string" then ids[#ids + 1] = o.id end end
    end
    if #ids == 0 then for _, s in ipairs(TAB_SPEC[class] or {}) do ids[#ids + 1] = s end end
    return ids
end

function G:ValidSpec(class, spec)
    for _, id in ipairs(self:SpecOptions(class)) do if id == spec then return true end end
    return false
end

-- spec, source: manual | gearquest (player's own pick) | talents | default
function G:Spec(snap)
    local c = FC:Char()
    if c.specOverride and self:ValidSpec(snap.class, c.specOverride) then return c.specOverride, "manual" end
    local fromTalents
    local best, bi = 0, nil
    for i, t in ipairs(snap.talents.tabs) do if (t.spent or 0) > best then best, bi = t.spent, i end end
    if bi and TAB_SPEC[snap.class] then fromTalents = TAB_SPEC[snap.class][bi] end
    local GQ = _G.GearQuest
    if GQ and GQ.Spec and GQ.Spec.GetDisplaySpec then
        local preview = GQ.IsPreviewEnabled and safe(GQ.IsPreviewEnabled, GQ)
        local s = not preview and safe(GQ.Spec.GetDisplaySpec, GQ.Spec, snap.class)
        local def = GQ.Spec.GetDefaultSpec and safe(GQ.Spec.GetDefaultSpec, GQ.Spec, snap.class)
        if type(s) == "string" then
            if s ~= def or s == fromTalents then return s, (s == fromTalents and best > 0) and "talents" or "gearquest" end
        end
    end
    if fromTalents and best > 0 then return fromTalents, "talents" end
    return TAB_SPEC[snap.class] and TAB_SPEC[snap.class][1], "default"
end

function G:Tables(class)
    local GQ = _G.GearQuest
    local p = PREFIX[class]
    if not (GQ and GQ.Data and p) then return end
    if GQ.Data.EnsureClassLoaded then safe(GQ.Data.EnsureClassLoaded, GQ.Data, class) end
    local D = GQ.Data
    local facts = D[p .. "ItemFacts"] or (class == "PALADIN" and D.itemFacts)
    return { D[p .. "Picks"], D[p .. "Early1to9"] }, { facts, D[p .. "Early1to9Facts"] }
end

function G.Short(s)
    s = tostring(s or "")
    s = s:gsub("&nbsp;", " "):gsub("&lt;", "<"):gsub("&gt;", ">"):gsub("&amp;", "&"):gsub("&#39;", "'"):gsub("&quot;", '"')
    s = s:gsub("%s*Objective:.*$", ""):gsub("%s+", " ")
    if #s > 170 then s = s:sub(1, 167) .. "..." end
    return s
end

local function fact(factTables, id)
    for _, t in ipairs(factTables) do if type(t) == "table" and t[id] then return t[id] end end
end

local function raceKey(r) return (type(r) == "string" and r or ""):gsub("%s", ""):gsub("Undead", "Scourge") end

-- Hard blocks: the player can't get this item (now). Returns reason or nil, plus soft notes.
function G:Eligibility(f, snap)
    if not f then return "unknown item" end
    local rules = FC:Rules()
    local ins = (f.instructions or "") .. " " .. (f.zone or "")
    local race = raceKey(snap.race)
    for q, races in pairs(rules.raceOnlyQuests or {}) do
        if ins:find("'" .. q .. "'", 1, true) then
            local ok = false
            for _, r in ipairs(races) do if r == race then ok = true end end
            if not ok then return q .. " is a " .. table.concat(races, "/") .. " quest" end
        end
    end
    for _, pat in ipairs(rules.unavailablePatterns or {}) do
        if ins:find(pat, 1, true) then return "not in Forever" end
    end
    for pat, why in pairs(rules.flagPatterns or {}) do
        if race ~= "Skyborne" and ins:find(pat, 1, true) then return why end
    end
    if f.sourceType == "unsourced" then return "no known source" end
    local notes = {}
    if type(f.reqRep) == "table" then
        for _, rr in ipairs(f.reqRep) do
            local name, need = rr[1], rr[2]
            local needIdx
            for i, s in ipairs(FC.STANDING) do if s == need then needIdx = i end end
            local have = snap.reps and snap.reps[name]
            if needIdx and have and have < needIdx then
                return "needs " .. tostring(name) .. " " .. tostring(need) .. " (you're " .. (FC.STANDING[have] or "?") .. ")"
            elseif needIdx and not have then
                return "needs " .. tostring(name) .. " " .. tostring(need)
            end
        end
    end
    if f.sourceType == "profession" and f.profession then
        local sk = snap.skills and snap.skills[f.profession]
        local need = 1
        if type(f.reqSkills) == "table" and type(f.reqSkills[1]) == "table" then need = tonumber(f.reqSkills[1][2]) or 1 end
        if sk and sk.rank < need then notes[#notes + 1] = f.profession .. " " .. need .. " needed" end
    end
    return nil, notes
end

local function rowFields(r) -- generated rows: {id, slot, lo, hi, rank, spec, faction, score}
    if type(r[5]) == "number" then return r[1], r[2], r[3], r[4], r[5], r[6], r[7], r[8] end
    return r[1], r[2], r[3], r[4], r.rank or 4, r[5], r[6], r[8]
end

local function minLevel(f, id)
    local req = f and tonumber(f.reqLevel)
    if req then return req end
    if C_Item and C_Item.GetItemInfo then
        local _, _, _, _, ml = safe(C_Item.GetItemInfo, id)
        if num(ml) then return ml end
    elseif GetItemInfo then
        local _, _, _, _, ml = safe(GetItemInfo, id)
        if num(ml) then return ml end
    end
end

function G:Audit(snap)
    local out = { status = self:Status(), slots = {}, owned = {}, assessed = 0, unknown = 0 }
    local spec, specSource = self:Spec(snap)
    out.spec, out.specSource = spec, specSource
    if out.status ~= "ok" then return out end
    local pickTables, factTables = self:Tables(snap.class)
    if not pickTables or type(pickTables[1]) ~= "table" then out.status = "classdata"; return out end

    local lvl, fac = snap.level, snap.faction and snap.faction:sub(1, 1) == "H" and "Horde" or "Alliance"
    local route = FC.Route:ForPlayer(snap)
    out.routeZones = 0
    for _ in pairs(route) do out.routeZones = out.routeZones + 1 end

    local cands, scoreOf, rows = {}, {}, 0
    for _, list in ipairs(pickTables) do
        if type(list) == "table" then
            for i = 1, #list do
                local r = list[i]
                if type(r) == "table" then
                    local id, slot, lo, hi, rank, rspec, rfac, score = rowFields(r)
                    if rspec == spec and rfac == fac and type(score) == "number" and type(id) == "number"
                        and type(lo) == "number" and type(hi) == "number" then
                        rows = rows + 1
                        if not scoreOf[id] or score > scoreOf[id] then scoreOf[id] = score end
                        if lo <= lvl and lvl <= hi then
                            cands[slot] = cands[slot] or {}
                            if not cands[slot][id] then cands[slot][id] = { id = id, score = score, rank = rank } end
                        end
                    end
                end
            end
        end
    end
    out.rows = rows
    if rows == 0 then out.status = "nodata"; return out end

    -- Evaluate every candidate once: eligibility + practicality.
    local ranked = {}
    for slot, set in pairs(cands) do
        local list = {}
        for id, cnd in pairs(set) do
            local f = fact(factTables, id)
            local req = minLevel(f, id) or 0
            if req <= lvl then
                local blocked, notes = self:Eligibility(f, snap)
                local kind, factor, label, deny = FC.Route:Place(f, snap, route)
                if not blocked and not deny then
                    list[#list + 1] = {
                        id = id, score = cnd.score, rank = cnd.rank, practical = cnd.score * factor, kind = kind, where = label,
                        name = f and f.name, source = f and f.sourceType, how = G.Short(f and f.instructions), notes = notes,
                        loc = FC:EquipLoc(id),
                    }
                end
            end
        end
        table.sort(list, function(a, b)
            if a.practical ~= b.practical then return a.practical > b.practical end
            return a.id < b.id
        end)
        ranked[slot] = list
    end

    local function equippedScore(eq)
        if not eq then return nil end
        local s = scoreOf[eq.id]
        if s then return s, "known" end
        -- Compare:ScoreItemStats uses provider preview/spec context and
        -- different units from generated pipeline scores. It is not a valid
        -- fallback for an upgrade percentage; preserve unknown instead.
        return nil, "unknown"
    end

    local taken = {}
    local function pick(slot, exclude)
        for _, c in ipairs(ranked[slot] or {}) do
            if not (exclude and exclude[c.id]) then return c end
        end
    end

    -- weapons as a set: a two-hander main hand means no off-hand recommendation
    local order = { 1, 2, 3, 15, 5, 9, 10, 6, 7, 8, 16, 17, 18, 11, 12, 13, 14 }
    local mhBest
    for _, inv in ipairs(order) do
        local slot = self.SLOTS[inv]
        local eq = snap.equipped[inv]
        local entry = { slot = slot, label = self.LABELS[inv], inv = inv }
        if eq then
            entry.id = eq.id; entry.score, entry.scoreStatus = equippedScore(eq)
        elseif snap.equipmentState and snap.equipmentState[inv] == "empty" then
            entry.scoreStatus = "empty"
        else
            entry.scoreStatus = "unreadable"
            entry.note = "Equipment could not be read; this slot is not confirmed empty."
        end
        local exclude = {}
        if inv == 12 or inv == 14 then -- second ring/trinket: never the same item as the first
            local other = snap.equipped[inv - 1]
            if other then exclude[other.id] = true end
            if taken[inv - 1] then exclude[taken[inv - 1]] = true end
        elseif inv == 11 or inv == 13 then
            local other = snap.equipped[inv + 1]
            if other then exclude[other.id] = true end
        end
        local best = pick(slot, exclude)
        if inv == 16 then mhBest = best end
        if inv == 17 then
            local mhLoc = (snap.equipped[16] and snap.equipped[16].loc)
            local planned2H = mhBest and mhBest.loc == "INVTYPE_2HWEAPON"
            if planned2H or (not mhBest and mhLoc == "INVTYPE_2HWEAPON") then best = nil; entry.note = "two-hander in main hand" end
        end
        entry.best = best
        if best then taken[inv] = best.id end
        if best and eq and eq.id == best.id then
            entry.gapPct = 0
        elseif best and entry.score then
            entry.gapPct = math.max(0, math.floor((best.score - entry.score) / best.score * 100 + 0.5))
        elseif best and entry.scoreStatus == "empty" then
            entry.gapPct = 100
        end
        if entry.scoreStatus == "known" or entry.scoreStatus == "estimated" then out.assessed = out.assessed + 1
        elseif entry.scoreStatus == "unknown" then out.unknown = out.unknown + 1 end
        -- Core requests genuinely uncached inventory links once. A missing
        -- provider score is not an item-cache miss; do not create a load-event loop.
        entry.jewelry = JEWELRY[inv] or nil
        out.slots[inv] = entry
    end

    -- Upgrades already in the bags (never auto-equipped)
    for _, it in ipairs(snap.bags or {}) do
        local s = scoreOf[it.id]
        local targets = it.loc and self.LOC_SLOTS[it.loc]
        if s and targets then
            local f = fact(factTables, it.id)
            local req = minLevel(f, it.id)
            if req and req <= lvl then
                local worst, worstScore
                for _, inv in ipairs(targets) do
                    local e = out.slots[inv]
                    local have = e and (e.scoreStatus == "empty" and 0 or e.score)
                    if have and (not worstScore or have < worstScore) then worst, worstScore = inv, have end
                end
                if worst and worstScore and s > worstScore * 1.03 and not (out.slots[worst].id == it.id) then
                    local gain = worstScore > 0 and math.floor((s - worstScore) / worstScore * 100 + 0.5) or 100
                    out.owned[#out.owned + 1] = { id = it.id, link = it.link, inv = worst, label = self.LABELS[worst], gain = gain,
                        name = (f and f.name) or (it.link and it.link:match("%[(.-)%]")) }
                end
            end
        end
    end
    table.sort(out.owned, function(a, b) return a.gain > b.gain end)
    return out
end
