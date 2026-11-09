--[[ Copyright (C) 2018-2026 TomCat's Gaming, LLC. All Rights Reserved.
     Unauthorized use, reproduction, or distribution is prohibited.
     See license.txt included with this distribution for full terms. ]]
select(2, ...).SetupGlobalFacade()

local continentIDCache = { }

LocationsInitService = { }

local function createTaskLocationCondition(task, taskGroup, objectiveIdx)
    local condition
    if (objectiveIdx) then
        condition = {
            "(ROOT_COMPLETED or not OBJECTIVE[",
            task.complete,
            "][",
            objectiveIdx,
            "])"
        }
    else
        condition = {
            "TGLOCATION_ENABLED[",
            taskGroup.idx,
            "]"
        }
    end
    if (taskGroup.condition) then
        table.insert(condition, " and (")
        table.insert(condition, taskGroup.condition)
        table.insert(condition, ")")
    end
    if (task.condition) then
        table.insert(condition, " and (")
        table.insert(condition, task.condition)
        table.insert(condition, ")")
    end
    return condition
end

local MoPMapAliases = {
    [1454] = 85, -- Orgrimmar
    [1419] = 17, -- Blasted Lands
    [1945] = 1467, -- Outland
    [1944] = 100, -- Hellfire Peninsula
    [1946] = 102, -- Zangarmarsh
    [1948] = 104, -- Shadowmoon Valley
    [1949] = 105, -- Blade's Edge
    [1951] = 107, -- Nagrand
    [1952] = 108, -- Terrokar Forest
    [1953] = 109, -- Netherstorm
    [1955] = 111, -- Shattrath City
}

local function remap(map)
    if (GetWowVersion() < 50000) then return map end
    return MoPMapAliases[map] or map
end

function LocationsInitService.InitGuide(guide, relaxCheckpoint)
    for _, step in ipairs(guide) do
        if (relaxCheckpoint) then relaxCheckpoint() end -- Relax local guides: async scan.
        step.locations = step.locations or { }
        for taskGroupIdx, taskGroup in ipairs(step) do
            taskGroup.idx = taskGroupIdx
            for _, task in ipairs(taskGroup) do
                if ((task.npc or task.fromnpc) and task.location) then
                    table.insert(step.locations,{
                        task.location,
                        npc = task.npc or task.fromnpc,
                        condition = table.concat(createTaskLocationCondition(task, taskGroup)),
                    })
                elseif (task.gameobject and task.location) then
                    table.insert(step.locations,{
                        task.location,
                        gameobject = task.gameobject,
                        condition = table.concat(createTaskLocationCondition(task, taskGroup)),
                    })
                elseif (task.complete) then
                    local questInfo = guide.quests[task.complete]
                    if (questInfo) then
                        local objectives = CreateObjectivesList(task, questInfo)
                        for _, objectiveIdx in ipairs(objectives) do
                            local objective = questInfo.objectives[objectiveIdx]
                            local locField = objective.around or objective.at or objective.location
                            if (locField) then
                                local locStrings = type(locField) == "table" and locField or { locField }
                                local condition = table.concat(createTaskLocationCondition(task, taskGroup, objectiveIdx))
                                for _, locString in ipairs(locStrings) do
                                    local location = {
                                        locString,
                                        label = objective.label,
                                        area = objective.area,
                                        gameobject = objective.gameobject,
                                        condition = condition
                                    }
                                    if (not (objective.label or objective.area)) then
                                        if (objective.around) then
                                            location.label = "Around the area"
                                        elseif (objective.at) then
                                            if (objective.clicknpc and type(objective.clicknpc) ~= "table") then
                                                location.npc = objective.clicknpc
                                            elseif (objective.kill and type(objective.kill) ~= "table") then
                                                location.npc = objective.kill
                                            elseif (objective.clickgameobject and type(objective.clickgameobject) ~= "table") then
                                                location.gameobject = objective.clickgameobject
                                            elseif (objective.speakto) then
                                                location.npc = objective.speakto
                                            end
                                        end
                                    end
                                    table.insert(step.locations, location)
                                end
                            elseif (objective.goto) then
                                local gotoField = objective.goto
                                local gotoStrings = type(gotoField) == "table" and gotoField or { gotoField }
                                local condition = table.concat(createTaskLocationCondition(task, taskGroup, objectiveIdx))
                                for _, gotoString in ipairs(gotoStrings) do
                                    table.insert(step.locations, {
                                        gotoString,
                                        label = objective.label,
                                        area = objective.area,
                                        zone = objective.zone,
                                        gameobject = objective.gameobject,
                                        condition = condition,
                                    })
                                end
                            end
                        end
                    end
                end
            end
        end
        local newLocations = { }
        for idx, location in ipairs(step.locations) do
            if (type(location) == "string" or type(location[1]) == "string") then
                local condition
                local locationAsTable
                if (type(location) == "table") then
                    condition = location.condition
                    locationAsTable = location
                    location = location[1]
                end
                local parts = { strsplit("|", location) }
                local flags = parts[1]
                local map = remap(tonumber(parts[2]))
                local polygon, x, y, radius
                if (string.find(flags, "B")) then
                    polygon = { }
                    local cx, cy = 0, 0
                    for i = 3, #parts, 2 do
                        local px, py = tonumber(parts[i]), tonumber(parts[i + 1])
                        polygon[#polygon + 1] = { px, py }
                        cx = cx + px
                        cy = cy + py
                    end
                    x = cx / #polygon
                    y = cy / #polygon
                else
                    x = tonumber(parts[3])
                    y = tonumber(parts[4])
                    radius = parts[5]
                end
                local newLocation = Mixin({
                    flags = flags,
                    originalFlags = flags,
                    map = map,
                    x = 0,
                    y = 0,
                    worldX = 0,
                    worldY = 0,
                    within = false,
                    radius = radius ~= nil and tonumber(radius) or LOCATION_DEFAULT_RADIUS,
                    polygon = polygon,
                    condition = condition,
                    label = locationAsTable and locationAsTable.label,
                    area = locationAsTable and locationAsTable.area,
                    wmoarea = locationAsTable and locationAsTable.wmoarea,
                    npc = locationAsTable and locationAsTable.npc,
                    quest = locationAsTable and locationAsTable.quest,
                    item = locationAsTable and locationAsTable.item,
                    zone = locationAsTable and locationAsTable.zone,
                    gameobject = locationAsTable and locationAsTable.gameobject,
                    conditionPassed = false,
                    onactivate = locationAsTable.onactivate,
                    onenter = locationAsTable.onenter,
                    onleave = locationAsTable.onleave,
                    root = step,
                    idx = idx
                }, LocationMixin)
                if (Flags.HasFlag(newLocation, "B")) then
                    continentIDCache[map] = continentIDCache[map]
                            or C_Map.GetWorldPosFromMapPos(map, { x = 0.5, y = 0.5 })
                    newLocation.continentID = continentIDCache[map]
                    if (Flags.HasFlag(newLocation, "W")) then
                        newLocation.worldX = x
                        newLocation.worldY = y
                        local _, centroidPos = C_Map.GetMapPosFromWorldPos(continentIDCache[map], { x = x, y = y }, map)
                        newLocation.x = centroidPos.x * 100
                        newLocation.y = centroidPos.y * 100
                        for _, vertex in ipairs(newLocation.polygon) do
                            local _, mapPos = C_Map.GetMapPosFromWorldPos(continentIDCache[map], { x = vertex[1], y = vertex[2] }, map)
                            vertex[3] = mapPos.x * 100
                            vertex[4] = mapPos.y * 100
                        end
                    else
                        newLocation.x = x
                        newLocation.y = y
                        local _, worldCentroid = C_Map.GetWorldPosFromMapPos(map, { x = x / 100, y = y / 100 })
                        newLocation.worldX = worldCentroid and worldCentroid.x or 0
                        newLocation.worldY = worldCentroid and worldCentroid.y or 0
                        for _, vertex in ipairs(newLocation.polygon) do
                            local _, worldPos = C_Map.GetWorldPosFromMapPos(map, { x = vertex[1] / 100, y = vertex[2] / 100 })
                            local mapX, mapY = vertex[1], vertex[2]
                            vertex[1] = worldPos and worldPos.x or 0
                            vertex[2] = worldPos and worldPos.y or 0
                            vertex[3] = mapX
                            vertex[4] = mapY
                        end
                    end
                elseif (Flags.HasFlag(newLocation, "W")) then
                    newLocation.worldX = x
                    newLocation.worldY = y
                    continentIDCache[map] = continentIDCache[map]
                            or C_Map.GetWorldPosFromMapPos(map, { x = 0.5, y = 0.5 })
                    newLocation.continentID = continentIDCache[map]
                    local _, mapPosition = C_Map.GetMapPosFromWorldPos(continentIDCache[map], { x = x, y = y }, map)
                    newLocation.x = mapPosition.x * 100
                    newLocation.y = mapPosition.y * 100
                else
                    newLocation.x = x
                    newLocation.y = y
                    local continentID, worldCoordinate = C_Map.GetWorldPosFromMapPos(map,
                            { x = newLocation.x / 100, y = newLocation.y / 100 })
                    newLocation.continentID = continentID
                    newLocation.worldX = worldCoordinate and worldCoordinate.x or 0
                    newLocation.worldY = worldCoordinate and worldCoordinate.y or 0
                end
                table.insert(newLocations, newLocation)
            else
                table.insert(newLocations,location)
            end
        end
        step.locations = newLocations
    end
end
