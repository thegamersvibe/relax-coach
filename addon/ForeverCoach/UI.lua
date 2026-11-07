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
    elseif cmd == "help" then
        FC:Print("/coach (window), /coach top (top 3 in chat), /coach spec <id|auto>, /coach macros, /coach fun <on|off|status>")
    else
        UI:Toggle()
    end
end
