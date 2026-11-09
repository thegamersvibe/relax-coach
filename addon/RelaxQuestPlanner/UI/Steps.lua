-- Selection rules follow the tested RelaxDungeonPath prototype, not a cursor.
-- Never mark a quest done just because the player clicked a button.
local RQP = RelaxQuestPlanner
local UI = RQP.UI
local VERB = {accept="Pick up", complete="Do", turnin="Turn in"}

local function questFlag(api, q, fallback)
  local fn = C_QuestLog and C_QuestLog[api]
  local result = fn and fn(q)
  if RQP.IsSecret(result) then return false end
  return result == true or fallback == true
end

local function done(q, P)
  return questFlag("IsQuestFlaggedCompleted", q, P.done and P.done[q])
end
local function inlog(q, P)
  return questFlag("IsOnQuest", q, P.log and P.log[q])
end
local function ready(q, P)
  return questFlag("ReadyForTurnIn", q, P.ready and P.ready[q]) or questFlag("IsComplete", q)
end
local function skipped(s)
  local sk = RQP.GetDB().skipped
  return type(sk) == "table" and s.q and sk[s.q] == true
end
UI.IsSkipped = skipped
local function stepDone(s, P)
  if done(s.q, P) or skipped(s) then return true end
  if s.kind == "accept" then return inlog(s.q, P) end
  if s.kind == "complete" then return inlog(s.q, P) and ready(s.q, P) end
  return false
end
local function actionable(s, P, route)
  if s.kind == "accept" then
    for _, q in ipairs(s.needs or {}) do
      local ok = done(q, P) or (s.activeNeeds and s.activeNeeds[q] and inlog(q, P))
      if s.parentActive and s.parentActive[q] then ok = inlog(q, P) and not done(q, P) end
      if not ok then return false end
    end
    -- Current engine routes carry their gate results from the async build.
    -- Keep the legacy path for callers supplying hand-authored route tables.
    if route and route.availability then return route.availability[s.q] == "available" end
    if RQP.Engine and RQP.Engine.State then
      local state = RQP.Engine.State(s.q, P, {mode=RQP.GetDB().mode})
      return state == "available"
    end
    return true
  end
  if s.kind == "complete" then return inlog(s.q, P) end
  if s.kind == "turnin" then return inlog(s.q, P) and ready(s.q, P) end
  return false
end

-- First missing prerequisite of an accept step, for a readable "Needs:" line.
function UI.BlockingReason(s, P)
  if s.kind == "accept" then
    for _, q in ipairs(s.needs or {}) do
      local active = s.activeNeeds and s.activeNeeds[q]
      local ok = done(q, P) or (active and inlog(q, P))
      if s.parentActive and s.parentActive[q] then ok = inlog(q, P) and not done(q, P) end
      if not ok then
        local rec = RQP.Data and RQP.Data.Quest and RQP.Data.Quest(q)
        return "Needs: " .. ((rec and rec.name) or ("quest " .. tostring(q))) .. (active and " (pick up)" or " (turn in)")
      end
    end
    local lvl = RQP.Number(s.level) or RQP.Number(s.reqLevel)
    if lvl and P.level and P.level < lvl then return "Needs level " .. tostring(math.floor(lvl)) end
    return "Not offered yet"
  elseif s.kind == "complete" then return "Pick up the quest first"
  elseif s.kind == "turnin" then return "Finish the objectives first" end
end

function UI.QuestsLeft(route, P)
  local seen, n = {}, 0
  for _, s in ipairs(route.steps or {}) do
    if s.q and not seen[s.q] and not stepDone(s, P) then seen[s.q] = true; n = n + 1 end
  end
  return n
end

function UI.SkipCurrent()
  local s = UI.current
  if not (s and s.q) then RQP.Print("No step to skip."); return end
  local db = RQP.GetDB(); db.skipped = type(db.skipped) == "table" and db.skipped or {}
  db.skipped[s.q] = true
  RQP.Print("Skipped " .. (s.name or tostring(s.q)) .. " for this character. /rqp unskip to restore all.")
  UI.RenderRoute(RQP.Player.Snapshot())
  if UI.SyncJourney then UI.SyncJourney(UI.route) end
end

function UI.PickStep(route, P)
  local map = C_Map and C_Map.GetBestMapForUnit and C_Map.GetBestMapForUnit("player")
  if RQP.IsSecret(map) then map = nil end
  local zone = GetRealZoneText and GetRealZoneText()
  if RQP.IsSecret(zone) then zone = nil end
  -- Location preference cannot undo the engine's child-accept -> parent-turnin
  -- ordering. Keep parents active until every unskipped child has been accepted.
  local keepActive = {}
  for _, s in ipairs(route.steps or {}) do
    if s.kind == "accept" and not stepDone(s, P) then
      for q, required in pairs(s.parentActive or {}) do if required then keepActive[q] = true end end
    end
  end
  local firstHere, firstAct, firstUndone, left = nil, nil, nil, 0
  for i, s in ipairs(route.steps or {}) do
    if not stepDone(s, P) then
      left = left + 1
      if not (s.kind == "turnin" and keepActive[s.q]) then
        firstUndone = firstUndone or i
        if actionable(s, P, route) then
          firstAct = firstAct or i
          if (map and s.map == map) or (zone and s.zone == zone) then firstHere = firstHere or i end
        end
      end
    end
  end
  return firstHere or firstAct or firstUndone, left, firstAct ~= nil
end

function UI.RouteIssues(route)
  if not route or not route.issues or #route.issues == 0 then return nil end
  local issues = {}
  for _, issue in ipairs(route.issues) do
    -- Dependency diagnostics already have a persistent, dedicated status tag.
    if type(issue) ~= "table" or (issue[1] ~= "limitedData" and issue[1] ~= "missingData") then
      issues[#issues + 1] = issue
    end
  end
  if #issues == 0 then return nil end
  local issue, parts = issues[1], {}
  if type(issue) == "table" then
    for _, value in ipairs(issue) do
      if type(value) == "string" or type(value) == "number" then parts[#parts + 1] = tostring(value) end
    end
  else parts[1] = tostring(issue) end
  return tostring(#issues) .. " route issue(s): " .. table.concat(parts, " - ")
end

function UI.RenderRoute(P)
  if not UI.frame then return end
  local route = UI.route
  if not route then return end
  local index, left, canDo = UI.PickStep(route, P or RQP.Player.Snapshot())
  UI.current = index and route.steps[index]
  local qleft = UI.QuestsLeft(route, P or RQP.Player.Snapshot())
  UI.title:SetText((route.title or "Quest route") .. "  (" .. tostring(qleft) .. (qleft == 1 and " quest left)" or " quests left)"))
  if not UI.current then
    UI.line1:SetText(#(route.steps or {}) == 0 and "No available route steps." or "|cff33ff99All done.|r")
    UI.line2:SetText("")
    UI.tags:SetText("")
    UI.note:SetText(#(route.later or {}) > 0 and "More quests unlock at higher level, or switch Levels to Show all." or "")
    UI.lastPinned = nil
    return
  end
  local s = UI.current
  local hex = UI.KIND_HEX and UI.KIND_HEX[s.kind] or "ffffff"
  UI.line1:SetText("|cff" .. hex .. (VERB[s.kind] or s.kind or "Quest") .. "|r  " .. (s.name or tostring(s.q)))
  local c = UI.THEME and UI.THEME.kind[s.kind]
  if UI.stepStripe and c then UI.stepStripe:SetColorTexture(c[1], c[2], c[3], 1) end
  local where = s.who and (s.who .. (s.zone and (" - " .. s.zone) or "")) or (s.zone or "")
  local x, y = RQP.Number(s.x), RQP.Number(s.y)
  if x and y then where = where .. (" (%.1f, %.1f)"):format(x, y) end
  UI.line2:SetText(where)
  UI.tags:SetText((s.source or "unknown source") .. "  ·  " .. (s.confidence or "unverified"))
  local note = s.note or ""
  if s.below then
    local floor = RQP.Number(route.floor)
    local reason = note ~= "" and note or "optional quest / reward"
    note = "|cffffa500Your choice|r - below " .. (floor and ("level " .. tostring(math.floor(floor))) or "the level floor") .. "; " .. reason
  end
  if not canDo then note = "|cffff7070" .. (UI.BlockingReason(s, P or RQP.Player.Snapshot()) or "Waiting") .. "|r  " .. note end
  UI.note:SetText(note)
  -- Move the map pin along with the route unless the player turned it off.
  if RQP.GetDB().autoPin ~= false and canDo and UI.lastPinned ~= s then
    UI.lastPinned = s
    UI.SetWaypoint(true)
  end
end

function UI.SetWaypoint(quiet)
  quiet = quiet == true
  local s = UI.current
  local map, x, y = s and RQP.Number(s.map), s and RQP.Number(s.x), s and RQP.Number(s.y)
  if not (map and x and y) or x < 0 or x > 100 or y < 0 or y > 100 then
    if not quiet then RQP.Print("No map pin for this step.") end
    return
  end
  if not (C_Map and C_Map.SetUserWaypoint and UiMapPoint and UiMapPoint.CreateFromCoordinates) then
    if not quiet then RQP.Print("Map waypoints are unavailable on this client.") end
    return
  end
  local point = UiMapPoint.CreateFromCoordinates(map, x / 100, y / 100)
  C_Map.SetUserWaypoint(point)
  if C_SuperTrack and C_SuperTrack.SetSuperTrackedUserWaypoint then C_SuperTrack.SetSuperTrackedUserWaypoint(true) end
  if not quiet then RQP.Print(("%s %.1f, %.1f (map pin set)"):format(s.zone or "", x, y)) end
end
