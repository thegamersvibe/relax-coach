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
    return tostring(value or ""):gsub("|", "||")
end

function Journey.ToGuide(route)
    if type(route) ~= "table" or type(route.steps) ~= "table" then
        return nil, "A route with steps is required"
    end
    local guide = {}
    for index, action in ipairs(route.steps) do
        local q = id(action.q)
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
    -- Journey requires a passing last step even when every quest is turned in
    -- or all remaining actions are gated; do not fake a quest completion.
    guide[#guide + 1] = { id = "RQP-end", completed = "false",
        { { text = "No more currently available steps in " .. text(route.title or "this route")
            .. ". Check later quests/issues in Relax Quest Planner, or rebuild after quest changes." } } }
    return guide
end

function Journey.LoadRoute(route)
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
    local ok, result, failure = pcall(api.Goto, guideID, guide[1].id)
    if not ok then
        _G.RelaxQuestPlannerDB = _G.RelaxQuestPlannerDB or {}
        _G.RelaxQuestPlannerDB.lastError = tostring(result)
        return false, tostring(result)
    end
    if not result then return false, failure end
    return true, guideID
end
