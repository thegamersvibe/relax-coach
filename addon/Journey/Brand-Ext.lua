--[[ Copyright (C) 2018-2026 TomCat's Gaming, LLC. All Rights Reserved.
     Unauthorized use, reproduction, or distribution is prohibited.
     See license.txt included with this distribution for full terms. ]]
select(2, ...).SetupGlobalFacade()

-- Local artwork aliases leave the shared images/content resolver unchanged.
I["relax-icon"] = "Interface\\AddOns\\Journey\\assets\\relax-icon"
I["relax-banner"] = "Interface\\AddOns\\Journey\\assets\\relax-banner"
Brand.headerLogo = "relax-icon"
Brand.minimapIcon = "relax-icon"
Brand.headerLogoHC = "relax-icon"
Brand.headerIconBorderHidden = true
Brand.headerIconSize = 58
Brand.headerIconOffsetX = -2
Brand.hardcoreDisabled = true
Brand.slashCommand = "journey"
Brand.hasWebGuides = false
Brand.installerName = "Journey Guide Manager"


function Brand.configurationWindowSetupTitle(parent)
	-- The 1024x512 texture contains a 1024x434 banner above transparent padding.
	local bannerHeight = 256 * 434 / 1024
	local logo = parent:CreateTexture(nil, "ARTWORK")
	logo:SetTexture(I["relax-banner"])
	logo:SetTexCoord(0, 1, 0, 434 / 512)
	logo:SetSize(256, bannerHeight)
	logo:SetPoint("TOP", 0, -20)

	local title = parent:CreateFontString(nil, "ARTWORK", "GameFontNormal")
	title:SetPoint("TOP", logo, "BOTTOM", 0, -6)
	title:SetText(string.format("%s %s", addonTitle, GetDisplayVersion()))
	-- Keep the controls below the title as the banner's aspect ratio changes.
	return title, 20 + bannerHeight + 6 + 60
end

-- Deprecated: Remove this once Era guides are fully migrated to Forever context including guide ID conditions and
-- goto buttons have been resolved
function Brand.registerGuideAlias(guideID, context)
	local baseID = guideID:match("^(.+)_FVR$")
	if (baseID) then
		context[baseID] = true
	end
end

-- Deprecated: Remove this once Era guides are fully migrated to Forever context including guide ID conditions and
-- goto buttons have been resolved
function Brand.resolveGuideAlias(guideID)
	if (not CatalogService.IsInstalled(guideID)) then
		local fvrID = guideID .. "_FVR"
		if (CatalogService.IsInstalled(fvrID)) then
			return fvrID
		end
	end
	return guideID
end

-- Relax local guides: additive, session-only integration; catalog services are
-- not replaced. RQP_ is reserved for locally built tables, never catalog data.
local relaxLocalGuides = { }
local relaxAPI = { version = 1 }
_G.RelaxJourneyAPI = relaxAPI

function Brand.GetRelaxLocalGuide(guideID)
    return relaxLocalGuides[guideID]
end

function relaxAPI.RegisterLocalGuide(guideID, info, buildFn)
    if (type(guideID) ~= "string" or not guideID:match("^RQP_[%w_]+$")
            or type(info) ~= "table" or type(buildFn) ~= "function") then
        return false, "Invalid local guide registration"
    end
    for _, catalogInfo in ipairs(CatalogService.GetGuideInfos()) do
        if (catalogInfo.guideID == guideID) then
            return false, "A local guide cannot replace a catalog guide"
        end
    end
    relaxLocalGuides[guideID] = {
        info = { guideID = guideID, description = info.description or guideID,
            moduleID = "RQP_Local", flags = "B" },
        buildFn = buildFn,
    }
    GuideService.InvalidateRelaxLocalGuide(guideID)
    return true
end

function relaxAPI.UnregisterLocalGuide(guideID)
    if (not relaxLocalGuides[guideID]) then return false end
    relaxLocalGuides[guideID] = nil
    GuideService.InvalidateRelaxLocalGuide(guideID)
    return true
end

function relaxAPI.Goto(guideID, stepID)
    if (type(guideID) ~= "string") then return false, "Guide ID required" end
    local ok, guide = pcall(GuideService.GetGuide, guideID)
    if (not ok or not guide) then return false, "Guide unavailable" end
    if (stepID and not guide.stepLUT[stepID]) then return false, "Step unavailable" end
    local navigated, err = pcall(GuideNavigationService.Goto, guideID, stepID)
    if (not navigated) then return false, tostring(err) end
    State.SetGuideShown(true)
    return true
end

-- Relax local guides: read-only view of the loaded guide and the last one the player chose.
function relaxAPI.GetActiveGuide()
    local ok, guide = pcall(GuideNavigationService.GetGuide)
    local last = State and State.GetLastGuide and State.GetLastGuide() or nil
    return (ok and guide ~= nil) or false, last
end

-- Relax local guides: read-only catalog preview, never redirects signed loads.
function relaxAPI.ListCatalogGuides()
    local result = { }
    for _, info in ipairs(CatalogService.GetGuideInfos()) do
        local ok, passed = pcall(CheckCondition, info)
        if (info.guideID and ok and passed) then
            local copy = { }
            for key, value in pairs(info) do copy[key] = value end
            result[#result + 1] = copy
        end
    end
    return result
end

local function relaxConditions(step, group, task)
    local unknown
    for _, node in ipairs({ step, group, task }) do
        local ok, passed = pcall(CheckCondition, node)
        if (ok and not passed) then return false end
        if (not ok) then unknown = true end
    end
    if (not unknown) then return true end
end

local function relaxCompleted(step, group, task)
    for _, node in ipairs({ step, group, task }) do
        local ok, passed = pcall(CheckCompleted, node)
        if (ok and passed) then return true end
    end
    -- Individual objectives can already be done while the quest is in the log.
    if ((task.objective or task.objectives) and task.taskType) then
        local ok, passed = pcall(task.taskType.IsCompleted, task.taskType, task)
        if (ok and passed) then return true end
    end
    return false
end

-- checkpoint is optional: an async caller can yield between normalization steps
-- and extraction steps. coroutine.resume protects the loader without Lua 5.1's
-- pcall/yield restriction. Existing synchronous Journey callers are unchanged.
function relaxAPI.ScanGuideQuests(guideID, checkpoint)
    local ok, guide
    if (checkpoint) then
        local loader = coroutine.create(function()
            return GuideService.GetGuide(guideID, coroutine.yield)
        end)
        repeat
            ok, guide = coroutine.resume(loader)
            if (not ok) then break end
            if (coroutine.status(loader) ~= "dead") then checkpoint() end
        until coroutine.status(loader) == "dead"
    else
        ok, guide = pcall(GuideService.GetGuide, guideID)
    end
    if (not ok) then return nil, "Guide normalization failed" end
    if (not guide) then return nil, "Guide unavailable" end
    local scan = { accept = { }, complete = { }, turnin = { }, steps = { } }
    local kinds = { "accept", "acceptone", "complete", "startwork", "turnin" }
    for index, step in ipairs(guide) do
        local row = { id = step.id, index = index, actions = { } }
        scan.steps[#scan.steps + 1] = row
        for _, group in ipairs(step) do
            for _, task in ipairs(group) do
                for _, field in ipairs(kinds) do
                    local value = task[field]
                    if (value) then
                        local ids = type(value) == "table" and value or { value }
                        local kind = (field == "acceptone" and "accept")
                            or (field == "startwork" and "complete") or field
                        for _, questID in ipairs(ids) do
                            if (type(questID) == "number" and questID > 0) then
                                local optionalOK, optional = pcall(task.taskType.IsOptional, task.taskType, task)
                                scan[kind][questID] = true
                                row.actions[#row.actions + 1] = {
                                    questID = questID, kind = kind,
                                    active = relaxConditions(step, group, task),
                                    completed = relaxCompleted(step, group, task),
                                    optional = not optionalOK or optional == true or group.optional == true,
                                    alternatives = field == "acceptone" and ids or nil,
                                }
                            end
                        end
                    end
                end
            end
        end
        if (checkpoint) then checkpoint() end
    end
    return scan
end
