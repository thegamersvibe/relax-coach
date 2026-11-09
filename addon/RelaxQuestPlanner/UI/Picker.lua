local RQP = RelaxQuestPlanner
local UI = RQP.UI
local generation = 0
UI.dungeonCounts, UI.dungeonRows = {}, {}

function UI.CancelCounts()
  generation = generation + 1
  if type(UI.countJob) == "table" and UI.countJob.Cancel then UI.countJob:Cancel() end
  UI.countJob = nil
end

local function buildPicker()
  if UI.picker then return end
  local f = UI.Frame("RelaxQuestPlannerDungeonPicker", 390, 350)
  UI.picker = f
  UI.Movable(f, "pickerPosition")
  f:SetFrameStrata("DIALOG")
  UI.Text(f, "GameFontNormal", 12, -12, 335, 18):SetText("Dungeon quests")
  UI.Button(f, "X", 24, 352, -7, function() f:Hide() end)
  UI.pickerStatus = UI.Text(f, "GameFontHighlightSmall", 12, -38, 360, 30)
  local scroll = CreateFrame("ScrollFrame", "RelaxQuestPlannerDungeonScroll", f, "UIPanelScrollFrameTemplate")
  UI.dungeonScroll = scroll
  scroll:SetPoint("TOPLEFT", f, "TOPLEFT", 12, -76)
  scroll:SetSize(340, 226)
  local child = CreateFrame("Frame", nil, scroll)
  child:SetSize(338, 1)
  scroll:SetScrollChild(child)
  UI.dungeonChild = child
  scroll:EnableMouseWheel(true)
  scroll:SetScript("OnMouseWheel", RQP.Safe(function(self, delta)
    self:SetVerticalScroll(math.max(0, math.min(self:GetVerticalScrollRange(), self:GetVerticalScroll() - delta * 28)))
  end))
  UI.Text(f, "GameFontDisableSmall", 12, -316, 365, 25):SetText("Counts are unique remaining route quests, including prerequisites.")
  f:SetScript("OnHide", RQP.Safe(UI.CancelCounts))
end

function UI.RefreshCounts()
  if not UI.picker or not UI.picker:IsShown() then return end
  UI.CancelCounts()
  local token = generation
  UI.dungeonCounts = {}
  local names, seen = {}, {}
  for _, dungeon in ipairs(RQP.Data.Dungeons() or {}) do
    if type(dungeon.name) == "string" and not seen[dungeon.name:lower()] then
      names[#names + 1] = dungeon.name
      seen[dungeon.name:lower()] = true
    end
  end
  table.sort(names)
  for _, row in ipairs(UI.dungeonRows) do row:Hide() end
  for i, name in ipairs(names) do
    local row = UI.dungeonRows[i]
    if not row then
      row = UI.Button(UI.dungeonChild, "", 336, 0, -(i - 1) * 28, function() end)
      UI.dungeonRows[i] = row
    end
    row:SetText(name .. " - counting...")
    UI.Tooltip(row, name .. "\nUnique remaining route quests for this character and level mode, including prerequisites.")
    row:SetScript("OnClick", RQP.Safe(function() UI.SelectDungeon(name) end))
    row:Show()
  end
  UI.dungeonChild:SetHeight(math.max(1, #names * 28))
  UI.dungeonScroll:SetVerticalScroll(0)
  UI.pickerStatus:SetText(#names == 0 and "No dungeon data. Install QuestieDB, FDJ or DungeonGuideForever." or "Counting for this character (" .. RQP.GetDB().mode .. ")...")
  if #names == 0 then return end
  local player, mode = RQP.Player.Snapshot(), RQP.GetDB().mode
  local index = 0
  local function nextDungeon()
    if token ~= generation or not UI.picker:IsShown() then return end
    -- A user-selected route always takes priority over background counts.
    if UI.busy then RQP.After(0.1, nextDungeon); return end
    index = index + 1
    local name = names[index]
    if not name then UI.pickerStatus:SetText("Choose a dungeon (" .. mode .. ")."); return end
    local row = UI.dungeonRows[index]
    local ok, job = pcall(RQP.Engine.BuildPathAsync, name, player, {mode=mode}, RQP.Safe(function(route)
      if token ~= generation then return end
      UI.countJob = nil
      local failed = type(route) ~= "table" or type(route.steps) ~= "table"
      for _, issue in ipairs(route and route.issues or {}) do
        if type(issue) == "table" and (issue[1] == "error" or issue[1] == "missingData") then failed = true end
      end
      if failed then
        row:SetText(name .. " - count unavailable")
        UI.Tooltip(row, UI.RouteIssues(route) or "No usable quest data for this dungeon.")
        RQP.After(0, nextDungeon)
        return
      end
      local count, ids = 0, {}
      for _, step in ipairs(route.steps) do
        if step.q and not ids[step.q] and not (player.done and player.done[step.q]) then
          ids[step.q] = true
          count = count + 1
        end
      end
      UI.dungeonCounts[name] = count
      row:SetText(name .. " - " .. count .. (count == 1 and " quest" or " quests"))
      -- Even a synchronous optional provider cannot recurse through every dungeon.
      RQP.After(0, nextDungeon)
    end))
    if ok then UI.countJob = job
    else
      RQP.GetDB().lastError = tostring(job)
      row:SetText(name .. " - count unavailable")
      RQP.After(0, nextDungeon)
    end
  end
  RQP.After(0, nextDungeon)
end

function UI.ShowPicker()
  UI.Show()
  if UI.guidePicker then UI.guidePicker:Hide() end
  buildPicker()
  UI.picker:Show()
  UI.RefreshCounts()
end
