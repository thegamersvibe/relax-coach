-- Relax Raid logging switch (REQ-LOG-01, REQ-LOG-02; PLAN.md R1, PROBES.md P2).
--
-- On entering a raid or a five-player dungeon, a logger's client turns combat logging and Advanced Combat
-- Logging on, says so in chat once, and shows a reminder with the manual steps if it cannot. It never turns
-- logging off: no code path calls LoggingCombat(false) or writes advancedCombatLogging = 0.
--
-- Client quirks (PROBES.md P2, reported by the Forever Logs Companion author, not observed by us): a start is
-- ignored for about 10 s after a loading screen, the read lags a start by about 5 s, and each start while
-- logging is already on writes another header line. So the start is retried on a schedule and every try reads
-- first: only the client's read stops the retries (PLAN.md 7.1 point 5). The client can take a start without
-- error and still ignore it, so our own call never stops a retry; its age is used only for the status answer.
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
Log.TRUST = 35                            -- status only: our own start this recent is reported as "started"
Log.RANK_WAIT = 15                        -- guild data still missing this long after login: rank unreadable
-- Officer ranks (PLAN.md 7.1 point 1, confirmed by the owner): the top two ranks. A character defaults to
-- logger when either list matches. Indexes follow the rank's position (0 is the guild master), so a guild
-- that reorders its ranks must revisit them. Names are compared case-insensitively.
Log.officerRankIndexes = {0, 1}
Log.officerRankNames = {}
-- Instance map ids of raids that are not inside an instance. None is known yet; the list is data.
Log.outdoorRaidMaps = {}
Log.DB_VERSION = 1

local REMIND_LOG = "could not turn combat logging on. Type /combatlog now; the game answers \"Combat being logged\"."
local REMIND_ADV = "could not turn Advanced Combat Logging on. Switch it on in Options > Network > Advanced Combat Logging."
local HINT_RANK = "could not read your guild rank, so this character does not log dungeons (raids still log). To log dungeons too, type /relaxraid logger on."
local HINT_SIZE = "this raid is logged for you (raids log for every guild member). Combat logs are large (several hundred MB a raid night); do not delete them until they are uploaded."

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

-- The default for (guild member, rank, instance type), DEC-40 (PLAN.md 6B.4): in a raid every guild member
-- logs, rank or not; anywhere else (a five-player dungeon) only the officer ranks. Not in a guild: never.
-- Returns true or false, or nil while a dungeon default still waits for the guild rank.
function Log.default(rank, kind)
    if rank == "noguild" then return false end
    if kind == "raid" then return true end -- officer, member, unreadable or still loading: in the guild is enough
    if rank == "officer" then return true end
    if rank == "pending" then return nil end
    return false
end

-- The instance type we are in ("raid", "party"), or nil outside.
local function currentKind()
    local key = (state.run and state.run.key) or Log.where()
    return key and string.match(key, "^(%a+):") or nil
end

-- The logger setting: a stored choice, else the default for the instance we are in (or `kind`).
function Log.logger(kind)
    local db = RelaxRaidCharDB
    if not state.readOnly and type(db) == "table" and type(db.logger) == "boolean" then return db.logger end
    return Log.default(state.rank, kind or currentKind())
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
    if now then Log.restart() end -- start again where we stand
    return true
end

-- Guild rank -------------------------------------------------------------------------------------------------
-- "officer", "member", "noguild", "unreadable", or "pending" (guild data not loaded yet).
function Log.readRank()
    local okG, inGuild = pcall(IsInGuild)
    if not okG or isSecret(inGuild) then return "unreadable" end
    if not inGuild then return "noguild" end
    local ok, guildName, rankName, index = pcall(GetGuildInfo, "player")
    if not ok then return "unreadable" end
    if isSecret(guildName) or isSecret(rankName) or isSecret(index) then
        -- a secret is unknown, never nil: only a listed name or a plain index below can decide
    elseif guildName == nil and index == nil and rankName == nil then
        return "pending"
    end
    local indexOk = not isSecret(index) and index ~= nil and type(index) == "number"
    local nameOk = not isSecret(rankName) and rankName ~= nil and type(rankName) == "string"
    -- a listed name decides alone; otherwise the index must be readable to say "member"
    local okM, result = pcall(function()
        if nameOk then
            local lower = string.lower(rankName)
            for i = 1, #Log.officerRankNames do
                if lower == string.lower(Log.officerRankNames[i]) then return "officer" end
            end
        end
        if not indexOk then return "unreadable" end
        for i = 1, #Log.officerRankIndexes do
            if index == Log.officerRankIndexes[i] then return "officer" end
        end
        return "member"
    end)
    if not okM then return "unreadable" end
    return result
end

local function settleRank(final)
    local rank = Log.readRank()
    if rank == "pending" and (final or state.rank == "unreadable") then rank = "unreadable" end
    local before = Log.logger()
    state.rank = rank
    if rank == "unreadable" and not Log.chosen() and not state.hinted then
        state.hinted = true
        RR.say(HINT_RANK)
    end
    if before ~= true and Log.logger() == true then Log.restart() end
    Log.sizeHint() -- decided only now that the rank may be settled
end

-- On by default for someone who is not an officer (DEC-40): once per session, in a raid, say what it costs on
-- disk. Decided only on a settled rank: a pending rank may still turn out to be an officer's.
function Log.sizeHint()
    if state.sizeHinted or Log.chosen() or state.rank == "officer" or state.rank == "pending" then return end
    if currentKind() ~= "raid" or Log.logger() ~= true then return end
    state.sizeHinted = true
    RR.say(HINT_SIZE)
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
        for i = 1, #Log.outdoorRaidMaps do
            if Log.outdoorRaidMaps[i] == mapId then return "raid:" .. tostring(mapId) end
        end
    end
    return nil
end

-- The switch -------------------------------------------------------------------------------------------------
-- A secret or a failed read is "unknown": not on, so the retries go on.
local function readOn()
    local ok, on = pcall(LoggingCombat)
    if not ok or isSecret(on) then return false end
    return on == true
end

local function advancedIsOn()
    local ok, value = pcall(function() return C_CVar.GetCVar("advancedCombatLogging") end)
    return ok and not isSecret(value) and value == "1"
end

local function advanced(run)
    run.advDone = true
    if advancedIsOn() then run.adv = true return end
    pcall(function() return C_CVar.SetCVar("advancedCombatLogging", "1") end)
    run.adv = advancedIsOn()
    if not run.adv then RR.say(REMIND_ADV) end -- once per entry; the write is not retried
end

-- Each step of a try stands alone: a throw in one cannot skip the next.
local function step(label, fn, ...)
    local ok, result = pcall(fn, ...)
    if not ok then
        RR.errors[label] = (RR.errors[label] or 0) + 1
        return nil
    end
    return result
end

local function confirm(run)
    if run.said then return end
    run.said = true
    -- read the setting now: the player may have switched it since the first try
    RR.say("combat logging is on" .. (advancedIsOn() and ", with Advanced Combat Logging." or "."))
end

function Log.try(run)
    if state.run ~= run then return end
    if Log.logger() ~= true then return end -- off: nothing; nil: the rank is not in yet, a later try goes
    if not run.advDone then step("Log.advanced", advanced, run) end
    if step("Log.read", readOn) then step("Log.confirm", confirm, run) return end -- only the read stops a retry
    local ok = pcall(LoggingCombat, true)
    if ok then run.calledAt = RR.now() end
end

function Log.final(run)
    if state.run ~= run or Log.logger() ~= true then return end
    if not run.advDone then step("Log.advanced", advanced, run) end
    if step("Log.read", readOn) then step("Log.confirm", confirm, run) return end
    if not run.reminded then
        run.reminded = true
        RR.say(REMIND_LOG)
    end
end

-- Start the tries again inside the same entry (the logger was switched on, or the rank came in) without
-- resetting what this entry already did: one confirmation, one reminder and one setting write per entry.
function Log.restart()
    local old = state.run
    state.run = nil
    Log.check()
    local run = state.run
    if old and run and run.key == old.key then
        run.said, run.reminded, run.advDone, run.adv = old.said, old.reminded, old.advDone, old.adv
    end
end

function Log.begin(key)
    state.seq = state.seq + 1
    local run = {id = state.seq, key = key}
    state.run = run
    Log.sizeHint()
    for i = 1, #Log.SCHEDULE do
        RR.after(Log.SCHEDULE[i], "Log.try", function() Log.try(run) end)
    end
    RR.after(Log.CHECK_AT, "Log.final", function() Log.final(run) end)
    return run
end

-- Called on every loading screen and zone change. Leaving an instance only forgets the run: never "off".
local UNSETTLED = {pending = true, unreadable = true, noguild = true}

function Log.check()
    if not RR.loaded then return end
    -- the rank may have come in without PLAYER_GUILD_UPDATE: read it again at every loading screen until known
    if UNSETTLED[state.rank] and not Log.chosen() then settleRank(false) end
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

local function word(v)
    return v == nil and "not decided yet" or v and "on" or "off"
end

function Log.status()
    local raid, dungeon = Log.default(state.rank, "raid"), Log.default(state.rank, "party")
    if Log.chosen() then
        RR.say("logger " .. word(Log.logger()) .. " for this character (your choice).")
    elseif raid == dungeon then
        RR.say("logger " .. word(raid) .. " for this character (default: " .. RANK_WORDS[state.rank] .. ").")
    else -- the default differs by instance type (DEC-40): say both
        RR.say("logger for this character: default, " .. word(raid) .. " in raids, " .. word(dungeon) ..
            " in dungeons (" .. RANK_WORDS[state.rank] .. ").")
    end
    local run = state.run
    if readOn() then
        RR.say("combat logging: on.")
    elseif run and run.calledAt and RR.now() - run.calledAt < Log.TRUST then
        RR.say(string.format("combat logging: started %d s ago, waiting for the game to confirm.", RR.now() - run.calledAt))
    else
        RR.say("combat logging: off.")
    end
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
        -- shows the stored choice, else the raid default; setting it back to that default clears the choice,
        -- so ticking and unticking returns to "default" instead of storing "off"
        function() return Log.logger("raid") == true end,
        function(value)
            value = value == true
            if value == (Log.default(state.rank, "raid") == true) then Log.setLogger(nil) else Log.setLogger(value) end
        end)
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
