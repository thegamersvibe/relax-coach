-- Relax local guides: Relax Quest Planner button on the Journey header (mirrors the gear button).
select(2, ...).SetupGlobalFacade()

local component = UI.CreateComponent("HeaderPlannerButton")

local function planner()
	local rqp = _G.RelaxQuestPlanner
	return rqp and rqp.UI and rqp.UI.Show and rqp
end

function component.Init(components)
	local parent = components.Header.frame
	local frame = CreateFrame("Button", nil, parent)
	frame:SetSize(30, 30)
	frame:SetScale(0.7)
	frame:SetPoint("TOPRIGHT", parent.IconBorder, "TOPRIGHT", 2, 6)
	AddTexture(frame, "Interface\\Minimap\\MiniMap-TrackingBorder", 52, 52, "TOPLEFT")
	AddTexture(frame, CreateTexture("Interface\\Minimap\\UI-Minimap-Background", "BACKGROUND"), 24, 24, { "TOPLEFT", 2, -2 })
	AddNormalTexture(frame, "Interface\\Icons\\INV_Misc_Map_01", 18, 18, { "TOPLEFT", 6.5, -5.5 })
	AddHighlightTexture(frame, "Interface\\Minimap\\UI-Minimap-ZoomButton-Highlight")
	AddPushedTexture(frame, "Interface\\Icons\\INV_Misc_Map_01", 18, 18, { "TOPLEFT", 7.5, -6.5 })
	frame:RegisterForClicks("AnyUp")
	frame:SetScript("OnClick", function(_, button)
		local rqp = planner()
		if not rqp then return end
		local ui = rqp.UI
		if button == "RightButton" and ui.ShowPicker then
			ui.ShowPicker()
		elseif ui.frame and ui.frame:IsShown() then
			ui.Hide()
		else
			ui.Show()
		end
	end)
	frame:SetScript("OnEnter", function(self)
		GameTooltip:SetOwner(self, "ANCHOR_RIGHT")
		GameTooltip:SetText("Relax Quest Planner", 1, 1, 1)
		GameTooltip:AddLine("Left-click: open / close", 0.8, 0.8, 0.8)
		GameTooltip:AddLine("Right-click: dungeon quests", 0.8, 0.8, 0.8)
		GameTooltip:Show()
	end)
	frame:SetScript("OnLeave", function() GameTooltip:Hide() end)
	frame:SetShown(planner() ~= nil)
	component.frame = frame
end

function component.Update()
	if component.frame then component.frame:SetShown(planner() ~= nil) end
end

UI.Add(component)
