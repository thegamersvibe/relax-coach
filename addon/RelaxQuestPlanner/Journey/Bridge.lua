-- First-party route adapter; no third-party quest/content tables are bundled.
local RQP = _G.RelaxQuestPlanner or select(2, ...)
_G.RelaxQuestPlanner = RQP
RQP.Journey = RQP.Journey or {}
local Journey = RQP.Journey

local function number(value)
    if issecretvalue and issecretvalue(value) then return nil end
    local n = tonumber(value)
    if n and n == n and n ~= math.huge and n ~= -math.huge then return n end
end

local function id(value)
    local n = number(value)
    if n and n > 0 and n == math.floor(n) then return n end
end

local function text(value)
    -- Route labels are plain text, not Journey expressions or WoW hyperlinks.
    local out = tostring(value or ""):gsub("|", "||")
    return out
end

function Journey.ToGuide(route)
    if type(route) ~= "table" or type(route.steps) ~= "table" then
        return nil, "A route with steps is required"
    end
    local guide = {}
    local db = _G.RelaxQuestPlannerDB
    local skip = type(db) == "table" and type(db.skipped) == "table" and db.skipped or {}
    for index, action in ipairs(route.steps) do
        local q = id(action.q)
        if q and skip[q] then q = nil; action = nil end
        if action then
        local kind = action.kind
        if not q or (kind ~= "accept" and kind ~= "complete" and kind ~= "turnin") then
            return nil, "Invalid route quest action at step " .. tostring(index)
        end
        local quest = string.format("QUEST[%d]", math.floor(q))
        local condition = quest .. " < TURNEDIN"
        if kind == "accept" then
            for _, dependency in ipairs(action.needs or {}) do
                local need = id(dependency)
                if not need then return nil, "Invalid prerequisite quest ID" end
                if action.activeNeeds and action.activeNeeds[dependency] then
                    condition = condition .. string.format(" and QUEST[%d] >= ACCEPTED", math.floor(need))
                else
                    condition = condition .. string.format(" and QUEST[%d] >= TURNEDIN", math.floor(need))
                end
            end
        else
            condition = condition .. " and " .. quest .. " >= ACCEPTED"
        end
        local labels = { text(action.name or ("Quest " .. tostring(q))) }
        if action.who then labels[#labels + 1] = text(action.who) end
        if action.zone then labels[#labels + 1] = text(action.zone) end
        if action.below then labels[#labels + 1] = "Below level floor: your choice" end
        if action.note then labels[#labels + 1] = text(action.note) end
        if action.source then labels[#labels + 1] = "Source: " .. text(action.source) end
        local step = {
            id = string.format("%04d-%d-%s", math.floor(index), math.floor(q), kind),
            condition = condition,
            { { text = table.concat(labels, " — ") }, { [kind] = q, auto = false } },
        }
        local map, x, y = id(action.map), number(action.x), number(action.y)
        if map and x and y and x >= 0 and x <= 100 and y >= 0 and y <= 100 then
            step.locations = { { string.format("|%d|%.2f|%.2f", math.floor(map), x, y),
                label = text(action.who or action.name) } }
        end
        guide[#guide + 1] = step
        end
    end
    -- Journey requires a passing last step even when every quest is turned in
    -- or all remaining actions are gated; do not fake a quest completion.
    guide[#guide + 1] = { id = "RQP-end", completed = "false",
        { { text = "No more currently available steps in " .. text(route.title or "this route")
            .. ". Check later quests/issues in Relax Quest Planner, or rebuild after quest changes." } } }
    return guide
end

local function questFlag(name, q)
    local fn = C_QuestLog and C_QuestLog[name]
    local value = fn and fn(q)
    return not (issecretvalue and issecretvalue(value)) and value == true
end

local function firstUndone(guide)
    for _, step in ipairs(guide) do
        local action = step[1][2]
        if not action then return step.id end -- Always-passing end step.
        local q = action.accept or action.complete or action.turnin
        local done = questFlag("IsQuestFlaggedCompleted", q)
        if action.accept then done = done or questFlag("IsOnQuest", q)
        elseif action.complete then
            done = done or (questFlag("IsOnQuest", q) and
                (questFlag("ReadyForTurnIn", q) or questFlag("IsComplete", q)))
        end
        if not done then return step.id end
    end
end

function Journey.LoadRoute(route)
    if InCombatLockdown and InCombatLockdown() then return false, "Open the route after combat" end
    local api = _G.RelaxJourneyAPI
    if not api or not api.RegisterLocalGuide or not api.Goto then
        return false, "Relax Journey is not installed or needs its local-guide bridge update"
    end
    local guide, err = Journey.ToGuide(route)
    if not guide then return false, err end
    -- One replaceable session route avoids unbounded bookmarks/cache growth.
    local guideID = "RQP_ROUTE"
    local registered, reason = api.RegisterLocalGuide(guideID,
        { description = route.title or "Relax quest route" },
        function() return Journey.ToGuide(route) end)
    if not registered then return false, reason end
    local ok, result, failure = pcall(api.Goto, guideID, firstUndone(guide))
    if not ok then
        _G.RelaxQuestPlannerDB = _G.RelaxQuestPlannerDB or {}
        _G.RelaxQuestPlannerDB.lastError = tostring(result)
        return false, tostring(result)
    end
    if not result then return false, failure end
    return true, guideID
end
