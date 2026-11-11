-- Relax Coach: core state, snapshot of the character, saved data.
local ADDON, FC = ...
FC = FC or {}
_G.ForeverCoach = FC
FC.VERSION = "1.5.0"
FC.SCHEMA = 1

local function safe(f, ...)
    if type(f) ~= "function" then return nil end
    local ok, a, b, c, d, e, g, h, i, j, k = pcall(f, ...)
    if ok then return a, b, c, d, e, g, h, i, j, k end
end
FC.safe = safe

-- Forever hides some values from addons ("secret values"); never do math or comparisons on one.
local function num(v)
    -- Ask issecretvalue first: even `v == nil` is a comparison on a possibly secret value.
    if (issecretvalue and issecretvalue(v)) or type(v) ~= "number" then return nil end
    return v
end
local function plain(v)
    if issecretvalue and issecretvalue(v) then return nil end
    return v
end
FC.num, FC.plain = num, plain

local EMPTY_RULES = { thresholds = {}, raceOnlyQuests = {}, spellMilestones = {}, procSpells = {}, routes = {},
    zoneParents = {}, factionZoneDeny = {}, talentBuilds = {}, practicality = {} }
function FC:Rules()
    local r = _G.ForeverCoach_Rules
    if type(r) ~= "table" or r.schema ~= 1 then return EMPTY_RULES end
    return r
end

function FC:Print(msg) DEFAULT_CHAT_FRAME:AddMessage("|cff4fc3f7Relax Coach|r: " .. tostring(msg)) end

function FC:CharKey()
    local name = UnitName("player") or "?"
    local realm = (GetRealmName and GetRealmName()) or "?"
    return name .. "-" .. realm
end

function FC:Char()
    ForeverCoachDB = ForeverCoachDB or {}
    local db = ForeverCoachDB
    db.schema = FC.SCHEMA
    db.addonVersion = FC.VERSION
    db.chars = db.chars or {}
    local key = self:CharKey()
    db.chars[key] = db.chars[key] or { sessions = {} }
    return db.chars[key]
end

-- true / false / nil (nil = the client gave no usable answer)
local function knownBy(f, id)
    if type(f) ~= "function" then return nil end
    local ok, r = pcall(f, id)
    if not ok then return nil end
    return plain(r)
end
local function knowsSpellID(id)
    -- Same API order as before, without building a table of functions per call.
    local book = C_SpellBook
    local a = knownBy(IsPlayerSpell, id); if a == true then return true end
    local b = knownBy(IsSpellKnown, id); if b == true then return true end
    local c = knownBy(book and book.IsSpellKnown, id); if c == true then return true end
    local d = knownBy(book and book.IsSpellKnownOrInSpellBook, id); if d == true then return true end
    if a == false or b == false or c == false or d == false then return false end
    return nil
end

-- The player's spellbook as { [lower-case name] = highest rank seen (0 = no rank text) }, or
-- false when the client gives no usable answer. One pass over the book; no protected calls.
-- A bounded loop: a broken offset can't make it spin. A book that is only partly readable (a
-- tab whose offset/count is secret or missing, a slot without a readable name) is not used at
-- all: a spell missing from half a book is unknown, not "not learned".
local BOOK_LIMIT = 1024
local function scanSpellBook()
    local names, count, partial = {}, 0, false
    local function add(name, sub)
        if issecretvalue and (issecretvalue(name) or issecretvalue(sub)) then partial = true; return end
        if type(name) ~= "string" or name == "" then partial = true; return end
        local key = string.lower(name)
        local rank = type(sub) == "string" and tonumber(string.match(sub, "(%d+)")) or 0
        if not names[key] or rank > names[key] then names[key] = rank end
        count = count + 1
    end
    local function last(offset, n)
        offset, n = num(offset), num(n)
        if offset and n and offset >= 0 and n >= 0 then return offset + n end
        partial = true
        return 0
    end
    local total = 0
    if GetNumSpellTabs and GetSpellTabInfo and GetSpellBookItemName then
        -- Classic / Forever: tabs carry an offset and a count; skip spells not learned yet.
        local tabs = num(GetNumSpellTabs())
        if not tabs then return false end
        for t = 1, math.min(tabs, 32) do
            local _, _, offset, n = GetSpellTabInfo(t)
            total = math.max(total, last(offset, n))
        end
        local bookType = BOOKTYPE_SPELL or "spell"
        for i = 1, math.min(total, BOOK_LIMIT) do
            local kind = GetSpellBookItemInfo and GetSpellBookItemInfo(i, bookType)
            if kind ~= "FUTURESPELL" then add(GetSpellBookItemName(i, bookType)) end
        end
    elseif C_SpellBook and C_SpellBook.GetNumSpellBookSkillLines and C_SpellBook.GetSpellBookSkillLineInfo
        and C_SpellBook.GetSpellBookItemName then
        -- Newer clients: the same walk through C_SpellBook.
        local bank = Enum and Enum.SpellBookSpellBank and Enum.SpellBookSpellBank.Player or 0
        local future = Enum and Enum.SpellBookItemType and Enum.SpellBookItemType.FutureSpell
        local lines = num(C_SpellBook.GetNumSpellBookSkillLines())
        if not lines then return false end
        for t = 1, math.min(lines, 32) do
            local info = C_SpellBook.GetSpellBookSkillLineInfo(t)
            if type(info) == "table" then total = math.max(total, last(info.itemIndexOffset, info.numSpellBookItems)) else partial = true end
        end
        for i = 1, math.min(total, BOOK_LIMIT) do
            local info = C_SpellBook.GetSpellBookItemInfo and C_SpellBook.GetSpellBookItemInfo(i, bank)
            if not (future and type(info) == "table" and info.itemType == future) then add(C_SpellBook.GetSpellBookItemName(i, bank)) end
        end
    else
        return false
    end
    if partial or total > BOOK_LIMIT then return false end
    return count > 0 and names or false
end

-- Cached for a second. fresh = true (snapshots, SPELLS_CHANGED) always reads the current book,
-- so a spell learned a moment ago is never missed. A failed scan (false) is reused for that
-- second even when fresh, so a client without a usable book is not rescanned on every event.
function FC:SpellBook(fresh)
    local now = GetTime and num(GetTime())
    local cached, at = self.spellBookCache, self.spellBookAt
    if cached ~= nil and now and at then
        local age = now - at
        if age >= 0 and age < 1 and (cached == false or not fresh) then return cached end
    end
    local ok, res = pcall(scanSpellBook)
    if not ok then res = false end
    self.spellBookCache, self.spellBookAt = res, now
    return res
end

-- true / false / nil (nil = the client gave no usable answer).
-- The ID is asked first. Classic-style clients answer false for the rank-1 ID once a higher rank
-- replaced it, so a false from the ID is checked by name in the spellbook (book: from
-- FC:SpellBook, false = no usable book). label is the rules name; "Name (Rank N)" needs rank N.
-- Forever uses custom IDs for some spells, so the name is the truth when the ID says no.
local function knowsByName(self, id, book, label)
    if not book then return nil end
    local base, need = nil, nil
    if type(label) == "string" then
        base, need = string.match(label, "^(.-) %(Rank (%d+)%)$")
        need = tonumber(need)
        base = base or label
    end
    local names = {}
    local own = plain(self:SpellName(id))
    if type(own) == "string" then names[#names + 1] = own end
    if base then names[#names + 1] = base end
    if #names == 0 then return nil end
    local unsure = false
    for _, name in ipairs(names) do
        local rank = book[string.lower(name)]
        if rank then
            if not need or rank >= need then return true end
            if rank == 0 then unsure = true end
        end
    end
    if unsure then return nil end
    return false
end
-- A failing name lookup leaves only that spell unanswered (nil), never the whole snapshot.
local function knowsByNameSafe(self, id, book, label)
    local ok, r = pcall(knowsByName, self, id, book, label)
    if ok then return r end
    return nil
end
function FC:KnowsSpell(id, book, label)
    if knowsSpellID(id) == true then return true end
    if book == nil then book = self:SpellBook() end
    return knowsByNameSafe(self, id, book, label)
end

function FC:SpellName(id)
    if C_Spell and C_Spell.GetSpellInfo then
        local info = safe(C_Spell.GetSpellInfo, id)
        if type(info) == "table" and type(info.name) == "string" then return info.name end
    end
    if GetSpellInfo then
        local n = safe(GetSpellInfo, id)
        if type(n) == "string" then return n end
    end
end

local function itemIdFromLink(link)
    return type(link) == "string" and tonumber(link:match("item:(%d+)")) or nil
end
FC.itemIdFromLink = itemIdFromLink

function FC:EquipLoc(id)
    if not id then return nil end
    if C_Item and C_Item.GetItemInfoInstant then
        local _, _, _, loc = safe(C_Item.GetItemInfoInstant, id)
        if type(loc) == "string" then return loc end
    end
    if GetItemInfoInstant then
        local _, _, _, loc = safe(GetItemInfoInstant, id)
        if type(loc) == "string" then return loc end
    end
end

function FC:ReadTalents()
    local t = { tabs = {}, unspent = nil, ranks = {}, detailed = false }
    if UnitCharacterPoints then t.unspent = num(safe(UnitCharacterPoints, "player")) end
    local n = (GetNumTalentTabs and num(safe(GetNumTalentTabs))) or 0
    for tab = 1, n do
        local a, b, c, d = safe(GetTalentTabInfo, tab)
        local spent
        for _, v in ipairs({ c or false, d or false, b or false }) do if num(v) then spent = v break end end
        local count = (GetNumTalents and num(safe(GetNumTalents, tab))) or 0
        local sum = 0
        for i = 1, count do
            local name, _, _, _, rank = safe(GetTalentInfo, tab, i)
            if type(name) == "string" and num(rank) then
                t.ranks[name] = (t.ranks[name] or 0) + rank; sum = sum + rank; t.detailed = true
            end
        end
        if spent == nil then spent = sum end
        t.tabs[tab] = { name = type(a) == "string" and a or ("Tree " .. tab), spent = spent or 0 }
    end
    return t
end

function FC:Skills()
    local out = {}
    local n = (GetNumSkillLines and num(safe(GetNumSkillLines))) or 0
    for i = 1, n do
        local name, header, _, rank, _, _, maxRank = safe(GetSkillLineInfo, i)
        if type(name) == "string" and not header and num(rank) then out[name] = { rank = rank, max = num(maxRank) } end
    end
    return out
end

FC.STANDING = { "Hated", "Hostile", "Unfriendly", "Neutral", "Friendly", "Honored", "Revered", "Exalted" }
function FC:Reputations()
    local out = {}
    local n = (GetNumFactions and num(safe(GetNumFactions))) or 0
    for i = 1, n do
        local name, standing
        if C_Reputation and C_Reputation.GetFactionDataByIndex then
            local d = safe(C_Reputation.GetFactionDataByIndex, i)
            if type(d) == "table" then name, standing = d.name, num(d.reaction) end
        end
        if not name and GetFactionInfo then
            local nm, _, st = safe(GetFactionInfo, i)
            name, standing = nm, num(st)
        end
        if type(name) == "string" and standing then out[name] = standing end
    end
    return out
end

function FC:Equipped()
    local out, states = {}, {}
    for slot = 1, 19 do
        local link = plain(safe(GetInventoryItemLink, "player", slot))
        -- A failed/secret read is not the same as a successful empty slot.
        local readable, inventoryID = false, nil
        if type(GetInventoryItemID) == "function" then
            local ok, value = pcall(GetInventoryItemID, "player", slot)
            if ok and not (issecretvalue and issecretvalue(value)) then
                if value == nil or value == 0 then
                    readable = true
                elseif num(value) and value > 0 and value % 1 == 0 then
                    readable, inventoryID = true, value
                end
            end
        end
        -- Identity is available before the cached hyperlink/name.
        local id = inventoryID or itemIdFromLink(link)
        states[slot] = id and "equipped" or (readable and "empty" or "unreadable")
        if id then
            if itemIdFromLink(link) ~= id then link = nil end
            out[slot] = { id = id, link = link, loc = self:EquipLoc(id) }
            if not link and C_Item and C_Item.RequestLoadItemDataByID then
                self._itemRequests = self._itemRequests or {}
                if not self._itemRequests[id] then
                    self._itemRequests[id] = true
                    safe(C_Item.RequestLoadItemDataByID, id)
                end
            end
        end
    end
    return out, states
end

function FC:Bags()
    local items, info = {}, { free = 0, slots = 0, bagSlotsEmpty = 0, known = false }
    local C = C_Container
    for bag = 0, 4 do
        local n = (C and C.GetContainerNumSlots and num(safe(C.GetContainerNumSlots, bag))) or (GetContainerNumSlots and num(safe(GetContainerNumSlots, bag))) or 0
        if bag > 0 and n == 0 then info.bagSlotsEmpty = info.bagSlotsEmpty + 1 end
        info.slots = info.slots + n
        local free = (C and C.GetContainerNumFreeSlots and num((safe(C.GetContainerNumFreeSlots, bag)))) or (GetContainerNumFreeSlots and num((safe(GetContainerNumFreeSlots, bag))))
        if free then info.free = info.free + free; info.known = true end
        for s = 1, n do
            local link = (C and C.GetContainerItemLink and plain(safe(C.GetContainerItemLink, bag, s))) or (GetContainerItemLink and plain(safe(GetContainerItemLink, bag, s)))
            local id = itemIdFromLink(link)
            if id then items[#items + 1] = { id = id, link = link, bag = bag, slot = s, loc = self:EquipLoc(id) } end
        end
    end
    return items, info
end

function FC:Zone()
    local zone = plain(GetRealZoneText and safe(GetRealZoneText)) or plain(GetZoneText and safe(GetZoneText))
    local sub = plain(GetSubZoneText and safe(GetSubZoneText))
    if type(zone) ~= "string" or zone == "" then return {} end
    local P = self:Rules().zoneParents or {}
    return { zone = zone, sub = (type(sub) == "string" and sub ~= "") and sub or nil, parent = P[zone] or zone }
end

function FC:Journey()
    local j = _G.Journey_Character
    if type(j) ~= "table" then return nil end
    local g = j.lastGuide
    local bm = g and type(j.bookmarks) == "table" and j.bookmarks[g]
    local out = { guide = g, step = type(bm) == "table" and bm.stepID or nil }
    for _, r in ipairs(self:Rules().routes or {}) do
        if r.id == g then out.zone = table.concat(r.zones, " / "); out.from = r.from; out.to = r.to end
    end
    return out
end

function FC:QuestDone(id)
    if not id then return nil end
    if C_QuestLog and C_QuestLog.IsQuestFlaggedCompleted then
        local r = plain(safe(C_QuestLog.IsQuestFlaggedCompleted, id))
        if r ~= nil then return r == true end
    end
    if IsQuestFlaggedCompleted then
        local r = plain(safe(IsQuestFlaggedCompleted, id))
        if r ~= nil then return r == true end
    end
end

function FC:OnQuest(id)
    if id and C_QuestLog and C_QuestLog.IsOnQuest then return plain(safe(C_QuestLog.IsOnQuest, id)) == true end
    return false
end

function FC:Snapshot()
    local _, classFile = UnitClass("player")
    local _, raceFile = UnitRace("player")
    local faction = UnitFactionGroup("player")
    local bags, baginfo = self:Bags()
    local equipped, equipmentState = self:Equipped()
    local s = {
        time = time(), name = UnitName("player"), realm = GetRealmName and GetRealmName(),
        class = classFile, race = raceFile, faction = faction, level = num(UnitLevel("player")) or 1,
        xp = num(UnitXP("player")) or 0, xpMax = num(UnitXPMax("player")) or 0,
        rested = (GetXPExhaustion and num(GetXPExhaustion())) or 0,
        resting = (IsResting and plain(safe(IsResting)) == true) or false,
        money = (GetMoney and num(GetMoney())) or 0,
        talents = self:ReadTalents(), skills = self:Skills(), reps = self:Reputations(),
        equipped = equipped, equipmentState = equipmentState,
        bags = bags, baginfo = baginfo, zone = self:Zone(), journey = self:Journey(), spells = {},
    }
    local book -- read once per snapshot, only if a milestone needs it
    for _, m in ipairs(self:Rules().spellMilestones or {}) do
        if m.class == classFile then
            for _, id in ipairs(m.spellIds or { m.spellId }) do
                local k = knowsSpellID(id)
                if k ~= true then
                    if book == nil then book = self:SpellBook(true) end
                    k = knowsByNameSafe(self, id, book, m.name)
                end
                if k ~= nil then s.spells[id] = k end
            end
        end
    end
    return s
end

-- Each section is computed on its own; a failing section keeps its last good result and is marked stale.
local function section(c, key, fn, ...)
    c.stale = c.stale or {}
    local ok, res = pcall(fn, ...)
    if ok then c[key] = res; c.stale[key] = nil; return true end
    c.stale[key] = true
    c.lastError = { where = key, err = tostring(res), time = time(), version = FC.VERSION }
    return false
end

function FC:RefreshUnsafe(reason)
    if self._loggingOut then return end -- late timers must not sample torn-down client APIs
    local c = self:Char()
    c.stale = c.stale or {}
    if type(IsPlayerInWorld) == "function" and plain(safe(IsPlayerInWorld)) ~= true then
        c.stale.snapshot = true
        return
    end
    local okSnap, snap = pcall(self.Snapshot, self)
    if not okSnap then
        c.lastError = { where = "snapshot", err = tostring(snap), time = time(), version = FC.VERSION }
        return
    end
    c.snapshot = snap
    c.stale.snapshot = nil
    if FC.Gear then section(c, "gear", FC.Gear.Audit, FC.Gear, snap) end
    if FC.Talents then section(c, "talentPlan", FC.Talents.Plan, FC.Talents, snap, c.gear and c.gear.spec) end
    if FC.Dungeons then section(c, "dungeons", FC.Dungeons.Plan, FC.Dungeons, snap) end
    if FC.Audit then section(c, "findings", FC.Audit.Run, FC.Audit, c) end
    c.updated = time()
    c.reason = reason
    if FC.UI and FC.UI.Update then FC.UI:Update() end
end

function FC:Refresh(reason)
    local ok, err = pcall(self.RefreshUnsafe, self, reason)
    if not ok then
        local c = self:Char()
        c.lastError = { where = "Refresh:" .. tostring(reason), err = tostring(err), time = time(), version = FC.VERSION }
    end
end

local function later(sec, key, fn)
    if not C_Timer then fn() return end
    FC._pending = FC._pending or {}
    if FC._pending[key] then return end
    FC._pending[key] = true
    C_Timer.After(sec, function() FC._pending[key] = nil; fn() end)
end
FC.later = later

local function scanTrainer()
    local trade = IsTradeskillTrainer and plain(safe(IsTradeskillTrainer))
    if trade == true then return end -- profession trainers don't count as class training
    local avail, names = 0, {}
    for i = 1, ((GetNumTrainerServices and num(safe(GetNumTrainerServices))) or 0) do
        local name, _, kind = safe(GetTrainerServiceInfo, i)
        if plain(kind) == "available" then
            avail = avail + 1
            if type(name) == "string" and #names < 6 then names[#names + 1] = name end
        end
    end
    FC:Char().trainer = { level = num(UnitLevel("player")), time = time(), available = avail, names = names }
end

local f = CreateFrame("Frame")
FC.frame = f
for _, e in ipairs({ "ADDON_LOADED", "PLAYER_LOGIN", "PLAYER_LEVEL_UP", "PLAYER_LOGOUT", "TRAINER_SHOW", "TRAINER_UPDATE",
    "TRAINER_CLOSED", "PLAYER_EQUIPMENT_CHANGED", "ZONE_CHANGED_NEW_AREA", "SPELLS_CHANGED", "BAG_UPDATE_DELAYED",
    "QUEST_TURNED_IN", "GET_ITEM_INFO_RECEIVED", "ITEM_DATA_LOAD_RESULT", "PLAYER_ENTERING_WORLD" }) do
    f:RegisterEvent(e)
end
-- One shared handler: no closure is built per event (BAG_UPDATE_DELAYED and friends fire often).
local function onEvent(event, arg1)
        if event == "ADDON_LOADED" and arg1 == ADDON then
            FC:Char()
        elseif event == "PLAYER_LOGIN" then
            later(5, "refresh", function() FC:Refresh("login") end)
            if FC.Session then FC.Session:Start() end
        elseif event == "PLAYER_LEVEL_UP" then
            if FC.Session then FC.Session:LevelUp() end
            later(2, "levelup", function()
                FC:Refresh("levelup")
                -- the checklist must never break the rest of the level-up handling
                if FC.LevelUp and FC.LevelUp:Enabled() then
                    local ok, err = pcall(FC.LevelUp.OnLevelRefresh, FC.LevelUp)
                    if not ok then FC:Char().lastError = { where = "levelup", err = tostring(err), time = time(), version = FC.VERSION } end
                end
            end)
        elseif event == "TRAINER_SHOW" or event == "TRAINER_UPDATE" then
            scanTrainer()
        elseif event == "TRAINER_CLOSED" then
            scanTrainer(); later(1, "refresh", function() FC:Refresh("trainer") end)
        elseif event == "SPELLS_CHANGED" then
            if FC.Session then FC.Session:SyncSpells(true) end -- read the current book, not a cached one
            later(3, "refresh", function() FC:Refresh("spells") end)
        elseif event == "GET_ITEM_INFO_RECEIVED" or event == "ITEM_DATA_LOAD_RESULT" then
            later(5, "refresh", function() FC:Refresh("iteminfo") end)
        elseif event == "PLAYER_LOGOUT" then
            -- On Forever, inventory/XP/bag APIs can already be cleared here.
            -- Save the last in-world capture; only finalize derived session stats.
            FC._loggingOut = true
            if FC.Session then FC.Session:Finish() end
            local c = FC:Char()
            if c.snapshot and FC.Audit then section(c, "findings", FC.Audit.Run, FC.Audit, c) end
            c.savedAt = time()
        elseif event ~= "ADDON_LOADED" then
            later(3, "refresh", function() FC:Refresh(event) end)
        end
end
f:SetScript("OnEvent", function(_, event, arg1)
    local ok, err = pcall(onEvent, event, arg1)
    if not ok then FC:Char().lastError = { where = event, err = tostring(err), time = time(), version = FC.VERSION } end
end)
