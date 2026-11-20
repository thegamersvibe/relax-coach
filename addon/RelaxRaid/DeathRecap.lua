-- Shared, addon-independent recap reader. Relax Coach is the source of truth.
-- Forever source pin and every client API/field: docs/raid/DEATH-RECAP.md.
-- No live combat recorder, network, unit-health sampling or external media.
local addonName = ...
local VERSION = 1
local MAX_ID = 999999999999 -- largest exact integer accepted by the 12-digit link
if RelaxDeathRecap and RelaxDeathRecap.v >= VERSION then return end
local R = RelaxDeathRecap or {hooks = {}, history = {}, serial = 0}
RelaxDeathRecap = R
R.v, R.owner = VERSION, addonName
R.maxVisibleAttackers = 3 -- mockup-sized block; UI shows +X more, all shares stay intact

-- Check permission before type tests, comparisons, indexing or formatting.
-- Both the access check and the operation can throw; neither reveals a value.
function R.clean(value, kind)
    local ok, result, invalidNumber = pcall(function()
        if issecretvalue and issecretvalue(value) then return nil end
        if canaccessvalue and not canaccessvalue(value) then return nil end
        if type(value) ~= kind then return nil end
        if kind == "table" and canaccesstable and not canaccesstable(value) then return nil end
        if kind == "number" and (value ~= value or math.abs(value) > 1e12) then return nil, true end
        if kind == "string" and (#value > 160 or value:find("[%c|]")) then return nil end
        return value
    end)
    if ok then return result, invalidNumber end
end
function R.field(t, key, kind)
    if not R.clean(t, "table") then return nil end
    local ok, value = pcall(function() return t[key] end)
    if ok then return R.clean(value, kind) end
end
function R.call(fn, ...)
    local ok, value = pcall(fn, ...)
    if ok then return value end
end

local fields = {
    timestamp="number", event="string", spellId="number", spellName="string", environmentalType="string",
    amount="number", overkill="number", absorbed="number", resisted="number", blocked="number",
    school="number", currentHP="number", sourceName="string", sourceGUID="string", sourceFlags="number",
    hideCaster="boolean", deadly="boolean", avoidable="boolean", critical="boolean",
}
local environment = {FALLING="Falling", DROWNING="Drowning", FIRE="Fire", LAVA="Lava", SLIME="Slime", FATIGUE="Fatigue"}
function R.spell(row)
    if row.event == "ENVIRONMENTAL_DAMAGE" then return environment[row.environmentalType] or "Environment" end
    if row.event == "SWING_DAMAGE" then return "Melee" end
    return row.spellName or "Unknown"
end
local function copyRow(raw)
    if not R.clean(raw, "table") then return nil end
    local row = {}
    for key, kind in pairs(fields) do
        local value, invalidNumber = R.field(raw, key, kind)
        if key == "amount" and invalidNumber then return nil end
        row[key] = value
    end
    if row.amount and row.amount < 0 then return nil end
    if row.hideCaster ~= false then row.sourceName, row.sourceGUID, row.sourceFlags = nil, nil, nil end
    return row
end
function R.register(name, hooks)
    R.hooks[name] = hooks
    if name == R.owner and hooks.settings then R.configure(hooks.settings) end
end
local function identity(guid, name)
    local class = guid and R.clean(R.call(GetPlayerInfoByGUID, guid), "string")
    local level, guild
    for _, hook in pairs(R.hooks) do
        local ok, c, l, g = pcall(hook.identity, guid, name)
        if ok then
            class = R.clean(c, "string") or class
            level = R.clean(l, "number") or level
            guild = R.clean(g, "string") or guild
        end
    end
    return class, level, guild
end
local function flag(flags, mask)
    return flags and flags >= 0 and flags == math.floor(flags) and flags % (mask * 2) >= mask
end
function R.attackers(recap)
    local groups, list, total, complete, player = {}, {}, 0, not recap.incomplete, false
    for _, row in ipairs(recap.rows) do
        row.guildAvoidable = false
        for _, hook in pairs(R.hooks) do
            if row.spellId and R.clean(R.call(hook.avoidable, row.spellId), "boolean") == true then row.guildAvoidable = true end
        end
        if row.effective then total = total + row.effective else complete = false end
        -- CombatLogConstantsDocumentation.lua:18-27, not a combat-log API.
        local kind = flag(row.sourceFlags, 1024) and "player" or flag(row.sourceFlags, 256) and "pet"
        if flag(row.sourceFlags, 64) and kind and row.sourceGUID then
            if kind == "player" then player = true end
            local key = kind .. ":" .. row.sourceGUID
            local group = groups[key]
            if not group then
                group = {guid=row.sourceGUID, name=row.sourceName, kind=kind, amount=0}
                group.class, group.level, group.guild = identity(group.guid, group.name)
                if kind == "pet" then
                    local token = R.clean(R.call(UnitTokenFromGUID, group.guid), "string")
                    if token then group.ownerGUID = R.clean(R.call(UnitOwnerGUID, token), "string") end
                end
                groups[key], list[#list+1] = group, group
            end
            group.amount = group.amount + (row.effective or 0)
        end
    end
    table.sort(list, function(a,b) return a.amount > b.amount or (a.amount == b.amount and a.guid < b.guid) end)
    for _, group in ipairs(list) do
        if complete and total > 0 then group.share = group.amount / total * 100
        else group.amount = nil end
    end
    recap.attackers = player and list or {}
end
function R.describe(recap)
    local rows, latest, completeTime = recap.rows, nil, not recap.incomplete
    for _, row in ipairs(rows) do
        if not row.timestamp then completeTime = false
        elseif not latest or row.timestamp > latest then latest = row.timestamp end
    end
    table.sort(rows, function(a, b) return (a.timestamp or -1) > (b.timestamp or -1) end)
    for _, row in ipairs(rows) do
        row.effective, row.seconds = nil, nil
        if row.amount and row.overkill then row.effective = math.max(0, row.amount - math.max(0, row.overkill)) end
        if not row.effective then recap.incomplete = true end
        if completeTime and latest then row.seconds = latest - row.timestamp end
        if row.currentHP and recap.maxHealth and recap.maxHealth > 0 then
            row.healthPercent = math.max(0, math.min(100, row.currentHP / recap.maxHealth * 100))
        end
    end
    recap.killer, recap.killingBlow = "Unknown", nil
    if completeTime and rows[1] then
        recap.killingBlow = rows[1]
        recap.killer = rows[1].sourceName or (rows[1].event == "ENVIRONMENTAL_DAMAGE" and R.spell(rows[1])) or "Unknown"
    end
    R.attackers(recap)
    return recap
end
function R.read()
    local recap = {rows = {}}
    if R.clean(R.call(function() return C_DeathRecap.HasRecapEvents() end), "boolean") ~= true then return R.describe(recap) end
    local events = R.call(function() return C_DeathRecap.GetRecapEvents() end)
    recap.maxHealth = R.clean(R.call(function() return C_DeathRecap.GetRecapMaxHealth() end), "number")
    if not R.clean(events, "table") then recap.incomplete = true; return R.describe(recap) end
    for i = 1, 12 do
        local ok, row, ended = pcall(function()
            local raw = events[i]
            if issecretvalue and issecretvalue(raw) then return nil end
            if canaccessvalue and not canaccessvalue(raw) then return nil end
            return copyRow(raw), raw == nil
        end)
        if ok and ended then break end
        if ok and row then recap.rows[#recap.rows+1] = row
        else recap.incomplete = true end
    end
    return R.describe(recap)
end


local defaults = {auto=true, autoclose=true, keep=false}
R.settings = R.settings or {version=1, auto=true, autoclose=true, keep=false}
function R.say(text)
    pcall(function() DEFAULT_CHAT_FRAME:AddMessage(text) end)
end
local function array(value, cap)
    if not R.clean(value, "table") then return nil end
    local ok, count = pcall(function()
        local n, high = 0, 0
        for k in pairs(value) do
            if not R.clean(k, "number") or k < 1 or k > cap or k ~= math.floor(k) then return nil end
            n, high = n+1, math.max(high, k)
            if n > cap then return nil end
        end
        if high == n then return n end
    end)
    if ok then return count end
end
-- The same field copier as fresh reads; persisted wrong types reject the recap.
function R.validate(raw)
    local rows = R.field(raw, "rows", "table")
    local count = array(rows, 12)
    local id = R.field(raw, "id", "number")
    if not count or not id or id < 1 or id > MAX_ID or id ~= math.floor(id) then return nil end
    local recap = {id=id, rows={}, incomplete=R.field(raw, "incomplete", "boolean")}
    for _, key in ipairs({"clock", "zone"}) do
        recap[key] = R.field(raw, key, "string")
        if not recap[key] then return nil end
    end
    recap.maxHealth = R.field(raw, "maxHealth", "number")
    local ok = pcall(function()
        if raw.maxHealth ~= nil and not recap.maxHealth then error("invalid max health") end
        if raw.incomplete ~= nil and recap.incomplete == nil then error("invalid completeness") end
        for i=1,count do
            local row = copyRow(rows[i])
            if not row then error("invalid row") end
            for key, kind in pairs(fields) do
                if rows[i][key] ~= nil and R.field(rows[i], key, kind) == nil then error("invalid field") end
            end
            recap.rows[i] = row
        end
    end)
    if ok then return recap end
end
function R.configure(settings)
    if R.settings ~= settings and R.writable then R.settings.history = nil end
    R.writable = R.clean(settings, "table") ~= nil and (R.field(settings, "version", "number") or 1) <= 1
    if not R.writable then R.settings = {version=1, auto=true, autoclose=true, keep=false}; return end
    R.settings = settings
    settings.version = 1
    for key, default in pairs(defaults) do
        if type(settings[key]) ~= "boolean" then settings[key] = default end
    end
    if #R.history == 0 and settings.keep and array(settings.history, 3) then
        local seen = {}
        for _, raw in ipairs(settings.history) do
            local recap = R.validate(raw)
            if recap and not seen[recap.id] then
                seen[recap.id] = true
                R.history[#R.history+1] = R.describe(recap)
                R.serial = math.max(R.serial, recap.id)
            end
        end
    end
    R.persist()
    R.call(R.resetPosition)
end
function R.persist()
    if not R.writable then return end
    R.settings.history = nil
    if R.settings.keep then
        local saved = {}
        for i=1,math.min(3, #R.history) do saved[#saved+1] = R.validate(R.history[i]) end
        R.settings.history = saved
    end
    -- Hosts can bind the table lazily: defaults alone must not create a save.
    if R.settings.keep or R.settings.position or not R.settings.auto or not R.settings.autoclose then
        local hook = R.hooks[R.owner]
        R.call(hook and hook.saveSettings, R.settings)
    end
end
function R.option(key, value)
    if defaults[key] == nil or type(value) ~= "boolean" then return end
    if R.writable == false then R.say("Recap settings are from a newer version; not changed."); return end
    R.settings[key] = value
    R.persist()
    R.say("Recap " .. key .. " " .. (value and "on" or "off") .. ".")
end
function R.link(link)
    link = R.clean(link, "string")
    local id = link and link:match("^addon:RelaxDeathRecap:(%d+)$")
    if not id or #id > 12 then return end
    id = tonumber(id)
    if id < 1 then return end
    for i, recap in ipairs(R.history) do
        if recap.id == id then R.call(R.open, i); return end
    end
    R.say("that recap is no longer kept")
end
function R.slash(message)
    message = R.clean(message, "string") or ""
    local key, choice = message:match("^(%a+)%s+(%a+)$")
    if defaults[key] ~= nil and (choice == "on" or choice == "off") then R.option(key, choice == "on"); return end
    if message == "reset" then
        if R.writable ~= false then R.settings.position = nil end
        R.call(R.resetPosition); R.say("Recap position reset."); return
    end
    if message == "" or message:match("^[123]$") then R.call(R.open, tonumber(message) or 1); return end
    R.say("/recap [1|2|3|reset|auto on|off|autoclose on|off|keep on|off]")
end
function R.finish(recap)
    if recap.notified then return end
    recap.notified = true
    for _, hook in pairs(R.hooks) do R.call(hook.onRecap, recap) end
    R.persist()
end
-- Compare row identities, never mutable damage/health or display names.
-- Match as a multiset so equal-timestamp ordering cannot make old rows fresh.
local function sameRows(rows, previous)
    if not previous or #rows ~= #previous then return false end
    local used = {}
    for _, row in ipairs(rows) do
        local found
        for i, old in ipairs(previous) do
            if not used[i] and row.timestamp == old.timestamp and row.sourceGUID == old.sourceGUID
                and row.spellId == old.spellId and row.event == old.event
                and row.environmentalType == old.environmentalType then
                used[i], found = true, true
                break
            end
        end
        if not found then return false end
    end
    return true
end
function R.refresh(recap, final)
    if R.pending ~= recap then return end
    local read = R.read()
    -- Unavailable/previous-death rows cannot erase or replace fresh data.
    if #read.rows > 0 and not sameRows(read.rows, recap.previousRows) then
        recap.rows, recap.maxHealth, recap.incomplete = read.rows, read.maxHealth, read.incomplete
        R.lastRows = read.rows
        R.describe(recap)
    elseif read.incomplete then
        recap.incomplete = true
        R.describe(recap)
    end
    if final then R.finish(recap); R.pending = nil end
    R.call(R.render)
end
function R.event(event)
    if event == "PLAYER_DEAD" then
        if R.dead then return end
        if R.serial >= MAX_ID then R.say("Recap ID limit reached; clear saved recaps and reload."); return end
        if R.pending then R.finish(R.pending) end
        R.dead, R.serial = true, R.serial + 1
        local ok, hour, minute = pcall(GetGameTime)
        hour, minute = R.clean(hour, "number"), R.clean(minute, "number")
        local recap = {id=R.serial, rows={}, killer="Unknown", attackers={},
            previousRows=R.lastRows or (R.history[1] and R.history[1].rows),
            clock=ok and hour and minute and string.format("%02d:%02d",hour,minute) or "--:--",
            zone=R.clean(R.call(GetZoneText), "string") or "Unknown zone"}
        table.insert(R.history, 1, recap)
        if #R.history > 3 then table.remove(R.history) end
        R.pending = recap
        R.refresh(recap)
        R.say("|Haddon:RelaxDeathRecap:" .. recap.id .. "|h[You died: details]|h")
        if R.settings.auto then R.call(R.open, 1) end
        for _, delay in ipairs({0.3, 1, 2}) do
            pcall(function() C_Timer.After(delay, function() pcall(R.refresh, recap, delay == 2) end) end)
        end
    elseif event == "PLAYER_ALIVE" or event == "PLAYER_UNGHOST" then
        -- Release closes the UI, not the current death's bounded read window.
        R.dead = false
        if R.settings.autoclose then R.call(R.close) end
    end
end
if not R.eventFrame then R.eventFrame = CreateFrame("Frame") end
R.eventFrame:SetScript("OnEvent", function(_, event) pcall(R.event, event) end)
for _, event in ipairs({"PLAYER_DEAD", "PLAYER_ALIVE", "PLAYER_UNGHOST"}) do
    pcall(R.eventFrame.RegisterEvent, R.eventFrame, event)
end
SLASH_RELAXDEATHRECAP1 = "/recap"
SlashCmdList.RELAXDEATHRECAP = function(message) pcall(R.slash, message) end
