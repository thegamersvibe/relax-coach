local RQP = RelaxQuestPlanner
local UI = RQP.UI
local scanGeneration = 0

function UI.GuideAvailable()
  return RQP.Journey and type(RQP.Journey.PickGuide) == "function"
end

function UI.UpdateJourney()
  if UI.guideButton then
    if UI.GuideAvailable() then UI.guideButton:Enable() else UI.guideButton:Disable() end
  end
  if UI.journeyButton then
    local api = RelaxJourneyAPI
    if UI.route and not UI.busy and RQP.Journey and RQP.Journey.LoadRoute
      and api and api.RegisterLocalGuide and api.Goto then UI.journeyButton:Enable()
    else UI.journeyButton:Disable() end
  end
end

function UI.OpenInJourney()
  if InCombatLockdown and InCombatLockdown() then RQP.Print("Open the route in Journey after combat."); return end
  if not (UI.route and RQP.Journey and RQP.Journey.LoadRoute) then RQP.Print("A route and the Relax Journey bridge are required."); return end
  local ok, reason = RQP.Journey.LoadRoute(UI.route)
  if not ok then RQP.Print(reason or "Journey could not open this route.") end
end

local function buildGuides()
  if UI.guidePicker then return end
  local f = UI.Frame("RelaxQuestPlannerGuidePicker", 470, 328)
  UI.guidePicker = f
  UI.Movable(f, "guidePositionUnused")
  f:SetFrameStrata("DIALOG")
  UI.Header(f, "Pick my guide", 380)
  UI.CloseButton(f, function() f:Hide() end)
  UI.guideStatus = UI.Text(f, "GameFontHighlightSmall", 12, -42, 444, 40)
  UI.guideStatus:SetTextColor(0.42, 0.78, 1)
  UI.guideRows, UI.guideReasons = {}, {}
  for index = 1, 3 do
    local row = UI.Row(f, 444, 12, -86 - (index - 1) * 74, function() end)
    local reason = UI.Text(f, "GameFontHighlightSmall", 18, -114 - (index - 1) * 74, 432, 38)
    reason:SetTextColor(unpack(UI.THEME.muted))
    UI.guideRows[index], UI.guideReasons[index] = row, reason
    row:Hide()
  end
  -- Dismissing a scan must not reopen a dialog from a late callback.
  f:SetScript("OnHide", RQP.Safe(function() scanGeneration = scanGeneration + 1 end))
end

function UI.PickGuide()
  if not UI.GuideAvailable() then
    RQP.Print("Pick my guide requires the Relax Journey integration.")
    return
  end
  UI.Show()
  if UI.picker then UI.picker:Hide() end
  buildGuides()
  UI.Dock(UI.guidePicker)
  UI.guidePicker:Show()
  scanGeneration = scanGeneration + 1
  local generation = scanGeneration
  UI.guideStatus:SetText("Scanning installed guides for this character...")
  for index, row in ipairs(UI.guideRows) do row:Hide(); UI.guideReasons[index]:SetText("") end
  local ok, started, err = pcall(RQP.Journey.PickGuide, RQP.Safe(function(choices, reason)
    if generation ~= scanGeneration then return end
    choices = choices or {}
    UI.guideStatus:SetText(reason or (#choices == 0 and "No matching installed guides." or "Choose a guide to start at its first undone quest step."))
    for index = 1, math.min(3, #choices) do
      local choice = choices[index]
      local row = UI.guideRows[index]
      local label = choice.description or choice.title or choice.guideID or "Guide"
      row:SetText(tostring(index) .. ". " .. label)
      local reasons = table.concat(choice.reasons or {}, "\n")
      UI.guideReasons[index]:SetText(reasons)
      UI.Tooltip(row, label .. "\n" .. reasons)
      row:SetScript("OnClick", RQP.Safe(function()
        if InCombatLockdown and InCombatLockdown() then UI.guideStatus:SetText("Load the guide after combat."); return end
        if not (RQP.Journey and RQP.Journey.GotoBest) then UI.guideStatus:SetText("Journey guide navigation is unavailable."); return end
        local success, failure = RQP.Journey.GotoBest(choice)
        if success then
          UI.guidePicker:Hide()
          UI.notice = "Journey guide: " .. label
          UI.UpdateStatus()
        else UI.guideStatus:SetText(failure or "Could not load this guide.") end
      end))
      row:Show()
    end
  end))
  if not ok then
    RQP.GetDB().lastError = tostring(started)
    UI.guideStatus:SetText("Guide scan failed - type /rqp error")
  elseif started == false then UI.guideStatus:SetText(err or "Guide scan unavailable.") end
end
