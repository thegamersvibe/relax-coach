local RQP = RelaxQuestPlanner
local UI = RQP.UI

function UI.BuildWindow()
  if UI.frame then return end
  local f = UI.Frame("RelaxQuestPlannerFrame", 470, 322)
  UI.frame = f
  f:ClearAllPoints()
  f:SetPoint("TOP", UIParent, "TOP", 0, -140)
  UI.Movable(f, "position")
  UI.Header(f, "Relax Quest Planner", 330)
  UI.closeButton = UI.CloseButton(f, UI.Hide)
  UI.title = UI.Text(f, "GameFontNormal", 12, -42, 326, 26)
  UI.title:SetTextColor(0.42, 0.78, 1)
  UI.journeyButton = UI.Button(f, "In Journey", 107, 349, -40, function() UI.OpenInJourney() end, true)
  -- current-step card: coloured stripe = accept / do / turn in
  local ok, card = pcall(CreateFrame, "Frame", nil, f, "BackdropTemplate")
  card = ok and card or CreateFrame("Frame", nil, f)
  card:SetPoint("TOPLEFT", f, "TOPLEFT", 8, -68); card:SetSize(454, 134)
  UI.Flat(card, UI.THEME.card, {0.12, 0.2, 0.32, 0.9})
  UI.stepCard = card
  UI.stepStripe = card:CreateTexture(nil, "ARTWORK")
  UI.stepStripe:SetPoint("TOPLEFT", card, "TOPLEFT", 1, -1); UI.stepStripe:SetPoint("BOTTOMLEFT", card, "BOTTOMLEFT", 1, 1)
  UI.stepStripe:SetWidth(4); UI.stepStripe:SetColorTexture(unpack(UI.THEME.accent))
  UI.Tooltip(UI.journeyButton, "Open this route in Relax Journey. Requires a completed route and the local-guide bridge; unavailable in combat.")
  UI.line1 = UI.Text(f, "GameFontHighlightLarge", 18, -76, 436, 32)
  UI.line2 = UI.Text(f, "GameFontHighlight", 18, -110, 436, 30)
  UI.tags = UI.Text(f, "GameFontHighlightSmall", 18, -142, 436, 16)
  UI.tags:SetTextColor(unpack(UI.THEME.muted))
  UI.note = UI.Text(f, "GameFontHighlightSmall", 18, -160, 436, 40)
  UI.note:SetTextColor(0.78, 0.82, 0.88)
  UI.status = UI.Text(f, "GameFontHighlightSmall", 12, -288, 444, 28)
  UI.status:SetTextColor(unpack(UI.THEME.muted))
  UI.dungeonButton = UI.Button(f, "Dungeons", 108, 12, -208, function() UI.ShowPicker() end)
  UI.modeButton = UI.Button(f, "Game", 103, 128, -208, function() UI.CycleMode() end)
  UI.guideButton = UI.Button(f, "Pick my guide", 130, 239, -208, function() UI.PickGuide() end, true)
  UI.Tooltip(UI.guideButton, function()
    return UI.GuideAvailable() and "Scan installed Journey guides for this character and level mode. Choose from the top three matches."
      or "Requires the Relax Journey guide-picker integration."
  end)
  UI.mapButton = UI.Button(f, "Map", 79, 377, -208, UI.SetWaypoint, true)
  local gq = UI.Text(f, "GameFontHighlightSmall", 12, -243, 80, 16); gq:SetText("Get quest:"); gq:SetTextColor(0.42, 0.78, 1)
  local okBox, box = pcall(CreateFrame, "EditBox", "RelaxQuestPlannerGoal", f, "BackdropTemplate")
  UI.goalBox = okBox and box or CreateFrame("EditBox", "RelaxQuestPlannerGoal", f)
  UI.Flat(UI.goalBox, {0.02, 0.03, 0.05, 0.95}, {0.16, 0.26, 0.4, 0.9})
  UI.goalBox:SetFontObject(ChatFontNormal or GameFontHighlightSmall)
  UI.goalBox:SetTextInsets(6, 6, 0, 0)
  UI.goalBox:SetSize(300, 22)
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
  local hint = UI.Text(f, "GameFontHighlightSmall", 91, -266, 365, 16); hint:SetText("Quest ID, or focus here then shift-click a quest link."); hint:SetTextColor(unpack(UI.THEME.muted))
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
