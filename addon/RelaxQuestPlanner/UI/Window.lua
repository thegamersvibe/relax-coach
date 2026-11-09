local RQP = RelaxQuestPlanner
local UI = RQP.UI

function UI.BuildWindow()
  if UI.frame then return end
  local f = UI.Frame("RelaxQuestPlannerFrame", 470, 320)
  UI.frame = f
  f:ClearAllPoints()
  f:SetPoint("TOP", UIParent, "TOP", 0, -140)
  UI.Movable(f, "position")
  UI.Text(f, "GameFontNormal", 12, -12, 410, 18):SetText("Relax Quest Planner")
  UI.closeButton = UI.Button(f, "X", 24, 432, -7, UI.Hide)
  UI.title = UI.Text(f, "GameFontNormal", 12, -39, 326, 30)
  UI.journeyButton = UI.Button(f, "In Journey", 107, 349, -39, function() UI.OpenInJourney() end)
  UI.Tooltip(UI.journeyButton, "Open this route in Relax Journey. Requires a completed route and the local-guide bridge; unavailable in combat.")
  UI.line1 = UI.Text(f, "GameFontHighlight", 12, -74, 444, 32)
  UI.line2 = UI.Text(f, "GameFontHighlightSmall", 12, -109, 444, 30)
  UI.tags = UI.Text(f, "GameFontHighlightSmall", 12, -142, 444, 16)
  UI.note = UI.Text(f, "GameFontDisableSmall", 12, -162, 444, 40)
  UI.status = UI.Text(f, "GameFontHighlightSmall", 12, -286, 444, 28)
  UI.dungeonButton = UI.Button(f, "Dungeons", 108, 12, -208, function() UI.ShowPicker() end)
  UI.modeButton = UI.Button(f, "Game", 103, 128, -208, function() UI.CycleMode() end)
  UI.guideButton = UI.Button(f, "Pick my guide", 130, 239, -208, function() UI.PickGuide() end)
  UI.Tooltip(UI.guideButton, function()
    return UI.GuideAvailable() and "Scan installed Journey guides for this character and level mode. Choose from the top three matches."
      or "Requires the Relax Journey guide-picker integration."
  end)
  UI.mapButton = UI.Button(f, "Map", 79, 377, -208, UI.SetWaypoint)
  UI.Text(f, "GameFontHighlightSmall", 12, -241, 80, 16):SetText("Get quest:")
  UI.goalBox = CreateFrame("EditBox", "RelaxQuestPlannerGoal", f, "InputBoxTemplate")
  UI.goalBox:SetSize(300, 24)
  UI.goalBox:SetPoint("TOPLEFT", f, "TOPLEFT", 91, -238)
  UI.goalBox:SetAutoFocus(false)
  UI.goalBox:SetMaxLetters(512)
  UI.goalBox:SetScript("OnEnterPressed", RQP.Safe(function(self)
    if UI.SelectGoal(self:GetText()) then self:ClearFocus() end
  end))
  UI.goalBox:SetScript("OnEscapePressed", RQP.Safe(function(self) self:ClearFocus(); UI.Hide() end))
  UI.goalButton = UI.Button(f, "Go", 53, 403, -238, function()
    if UI.SelectGoal(UI.goalBox:GetText()) then UI.goalBox:ClearFocus() end
  end)
  UI.Text(f, "GameFontDisableSmall", 91, -267, 365, 16):SetText("Quest ID, or focus here then shift-click a quest link.")
  UI.Tooltip(UI.goalBox, "Enter a quest ID, or focus this box then shift-click a quest link in chat. Press Enter to plan.")
  f:SetScript("OnHide", RQP.Safe(function() UI.goalBox:ClearFocus(); RQP.GetDB().hidden = true end))
  UI.title:SetText("Choose a dungeon or a quest")
  UI.line1:SetText("Read-only guidance: you accept and turn in quests yourself.")
end

function UI.Initialize()
  UI.BuildWindow()
  UI.InstallLinkHooks()
  if RQP.GetDB().hidden then UI.frame:Hide() end
  UI.Refresh()
end
