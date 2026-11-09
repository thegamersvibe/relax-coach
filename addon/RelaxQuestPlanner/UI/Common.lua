local RQP = RelaxQuestPlanner
local UI = RQP.UI

function UI.Frame(name, width, height)
  local ok, frame = pcall(CreateFrame, "Frame", name, UIParent, "BackdropTemplate")
  frame = ok and frame or CreateFrame("Frame", name, UIParent)
  frame:SetSize(width, height)
  frame:SetPoint("CENTER", UIParent, "CENTER", 0, 0)
  frame:SetClampedToScreen(true)
  frame:EnableMouse(true)
  if frame.SetBackdrop then
    frame:SetBackdrop({bgFile="Interface\\Tooltips\\UI-Tooltip-Background",
      edgeFile="Interface\\Tooltips\\UI-Tooltip-Border", edgeSize=12,
      insets={left=3, right=3, top=3, bottom=3}})
    frame:SetBackdropColor(0, 0, 0, 0.85)
  end
  UISpecialFrames = UISpecialFrames or {}
  table.insert(UISpecialFrames, name)
  return frame
end

local ANCHORS = {TOP=true, TOPLEFT=true, TOPRIGHT=true, LEFT=true, RIGHT=true,
  CENTER=true, BOTTOM=true, BOTTOMLEFT=true, BOTTOMRIGHT=true}
function UI.Movable(frame, key)
  frame:SetMovable(true)
  frame:RegisterForDrag("LeftButton")
  local saved = RQP.GetDB()[key]
  if type(saved) == "table" and ANCHORS[saved.point] and ANCHORS[saved.relativePoint]
    and RQP.Number(saved.x) and RQP.Number(saved.y) then
    frame:ClearAllPoints()
    frame:SetPoint(saved.point, UIParent, saved.relativePoint, saved.x, saved.y)
  end
  frame:SetScript("OnDragStart", RQP.Safe(function(self)
    if not InCombatLockdown or not InCombatLockdown() then self:StartMoving() end
  end))
  frame:SetScript("OnDragStop", RQP.Safe(function(self)
    self:StopMovingOrSizing()
    local point, _, relativePoint, x, y = self:GetPoint(1)
    if ANCHORS[point] and ANCHORS[relativePoint] and RQP.Number(x) and RQP.Number(y) then
      RQP.GetDB()[key] = {point=point, relativePoint=relativePoint, x=x, y=y}
    end
  end))
end

function UI.Text(parent, font, x, y, width, height)
  local text = parent:CreateFontString(nil, "OVERLAY", font or "GameFontHighlightSmall")
  text:SetPoint("TOPLEFT", parent, "TOPLEFT", x, y)
  text:SetWidth(width)
  if height then text:SetHeight(height) end
  text:SetJustifyH("LEFT")
  text:SetJustifyV("TOP")
  text:SetWordWrap(true)
  return text
end

function UI.Button(parent, label, width, x, y, fn)
  local button = CreateFrame("Button", nil, parent, "UIPanelButtonTemplate")
  button:SetSize(width, 24)
  button:SetPoint("TOPLEFT", parent, "TOPLEFT", x, y)
  button:SetText(label)
  button:SetScript("OnClick", RQP.Safe(fn))
  return button
end

function UI.Tooltip(frame, text)
  -- Disabled buttons still need to explain unavailable optional dependencies.
  if frame.EnableMouseMotion then frame:EnableMouseMotion(true) end
  frame:SetScript("OnEnter", RQP.Safe(function(self)
    if not GameTooltip then return end
    GameTooltip:SetOwner(self, "ANCHOR_RIGHT")
    GameTooltip:SetText(type(text) == "function" and text() or text)
    GameTooltip:Show()
  end))
  frame:SetScript("OnLeave", RQP.Safe(function() if GameTooltip then GameTooltip:Hide() end end))
end

function UI.Show()
  if not UI.frame then UI.BuildWindow() end
  RQP.GetDB().hidden = nil
  UI.frame:Show()
end

function UI.Hide()
  RQP.GetDB().hidden = true
  if UI.frame then UI.frame:Hide() end
  if UI.picker then UI.picker:Hide() end
  if UI.guidePicker then UI.guidePicker:Hide() end
end
