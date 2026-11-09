local RQP = RelaxQuestPlanner
local UI = RQP.UI
local routeGeneration = 0

function UI.UpdateStatus()
  if not UI.status then return end
  local pieces = {}
  if not RQP.Data or not RQP.Data.Source or RQP.Data.Source() ~= "QuestieDB" then
    pieces[#pieces + 1] = "QuestieDB missing: limited data"
  end
  if RQP.Data and RQP.Data.Ready then
    local ready, reason = RQP.Data.Ready()
    if not ready then
      pieces[#pieces + 1] = reason and reason ~= pieces[1] and reason or "No usable quest data addons"
    end
  end
  if UI.busy then pieces[#pieces + 1] = "Planning..."
  elseif UI.notice then pieces[#pieces + 1] = UI.notice end
  UI.status:SetText(table.concat(pieces, " | "))
  if UI.UpdateJourney then UI.UpdateJourney() end
end

local function planFailed(err)
  UI.busy, UI.routeJob = false, nil
  RQP.GetDB().lastError = RQP.IsSecret(err) and "Restricted error" or tostring(err)
  UI.notice = "Planning failed - type /rqp error"
  if not UI.route then UI.line1:SetText("Unable to build a route.") end
  UI.UpdateStatus()
end

function UI.Plan(show, force)
  local selection = RQP.GetDB().selection
  if not selection then return end
  if UI.busy and not show and not force then UI.replanPending = true; return end
  if type(UI.routeJob) == "table" and UI.routeJob.Cancel then UI.routeJob:Cancel() end
  UI.routeJob, UI.replanPending = nil, nil
  routeGeneration = routeGeneration + 1
  local generation = routeGeneration
  if show then UI.Show() end
  UI.busy, UI.notice = true, nil
  if not UI.route then
    UI.title:SetText(selection.name or ("Quest " .. tostring(selection.q)))
    UI.line1:SetText("Planning...")
    UI.line2:SetText(""); UI.tags:SetText(""); UI.note:SetText("")
  end
  UI.UpdateStatus()
  local snapOK, player = pcall(RQP.Player.Snapshot)
  if not snapOK then planFailed(player); return end
  local callback = RQP.Safe(function(route)
    if generation ~= routeGeneration then return end
    UI.busy = false
    UI.routeJob = nil
    if type(route) ~= "table" or type(route.steps) ~= "table" then planFailed("Planner returned no valid route"); return end
    UI.route = route
    UI.current = nil
    local rendered, err = pcall(function() UI.RenderRoute(RQP.Player.Snapshot()) end)
    if not rendered then planFailed(err); return end
    UI.notice = UI.RouteIssues(route)
    UI.UpdateStatus()
    -- Journey is the step tracker: hand a freshly chosen route over once.
    if UI.autoJourneyPending and RQP.GetDB().autoJourney ~= false and #(route.steps or {}) > 0
      and RQP.Journey and RQP.Journey.LoadRoute and _G.RelaxJourneyAPI
      and not (InCombatLockdown and InCombatLockdown()) then
      UI.autoJourneyPending = nil
      pcall(RQP.Journey.LoadRoute, route)
    elseif not UI.journeyRestoreChecked then
      UI.journeyRestoreChecked = true
      UI.RestoreJourney(route)
    end
    if UI.replanPending then
      UI.replanPending = nil
      RQP.After(0, function() if generation == routeGeneration then UI.Plan(false) end end)
    end
  end)
  local builder = selection.kind == "goal" and RQP.Engine.BuildGoalAsync or RQP.Engine.BuildPathAsync
  local target = selection.kind == "goal" and selection.q or selection.name
  local ok, err = pcall(builder, target, player, {mode=RQP.GetDB().mode}, callback)
  if ok then UI.routeJob = UI.busy and err or nil
  else planFailed(err) end
end

-- Journey forgets the session route on reload. Put it back only when Journey has
-- nothing loaded or its last guide was our route; never replace a guide the player chose.
function UI.RestoreJourney(route)
  local api = _G.RelaxJourneyAPI
  if RQP.GetDB().autoJourney == false or not (api and api.GetActiveGuide and RQP.Journey and RQP.Journey.LoadRoute) then return false end
  if InCombatLockdown and InCombatLockdown() then UI.journeyRestoreChecked = nil; return false end
  if #(route.steps or {}) == 0 then return false end
  local ok, loaded, last = pcall(api.GetActiveGuide)
  if not ok then return false end
  if loaded and last ~= "RQP_ROUTE" then return false end
  return (pcall(RQP.Journey.LoadRoute, route))
end

function UI.SelectDungeon(name)
  local wanted = name:lower()
  for _, dungeon in ipairs(RQP.Data.Dungeons() or {}) do
    if dungeon.name:lower() == wanted then
      RQP.GetDB().selection = {kind="dungeon", name=dungeon.name}
      UI.route, UI.current, UI.autoJourneyPending = nil, nil, true
      if UI.picker then UI.picker:Hide() end
      if UI.guidePicker then UI.guidePicker:Hide() end
      UI.Plan(true)
      return
    end
  end
  RQP.Print("Unknown dungeon: " .. name .. ". Use /rqp dungeon to choose one.")
end

function UI.ParseQuestID(text)
  if RQP.IsSecret(text) or type(text) ~= "string" then return nil end
  local token = text:match("^%s*(%d+)%s*$") or text:match("|Hquest:(%d+):")
    or text:match("^quest:(%d+):") or text:match("^quest:(%d+)$")
  local id = token and RQP.Number(token)
  if id and id > 0 and id <= 2147483647 and id == math.floor(id) then return id end
end

function UI.SelectGoal(text)
  local id = UI.ParseQuestID(text)
  if not id then RQP.Print("Use /rqp goal <questID>, or focus the quest box and shift-click a quest link."); return false end
  RQP.GetDB().selection = {kind="goal", q=id}
  UI.route, UI.current, UI.autoJourneyPending = nil, nil, true
  UI.goalBox:SetText(tostring(id))
  if UI.picker then UI.picker:Hide() end
  if UI.guidePicker then UI.guidePicker:Hide() end
  UI.Plan(true)
  return true
end

local MODE_LABELS = {strict="Levels: Strict -5", game="Levels: Game", all="Levels: Show all"}
function UI.SetMode(mode)
  if not MODE_LABELS[mode] then RQP.Print("Mode must be strict, game, or all."); return end
  RQP.GetDB().mode = mode
  UI.modeButton:SetText(MODE_LABELS[mode])
  UI.Refresh(true)
end
function UI.CycleMode()
  local nextMode = {game="strict", strict="all", all="game"}
  UI.SetMode(nextMode[RQP.GetDB().mode])
end
function UI.InstallLinkHooks()
  if UI.linkHooks or not hooksecurefunc then return end
  UI.linkHooks = true
  local function insert(link)
    if not (UI.goalBox and UI.frame:IsShown() and UI.goalBox:HasFocus()) then return end
    local id = UI.ParseQuestID(link)
    if id then UI.goalBox:SetText(tostring(id)); UI.goalBox:HighlightText() end
  end
  if ChatEdit_InsertLink then hooksecurefunc("ChatEdit_InsertLink", RQP.Safe(insert)) end
  if SetItemRef then
    hooksecurefunc("SetItemRef", RQP.Safe(function(link)
      if (IsModifiedClick and IsModifiedClick("CHATLINK")) or (IsShiftKeyDown and IsShiftKeyDown()) then insert(link) end
    end))
  end
end

-- QUEST_LOG_UPDATE fires every few seconds for objective ticks and party changes.
-- Rebuild routes/counts only when something that affects them actually changed.
function UI.QuestFingerprint(P)
  local done, log = 0, {}
  for _ in pairs(P.done or {}) do done = done + 1 end
  for q in pairs(P.log or {}) do log[#log + 1] = tostring(q) .. ((P.ready and P.ready[q]) and "r" or "") end
  table.sort(log)
  return table.concat({tostring(P.level), RQP.GetDB().mode, tostring(done), table.concat(log, ",")}, "|")
end

function UI.Refresh(force)
  if not UI.frame then return end
  UI.modeButton:SetText(MODE_LABELS[RQP.GetDB().mode])
  UI.UpdateStatus()
  local P = RQP.Player.Snapshot()
  if UI.route then UI.RenderRoute(P) end
  local fp = UI.QuestFingerprint(P)
  if not force and fp == UI.lastFingerprint then return end
  UI.lastFingerprint = fp
  if RQP.GetDB().selection then UI.Plan(false, force) end
  if UI.RefreshCounts then UI.RefreshCounts() end
  if UI.guidePicker and UI.guidePicker:IsShown() then UI.PickGuide() end
end

local function help()
  RQP.Print("/rqp dungeon <name> | guide | goal <questID> | mode <strict|game|all> | map | skip | unskip | autopin | autojourney | hide | show | error")
end

SLASH_RELAXQUESTPLANNER1 = "/rqp"
SlashCmdList.RELAXQUESTPLANNER = RQP.Safe(function(message)
  local command, arg = (message or ""):match("^%s*(%S*)%s*(.-)%s*$")
  command = command:lower()
  if command == "hide" then UI.Hide()
  elseif command == "show" or command == "" then UI.Show()
  elseif command == "dungeon" then
    if arg ~= "" then UI.SelectDungeon(arg) else UI.ShowPicker() end
  elseif command == "map" then UI.SetWaypoint()
  elseif command == "goal" then UI.SelectGoal(arg)
  elseif command == "guide" then UI.PickGuide()
  elseif command == "mode" then UI.SetMode(arg:lower())
  elseif command == "skip" then UI.SkipCurrent()
  elseif command == "unskip" then
    RQP.GetDB().skipped = nil; RQP.Print("Skipped quests restored."); if UI.route then UI.RenderRoute(RQP.Player.Snapshot()) end
  elseif command == "autopin" then
    local db = RQP.GetDB(); db.autoPin = db.autoPin == false
    RQP.Print("Automatic map pin " .. (db.autoPin and "on" or "off") .. ".")
  elseif command == "autojourney" then
    local db = RQP.GetDB(); db.autoJourney = db.autoJourney == false
    RQP.Print("Open new routes in Journey " .. (db.autoJourney and "on" or "off") .. ".")
  elseif command == "error" then
    local db = RQP.GetDB()
    RQP.Print(db.lastError and ("Last error: " .. tostring(db.lastError)) or "No errors recorded.")
  else help() end
end)
