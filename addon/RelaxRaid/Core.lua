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

RR.on("ADDON_LOADED", "Core", function(_, loaded)
    if loaded ~= RR.name or RR.loaded then return end
    RR.loaded = true
    for i = 1, #RR.modules do
        local m = RR.modules[i]
        if m.Init then RR.safe(m.name .. ".Init", m.Init, RR) end
    end
end)

_G.RelaxRaid = RR
