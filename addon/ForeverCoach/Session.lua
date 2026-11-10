-- Forever Coach: per-session play stats from data WoW Forever gives every addon.
-- Forever hides exact health/power from addons ("secret values") and has no addon combat log, so these are
-- ESTIMATES, labelled as such in the UI:
--   * "stopped" = standing still out of combat for under IDLE_AFK seconds (eating, looting, vendors, chatting...)
--   * "afk" = standing still longer than that; taxi and dead time are tracked separately
--   * proc windows = the spell turning usable (C_Spell.IsSpellUsable) vs. you casting it (UNIT_SPELLCAST_SUCCEEDED)
-- Nothing here reads memory or automates input.
local _, FC = ...
local num, plain = FC.num, FC.plain
local S = {}
FC.Session = S

local MAX_SESSIONS = 30
local IDLE_AFK = 120
local PROC_POLL = 0.2
local cur, tick
local procs = {}  -- spell name -> window state
local idle = 0

local function now() return GetTime() end
local function noteError(where, err)
    FC:Char().lastError = { where = where, err = tostring(err), time = time(), version = FC.VERSION }
end

local function newSession()
    return {
        start = time(), level = num(UnitLevel("player")) or 1, xp = 0, seconds = 0, combat = 0, recovery = 0, dead = 0,
        afk = 0, taxi = 0, deaths = {}, capped = 0, powerSeconds = 0, procs = {}, levels = 0,
    }
end

local function usable(name)
    local f = (C_Spell and C_Spell.IsSpellUsable) or IsUsableSpell
    if not f then return false end
    local ok, u = pcall(f, name)
    return ok and plain(u) == true  -- "usable but not enough rage" does not count as a chance
end

function S:SyncSpells()
    if not cur then return end
    local _, cls = UnitClass("player")
    for _, p in ipairs(FC:Rules().procSpells or {}) do
        if p.class == cls and FC:KnowsSpell(p.spellId) then
            local name = FC:SpellName(p.spellId) or p.name
            if not procs[name] then
                procs[name] = { spell = name, label = p.name, window = p.window or 5, total = 0, used = 0, was = false }
            end
        end
    end
end

function S:Start()
    cur = newSession()
    S.lastXP, S.lastXPMax, S.lastLevel = num(UnitXP("player")), num(UnitXPMax("player")), num(UnitLevel("player"))
    procs, idle = {}, 0
    self:SyncSpells()
    if not tick then
        tick = CreateFrame("Frame")
        local acc, pacc = 0, 0
        tick:SetScript("OnUpdate", function(_, el)
            acc, pacc = acc + el, pacc + el
            if pacc >= PROC_POLL then
                local ok, err = pcall(S.PollProcs, S); pacc = 0
                if not ok then noteError("PollProcs", err) end
            end
            if acc >= 1 then
                local ok, err = pcall(S.Tick, S, acc); acc = 0
                if not ok then noteError("Tick", err) end
            end
        end)
    end
end

function S:PollProcs()
    if not cur or next(procs) == nil then return end -- nothing tracked: skip the 5x/second client reads
    local inCombat = plain(UnitAffectingCombat("player")) == true
    local t = now()
    for _, w in pairs(procs) do
        local u = usable(w.spell)
        if u and not w.was and (inCombat or w.window > 10) then
            w.total = w.total + 1; w.open = t + w.window
        end
        if w.open and t > w.open + 0.5 then w.open = nil end
        w.was = u
    end
end

function S:Cast(spellID)
    if not cur or not spellID or next(procs) == nil then return end
    local name = FC:SpellName(spellID)
    local w = name and procs[name]
    if w and w.open then w.used = w.used + 1; w.open = nil end
end

local function addIdle(dt)
    -- split dt at the AFK boundary so a long frame is never double counted
    local before = idle
    idle = idle + dt
    if before >= IDLE_AFK then cur.afk = cur.afk + dt return end
    if idle <= IDLE_AFK then cur.recovery = cur.recovery + dt return end
    -- crossed the boundary now: everything in this stop so far becomes AFK
    cur.recovery = math.max(0, cur.recovery - before)
    cur.afk = cur.afk + idle
end

function S:Tick(dt)
    if not cur then return end
    cur.seconds = cur.seconds + dt
    if plain(UnitIsDeadOrGhost("player")) then cur.dead = cur.dead + dt; idle = 0; return end
    if UnitOnTaxi and plain(UnitOnTaxi("player")) then cur.taxi = cur.taxi + dt; idle = 0; return end
    if plain(UnitAffectingCombat("player")) then
        cur.combat = cur.combat + dt; idle = 0
        local ptype = num(UnitPowerType("player"))
        if ptype == 1 then
            local p, mx = num(UnitPower("player", 1)), num(UnitPowerMax("player", 1))
            if p and mx and mx > 0 then
                cur.powerSeconds = cur.powerSeconds + dt
                if p >= mx * 0.9 then cur.capped = cur.capped + dt end
            end
        end
    else
        local moving = IsPlayerMoving and plain(IsPlayerMoving())
        if moving then idle = 0 else addIdle(dt) end
    end
end

function S:Death()
    if not cur then return end
    local z = plain(GetRealZoneText and GetRealZoneText())
    table.insert(cur.deaths, { level = num(UnitLevel("player")), zone = type(z) == "string" and z or "?", time = time() })
end

function S:LevelUp()
    -- the XP bar size of the level just finished is needed to count XP across a level-up
    if not cur then return end
    S.pendingLevelMax = S.lastXPMax
    cur.levels = cur.levels + 1
end

function S:XP()
    if not cur then return end
    local xp, xpMax, lvl = num(UnitXP("player")), num(UnitXPMax("player")), num(UnitLevel("player"))
    if not (xp and xpMax and lvl) then return end
    if S.lastXP and S.lastXPMax and S.lastLevel then
        if lvl == S.lastLevel and xp >= S.lastXP then
            cur.xp = cur.xp + (xp - S.lastXP)
        elseif lvl == S.lastLevel + 1 then
            cur.xp = cur.xp + ((S.pendingLevelMax or S.lastXPMax) - S.lastXP) + xp
        end -- multi-level jumps: skip rather than guess
    end
    S.lastXP, S.lastXPMax, S.lastLevel, S.pendingLevelMax = xp, xpMax, lvl, nil
end

local function syncProcs()
    if not cur then return end
    for _, w in pairs(procs) do cur.procs[w.label] = { windows = w.total, used = w.used } end
end

function S:Finish()
    if not cur or cur.seconds < 30 then return end
    cur.finish = time(); cur.endLevel = num(UnitLevel("player"))
    syncProcs()
    local c = FC:Char()
    table.insert(c.sessions, cur)
    while #c.sessions > MAX_SESSIONS do table.remove(c.sessions, 1) end
    cur = nil
end

function S:Current() syncProcs(); return cur end

local ev = CreateFrame("Frame")
ev:RegisterEvent("PLAYER_DEAD")
ev:RegisterEvent("PLAYER_XP_UPDATE")
ev:RegisterEvent("UNIT_SPELLCAST_SUCCEEDED")
-- UNIT_SPELLCAST_SUCCEEDED fires for every unit's casts; keep the handler closure-free.
local function onEvent(e, unit, spellID)
    if e == "UNIT_SPELLCAST_SUCCEEDED" then
        if plain(unit) == "player" then S:Cast(num(spellID)) end
    elseif e == "PLAYER_DEAD" then S:Death()
    elseif e == "PLAYER_XP_UPDATE" then S:XP() end
end
ev:SetScript("OnEvent", function(_, e, unit, _, spellID)
    local ok, err = pcall(onEvent, e, unit, spellID)
    if not ok then noteError(e, err) end
end)
