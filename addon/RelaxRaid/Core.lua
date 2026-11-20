-- Relax Raid core: the namespace, guarded event and timer dispatch, and the module registry (PLAN.md 3.3).
-- Every handler and timer runs under pcall; an error is reported once per handler, never raised into the client.
-- Client API, confirmed in Gethe/wow-ui-source@eac30af597e5aaa43a60f6675ef5fdcea100375f: CreateFrame("Frame")
-- (Blizzard_PTRFeedbackGlue/Blizzard_PTRFeedback_Events_Glue.lua:2), C_Timer.After (UITimerDocumentation.lua:11),
-- GetTime (Blizzard_FrameXML/Shared/LowHealthFrame.lua:161), ADDON_LOADED (AddOnsDocumentation.lua:391),
-- DEFAULT_CHAT_FRAME:AddMessage (Blizzard_EncounterWarnings/EncounterWarningsUtil.lua:52).
local addonName, RR = ...
RR = RR or {}
RR.name = addonName or "RelaxRaid"
RR.modules = {}
RR.errors = {}

local PREFIX = "|cff66ccffRelax Raid:|r "

function RR.say(text)
    if DEFAULT_CHAT_FRAME and DEFAULT_CHAT_FRAME.AddMessage then
        DEFAULT_CHAT_FRAME:AddMessage(PREFIX .. text)
    end
end

-- Runs fn(...) under pcall. The first error from each label is said once; later ones are only counted.
function RR.safe(label, fn, ...)
    local ok, err = pcall(fn, ...)
    if not ok then
        local seen = RR.errors[label]
        RR.errors[label] = (seen or 0) + 1
        if not seen then RR.say("internal error in " .. label .. " (reported once): " .. tostring(err)) end
    end
    return ok
end

function RR.now()
    return GetTime()
end

-- C_Timer.After with the callback guarded.
function RR.after(seconds, label, fn)
    C_Timer.After(seconds, function() RR.safe(label, fn) end)
end

local handlers = {}  -- event -> list of {label, fn}
local frame = CreateFrame("Frame")
frame:SetScript("OnEvent", function(_, event, ...)
    local list = handlers[event]
    if not list then return end
    for i = 1, #list do RR.safe(list[i][1] .. ":" .. event, list[i][2], event, ...) end
end)

function RR.on(event, label, fn)
    if not handlers[event] then
        handlers[event] = {}
        frame:RegisterEvent(event)
    end
    local list = handlers[event]
    list[#list + 1] = {label, fn}
end

-- A module is a plain table; Init(RR) runs once at our ADDON_LOADED, after the saved variables are in.
function RR.module(name)
    local m = RR.modules[name]
    if not m then
        m = {name = name}
        RR.modules[name] = m
        RR.modules[#RR.modules + 1] = m
    end
    return m
end

-- The shared module is optional when Core is tested alone. Initialize after Log's
-- existing settings category and after SavedVariables, without coupling the reader
-- to this namespace. Source pin/API details: docs/raid/DEATH-RECAP.md.
local function initRecap()
    local recap = RelaxDeathRecap
    if not recap then return end
    local settings
    if recap.owner == RR.name then
        local db = RelaxRaidCharDB
        if type(db) == "table" and type(db.version) == "number" and db.version > 1 then
            settings = {version=99} -- read-only session defaults; leave the newer save alone
        else
            settings = type(db) == "table" and type(db.deathRecap) == "table" and db.deathRecap or {version=1}
        end
    end
    recap.register(RR.name, {settings=settings, saveSettings=function(value)
        if recap.owner ~= RR.name then return end
        local db = RelaxRaidCharDB
        if type(db) == "table" and type(db.version) == "number" and db.version > 1 then return end
        if type(db) ~= "table" then db = {version=1}; RelaxRaidCharDB = db end
        db.deathRecap = value
    end, avoidable=function(id)
        return RR.avoidableSpells and RR.avoidableSpells[id] == true
    end})
    local category = RR.modules.Log and RR.modules.Log.category
    if not category or not Settings then return end
    for _, option in ipairs({
        {"auto", "Open the recap when I die", true},
        {"autoclose", "Close the recap when I release or revive", true},
        {"keep", "Keep my last 3 recaps after logout", false},
    }) do
        local key = option[1]
        local setting = Settings.RegisterProxySetting(category, "RELAXRAID_RECAP_" .. string.upper(key),
            Settings.VarType.Boolean, option[2], option[3],
            function() return recap.settings[key] == true end,
            function(value) pcall(recap.option, key, value) end)
        Settings.CreateCheckbox(category, setting, "Local only. Recaps may contain other players' names. Nothing is sent.")
    end
end

RR.on("ADDON_LOADED", "Core", function(_, loaded)
    if loaded ~= RR.name or RR.loaded then return end
    RR.loaded = true
    for i = 1, #RR.modules do
        local m = RR.modules[i]
        if m.Init then RR.safe(m.name .. ".Init", m.Init, RR) end
    end
    RR.safe("DeathRecap.Init", initRecap)
end)

_G.RelaxRaid = RR
