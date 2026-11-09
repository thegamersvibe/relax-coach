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
  UI.Movable(f, "pickerPositionUnused")
  f:SetFrameStrata("DIALOG")
  UI.Header(f, "Dungeon quests", 300)
  UI.CloseButton(f, function() f:Hide() end)
  UI.pickerStatus = UI.Text(f, "GameFontHighlightSmall", 12, -42, 360, 30)
  UI.pickerStatus:SetTextColor(0.42, 0.78, 1)
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
  local foot = UI.Text(f, "GameFontHighlightSmall", 12, -316, 365, 25); foot:SetText("Counts are unique remaining route quests, including prerequisites."); foot:SetTextColor(unpack(UI.THEME.muted))
  f:SetScript("OnHide", RQP.Safe(UI.CancelCounts))
end

function UI.RefreshCounts()
  if not UI.picker or not UI.picker:IsShown() then return end
  UI.CancelCounts()
  local token = generation
  UI.dungeonCounts = {}
  local names, seen, info = {}, {}, {}
  for _, dungeon in ipairs(RQP.Data.Dungeons() or {}) do
    if type(dungeon.name) == "string" and not seen[dungeon.name:lower()] then
      names[#names + 1] = dungeon.name
      seen[dungeon.name:lower()] = true
      info[dungeon.name] = dungeon
    end
  end
  local level = RQP.Number(UnitLevel and UnitLevel("player")) or 0
  UI.dungeonInfo = info
  local function lo(n) return RQP.Number(info[n].minLevel) or 99 end
  table.sort(names, function(a, b) if lo(a) ~= lo(b) then return lo(a) < lo(b) end return a < b end)
  local function label(n, tail)
    local d = info[n]
    local range = d.minLevel and (" |cff8899aa" .. d.minLevel .. "-" .. d.maxLevel .. "|r") or ""
    return n .. range .. "  " .. tail
  end
  UI.DungeonLabel = label
  local function far(n)
    local d = info[n]
    return d.minLevel and (level < d.minLevel - 3 or level > d.maxLevel + 5)
  end
  -- Count in-range dungeons first so the useful rows fill in quickly.
  local order = {}
  for i, n in ipairs(names) do if not far(n) then order[#order + 1] = i end end
  for i, n in ipairs(names) do if far(n) then order[#order + 1] = i end end
  for _, row in ipairs(UI.dungeonRows) do row:Hide() end
  for i, name in ipairs(names) do
    local row = UI.dungeonRows[i]
    if not row then
      row = UI.Row(UI.dungeonChild, 336, 0, -(i - 1) * 28, function() end)
      UI.dungeonRows[i] = row
    end
    row:SetText(label(name, "|cff8899aacounting...|r"))
    row:SetAlpha(far(name) and 0.55 or 1)
    UI.Tooltip(row, name .. "\nUnique remaining route quests for this character and level mode, including prerequisites.")
    row:SetScript("OnClick", RQP.Safe(function() UI.SelectDungeon(name) end))
    row:Show()
  end
  UI.dungeonChild:SetHeight(math.max(1, #names * 28))
  UI.dungeonScroll:SetVerticalScroll(0)
  UI.pickerStatus:SetText(#names == 0 and "No dungeon data. Install QuestieDB or ForeverDungeonJournal." or "Counting quests for this character...")
  if #names == 0 then return end
  local player, mode = RQP.Player.Snapshot(), RQP.GetDB().mode
  local index = 0
  local function nextDungeon()
    if token ~= generation or not UI.picker:IsShown() then return end
    -- A user-selected route always takes priority over background counts.
    if UI.busy then RQP.After(0.1, nextDungeon); return end
    index = index + 1
    local slot = order[index]
    local name = slot and names[slot]
    if not name then UI.pickerStatus:SetText("Choose a dungeon. Faded = out of level range or nothing left."); return end
    local row = UI.dungeonRows[slot]
    local ok, job = pcall(RQP.Engine.BuildPathAsync, name, player, {mode=mode}, RQP.Safe(function(route)
      if token ~= generation then return end
      UI.countJob = nil
      local failed = type(route) ~= "table" or type(route.steps) ~= "table"
      for _, issue in ipairs(route and route.issues or {}) do
        if type(issue) == "table" and (issue[1] == "error" or issue[1] == "missingData") then failed = true end
      end
      if failed then
        row:SetText(label(name, "|cff8899aano data|r"))
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
      row:SetText(label(name, (count == 0 and "|cff8899aa0 quests|r" or ("|cffffd100" .. count .. (count == 1 and " quest" or " quests") .. "|r"))))
      if count == 0 then row:SetAlpha(0.55) end
      -- Even a synchronous optional provider cannot recurse through every dungeon.
      RQP.After(0, nextDungeon)
    end))
    if ok then UI.countJob = job
    else
      RQP.GetDB().lastError = tostring(job)
      row:SetText(label(name, "|cff8899aano data|r"))
      RQP.After(0, nextDungeon)
    end
  end
  RQP.After(0, nextDungeon)
end

function UI.ShowPicker()
  UI.Show()
  if UI.guidePicker then UI.guidePicker:Hide() end
  buildPicker()
  UI.Dock(UI.picker)
  UI.picker:Show()
  UI.RefreshCounts()
end
