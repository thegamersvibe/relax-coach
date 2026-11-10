-- First-party, runtime-only guide picker. Journey owns decoding/integrity checks.
local RQP = _G.RelaxQuestPlanner or select(2, ...)
_G.RelaxQuestPlanner = RQP
RQP.Journey = RQP.Journey or {}
local Journey = RQP.Journey

local function numeric(value)
    if issecretvalue and issecretvalue(value) then return nil end
    local n = tonumber(value)
    if n and n == n and n ~= math.huge and n ~= -math.huge then return n end
end

local function read(fn, ...)
    if type(fn) ~= "function" then return nil end
    local ok, value = pcall(fn, ...)
    if not ok or (issecretvalue and issecretvalue(value)) then return nil end
    return value
end

-- Engine/Data are first-party Lua and cooperate through Engine.Checkpoint.
-- Their calls must not sit inside pcall: Lua 5.1 cannot yield through it.
-- coroutine.resume protects async execution; GotoBest protects the sync path.
local function engineRead(fn, ...)
    if type(fn) == "function" then return fn(...) end
end

local function recordError(err)
    _G.RelaxQuestPlannerDB = _G.RelaxQuestPlannerDB or {}
    if RQP.SetError then RQP.SetError(err) else _G.RelaxQuestPlannerDB.lastError = tostring(err) end
end

local function snapshot()
    local P = RQP.Player and read(RQP.Player.Snapshot)
    if type(P) ~= "table" then
        if not C_QuestLog then return nil, "Quest log API unavailable" end
        P = { level = read(UnitLevel, "player"), faction = read(UnitFactionGroup, "player"),
            done = {}, log = {}, ready = {}, query = true,
            trivial = read(C_QuestLog.GetTrivialRange) }
        for _, q in ipairs(read(C_QuestLog.GetAllCompletedQuestIDs) or {}) do
            if numeric(q) then P.done[q] = true end
        end
        local count = numeric(read(C_QuestLog.GetNumQuestLogEntries)) or 0
        for index = 1, count do
            local info = read(C_QuestLog.GetInfo, index)
            if info and not info.isHeader and numeric(info.questID) then
                P.log[info.questID] = true
            end
        end
    end
    local level = numeric(P.level)
    if not level or level < 1 then return nil, "Player level is unavailable" end
    P.level = level
    P.done, P.log, P.ready = P.done or {}, P.log or {}, P.ready or {}
    local mode = (_G.RelaxQuestPlannerDB or {}).mode or "game"
    if mode ~= "strict" and mode ~= "all" then mode = "game" end
    local trivial = numeric(P.trivial) or 5
    return P, { mode = mode, floor = mode == "all" and 0 or level - (mode == "strict" and 5 or trivial) }
end

local function band(info, P, opts)
    if info.stub then return nil, "stub" end
    if info.faction and info.faction ~= "Both" and info.faction ~= P.faction then return nil, "faction" end
    local low, high = tostring(info.levels or ""):match("^(%d+)%s*%-%s*(%d+)$")
    low, high = tonumber(low), tonumber(high)
    if not low or not high or high < low then return nil, "unknown level band" end
    if high < opts.floor then return nil, "below level floor" end
    if opts.mode ~= "all" and low > P.level + 2 then return nil, "too far ahead" end
    return { low = low, high = high }
end

local function questState(q, P, opts, cache)
    if cache[q] then return cache[q] end
    if P.query then
        P.done[q] = P.done[q] or read(C_QuestLog.IsQuestFlaggedCompleted, q) == true
        P.log[q] = P.log[q] or read(C_QuestLog.IsOnQuest, q) == true
        P.ready[q] = read(C_QuestLog.ReadyForTurnIn, q) == true or read(C_QuestLog.IsComplete, q) == true
    end
    local row = RQP.Data and engineRead(RQP.Data.Quest, q)
    local level = row and numeric(row.qLevel)
    if level and level <= 0 then level = nil end
    local state
    if P.done[q] then state = "done"
    elseif row and RQP.Engine and RQP.Engine.State then state = engineRead(RQP.Engine.State, q, P, opts)
    elseif P.log[q] then state = "inlog"
    else state = "unverified" end
    local known = state ~= "unverified" and state ~= nil and level ~= nil
    local result = { state = state or "unknownGate", level = level, known = known,
        levelOK = not level or level >= opts.floor }
    cache[q] = result
    return result
end

local function actionable(action, P, opts, cache)
    local q = action.questID
    local state = questState(q, P, opts, cache)
    if action.active ~= true or action.completed or action.optional or P.done[q] or not state.levelOK then return false end
    if action.alternatives then
        for _, other in ipairs(action.alternatives) do
            questState(other, P, opts, cache)
            if P.done[other] or P.log[other] then return false end
        end
    end
    if action.kind == "accept" then
        return not P.log[q] and (state.state == "available" or state.state == "unverified")
    elseif action.kind == "complete" then
        return P.log[q] and not P.ready[q]
    elseif action.kind == "turnin" then
        return P.log[q] and P.ready[q]
    end
    return false
end

local fitByDelta = { [-5]=0.2, [-4]=0.4, [-3]=0.6, [-2]=0.8, [-1]=0.9,
    [0]=1, [1]=1, [2]=0.8, [3]=0.6, [4]=0.4 }
local function rankGuide(info, scan, P, opts, cache, unlockCache, checkpoint)
    local range = band(info, P, opts)
    if not range then return nil end
    local all, usable, inlog, unknown, done, future = {}, {}, {}, {}, {}, {}
    local startStepID
    for _, step in ipairs(scan.steps) do
        for _, action in ipairs(step.actions) do
            local q = action.questID
            local state = questState(q, P, opts, cache)
            -- Keep turned-in quests in the denominator, but never foreign class,
            -- race or policy-excluded work. An unknown condition is not a start.
            if state.state ~= "race" and state.state ~= "class" and state.levelOK then
                all[q] = true
                if P.done[q] then done[q] = true end
            end
            if actionable(action, P, opts, cache) then
                if not startStepID then startStepID = step.id end
                usable[q] = true
                if P.log[q] then inlog[q] = true end
                if not state.known then unknown[q] = true end
            end
            if checkpoint then checkpoint() end
        end
    end
    if not startStepID then return nil end
    local function count(set)
        local n = 0
        for _ in pairs(set) do n = n + 1 end
        return n
    end
    local total, available, completed, overlap = count(all), count(usable), count(done), count(inlog)
    local hasUnlocks = RQP.Engine and RQP.Engine.Unlocks
    local checked = 0
    -- Bounded marginal look-ahead; reuse expensive engine answers across guides.
    local ordered = {}
    for q in pairs(usable) do ordered[#ordered + 1] = q end
    table.sort(ordered)
    for _, q in ipairs(ordered) do
        if hasUnlocks and cache[q].known and checked < 20 then
            if not unlockCache[q] then unlockCache[q] = engineRead(hasUnlocks, q, P) or {} end
            for _, nextID in ipairs(unlockCache[q]) do
                if not P.done[nextID] then future[nextID] = true end
            end
            checked = checked + 1
        end
        if checkpoint then checkpoint() end
    end
    local distance = math.max(range.low - P.level, 0, P.level - range.high)
    local fit = math.max(0, 1 - distance / 6)
    local workFit, leveled = 0, 0

    for q in pairs(usable) do
        local level = cache[q].level
        if level then
            workFit = workFit + (fitByDelta[math.floor(level - P.level)] or 0.2)
            leveled = leveled + 1
        end
    end
    if leveled > 0 then fit = workFit / leveled end
    local futureCount = count(future)
    local score = 30 * available / math.max(1, total) + 20 * math.min(1, overlap / 5)
        + 20 * fit + 20 * math.min(1, futureCount / 10) - 15 * completed / math.max(1, total)
        - 15 * count(unknown) / math.max(1, total)
    local reasons = {
        string.format("%d remaining actionable quests; %d already turned in", math.floor(available), math.floor(completed)),
        string.format("%d quests in your log; guide levels %d–%d", math.floor(overlap), math.floor(range.low), math.floor(range.high)),
        string.format("Level mode: %s (floor %d)", opts.mode, math.floor(opts.floor)),
    }
    if hasUnlocks then
        reasons[#reasons + 1] = string.format("%d future quest unlocks (up to 20 current quests checked)", math.floor(futureCount))
    else
        reasons[#reasons + 1] = "Future unlocks not verified: quest engine unavailable"
    end
    local provisional = count(unknown) > 0
    if provisional then
        reasons[#reasons + 1] = "Provisional: prerequisites/quest levels not verified; quest-log and guide-band fallback"
    end
    reasons[#reasons + 1] = "Suggested first undone quest step; review travel/setup instructions before it"
    return { guideID = info.guideID, description = info.description or info.guideID,
        score = math.floor(score * 1000 + 0.5) / 1000, reasons = reasons,
        startStepID = startStepID, provisional = provisional, logOverlap = overlap }
end

local workerFrame, activeCallback, worker
local function deliver(callback, result, reason)
    local ok, err = pcall(callback, result, reason)
    if not ok then recordError(err) end
end

-- cb(top3, reason): an empty list explains missing/stub/blocked content. No auto-Goto.
function Journey.PickGuide(cb)
    if type(cb) ~= "function" then return false, "Callback required" end
    if not CreateFrame then return false, "Frame scheduler unavailable" end
    if activeCallback then deliver(activeCallback, {}, "Scan superseded by a new request") end
    activeCallback = cb
    workerFrame = workerFrame or CreateFrame("Frame")
    worker = coroutine.create(function()
        local api = _G.RelaxJourneyAPI
        if not api or not api.ListCatalogGuides or not api.ScanGuideQuests then
            return {}, "Relax Journey is not installed or needs its bridge update"
        end
        local P, opts = snapshot()
        if not P then return {}, opts end
        local candidates, cache, unlockCache = {}, {}, {}
        local skipped, stubs = 0, 0
        local checkpoint = coroutine.yield
        for _, info in ipairs(api.ListCatalogGuides()) do
            if info.stub then stubs = stubs + 1 end
            if band(info, P, opts) then
                local scan = api.ScanGuideQuests(info.guideID, checkpoint)
                if scan then
                    local result = rankGuide(info, scan, P, opts, cache, unlockCache, checkpoint)
                    if result then candidates[#candidates + 1] = result end
                else skipped = skipped + 1 end
            end
            checkpoint()
        end
        table.sort(candidates, function(a, b)
            if a.provisional ~= b.provisional then return not a.provisional end
            if a.score ~= b.score then return a.score > b.score end
            if a.logOverlap ~= b.logOverlap then return a.logOverlap > b.logOverlap end
            return a.guideID < b.guideID
        end)
        while #candidates > 3 do table.remove(candidates) end
        local reason
        if #candidates == 0 then
            reason = string.format("No actionable installed guide matches this level mode (%d stubs; %d unreadable guides). Try manual guides or Dungeon quests.", stubs, skipped)
        end
        return candidates, reason
    end)
    local function tick()
        local start = debugprofilestop and debugprofilestop() or 0
        -- Keep headroom below 8 ms. Only Journey's signed decode stays atomic;
        -- normalization, location/button setup and engine graph loops cooperate.
        local operations = 0
        local function engineCheckpoint()
            operations = operations + 1
            if operations >= 40 or (debugprofilestop and debugprofilestop() - start >= 4) then
                operations = 0
                coroutine.yield()
            end
        end
        for _ = 1, 80 do
            local engine = RQP.Engine
            local previous = engine and engine.Checkpoint
            if engine then engine.Checkpoint = engineCheckpoint end
            local ok, result, reason = coroutine.resume(worker)
            if engine then engine.Checkpoint = previous end
            if not ok or coroutine.status(worker) == "dead" then
                workerFrame:SetScript("OnUpdate", nil)
                local callback = activeCallback
                activeCallback, worker = nil, nil
                if not ok then recordError(result); deliver(callback, {}, "Guide scan failed: " .. tostring(result))
                else deliver(callback, result, reason) end
                return
            end
            if debugprofilestop and debugprofilestop() - start >= 4 then return end
        end
    end
    workerFrame:SetScript("OnUpdate", function()
        local ok, err = pcall(tick)
        if not ok then
            workerFrame:SetScript("OnUpdate", nil)
            recordError(err)
            local callback = activeCallback
            activeCallback, worker = nil, nil
            if callback then deliver(callback, {}, "Guide scan failed: " .. tostring(err)) end
        end
    end)
    return true
end

function Journey.GotoBest(choice)
    if type(choice) ~= "table" or type(choice.guideID) ~= "string" then return false, "Choose a guide first" end
    if InCombatLockdown and InCombatLockdown() then return false, "Load the guide after combat" end
    local api = _G.RelaxJourneyAPI
    if not api or not api.Goto then return false, "Relax Journey is unavailable" end
    local function apply()
        local P, opts = snapshot()
        if not P then return false, opts end
        for _, info in ipairs(api.ListCatalogGuides()) do
            if info.guideID == choice.guideID and band(info, P, opts) then
                local scan, err = api.ScanGuideQuests(info.guideID)
                if not scan then return false, err end
                -- Recheck conditions, completion and prerequisites at the click;
                -- scores need not be recomputed just to find the first valid step.
                local cache = {}
                for _, step in ipairs(scan.steps) do
                    for _, action in ipairs(step.actions) do
                        if actionable(action, P, opts, cache) then return api.Goto(info.guideID, step.id) end
                    end
                end
            end
        end
        return false, "Guide no longer has an actionable step; scan again"
    end
    local ok, result, reason = pcall(apply)
    if not ok then recordError(result); return false, tostring(result) end
    return result, reason
end
