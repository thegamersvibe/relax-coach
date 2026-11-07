-- Optional flavor inside Coach: event -> named reaction -> eligibility -> playback.
-- No combat gating, gameplay automation, health polling, or SoundyFun dependency.
local _, FC = ...
local R = {}
FC.Reactions = R

local MEDIA = "Interface\\AddOns\\ForeverCoach\\Media\\Reactions\\"
local CLIPS = {
    level = { "level_up.mp3", 1.66 },
    bags1 = { "bags_1.mp3", 2.62 }, bags2 = { "bags_2.mp3", 1.27 },
    death = { "death.mp3", 2.12 }, campfire = { "campfire.mp3", 34 },
}
local CAMPFIRES, HOUR = { 1289723, 1229739 }, 3600
local RETRIES, RETRY_DELAY, END_MARGIN = 3, 1, 0.15

-- Do not compare, coerce, stringify or index secret values. Checks fail closed.
local function readable(v)
    if type(issecretvalue) == "function" then
        local ok, secret = pcall(issecretvalue, v)
        if not ok or secret then return false end
    end
    if type(canaccessvalue) == "function" then
        local ok, allowed = pcall(canaccessvalue, v)
        if not ok or not allowed then return false end
    end
    return true
end
local function scalar(v, kind)
    if readable(v) and type(v) == kind then return v end
end
local function number(v)
    local n = scalar(v, "number")
    if n and n == n and n > -math.huge and n < math.huge then return n end
end
local function call(fn, ...)
    if type(fn) ~= "function" then return nil end
    local ok, value = pcall(fn, ...)
    if ok and readable(value) then return value end
end
local function now() return number(call(GetTime)) end
local function epoch() return number(call(time)) end -- one clock basis for persisted stamps
local function aliveState() return scalar(call(UnitIsDeadOrGhost, "player"), "boolean") end
local function inWorld()
    if type(IsPlayerInWorld) ~= "function" then return true end
    return scalar(call(IsPlayerInWorld), "boolean") == true
end
local function tableReadable(v)
    if not readable(v) or type(v) ~= "table" then return false end
    if type(canaccesstable) == "function" then
        local ok, allowed = pcall(canaccesstable, v)
        if not ok or not allowed then return false end
    end
    if type(issecrettable) == "function" then
        local ok, secret = pcall(issecrettable, v)
        if not ok or secret then return false end
    end
    return true
end
local function auraResult(...)
    -- RequiresNonSecretAura may return ZERO values without an error. Preserve arity.
    if select("#", ...) < 2 then return nil end
    local ok, aura = ...
    if not ok or not readable(aura) then return nil end
    if aura == nil then return false end -- explicit, ordinary nullable result
    if tableReadable(aura) then return true end -- never inspect restricted aura fields
end
local function auraPresent()
    local fn = C_UnitAuras and C_UnitAuras.GetPlayerAuraBySpellID
    if type(fn) ~= "function" then return nil end
    local unknown = false
    for _, spellID in ipairs(CAMPFIRES) do
        local present = auraResult(pcall(fn, spellID))
        if present == true then return true end
        if present == nil then unknown = true end
    end
    if unknown then return nil end
    return false
end
local function bagsFull()
    if not inWorld() or not C_Container then return nil end
    local slotsFn, freeFn = C_Container.GetContainerNumSlots, C_Container.GetContainerNumFreeSlots
    if type(slotsFn) ~= "function" or type(freeFn) ~= "function" then return nil end
    local total, free = 0, 0
    for bag = 0, 4 do -- normal inventory only; no bank, keyring or reagent bag
        local slots = number(call(slotsFn, bag))
        if not slots or slots < 0 or slots % 1 ~= 0 then return nil end
        if slots > 0 then
            local ok, available, family = pcall(freeFn, bag)
            available, family = number(available), number(family)
            if not ok or not available or not family or available < 0 or available > slots then return nil end
            if available % 1 ~= 0 or family < 0 or family % 1 ~= 0 then return nil end
            if family == 0 then total, free = total + slots, free + available end
        end
    end
    if total > 0 then return free == 0, free end -- zero capacity is not proof of fullness
end
local function guarded(where, fn, ...)
    local ok, err = pcall(fn, ...)
    if not ok then
        FC:Char().lastError = { where = "Reactions:" .. where, err = scalar(err, "string") or "unreadable reaction error",
            time = epoch(), version = FC.VERSION }
    end
end

function R:Enabled() return self.ready and ForeverCoachDB.reactionsEnabled == true end
function R:CampfireEligible(at)
    local t = at or epoch()
    if not t or t <= 0 then return false end
    local stamp = self.store.campfireLastPlayedAt
    if stamp == nil then return true end
    stamp = number(stamp)
    return stamp ~= nil and stamp > 0 and t >= stamp and t - stamp >= HOUR
end
function R:Wake()
    self.frame:SetScript("OnUpdate", self.update)
end
function R:Stop()
    local a = self.active
    self.active = nil -- clear ownership before a stop can generate a completion event
    if a and a.handle and type(StopSound) == "function" then pcall(StopSound, a.handle) end
    self.queue, self.pending = {}, {}
    self.popupUntil = nil
    if self.popup then self.popup:Hide() end
    self.frame:SetScript("OnUpdate", nil)
end
function R:ShowPopup(t)
    if not self.popup then
        local f = CreateFrame("Frame", nil, UIParent)
        f:SetSize(512, 288); f:SetPoint("TOP", UIParent, "TOP", 0, -132)
        f:SetFrameStrata("HIGH"); f:EnableMouse(false)
        f:EnableMouseWheel(false)
        local tex = f:CreateTexture(nil, "ARTWORK")
        tex:SetAllPoints(); tex:SetTexture(MEDIA .. "level_up.tga")
        tex:SetTexCoord(0, 1, 0, 0.5625) -- full 1024x576 artwork in a padded 1024x1024 TGA
        self.popup = f
    end
    self.popup:Show()
    self.popupUntil = t + 3
end
function R:Request(kind, token)
    if not self:Enabled() or self.failed[kind] == token then return end
    local pending = self.pending[kind]
    if pending then
        pending.token = token -- latest episode, same FIFO position; one pending reaction per kind
        return
    end
    if self.active and self.active.kind == kind and self.active.token == token then return end
    local job = { kind = kind, token = token, tries = 0 }
    self.pending[kind] = job
    self.queue[#self.queue + 1] = job
    self:Wake()
    self:Pump()
end
function R:RemoveFirst()
    local job = table.remove(self.queue, 1)
    if job then self.pending[job.kind] = nil end
    return job
end
function R:Pump()
    if not self:Enabled() or self.active or not self.world or not inWorld() then return end
    local t = now()
    if not t then return end
    local job = self.queue[1]
    if not job or (job.retryAt and t < job.retryAt) then return end
    local campStamp = job.kind == "campfire" and epoch() or nil
    if job.kind == "campfire" and (not campStamp or not self:CampfireEligible(campStamp)) then
        self:RemoveFirst()
        return
    end
    local clip = CLIPS[job.kind == "bags" and ("bags" .. self.store.bagNext) or job.kind]
    if not clip then self:RemoveFirst() return end
    local ok, played, handle = false, false, nil
    if type(PlaySoundFile) == "function" and type(StopSound) == "function" then
        ok, played, handle = pcall(PlaySoundFile, MEDIA .. clip[1], "Master")
    end
    if ok and scalar(played, "boolean") == true then
        self:RemoveFirst()
        handle = number(handle)
        if handle and (handle <= 0 or handle % 1 ~= 0) then handle = nil end
        -- A success without a usable handle still must not be retried (could duplicate audio).
        self.active = { kind = job.kind, token = job.token, handle = handle, ends = t + clip[2] + END_MARGIN }
        if job.kind == "bags" then
            self.store.bagNext = self.store.bagNext == 1 and 2 or 1
            if self.full == true and self.bagEpisode == job.token then self.store.bagsAnnounced = true end
        elseif job.kind == "campfire" then
            self.store.campfireLastPlayedAt = campStamp
        elseif job.kind == "level" then
            self:ShowPopup(t)
        end
    else
        job.tries = job.tries + 1
        if job.tries >= RETRIES then
            self.failed[job.kind] = job.token
            self:RemoveFirst()
        else
            job.retryAt = t + RETRY_DELAY
        end
    end
end
function R:Tick()
    if not self:Enabled() then self:Stop() return end
    local t = now()
    if not t then return end
    if self.popupUntil and t >= self.popupUntil then self.popup:Hide(); self.popupUntil = nil end
    if self.active and t >= self.active.ends then
        local handle = self.active.handle
        self.active = nil
        -- Measured-duration fallback; file completion events are not assumed to be delivered.
        if handle and type(StopSound) == "function" then pcall(StopSound, handle) end
    end
    self:Pump()
    if not self.active and #self.queue == 0 and not self.popupUntil then self.frame:SetScript("OnUpdate", nil) end
end
function R:CheckBags()
    if not self.world then return end
    local full, free = bagsFull()
    if full == nil then return end
    if self.store.bagErrorActive then
        local previous = number(self.store.bagErrorFree)
        if previous and free > previous then
            self.store.bagErrorActive, self.store.bagErrorFree = nil, nil
        else
            -- A rejected transaction can need more than the reported spare capacity.
            -- Unchanged counts are not new space; unknown recovery sets a baseline.
            self.store.bagErrorFree = free
            full = true
        end
    end
    if self.bagBaselinePending then self.bagSuppressed = full end
    self.bagBaselinePending = nil
    local was = self.full
    self.full = full
    if not full then
        self.store.bagsAnnounced, self.bagSuppressed = false, false
    elseif was == false then
        self.bagEpisode = self.bagEpisode + 1
        if not self:Enabled() then self.bagSuppressed = true end
        -- Counts only track/rearm episodes; the actual full-inventory error triggers audio.
    elseif not self:Enabled() then
        self.bagSuppressed = true
    end
    return free
end
function R:InventoryFull(message)
    if not self.world or not inWorld() then return end
    local expected = scalar(ERR_INV_FULL, "string")
    if not expected or scalar(message, "string") ~= expected then return end
    -- Only the game's localized rejection triggers audio; counts merely rearm it.
    local free = self:CheckBags()
    if not self.store.bagErrorActive then
        self.store.bagErrorActive, self.store.bagErrorFree = true, free
    end
    if self.full ~= true then
        self.full = true
        self.bagEpisode = self.bagEpisode + 1
    end
    if not self:Enabled() or self.bagBaselinePending then self.bagSuppressed = true end
    if not self.store.bagsAnnounced and not self.bagSuppressed then self:Request("bags", self.bagEpisode) end
end
function R:CheckCampfire(baseline)
    if not self.world or not inWorld() then return end
    local present = auraPresent()
    if present == nil then return end -- retain last known presence across restricted/failed queries
    baseline = baseline or self.campBaselinePending
    self.campBaselinePending = nil
    local was = self.campPresent
    self.campPresent = present
    if present and was ~= true and not baseline then
        self.campEpisode = self.campEpisode + 1
        if self:CampfireEligible() then self:Request("campfire", self.campEpisode) end
    end
end
function R:SyncLife()
    local dead = aliveState()
    if dead ~= nil then self.dead = dead end
end
function R:SetEnabled(enabled)
    if not self.ready then return end
    enabled = enabled == true
    if ForeverCoachDB.reactionsEnabled == enabled then return end
    ForeverCoachDB.reactionsEnabled = enabled
    self:Stop()
    -- Unknown observations defer each silent baseline independently across the mute boundary.
    -- Unobserved gains before/after unmute cannot be distinguished; the first known read is silent.
    self.bagBaselinePending, self.campBaselinePending = true, true
    self:CheckBags()
    if self.full == true then self.bagSuppressed = true end
    self:CheckCampfire(true)
    self:SyncLife()
    local level = number(call(UnitLevel, "player"))
    if level and (not self.lastLevel or level > self.lastLevel) then self.lastLevel = level end
end
function R:Init()
    if self.ready or type(ForeverCoachDB) ~= "table" then return end
    ForeverCoachDB.reactionsEnabled = ForeverCoachDB.reactionsEnabled ~= false
    local c = FC:Char()
    if type(c.reactions) ~= "table" then c.reactions = {} end
    self.store = c.reactions
    if self.store.bagNext ~= 1 and self.store.bagNext ~= 2 then self.store.bagNext = 1 end
    self.store.bagsAnnounced = self.store.bagsAnnounced == true
    self.queue, self.pending, self.failed = {}, {}, {}
    self.bagEpisode, self.deathEpisode, self.campEpisode = 0, 0, 0
    self.lastLevel = number(call(UnitLevel, "player"))
    self:SyncLife()
    self.ready, self.world = true, inWorld()
    self:CheckBags() -- full bags on load are silent; an actual full-error can still catch them
    self:CheckCampfire() -- initial aura detected; persisted guard prevents reload repeats
end
function R:OnEvent(event, first, second)
    if event == "PLAYER_LOGIN" then self:Init() return end
    if not self.ready then return end
    if event == "PLAYER_LOGOUT" then self:Stop(); self.ready = false
    elseif event == "PLAYER_LEAVING_WORLD" then self.world = false
    elseif event == "PLAYER_ENTERING_WORLD" then
        self.world = true; self:SyncLife(); self:CheckBags(); self:CheckCampfire()
    elseif event == "BAG_UPDATE_DELAYED" then self:CheckBags()
    elseif event == "UI_ERROR_MESSAGE" then self:InventoryFull(second)
    elseif event == "UNIT_AURA" then
        if scalar(first, "string") == "player" then self:CheckCampfire() end
    elseif event == "PLAYER_DEAD" then
        if self.dead ~= true then
            self.dead = true; self.deathEpisode = self.deathEpisode + 1
            self:Request("death", self.deathEpisode)
        end
    elseif event == "PLAYER_ALIVE" or event == "PLAYER_UNGHOST" then self:SyncLife()
    elseif event == "PLAYER_LEVEL_UP" then
        local level = number(first) -- the event precedes UnitLevel's new value
        if level and level > 0 and level % 1 == 0 and (not self.lastLevel or level > self.lastLevel) then
            self.lastLevel = level
            self:Request("level", level)
        end
    elseif event == "SOUNDKIT_FINISHED" then
        local handle = number(first)
        if handle and self.active and self.active.handle == handle then
            self.active = nil; self:Tick()
        end
    end
end

R.frame = CreateFrame("Frame")
R.update = function() guarded("tick", R.Tick, R) end
for _, event in ipairs({ "PLAYER_LOGIN", "PLAYER_LOGOUT", "PLAYER_ENTERING_WORLD", "PLAYER_LEAVING_WORLD",
    "BAG_UPDATE_DELAYED", "UI_ERROR_MESSAGE", "PLAYER_DEAD", "PLAYER_ALIVE", "PLAYER_UNGHOST", "PLAYER_LEVEL_UP" }) do
    R.frame:RegisterEvent(event)
end
-- Completion only accelerates dispatch; the measured-duration fallback is always present.
pcall(R.frame.RegisterEvent, R.frame, "SOUNDKIT_FINISHED")
R.frame:RegisterUnitEvent("UNIT_AURA", "player")
R.frame:SetScript("OnEvent", function(_, event, ...)
    guarded(event, R.OnEvent, R, event, ...)
end)
