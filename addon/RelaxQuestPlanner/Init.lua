-- First-party, read-only quest navigation. No quest actions or secure frames.
RelaxQuestPlanner = RelaxQuestPlanner or {}
local RQP = RelaxQuestPlanner
RQP.UI = RQP.UI or {}
RQP.version = "0.1.2-test"

function RQP.GetDB()
  if type(RelaxQuestPlannerDB) ~= "table" then RelaxQuestPlannerDB = {} end
  local db = RelaxQuestPlannerDB
  if db.mode ~= "strict" and db.mode ~= "game" and db.mode ~= "all" then db.mode = "game" end
  return db
end

function RQP.IsSecret(value)
  return issecretvalue and issecretvalue(value)
end

function RQP.Number(value)
  if RQP.IsSecret(value) then return nil end
  local number = tonumber(value)
  if number and number == number and number ~= math.huge and number ~= -math.huge then return number end
end

-- Guard script hooks, events, timers and async callbacks alike.
function RQP.Safe(fn)
  return function(...)
    local ok, a, b = pcall(fn, ...)
    if not ok then
      RQP.GetDB().lastError = RQP.IsSecret(a) and "Restricted error" or tostring(a)
      return nil
    end
    return a, b
  end
end

function RQP.Print(text)
  local message = "|cff33ff99RQP|r: " .. tostring(text)
  if DEFAULT_CHAT_FRAME then DEFAULT_CHAT_FRAME:AddMessage(message) else print(message) end
end

local events = CreateFrame("Frame")
RQP.eventFrame = events
local timers = {}
function RQP.After(delay, fn)
  local safe = RQP.Safe(fn)
  if C_Timer and C_Timer.After then C_Timer.After(delay, safe)
  else timers[#timers + 1] = { delay = delay, fn = safe } end
end

events:SetScript("OnUpdate", RQP.Safe(function(_, elapsed)
  if #timers == 0 then return end
  local pending = timers
  timers = {}
  for _, timer in ipairs(pending) do
    timer.delay = timer.delay - elapsed
    if timer.delay <= 0 then timer.fn() else timers[#timers + 1] = timer end
  end
end))

local refreshPending
function RQP.RequestRefresh()
  if refreshPending then return end
  refreshPending = true
  RQP.After(0.2, function()
    refreshPending = nil
    if RQP.UI.Refresh then RQP.UI.Refresh() end
  end)
end

local function record(kind, qid)
  if not (RQP.Learn and RQP.Learn.Record) then return end
  qid = RQP.Number(qid)
  if not qid or qid <= 0 or qid ~= math.floor(qid) then return end
  local guid = UnitGUID and UnitGUID("target")
  local npc
  if not RQP.IsSecret(guid) and type(guid) == "string" and strsplit then
    npc = tonumber((select(6, strsplit("-", guid))))
  end
  RQP.Learn.Record(kind, qid, npc)
end
local safeRecord = RQP.Safe(record)

local function onEvent(_, event, ...)
  if event == "ADDON_LOADED" then
    if (...) == "RelaxQuestPlanner" then RQP.GetDB() end
    return
  end
  if event == "PLAYER_LOGIN" then
    if RQP.UI.Initialize then RQP.UI.Initialize() end
    return
  end
  if event == "PLAYER_REGEN_ENABLED" then
    if RQP.UI.RetryJourney then RQP.UI.RetryJourney() end
    return
  end
  if event == "QUEST_ACCEPTED" then
    -- Forever builds expose questID alone or logIndex, questID.
    local qid
    if select("#", ...) >= 2 then qid = RQP.Number((select(2, ...)))
    else qid = RQP.Number((select(1, ...))) end
    safeRecord("accept", qid)
  elseif event == "QUEST_TURNED_IN" then
    safeRecord("turnin", (select(1, ...)))
  end
  RQP.RequestRefresh()
end

for _, event in ipairs({ "ADDON_LOADED", "PLAYER_LOGIN", "QUEST_ACCEPTED", "QUEST_TURNED_IN",
  "QUEST_REMOVED", "QUEST_LOG_UPDATE", "ZONE_CHANGED", "ZONE_CHANGED_INDOORS", "ZONE_CHANGED_NEW_AREA",
  "PLAYER_LEVEL_UP", "PLAYER_ENTERING_WORLD", "PLAYER_REGEN_ENABLED" }) do
  events:RegisterEvent(event)
end
events:SetScript("OnEvent", RQP.Safe(onEvent))
