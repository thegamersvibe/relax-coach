--[[ Copyright (C) 2018-2026 TomCat's Gaming, LLC. All Rights Reserved.
     Unauthorized use, reproduction, or distribution is prohibited.
     See license.txt included with this distribution for full terms. ]]
select(2, ...).SetupGlobalFacade()

ButtonGenerationService = { }

local function addAll(tbl, src)
    if (src) then
        if (type(src) == "table") then
            for _, e in ipairs(src) do
                table.insert(tbl, e)
            end
        else
            table.insert(tbl, src)
        end
    end
end

function ButtonGenerationService.GenerateButtons(guide, relaxCheckpoint)
    local questInfos = guide.quests
    for _, step in ipairs(guide) do
        if (relaxCheckpoint) then relaxCheckpoint() end -- Relax local guides: async scan.
        for _, taskGroup in ipairs(step) do
            local npcsFound
            for _, task in ipairs(taskGroup) do
                local npcID = task.npc or task.fromnpc
                if (npcID) then
                    taskGroup.buttons = taskGroup.buttons or { }
                    if (not npcsFound) then
                        npcsFound = { }
                        for _, button in ipairs(taskGroup.buttons) do
                            if (button.target) then
                                npcsFound[button.target] = true
                            elseif (button.targets) then
                                for _, target in ipairs(button.targets) do
                                    npcsFound[target] = true
                                end
                            end
                        end
                    end
                    if (not npcsFound[npcID]) then
                        table.insert(taskGroup.buttons, {
                            target = npcID,
                            condition = task.condition
                        })
                    end
                end
                if (questInfos and (task.complete or task.startwork)) then
                    local questid = task.complete or task.startwork
                    local questInfo = questInfos[questid]
                    if (questInfo) then
                        local objectives = CreateObjectivesList(task, questInfo)
                        for _, objectiveIdx in ipairs(objectives) do
                            local objective = questInfo.objectives[objectiveIdx]
                            local condition = { }
                            if (task.condition) then
                                table.insert(condition, "(")
                                table.insert(condition, task.condition)
                                table.insert(condition, ") and ")
                            end
                            table.insert(condition, "not OBJECTIVE[")
                            table.insert(condition, questid)
                            table.insert(condition, "][")
                            table.insert(condition, objectiveIdx)
                            table.insert(condition, "]")
                            condition = table.concat(condition)
                            if (objective.use or objective.useoncorpse) then
                                taskGroup.buttons = taskGroup.buttons or { }
                                table.insert(taskGroup.buttons, {
                                    item = objective.use or objective.useoncorpse,
                                    condition = condition
                                })
                            end
                            if (objective.questitem) then
                                taskGroup.buttons = taskGroup.buttons or { }
                                table.insert(taskGroup.buttons, {
                                    item = objective.questitem,
                                    condition = condition
                                })
                            end
                            local npctargets = { }
                            addAll(npctargets, objective.kill)
                            addAll(npctargets, objective.fromkill)
                            addAll(npctargets, objective.clicknpc)
                            addAll(npctargets, objective.npc)
                            addAll(npctargets, objective.speakto)
                            addAll(npctargets, objective.dupes)
                            if (#npctargets ~= 0) then
                                taskGroup.buttons = taskGroup.buttons or { }
                                local allowdead = objective.allowdead or (objective.useoncorpse and true) or false
                                if (type(npctargets) == "number") then
                                    table.insert(taskGroup.buttons, {
                                        target = npctargets,
                                        condition = condition,
                                        allowdead = allowdead
                                    })
                                else
                                    table.insert(taskGroup.buttons, {
                                        targets = npctargets,
                                        condition = condition,
                                        allowdead = allowdead
                                    })
                                end
                            end
                        end
                    end
                end
                if (task.hearth) then
                    taskGroup.buttons = taskGroup.buttons or { }
                    table.insert(taskGroup.buttons, {
                        item = 6948,
                        condition = task.condition
                    })
                end
            end
        end
    end
end
