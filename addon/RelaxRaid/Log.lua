-- Relax Raid logging switch (REQ-LOG-01, REQ-LOG-02; PLAN.md R1, PROBES.md P2).
--
-- On entering a raid or a five-player dungeon, a logger's client turns combat logging and Advanced Combat
-- Logging on, says so in chat once, and shows a reminder with the manual steps if it cannot. It never turns
-- logging off: no code path calls LoggingCombat(false) or writes advancedCombatLogging = 0.
--
-- Client quirks (PROBES.md P2, reported by the Forever Logs Companion author, not observed by us): a start is
-- ignored for about 10 s after a loading screen, the read lags a start by about 5 s, and each start while
-- logging is already on writes another header line. So the start is retried on a schedule, every try reads
-- first, and a start the client accepted is trusted for a while instead of the lagging read.
--
-- Client API, each confirmed in Gethe/wow-ui-source@eac30af597e5aaa43a60f6675ef5fdcea100375f (build 70790):
--   LoggingCombat()/LoggingCombat(true)           SlashCommands.lua:858-862 (/combatlog)
--   C_CVar.GetCVar/SetCVar                         CVarDocumentation.lua:20, 131
--   IsInInstance, GetInstanceInfo                  InstanceDocumentation.lua:170, 103
--   GetGuildInfo("player") -> name, rank, index    TabardFrame.lua:91; IsInGuild InstanceDifficulty.lua:27
--   PLAYER_GUILD_UPDATE, PLAYER_ENTERING_WORLD     GuildInfoDocumentation.lua:652, SystemDocumentation.lua:136
--   ZONE_CHANGED_NEW_AREA                          MapDocumentation.lua:665
--   SLASH_<KEY>1, SlashCmdList                     Blizzard_ChatFrameBase/Shared/SlashCommands.lua:1391
--   issecretvalue                                  Blizzard_GamepadSmartNavigation/SecureHelper.lua:6
--   Settings.RegisterVerticalLayoutCategory, RegisterProxySetting, CreateCheckbox, RegisterAddOnCategory
--                                                  Blizzard_Settings_Shared/Blizzard_Settings.lua:154, 178, 388, 134
local _, RR = ...
local Log = RR.module("Log")

-- Data -------------------------------------------------------------------------------------------------------
Log.SCHEDULE = {0, 3, 6, 10, 15, 20, 30} -- seconds after entering: try the start (donor's 3..30 s, plus at once)
Log.CHECK_AT = 40                         -- one last read; still off means the reminder
Log.TRUST = 35                            -- an accepted start is believed this long over the lagging read
Log.RANK_WAIT = 15                        -- guild data still missing this long after login: rank unreadable
-- Guild rank indexes that default the logger on (0 is the guild master). Officer ranks: PENDING عقيل R1-Q1.
Log.OFFICER_RANKS = {0, 1}
-- Instance map ids of raids that are not inside an instance. None is known yet; the list is data.
Log.OUTDOOR_RAID_MAPS = {}
Log.DB_VERSION = 1

local REMIND_LOG = "could not turn combat logging on. Type /combatlog now; the game answers \"Combat being logged\"."
local REMIND_ADV = "could not turn Advanced Combat Logging on. Switch it on in Options > Network > Advanced Combat Logging."
local HINT_RANK = "could not read your guild rank, so this character is not a logger. If you log raids, type /relaxraid logger on."

local state = {run = nil, seq = 0, rank = "pending", hinted = false, readOnly = false}
Log.state = state

local isSecret = function(v)
    return type(issecretvalue) == "function" and issecretvalue(v) == true
end

-- Saved setting (RelaxRaidCharDB) ------------------------------------------------------------------------------
local function charDB()
    local db = RelaxRaidCharDB
    if type(db) ~= "table" then
        db = {version = Log.DB_VERSION}
        RelaxRaidCharDB = db
    end
    return db
end

-- The logger setting: true or false, or nil while the default waits for the guild rank.
function Log.logger()
    local db = RelaxRaidCharDB
    if not state.readOnly and type(db) == "table" and type(db.logger) == "boolean" then return db.logger end
    if state.rank == "officer" then return true end
    if state.rank == "pending" then return nil end
    return false
end

function Log.chosen()
    local db = RelaxRaidCharDB
    return not state.readOnly and type(db) == "table" and type(db.logger) == "boolean"
end

-- value true/false stores a choice; nil goes back to the rank default.
function Log.setLogger(value)
    if state.readOnly then
        RR.say("this character's settings were saved by a newer Relax Raid; not changed.")
        return false
    end
    charDB().logger = value
    local now = Log.logger()
    RR.say("logger " .. (now and "on" or "off") .. " for this character" .. (value == nil and " (guild rank default)." or "."))
    if now then
        state.run = nil -- start again where we stand
        Log.check()
    end
    return true
end

-- Guild rank -------------------------------------------------------------------------------------------------
-- "officer", "member", "noguild", "unreadable", or "pending" (guild data not loaded yet).
function Log.readRank()
    local okG, inGuild = pcall(IsInGuild)
    if not okG or isSecret(inGuild) then return "unreadable" end
    if not inGuild then return "noguild" end
    local ok, guildName, _, index = pcall(GetGuildInfo, "player")
    if not ok then return "unreadable" end
    if guildName == nil and index == nil then return "pending" end
    if index == nil or isSecret(index) or type(index) ~= "number" then return "unreadable" end
    local okM, officer = pcall(function()
        for i = 1, #Log.OFFICER_RANKS do
            if index == Log.OFFICER_RANKS[i] then return true end
        end
        return false
    end)
    if not okM then return "unreadable" end
    return officer and "officer" or "member"
end

local function settleRank(final)
    local rank = Log.readRank()
    if rank == "pending" and final then rank = "unreadable" end
    local before = Log.logger()
    state.rank = rank
    if rank == "unreadable" and not Log.chosen() and not state.hinted then
        state.hinted = true
        RR.say(HINT_RANK)
    end
    if before ~= true and Log.logger() == true then
        state.run = nil
        Log.check()
    end
end

-- Where we are -----------------------------------------------------------------------------------------------
-- A key for the current raid or dungeon instance, or nil anywhere else.
function Log.where()
    local ok, inside, kind = pcall(IsInInstance)
    if not ok or isSecret(kind) then return nil end
    local okI, _, _, _, _, _, _, _, mapId = pcall(GetInstanceInfo)
    if not okI or isSecret(mapId) then mapId = nil end
    if inside and (kind == "raid" or kind == "party") then
        return kind .. ":" .. tostring(mapId)
    end
    if mapId ~= nil then
        for i = 1, #Log.OUTDOOR_RAID_MAPS do
            if Log.OUTDOOR_RAID_MAPS[i] == mapId then return "raid:" .. tostring(mapId) end
        end
    end
    return nil
end

-- The switch -------------------------------------------------------------------------------------------------
local function readOn()
    local ok, on = pcall(LoggingCombat)
    return ok and on == true
end

local function advanced(run)
    run.advDone = true
    local ok, value = pcall(C_CVar.GetCVar, "advancedCombatLogging")
    if ok and value == "1" then run.adv = true return end
    pcall(C_CVar.SetCVar, "advancedCombatLogging", "1")
    local ok2, value2 = pcall(C_CVar.GetCVar, "advancedCombatLogging")
    run.adv = ok2 and value2 == "1"
    if not run.adv then RR.say(REMIND_ADV) end -- once per entry; the write is not retried
end

local function confirm(run)
    if run.said then return end
    run.said = true
    RR.say("combat logging is on" .. (run.adv and ", with Advanced Combat Logging." or "."))
end

function Log.try(run)
    if state.run ~= run then return end
    if Log.logger() ~= true then return end -- off: nothing; nil: the rank is not in yet, a later try goes
    if not run.advDone then advanced(run) end
    if readOn() then confirm(run) return end
    if run.accepted and RR.now() - run.accepted < Log.TRUST then return end
    local ok, started = pcall(LoggingCombat, true)
    if ok and started == true then run.accepted = RR.now() end
end

function Log.final(run)
    if state.run ~= run or Log.logger() ~= true then return end
    if not run.advDone then advanced(run) end
    if readOn() then confirm(run) return end
    if not run.reminded then
        run.reminded = true
        RR.say(REMIND_LOG)
    end
end

function Log.begin(key)
    state.seq = state.seq + 1
    local run = {id = state.seq, key = key}
    state.run = run
    for i = 1, #Log.SCHEDULE do
        RR.after(Log.SCHEDULE[i], "Log.try", function() Log.try(run) end)
    end
    RR.after(Log.CHECK_AT, "Log.final", function() Log.final(run) end)
    return run
end

-- Called on every loading screen and zone change. Leaving an instance only forgets the run: never "off".
function Log.check()
    if not RR.loaded then return end
    local key = Log.where()
    if not key then
        state.run = nil
        return
    end
    if state.run and state.run.key == key then return end
    Log.begin(key)
end

-- Slash command and panel ------------------------------------------------------------------------------------
local RANK_WORDS = {officer = "officer rank", member = "not an officer rank", noguild = "not in a guild",
                    unreadable = "guild rank unreadable", pending = "waiting for guild data"}

function Log.status()
    local on = Log.logger()
    local why = Log.chosen() and "your choice" or ("default: " .. RANK_WORDS[state.rank])
    RR.say("logger " .. (on == nil and "not decided yet" or on and "on" or "off") .. " for this character (" .. why .. ").")
end

local function slash(msg)
    local words = {}
    for w in string.gmatch(string.lower(msg or ""), "%S+") do words[#words + 1] = w end
    if words[1] == "logger" and words[2] == "on" then return Log.setLogger(true) end
    if words[1] == "logger" and words[2] == "off" then return Log.setLogger(false) end
    if words[1] == "logger" and words[2] == "default" then return Log.setLogger(nil) end
    if words[1] == "logger" or words[1] == "status" then return Log.status() end
    RR.say("/relaxraid logger on | off | default | status")
end

SLASH_RELAXRAID1 = "/relaxraid"
SLASH_RELAXRAID2 = "/rraid"
SlashCmdList.RELAXRAID = function(msg) RR.safe("slash", slash, msg) end

local function panel()
    if type(Settings) ~= "table" or not Settings.RegisterVerticalLayoutCategory then return end
    local category = Settings.RegisterVerticalLayoutCategory("Relax Raid")
    local setting = Settings.RegisterProxySetting(category, "RELAXRAID_LOGGER", Settings.VarType.Boolean,
        "Logger (this character)", false,
        function() return Log.logger() == true end,
        function(value) Log.setLogger(value == true) end)
    Settings.CreateCheckbox(category, setting,
        "Turn combat logging and Advanced Combat Logging on when this character enters a raid or dungeon.")
    Settings.RegisterAddOnCategory(category)
    Log.category = category
end

-- Wiring -----------------------------------------------------------------------------------------------------
function Log.Init()
    local db = RelaxRaidCharDB
    if type(db) == "table" and type(db.version) == "number" and db.version > Log.DB_VERSION then
        state.readOnly = true -- a newer save: read nothing from it, write nothing to it
    elseif type(db) == "table" then
        db.version = Log.DB_VERSION
    end
    settleRank(false)
    if state.rank == "pending" then
        RR.after(Log.RANK_WAIT, "Log.rank", function()
            if state.rank == "pending" then settleRank(true) end
        end)
    end
    RR.safe("Log.panel", panel)
end

RR.on("PLAYER_GUILD_UPDATE", "Log", function(_, unit)
    if RR.loaded and (unit == nil or unit == "player") then settleRank(false) end
end)
RR.on("PLAYER_ENTERING_WORLD", "Log", function() Log.check() end)
RR.on("ZONE_CHANGED_NEW_AREA", "Log", function() Log.check() end)
