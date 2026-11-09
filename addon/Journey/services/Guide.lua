--[[ Copyright (C) 2018-2026 TomCat's Gaming, LLC. All Rights Reserved.
     Unauthorized use, reproduction, or distribution is prohibited.
     See license.txt included with this distribution for full terms. ]]
select(2, ...).SetupGlobalFacade()

local guideCache
local guideInfoLookup
local SetupHierarchicalReferences

function SetupHierarchicalReferences(node, root, parent)
    root = root or node
    parent = parent or root
    if (type(node) == "table") then
        if (node ~= root) then
            node.root = root
        end
        if (node ~= parent) then
            node.parent = parent
        end
        for _, child in ipairs(node) do
            if (child ~= root and child ~= parent) then
                SetupHierarchicalReferences(child, root, node)
            end
        end
    end
end

GuideService = { }

function GuideService.Init()
    guideCache = { }
end

-- Relax local guides: invalidate only our separate, unsigned namespace.
function GuideService.InvalidateRelaxLocalGuide(guideID)
    if (guideCache and type(guideID) == "string" and guideID:match("^RQP_")) then
        guideCache[guideID] = nil
    end
end

function GuideService.GetGuide(guideID, relaxCheckpoint)
    -- Relax local guides: catalog admission below is unchanged for catalog IDs.
    local localGuide = Brand.GetRelaxLocalGuide and Brand.GetRelaxLocalGuide(guideID)
    if (not localGuide) then
    if (GuideModules.IsExpired()) then return nil end
    if (CatalogService.CheckCompatibility()) then return nil end
    end -- Relax local guides
    local guide = guideCache[guideID]
    if (not guide) then
        GuideModules.Reload()
        -- Relax local guides: feed the same normalization, never the signed loader.
        local guideInfo
        if (localGuide) then
            guideInfo = localGuide.info
            guide = localGuide.buildFn()
            if (type(guide) ~= "table" or #guide == 0) then return nil end
        else
        if (not guideInfoLookup) then
            guideInfoLookup = { }
            for _, guideInfo in ipairs(CatalogService.GetGuideInfos()) do
                guideInfoLookup[guideInfo.guideID] = guideInfo
            end
        end
        guideInfo = guideInfoLookup[guideID]
        if (not guideInfo) then
            guideID = "B" .. string.sub(guideID,2)
            guideInfo = guideInfoLookup[guideID]
            if (not guideInfo) then
                return
            end
        end
        local guideTemplate = CatalogService.GetGuideTemplate(guideID)
        if (not guideTemplate) then return end
        if (CatalogService.IsExpired(guideID)) then
            print("|cFFFF4444[" .. addonTitle .. "]|r This guide has expired. Run the " .. addonTitle .. " Manager to refresh your guides.")
            return nil
        end
        local decryptKey = CatalogService.GetDecryptKey(guideID)
        local expectedHash = CatalogService.GetContentHash(guideID)
        local failReason
        guide, failReason = LoadHTMLData(guideTemplate, MERCHANT_TYPE, decryptKey, expectedHash)
        if (not guide and failReason == "hash_mismatch") then
            CatalogService.Reload()
            guideTemplate = CatalogService.GetGuideTemplate(guideID)
            if (guideTemplate) then
                decryptKey = CatalogService.GetDecryptKey(guideID)
                expectedHash = CatalogService.GetContentHash(guideID)
                guide, failReason = LoadHTMLData(guideTemplate, MERCHANT_TYPE, decryptKey, expectedHash)
            end
        end
        if (not guide) then
            if (failReason == "hash_mismatch") then
                local slot = CatalogService.GetSlotInfo(guideID)
                if (slot and slot.prefix == "External") then
                    print("|cFFFF4444[" .. addonTitle .. "]|r Guide data integrity check failed. Please re-download your guide content.")
                else
                    print("|cFFFF4444[" .. addonTitle .. "]|r Guide data integrity check failed. Please reinstall the addon.")
                end
            end
            return nil
        end
        end -- Relax local guides: shared normalization starts here.
        guide.info = guideInfo
        assert(guide.info, ("GuideInfo is not present: %s"):format(guideID))
        guide.id = guideID
        guide.quests = guide.quests or { }
        for _, step in ipairs(guide) do
            if (relaxCheckpoint) then relaxCheckpoint() end -- Relax local guides: async scan.
            if (step.quests) then
                for questID, questInfo in pairs(step.quests) do
                    guide.quests[questID] = questInfo
                    step.quests = nil
                end
            end
            for _, taskGroup in ipairs(step) do
                for taskIdx, task in ipairs(taskGroup) do
                    if (task.seq) then
                        if (type(task.seq) == "number") then
                            local newSeq = { }
                            for seqNum = 1, task.seq do
                                newSeq[seqNum] = seqNum
                            end
                            task.seq = newSeq
                        end
                        for seqIdx = #task.seq, 1, -1 do
                            local objectiveIdx = task.seq[seqIdx]
                            local newTask = {
                                complete = task.complete,
                                objective = objectiveIdx
                            }
                            local condition = { }
                            if (task.condition) then
                                table.insert(condition, "(")
                                table.insert(condition, task.condition)
                                table.insert(condition, ") and ")
                            end
                            if (seqIdx == 1) then
                                table.insert(condition, "(")
                                table.insert(condition, "not OBJECTIVE[")
                                table.insert(condition, task.complete)
                                table.insert(condition, "][")
                                table.insert(condition, task.seq[seqIdx])
                                table.insert(condition, "] or QUEST[")
                                table.insert(condition, task.complete)
                                table.insert(condition, "] >= COMPLETED)")
                                newTask.condition = table.concat(condition)
                                taskGroup[taskIdx] = newTask
                                task = newTask
                            else
                                table.insert(condition, "(")
                                for seqIdxB = 1, seqIdx - 1 do
                                    table.insert(condition, "OBJECTIVE[")
                                    table.insert(condition, task.complete)
                                    table.insert(condition, "][")
                                    table.insert(condition, task.seq[seqIdxB])
                                    table.insert(condition, "] and ")
                                end
                                table.insert(condition, "not OBJECTIVE[")
                                table.insert(condition, task.complete)
                                table.insert(condition, "][")
                                table.insert(condition, task.seq[seqIdx])
                                table.insert(condition, "])")
                                newTask.condition = table.concat(condition)
                                table.insert(taskGroup, taskIdx + 1, newTask)
                            end
                        end
                    end
                end
            end
        end
        local function deepCopy(src)
            if (type(src) ~= "table") then return src end
            local copy = { }
            for k, v in pairs(src) do
                copy[k] = deepCopy(v)
            end
            return copy
        end
        local repeatingGroups = { }
        for _, step in ipairs(guide) do
            if (relaxCheckpoint) then relaxCheckpoint() end -- Relax local guides: async scan.
            for i = #step, 1, -1 do
                local taskGroup = step[i]
                if (type(taskGroup) == "table" and taskGroup.repeatuntil) then
                    table.insert(repeatingGroups, {
                        source = taskGroup,
                        untilID = taskGroup.repeatuntil,
                    })
                    taskGroup.repeatuntil = nil
                    table.remove(step, i)
                end
            end
            for _, rg in ipairs(repeatingGroups) do
                local alreadyPresent = false
                for _, tg in ipairs(step) do
                    if (tg == rg.source) then
                        alreadyPresent = true
                        break
                    end
                end
                if (not alreadyPresent) then
                    table.insert(step, deepCopy(rg.source))
                end
            end
            for i = #repeatingGroups, 1, -1 do
                if (repeatingGroups[i].untilID == step.id) then
                    table.remove(repeatingGroups, i)
                end
            end
        end
        for _, step in ipairs(guide) do
            if (relaxCheckpoint) then relaxCheckpoint() end -- Relax local guides: async scan.
            for _, taskGroup in ipairs(step) do
                for taskIdx = #taskGroup, 1, -1 do
                    local task = taskGroup[taskIdx]
                    if (task.complete and task.startduring) then
                        local quests = task.startduring
                        if (type(quests) == "number") then
                            quests = { quests }
                        end
                        local startParts = { }
                        local completeParts = { }
                        for i, questID in ipairs(quests) do
                            if (i > 1) then
                                table.insert(startParts, " or ")
                                table.insert(completeParts, " and ")
                            end
                            table.insert(startParts, "QUEST[")
                            table.insert(startParts, questID)
                            table.insert(startParts, "] < COMPLETED")
                            table.insert(completeParts, "QUEST[")
                            table.insert(completeParts, questID)
                            table.insert(completeParts, "] >= COMPLETED")
                        end
                        local startCond = table.concat(startParts)
                        local completeCond = table.concat(completeParts)
                        if (#quests > 1) then
                            startCond = "(" .. startCond .. ")"
                        end
                        if (task.condition) then
                            startCond = "(" .. task.condition .. ") and " .. startCond
                            completeCond = "(" .. task.condition .. ") and " .. completeCond
                        end
                        local startTask = { startwork = task.complete, condition = startCond }
                        if (task.objective) then startTask.objective = task.objective end
                        if (task.objectives) then startTask.objectives = task.objectives end
                        task.condition = completeCond
                        task.startduring = nil
                        table.insert(taskGroup, taskIdx, startTask)
                    end
                end
            end
        end
        local locationFields = { "around", "at", "goto", "location" }
        for _, questInfo in pairs(guide.quests) do
            if (questInfo.locations and questInfo.objectives) then
                for _, objective in ipairs(questInfo.objectives) do
                    for _, field in ipairs(locationFields) do
                        local val = objective[field]
                        if (val) then
                            if (type(val) == "number") then
                                objective[field] = questInfo.locations[val]
                            elseif (type(val) == "table") then
                                local resolved = { }
                                for _, idx in ipairs(val) do
                                    resolved[#resolved + 1] = questInfo.locations[idx]
                                end
                                objective[field] = resolved
                            end
                        end
                    end
                end
            end
        end
        LocationsInitService.InitGuide(guide, relaxCheckpoint)
        if (relaxCheckpoint) then relaxCheckpoint() end -- Relax local guides: async scan.
        ButtonGenerationService.GenerateButtons(guide, relaxCheckpoint)
        if (relaxCheckpoint) then relaxCheckpoint() end -- Relax local guides: async scan.
        guide.stepLUT = { }
        for stepIdx, step in ipairs(guide) do
            if (relaxCheckpoint) then relaxCheckpoint() end -- Relax local guides: async scan.
            step.idx = stepIdx
            guide.stepLUT[step.id] = step
            for taskGroupIdx, taskGroup in ipairs(step) do
                taskGroup.idx = taskGroupIdx
                local lastInsert = nil
                for taskIdx, task in ipairs(taskGroup) do
                    if (type(task) == "string") then
                        task = {
                            text = task
                        }
                        taskGroup[taskIdx] = task
                    else
                        if (not lastInsert or taskGroup[taskIdx-1] ~= lastInsert) then
                            lastInsert = QuestTextService.TryCreateTextTask(task,guide,step.id)
                            if (lastInsert) then
                                table.insert(taskGroup, taskIdx, lastInsert)
                                task = lastInsert
                            end
                        end
                    end
                    local multitext = true
                    for key in pairs(task) do
                        if (not (MULTITEXT_IGNORED_KEYS[key] or type(key) == "number")) then
                            multitext = nil
                        end
                    end
                    if (multitext) then
                        task.text = "multitext"
                    end
                    task.taskType = GetTaskType(task)
                    if (task.hintbutton) then
                        if (task.condition) then
                            task.condition = string.format("(%s) and HINTLEVEL <= %s", task.condition, task.hintbutton)
                        else
                            task.condition = string.format("HINTLEVEL <= %s", task.hintbutton)
                        end
                    end
                end
                if (taskGroup.buttons) then
                    for _, buttonRef in ipairs(taskGroup.buttons) do
                        buttonRef.parent = taskGroup
                        buttonRef.root = step
                    end
                end
            end
            SetupHierarchicalReferences(step)
            for _, taskGroup in ipairs(step) do
                for _, task in ipairs(taskGroup) do
                    task.taskType:Setup(task)
                end
            end
        end
        guideCache[guideID] = guide
        local moduleInfo = GuideModules.GetModule(guide.info.moduleID)
        if (moduleInfo and moduleInfo.installed and moduleInfo.compatible) then
            guide.moduleInfo = moduleInfo
            if (State.IsDebugEnabled()) then
                ValidateGuide(guide)
            end
        else
            guide = nil
        end
    end
    if (guide) then
        if (Flags.HasFlag(guide.info, "B")) then
            guide.bookmark = State.GetBookmark(guideID)
        else
            guide.bookmark = { }
        end
    end
    return guide
end

function GuideService.GetGuideType(guideID)
    return ((not guideID) and GUIDE_TYPE.NONE)
            or (RunesService.IsRuneGuide(guideID) and GUIDE_TYPE.RUNE)
            or GUIDE_TYPE.LEVELING
end
