-- Relax Coach: /coach window and slash commands.
local _, FC = ...
local UI = {}
FC.UI = UI

local COLORS = { Build = "ffd54f", Gear = "81c784", Play = "4fc3f7", Next = "b0bec5" }

local function line(f)
    local col = COLORS[f.area] or "ffffff"
    local s = string.format("|cff%s%s|r  %s", col, f.area, f.text)
    if f.where then s = s .. "  |cff8899aa[" .. f.where .. "]|r" end
    if f.fix and f.fix ~= "" then s = s .. "\n      |cffaaaaaa" .. f.fix .. "|r" end
    return s
end

function UI:Build()
    if self.frame then return end
    local f = CreateFrame("Frame", "ForeverCoachFrame", UIParent, BackdropTemplateMixin and "BackdropTemplate" or nil)
    f:SetSize(500, 420); f:SetPoint("CENTER"); f:SetFrameStrata("DIALOG")
    f:SetMovable(true); f:EnableMouse(true); f:RegisterForDrag("LeftButton")
    f:SetScript("OnDragStart", f.StartMoving); f:SetScript("OnDragStop", f.StopMovingOrSizing)
    if f.SetBackdrop then
        f:SetBackdrop({ bgFile = "Interface\\Tooltips\\UI-Tooltip-Background", edgeFile = "Interface\\Tooltips\\UI-Tooltip-Border", edgeSize = 14,
            insets = { left = 3, right = 3, top = 3, bottom = 3 } })
        f:SetBackdropColor(0.05, 0.06, 0.08, 0.94)
    end
    f.title = f:CreateFontString(nil, "OVERLAY", "GameFontNormalLarge"); f.title:SetPoint("TOPLEFT", 14, -12)
    f.sub = f:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall"); f.sub:SetPoint("TOPLEFT", 14, -32)
    local close = CreateFrame("Button", nil, f, "UIPanelCloseButton"); close:SetPoint("TOPRIGHT", -2, -2)
    local sf = CreateFrame("ScrollFrame", nil, f, "UIPanelScrollFrameTemplate")
    sf:SetPoint("TOPLEFT", 12, -52); sf:SetPoint("BOTTOMRIGHT", -30, 12)
    local body = CreateFrame("Frame", nil, sf); body:SetSize(450, 10); sf:SetScrollChild(body)
    f.text = body:CreateFontString(nil, "OVERLAY", "GameFontHighlight")
    f.text:SetPoint("TOPLEFT"); f.text:SetWidth(450); f.text:SetJustifyH("LEFT"); f.text:SetSpacing(3)
    f.body = body
    f:Hide()
    tinsert(UISpecialFrames, "ForeverCoachFrame")
    self.frame = f
end

function UI:Update()
    if not self.frame or not self.frame:IsShown() then return end
    local c = FC:Char()
    local s = c.snapshot or {}
    local g = c.gear or {}
    local z = s.zone and s.zone.zone or "?"
    self.frame.title:SetText("Relax Coach")
    self.frame.sub:SetText(string.format("%s  level %s %s %s  |  %s  |  spec: %s (%s)  |  rules %s", s.name or "?", s.level or "?", s.race or "",
        s.class or "", z, tostring(g.spec or "?"), tostring(g.specSource or "?"), (FC:Rules().version or "none")))
    local out = {}
    for i, f in ipairs(c.findings or {}) do
        if i > 12 then break end
        out[#out + 1] = line(f)
    end
    if #out == 0 then out[1] = "Nothing found yet. Play a bit, then /reload or /coach." end
    local tp = c.talentPlan
    if tp and tp.upcoming and tp.upcoming[1] then
        local up = {}
        for _, u in ipairs(tp.upcoming) do up[#up + 1] = u.level .. ": " .. u.name end
        out[#out + 1] = "|cffffd54fTalents next|r  " .. table.concat(up, ", ") .. "\n      |cff888888" .. tostring(tp.source) .. "|r"
    end
    if c.stale and next(c.stale) then
        local keys = {}
        for k in pairs(c.stale) do keys[#keys + 1] = k end
        out[#out + 1] = "|cffffb454Out of date (failed to refresh):|r " .. table.concat(keys, ", ")
    end
    if c.lastError and c.lastError.version == FC.VERSION then
        out[#out + 1] = "|cffff6b6bInternal error (please report):|r " .. tostring(c.lastError.where) .. ": " .. tostring(c.lastError.err)
    end
    if c.xpPerHour then out[#out + 1] = "\n|cff888888Recent pace: " .. c.xpPerHour .. " XP per active hour|r" end
    self.frame.text:SetText(table.concat(out, "\n\n"))
    self.frame.body:SetHeight(self.frame.text:GetStringHeight() + 10)
end

function UI:Toggle()
    self:Build()
    if self.frame:IsShown() then self.frame:Hide() return end
    FC:Refresh("open")
    self.frame:Show(); self:Update()
end

-- Relax Coach: level-up checklist. Right after you level, one small window lists what to do now:
-- talent point, training, best gear upgrade and the next dungeon/guide. Built from the findings
-- Audit already computes (tagged with `kind`), so it adds no extra scanning. Once per level, never
-- in combat, and /coach levelup off turns it off.
local L = {}
FC.LevelUp = L

local MAX_TRAIN, MAX_GEAR = 2, 2

local function db() ForeverCoachDB = ForeverCoachDB or {}; return ForeverCoachDB end
function L:Enabled() return db().levelUpChecklist ~= false end -- on by default
function L:SetEnabled(on) db().levelUpChecklist = on and true or false end

local function inCombat()
    return UnitAffectingCombat and FC.plain(FC.safe(UnitAffectingCombat, "player")) == true
end

-- Pure: findings -> checklist rows { kind, text, fix, where }
function L:Build(c)
    local rows, train, gear = {}, 0, 0
    local talent, dungeon
    for _, f in ipairs(c.findings or {}) do
        if f.kind == "talent" and not talent then
            talent = f
        elseif (f.kind == "train" or f.kind == "classquest") and train < MAX_TRAIN then
            train = train + 1; rows[#rows + 1] = f
        elseif f.kind == "gear" and gear < MAX_GEAR then
            gear = gear + 1; rows[#rows + 1] = f
        elseif f.kind == "dungeon" and not dungeon then
            dungeon = f
        end
    end
    local out = {}
    if talent then out[#out + 1] = talent end
    for _, f in ipairs(rows) do if f.kind ~= "gear" then out[#out + 1] = f end end
    for _, f in ipairs(rows) do if f.kind == "gear" then out[#out + 1] = f end end
    if dungeon then out[#out + 1] = dungeon end
    return out
end

local LABEL = { talent = "Talent", train = "Train", classquest = "Class quest", gear = "Gear", dungeon = "Next" }
local COLOR = { talent = "ffd54f", train = "ffd54f", classquest = "ffd54f", gear = "81c784", dungeon = "b0bec5" }

function L:Text(rows)
    local out = {}
    for _, f in ipairs(rows) do
        local s = string.format("|cff%s%s|r  %s", COLOR[f.kind] or "ffffff", LABEL[f.kind] or "", f.text)
        if f.where then s = s .. "  |cff8899aa[" .. f.where .. "]|r" end
        if f.fix and f.fix ~= "" then s = s .. "\n      |cffaaaaaa" .. f.fix .. "|r" end
        out[#out + 1] = s
    end
    return table.concat(out, "\n\n")
end

local function plannerAvailable()
    return type(SlashCmdList) == "table" and type(SlashCmdList.RELAXQUESTPLANNER) == "function"
end

function L:Frame()
    if self.frame then return self.frame end
    local f = CreateFrame("Frame", "ForeverCoachLevelUpFrame", UIParent, BackdropTemplateMixin and "BackdropTemplate" or nil)
    f:SetSize(420, 260); f:SetPoint("TOP", 0, -140); f:SetFrameStrata("DIALOG")
    f:SetMovable(true); f:EnableMouse(true); f:RegisterForDrag("LeftButton")
    f:SetScript("OnDragStart", f.StartMoving); f:SetScript("OnDragStop", f.StopMovingOrSizing)
    if f.SetBackdrop then
        f:SetBackdrop({ bgFile = "Interface\\Tooltips\\UI-Tooltip-Background", edgeFile = "Interface\\Tooltips\\UI-Tooltip-Border", edgeSize = 14,
            insets = { left = 3, right = 3, top = 3, bottom = 3 } })
        f:SetBackdropColor(0.05, 0.06, 0.08, 0.94)
    end
    f.title = f:CreateFontString(nil, "OVERLAY", "GameFontNormalLarge"); f.title:SetPoint("TOPLEFT", 14, -12)
    local close = CreateFrame("Button", nil, f, "UIPanelCloseButton"); close:SetPoint("TOPRIGHT", -2, -2)
    f.text = f:CreateFontString(nil, "OVERLAY", "GameFontHighlight")
    f.text:SetPoint("TOPLEFT", 14, -40); f.text:SetWidth(392); f.text:SetJustifyH("LEFT"); f.text:SetSpacing(3)
    local function button(label, x, onClick)
        local b = CreateFrame("Button", nil, f, "UIPanelButtonTemplate")
        b:SetSize(120, 22); b:SetPoint("BOTTOMLEFT", x, 12); b:SetText(label)
        b:SetScript("OnClick", function() local ok, err = pcall(onClick); if not ok then FC:Char().lastError = { where = "levelup:" .. label, err = tostring(err), time = time(), version = FC.VERSION } end end)
        return b
    end
    f.coach = button("Open Coach", 14, function() f:Hide(); if FC.UI and not (FC.UI.frame and FC.UI.frame:IsShown()) then FC.UI:Toggle() end end)
    f.guide = button("Pick my guide", 146, function() f:Hide(); SlashCmdList.RELAXQUESTPLANNER("guide") end)
    f.ok = button("Got it", 278, function() f:Hide() end)
    f:Hide()
    tinsert(UISpecialFrames, "ForeverCoachLevelUpFrame")
    self.frame = f
    return f
end

-- Show the checklist for the current level. force=true ignores the once-per-level/off checks (slash command).
function L:Show(force)
    local c = FC:Char()
    local lvl = c.snapshot and c.snapshot.level
    if not lvl then return false end
    if not force then
        if not self:Enabled() or c.levelUpShown == lvl then return false end
        if inCombat() then self.pending = true; return false end
    end
    self.pending = nil
    c.levelUpShown = lvl
    local rows = self:Build(c)
    if #rows == 0 then
        FC:Print("Level " .. lvl .. ": all set, nothing to do right now.")
        return false
    end
    local f = self:Frame()
    f.title:SetText("Level " .. lvl .. "! Do these now")
    f.text:SetText(self:Text(rows))
    if plannerAvailable() then f.guide:Show() else f.guide:Hide() end
    f:SetHeight(math.max(160, (f.text:GetStringHeight() or 0) + 90))
    f:Show()
    self.rows = rows
    return true
end

-- Called by Core after the post-level-up refresh.
function L:OnLevelRefresh() return self:Show(false) end

local ev = CreateFrame("Frame")
ev:RegisterEvent("PLAYER_REGEN_ENABLED")
ev:SetScript("OnEvent", function()
    if L.pending then
        local ok, err = pcall(L.Show, L, false)
        if not ok then FC:Char().lastError = { where = "levelup", err = tostring(err), time = time(), version = FC.VERSION } end
    end
end)

SLASH_FOREVERCOACH1 = "/coach"
SlashCmdList.FOREVERCOACH = function(msg)
    msg = (msg or ""):lower():gsub("^%s+", ""):gsub("%s+$", "")
    local cmd, arg = msg:match("^(%S*)%s*(.*)$")
    if cmd == "spec" then
        local c = FC:Char()
        local _, cls = UnitClass("player")
        if arg == "" or arg == "auto" then
            c.specOverride = nil
        elseif FC.Gear:ValidSpec(cls, arg) then
            c.specOverride = arg
        else
            FC:Print("unknown spec '" .. arg .. "'. Options: " .. table.concat(FC.Gear:SpecOptions(cls), ", ") .. ", auto")
            return
        end
        FC:Refresh("spec")
        FC:Print("gear spec: " .. tostring(c.gear and c.gear.spec) .. (c.specOverride and " (manual)" or " (auto)"))
    elseif cmd == "top" then
        FC:Refresh("top")
        for i, f in ipairs(FC:Char().findings or {}) do
            if i > 3 then break end
            FC:Print(f.area .. ": " .. f.text .. (f.where and (" [" .. f.where .. "]") or "") .. (f.fix and (" -> " .. f.fix) or ""))
        end
    elseif cmd == "macros" then
        local _, cls = UnitClass("player")
        local made, skipped, full, err, updated = FC.Macros:Install(cls)
        if err then FC:Print(err) return end
        if #made > 0 then FC:Print("created: " .. table.concat(made, ", ")) end
        if #updated > 0 then FC:Print("updated: " .. table.concat(updated, ", ")) end
        if #skipped > 0 then FC:Print("already up to date: " .. table.concat(skipped, ", ")) end
        if #full > 0 then FC:Print("couldn't create (no free character macro slot?): " .. table.concat(full, ", ")) end
        FC:Print("Open /macro, Character Specific tab, and drag them to a bar that doesn't swap with stance.")
        if ShowMacroFrame then pcall(ShowMacroFrame) end
    elseif cmd == "fun" then
        if arg == "on" or arg == "off" then
            FC.Reactions:SetEnabled(arg == "on")
        elseif arg ~= "" and arg ~= "status" then
            FC:Print("usage: /coach fun on|off|status")
            return
        end
        FC:Print("fun extras: " .. (FC.Reactions:Enabled() and "on" or "off"))
    elseif cmd == "levelup" then
        if arg == "on" or arg == "off" then
            FC.LevelUp:SetEnabled(arg == "on")
            FC:Print("level-up checklist: " .. arg)
        elseif arg == "" or arg == "show" then
            FC:Refresh("levelup-show")
            FC.LevelUp:Show(true)
        else
            FC:Print("usage: /coach levelup [show|on|off]")
        end
    elseif cmd == "help" then
        FC:Print("/coach (window), /coach top (top 3 in chat), /coach spec <id|auto>, /coach macros, /coach fun <on|off|status>, /coach levelup [show|on|off]")
    else
        UI:Toggle()
    end
end
