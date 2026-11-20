-- Shared recap presentation, built fresh from stock Blizzard parts only.
-- API/asset evidence and the accepted layout: docs/raid/DEATH-RECAP.md.
local VERSION = 1
local R = RelaxDeathRecap
if not R or R.v ~= VERSION or (R.uiVersion or 0) >= VERSION then return end
R.uiVersion = VERSION
local function script(frame, name, fn)
    frame:SetScript(name, function(...) pcall(fn, ...) end)
end
local function text(parent, font, x, y, width, height)
    local label = parent:CreateFontString(nil, "OVERLAY", font)
    label:SetPoint("TOPLEFT", parent, "TOPLEFT", x, y)
    label:SetSize(width, height or 16)
    label:SetJustifyH("LEFT")
    return label
end
local function texture(parent, x, y, width, height)
    local icon = parent:CreateTexture(nil, "ARTWORK")
    icon:SetPoint("TOPLEFT", parent, "TOPLEFT", x, y)
    icon:SetSize(width, height)
    return icon
end
local function bar(parent, x, y, width, height)
    local health = CreateFrame("StatusBar", nil, parent)
    health:SetPoint("TOPLEFT", parent, "TOPLEFT", x, y)
    health:SetSize(width, height)
    health:SetStatusBarTexture("Interface\\TargetingFrame\\UI-StatusBar")
    health:SetStatusBarColor(0.2, 0.65, 0.3)
    health:SetMinMaxValues(0,100)
    return health
end
local function button(parent, label, x, y, width, fn)
    local b = CreateFrame("Button", nil, parent, "UIPanelButtonTemplate")
    b:SetPoint("TOPLEFT", parent, "TOPLEFT", x, y)
    b:SetSize(width, 22); b:SetText(label)
    script(b, "OnClick", fn)
    return b
end
local function tooltip(frame, title, lines)
    GameTooltip:SetOwner(frame, "ANCHOR_RIGHT")
    GameTooltip:SetText(title, 1, 1, 1, 1, true)
    for _, line in ipairs(lines or {}) do GameTooltip:AddLine(line, 1, 1, 1, true) end
    GameTooltip:Show()
end
local function leave() GameTooltip:Hide() end
local function short(s, n)
    return #s > n and (s:sub(1,n-3) .. "...") or s
end
local icons = {DROWNING="spell_shadow_demonbreath", FALLING="ability_rogue_quickrecovery",
    FIRE="spell_fire_fire", LAVA="spell_fire_fire", SLIME="inv_misc_slime_01", FATIGUE="ability_creature_cursed_05"}
local function iconFor(row)
    if row.event == "ENVIRONMENTAL_DAMAGE" then
        return "Interface\\Icons\\" .. (icons[row.environmentalType] or "ability_creature_cursed_05")
    end
    if row.spellId then return R.clean(R.call(function() return C_Spell.GetSpellTexture(row.spellId) end), "number") end
end
local function extras(row)
    local out = {}
    for _, pair in ipairs({{"overkill","Overkill"},{"blocked","Blocked"},{"absorbed","Absorbed"},{"resisted","Resisted"},{"school","School"}}) do
        local value = row[pair[1]]
        if value and value > 0 then out[#out+1] = pair[2] .. " " .. string.format("%.0f",value) end
    end
    if row.guildAvoidable then out[#out+1] = "on the guild's avoidable list" end
    if row.avoidable then out[#out+1] = "flagged by the game" end
    return out
end
local function identity(person)
    return (person.class or "class unknown") .. " " .. (person.level or "level unknown") .. "  " ..
        (person.guild and ("<"..person.guild..">") or "guild unknown")
end
local function popupNumber(method)
    local popup = R.popup
    if not popup then return nil end
    local value = R.clean(R.call(function() return popup[method](popup) end), "number")
    local scale = R.clean(R.call(function() return popup:GetEffectiveScale() end), "number")
    local parentScale = R.clean(R.call(function() return UIParent:GetEffectiveScale() end), "number")
    if value and scale and parentScale and parentScale > 0 then return value * scale / parentScale end
end
function R.resetPosition()
    if not R.frame then return end
    local saved = R.settings.position
    local x, y = R.field(saved, "x", "number"), R.field(saved, "y", "number")
    if not x or not y then x, y = (popupNumber("GetRight") or UIParent:GetWidth()/2) + 12, popupNumber("GetTop") or UIParent:GetHeight()*0.7 end
    x = math.max(0, math.min(x, UIParent:GetWidth()-410))
    y = math.max(R.frame:GetHeight(), math.min(y, UIParent:GetHeight()-40))
    R.frame:ClearAllPoints()
    R.frame:SetPoint("TOPLEFT", UIParent, "BOTTOMLEFT", x, y)
end
function R.close() if R.frame then R.frame:Hide() end end
function R.open(index)
    R.buildUI()
    if not R.frame then return end
    R.selected, R.offset = index or 1, 0
    R.render(); R.resetPosition(); R.frame:Show()
end
function R.scroll(delta)
    local recap = R.history[R.selected or 1]
    R.offset = math.max(0, math.min((R.offset or 0)+delta, math.max(0, recap and #recap.rows-5 or 0)))
    R.render()
end
function R.render()
    if not R.frame then return end
    local recap = R.history[R.selected or 1]
    for i, tab in ipairs(R.tabs) do
        local entry = R.history[i]
        tab:SetShown(entry ~= nil)
        if entry then
            tab.full = entry.clock .. "  " .. entry.zone .. "\n" .. entry.killer
            tab.label:SetText(short(entry.clock .. " " .. entry.zone, 20) .. "\n" .. short(entry.killer,20))
            tab.middle:SetAtlas(i == (R.selected or 1) and "_uiframe-activetab-center" or "_uiframe-tab-center")
        end
    end
    local rows, attackers = recap and recap.rows or {}, recap and recap.attackers or {}
    local killing = recap and recap.killingBlow
    R.header:SetText(killing and ("Killed by " .. R.spell(killing)) or "Death Recap")
    local killerText = recap and recap.killer or "Unknown"
    for _, person in ipairs(attackers) do
        if killing and person.guid == killing.sourceGUID then killerText = killerText .. "  " .. identity(person); break end
    end
    R.killer:SetText(killerText)
    R.headerIcon:SetTexture(killing and iconFor(killing))
    R.headerAvoid:SetShown(killing and (killing.guildAvoidable or killing.avoidable) or false)
    R.pvpTitle:SetShown(#attackers > 0)
    for i, a in ipairs(R.attackerFrames) do
        local person = attackers[i]
        a:SetShown(person ~= nil)
        if person then
            local name = person.name or "Unknown"
            if person.kind == "pet" then name = name .. (person.ownerGUID and " - pet (owner resolved)" or " - pet (owner unknown)") end
            a.name:SetText(short(name,34))
            a.details:SetText(short(identity(person),56))
            a.share:SetText(person.share and string.format("%.1f%%",person.share) or "")
            a.bar:SetShown(person.share ~= nil)
            if person.share then a.bar:SetValue(person.share) end
            a.full = name .. "\n" .. identity(person) .. (person.ownerGUID and ("\nOwner: "..person.ownerGUID) or "")
        end
    end
    local top = 100 + (#attackers > 0 and 28 + #attackers*24 or 0)
    for i, frame in ipairs(R.rowFrames) do
        local row = rows[i+(R.offset or 0)]
        frame:SetShown(row ~= nil)
        frame:ClearAllPoints(); frame:SetPoint("TOPLEFT", R.frame, "TOPLEFT",12,-top-(i-1)*32)
        if row then
            frame.row = row
            frame.amount:SetText(row.amount and string.format("-%.0f",row.amount) or "")
            frame.spell:SetText(short(R.spell(row),20) .. (row.critical == true and " (Critical)" or ""))
            frame.source:SetText(short(row.sourceName or "Unknown",25))
            frame.extra:SetText(row.overkill and row.overkill > 0 and ("Overkill " .. string.format("%.0f",row.overkill)) or
                (row.blocked and row.blocked > 0 and ("Blocked " .. string.format("%.0f",row.blocked)) or ""))
            frame.time:SetText(row.seconds and string.format("%.1f s",row.seconds) or "")
            frame.health:SetShown(row.healthPercent ~= nil)
            if row.healthPercent then frame.health:SetValue(row.healthPercent) end
            frame.hp:SetText(row.healthPercent and string.format("%.0f%%",row.healthPercent) or "")
            frame.icon:SetTexture(iconFor(row))
            frame.avoid:SetShown(row.guildAvoidable or row.avoidable or false)
        end
    end
    R.empty:SetShown(#rows == 0)
    R.empty:SetText("the game recorded no recap for this death")
    R.footer:SetText("Shows the last " .. #rows .. " hits the game recorded,\nnot a full 10 seconds. No heals.")
    local bottom = top + 160
    R.frame:SetHeight(bottom+96)
    R.footer:ClearAllPoints(); R.footer:SetPoint("TOPLEFT",R.frame,"TOPLEFT",16,-bottom-6)
    R.closeButton:ClearAllPoints(); R.closeButton:SetPoint("TOPLEFT",R.frame,"TOPLEFT",286,-bottom-60)
    R.previous:ClearAllPoints(); R.previous:SetPoint("TOPLEFT",R.frame,"TOPLEFT",16,-bottom-60)
    R.next:ClearAllPoints(); R.next:SetPoint("TOPLEFT",R.frame,"TOPLEFT",100,-bottom-60)
    R.previous:SetShown(#rows > 5); R.next:SetShown(#rows > 5)
end
function R.popupOpened(dialog)
    if R.field(dialog,"which","string") ~= "DEATH" then
        if R.popup == dialog then R.popup = nil; if R.detailButton then R.detailButton:Hide() end end
        return
    end
    R.popup = dialog
    R.buildUI()
    local x, y = popupNumber("GetCenter"), popupNumber("GetBottom")
    if x and y and R.detailButton then
        R.detailButton:ClearAllPoints()
        R.detailButton:SetPoint("CENTER", UIParent, "BOTTOMLEFT", x, y-16)
        R.detailButton:Show()
    end
    if not R.settings.position then R.resetPosition() end
end
function R.popupClosed(dialog)
    if R.popup == dialog then R.popup=nil; if R.detailButton then R.detailButton:Hide() end end
end
function R.buildUI()
    if R.frame or not UIParent then return end
    local frame = CreateFrame("Frame", "RelaxDeathRecapFrame", UIParent, "BackdropTemplate")
    R.frame = frame
    frame:SetSize(410,356); frame:SetFrameStrata("MEDIUM")
    frame:SetMovable(true); frame:SetClampedToScreen(true); frame:EnableMouse(true); frame:EnableMouseWheel(true)
    frame:SetBackdrop({bgFile="Interface\\DialogFrame\\UI-DialogBox-Background-Dark", edgeFile="Interface\\DialogFrame\\UI-DialogBox-Border",
        tile=true, tileSize=32, edgeSize=32, insets={left=11,right=12,top=12,bottom=11}})
    frame:SetBackdropColor(0.08,0.08,0.08,0.95)
    script(frame,"OnMouseWheel",function(_,delta) R.scroll(-delta) end)
    R.tabs, R.rowFrames, R.attackerFrames = {}, {}, {}
    for i=1,3 do
        local tab = CreateFrame("Button",nil,frame)
        tab:SetPoint("TOPLEFT",frame,"TOPLEFT",10+(i-1)*130,38); tab:SetSize(128,38)
        tab.middle=texture(tab,0,0,128,38); tab.middle:SetAtlas("_uiframe-tab-center")
        tab.label=text(tab,"GameFontHighlightSmall",6,-2,116,32)
        script(tab,"OnClick",function() R.open(i) end)
        script(tab,"OnEnter",function() tooltip(tab,tab.full or "") end); script(tab,"OnLeave",leave)
        R.tabs[i]=tab
    end
    R.titleBar=CreateFrame("Button",nil,frame)
    R.titleBar:SetPoint("TOPLEFT",frame,"TOPLEFT",12,-4); R.titleBar:SetSize(350,28)
    R.titleBar:RegisterForDrag("LeftButton")
    text(R.titleBar,"GameFontNormal",0,-4,300):SetText("Death Recap")
    script(R.titleBar,"OnDragStart",function() frame:StartMoving() end)
    script(R.titleBar,"OnDragStop",function()
        frame:StopMovingOrSizing()
        if R.writable ~= false then R.settings.position={x=frame:GetLeft(), y=frame:GetTop()}; R.persist() end
        R.resetPosition()
    end)
    R.x=CreateFrame("Button",nil,frame,"UIPanelCloseButton")
    R.x:SetPoint("TOPRIGHT",frame,"TOPRIGHT",-4,-4); script(R.x,"OnClick",R.close)
    R.headerIcon=texture(frame,16,-36,32,32)
    R.header=text(frame,"GameFontNormal",56,-36,310)
    R.killer=text(frame,"GameFontHighlightSmall",56,-56,320)
    R.headerAvoid=texture(frame,380,-38,16,16); R.headerAvoid:SetAtlas("damagemeters-avoidabledamage-icon")
    R.pvpTitle=text(frame,"GameFontNormal",16,-82,378,32)
    R.pvpTitle:SetText("WHO KILLED YOU\n(share of the damage shown below)")
    for i=1,12 do
        local a=CreateFrame("Frame",nil,frame); a:SetSize(378,24); a:SetPoint("TOPLEFT",frame,"TOPLEFT",16,-116-(i-1)*24)
        a.name=text(a,"GameFontHighlightSmall",0,0,235,12)
        a.details=text(a,"GameFontHighlightSmall",0,-12,378,12)
        a.bar=bar(a,240,-2,80,10); a.share=text(a,"GameFontHighlightSmall",326,0,52)
        a:EnableMouse(true); script(a,"OnEnter",function() tooltip(a,a.full or "") end); script(a,"OnLeave",leave)
        R.attackerFrames[i]=a
    end
    for i=1,5 do
        local row=CreateFrame("Frame",nil,frame); row:SetSize(386,32); row:EnableMouse(true)
        row.amount=text(row,"GameFontRed",0,0,58)
        row.icon=texture(row,60,0,28,28)
        row.spell=text(row,"GameFontNormal",94,0,210)
        row.source=text(row,"GameFontHighlightSmall",94,-16,132)
        row.extra=text(row,"GameFontHighlightSmall",228,-16,90)
        row.time=text(row,"GameFontHighlightSmall",306,0,50)
        row.health=bar(row,356,-2,28,8); row.hp=text(row,"GameFontHighlightSmall",356,-16,30)
        row.avoid=texture(row,74,-16,12,12); row.avoid:SetAtlas("damagemeters-avoidabledamage-icon")
        script(row,"OnEnter",function() if row.row then tooltip(row,R.spell(row.row),extras(row.row)) end end)
        script(row,"OnLeave",leave)
        R.rowFrames[i]=row
    end
    R.empty=text(frame,"GameFontHighlightSmall",24,-140,360,40)
    R.footer=text(frame,"GameFontHighlightSmall",16,-266,378,32)
    R.closeButton=button(frame,"Close",286,-316,108,R.close)
    R.previous=button(frame,"Newer",16,-316,80,function() R.scroll(-1) end)
    R.next=button(frame,"Older",100,-316,80,function() R.scroll(1) end)
    R.detailButton=button(UIParent,"Detailed recap",0,0,144,function() R.open(1) end)
    R.detailButton:SetFrameStrata("MEDIUM"); R.detailButton:Hide()
    if UISpecialFrames then table.insert(UISpecialFrames,"RelaxDeathRecapFrame") end
    R.render(); R.resetPosition(); frame:Hide()
end
-- Public callback registry, no edits to the built-in dialog or link handler.
pcall(function() EventRegistry:RegisterCallback("SetItemRef", function(_,link) pcall(R.link,link) end, R) end)
pcall(function()
    PopupEventManager:RegisterCallback("PopupOpened", function(_,dialog) pcall(R.popupOpened,dialog) end, R)
    PopupEventManager:RegisterCallback("PopupClosed", function(_,dialog) pcall(R.popupClosed,dialog) end, R)
end)
pcall(R.buildUI)
pcall(function() local dialog=StaticPopup_FindVisible("DEATH"); if dialog then R.popupOpened(dialog) end end)
