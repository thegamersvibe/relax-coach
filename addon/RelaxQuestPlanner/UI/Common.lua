local RQP = RelaxQuestPlanner
local UI = RQP.UI

-- Relax theme: flat dark panels with the Journey blue accent instead of default WoW tooltip chrome.
local WHITE = "Interface\\Buttons\\WHITE8x8"
UI.THEME = {
  bg = {0.035, 0.047, 0.075, 0.94}, header = {0.027, 0.224, 0.361, 0.95}, border = {0.0, 0.518, 0.922, 0.85},
  card = {0.07, 0.09, 0.14, 0.9}, button = {0.09, 0.12, 0.19, 0.95}, hover = {0.0, 0.42, 0.75, 0.95},
  accent = {0.0, 0.518, 0.922, 1}, muted = {0.55, 0.62, 0.72, 1},
  kind = {accept = {1, 0.82, 0}, complete = {0.0, 0.518, 0.922}, turnin = {1, 0.65, 0}},
}
UI.KIND_HEX = {accept = "ffd100", complete = "33a3ff", turnin = "ffa500"}
local ICON = "Interface\\AddOns\\RelaxQuestPlanner\\Media\\relax-icon"

local function flat(frame, fill, edge)
  if not frame.SetBackdrop then return end
  frame:SetBackdrop({bgFile = WHITE, edgeFile = WHITE, edgeSize = 1, insets = {left = 1, right = 1, top = 1, bottom = 1}})
  frame:SetBackdropColor(unpack(fill))
  frame:SetBackdropBorderColor(unpack(edge or UI.THEME.border))
end
UI.Flat = flat

function UI.Frame(name, width, height)
  local ok, frame = pcall(CreateFrame, "Frame", name, UIParent, "BackdropTemplate")
  frame = ok and frame or CreateFrame("Frame", name, UIParent)
  frame:SetSize(width, height)
  frame:SetPoint("CENTER", UIParent, "CENTER", 0, 0)
  frame:SetClampedToScreen(true)
  frame:EnableMouse(true)
  flat(frame, UI.THEME.bg)
  UISpecialFrames = UISpecialFrames or {}
  table.insert(UISpecialFrames, name)
  return frame
end

-- Accent header strip with the Relax icon. Returns the title font string.
function UI.Header(frame, title, width)
  local bar = frame:CreateTexture(nil, "BACKGROUND", nil, 1)
  bar:SetPoint("TOPLEFT", frame, "TOPLEFT", 1, -1)
  bar:SetPoint("TOPRIGHT", frame, "TOPRIGHT", -1, -1)
  bar:SetHeight(32)
  bar:SetColorTexture(unpack(UI.THEME.header))
  local line = frame:CreateTexture(nil, "BORDER")
  line:SetPoint("TOPLEFT", bar, "BOTTOMLEFT", 0, 0)
  line:SetPoint("TOPRIGHT", bar, "BOTTOMRIGHT", 0, 0)
  line:SetHeight(1)
  line:SetColorTexture(unpack(UI.THEME.accent))
  local icon = frame:CreateTexture(nil, "ARTWORK")
  icon:SetSize(24, 24)
  icon:SetPoint("TOPLEFT", frame, "TOPLEFT", 6, -5)
  icon:SetTexture(ICON)
  local text = UI.Text(frame, "GameFontNormal", 36, -10, width or 300, 18)
  text:SetTextColor(1, 1, 1)
  text:SetText(title)
  frame.headerBar, frame.headerIcon = bar, icon
  return text
end

local function paint(button)
  local t = UI.THEME
  if button.IsEnabled and not button:IsEnabled() then
    flat(button, {0.06, 0.07, 0.1, 0.8}, {0.2, 0.24, 0.3, 0.8})
    if button.label then button.label:SetTextColor(0.45, 0.48, 0.55) end
  elseif button.hovered then
    flat(button, t.hover, t.accent)
    if button.label then button.label:SetTextColor(1, 1, 1) end
  else
    flat(button, button.primary and {0.0, 0.33, 0.6, 0.95} or t.button, button.primary and t.accent or {0.16, 0.26, 0.4, 0.9})
    if button.label then button.label:SetTextColor(0.92, 0.95, 1) end
  end
end
UI.PaintButton = paint
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

function UI.Button(parent, label, width, x, y, fn, primary)
  local ok, button = pcall(CreateFrame, "Button", nil, parent, "BackdropTemplate")
  button = ok and button or CreateFrame("Button", nil, parent)
  button:SetSize(width, 22)
  button:SetPoint("TOPLEFT", parent, "TOPLEFT", x, y)
  local fs = button:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
  fs:SetPoint("LEFT", button, "LEFT", 8, 0)
  fs:SetPoint("RIGHT", button, "RIGHT", -8, 0)
  fs:SetJustifyH("CENTER")
  if button.SetFontString then button:SetFontString(fs) end
  button.label, button.primary = fs, primary
  button:SetText(label)
  button:SetScript("OnClick", RQP.Safe(fn))
  button:HookScript("OnEnter", function(self) self.hovered = true; paint(self) end)
  button:HookScript("OnLeave", function(self) self.hovered = nil; paint(self) end)
  button:SetScript("OnEnable", function(self) paint(self) end)
  button:SetScript("OnDisable", function(self) paint(self) end)
  paint(button)
  return button
end

-- List rows: left-aligned text on a card background.
function UI.Row(parent, width, x, y, fn)
  local row = UI.Button(parent, "", width, x, y, fn)
  row:SetHeight(24)
  if row.label then row.label:SetJustifyH("LEFT") end
  return row
end

function UI.CloseButton(frame, fn)
  local b = UI.Button(frame, "X", 22, frame:GetWidth() - 28, -6, fn)
  b:SetHeight(20)
  return b
end

function UI.Tooltip(frame, text)
  -- Disabled buttons still need to explain unavailable optional dependencies.
  if frame.EnableMouseMotion then frame:EnableMouseMotion(true) end
  frame:SetScript("OnEnter", RQP.Safe(function(self)
    if self.label then self.hovered = true; paint(self) end
    if not GameTooltip then return end
    GameTooltip:SetOwner(self, "ANCHOR_RIGHT")
    GameTooltip:SetText(type(text) == "function" and text() or text)
    GameTooltip:Show()
  end))
  frame:SetScript("OnLeave", RQP.Safe(function(self)
    if self and self.label then self.hovered = nil; paint(self) end
    if GameTooltip then GameTooltip:Hide() end
  end))
end

-- Side windows dock to the main window instead of stacking on top of it.
function UI.Dock(child)
  if not (child and UI.frame) then return end
  child:ClearAllPoints()
  local right = UI.frame.GetRight and UI.frame:GetRight()
  local screen = UIParent and UIParent.GetWidth and UIParent:GetWidth()
  local w = child.GetWidth and child:GetWidth() or 0
  if right and screen and right + w + 6 > screen then
    child:SetPoint("TOPRIGHT", UI.frame, "TOPLEFT", -6, 0)
  else
    child:SetPoint("TOPLEFT", UI.frame, "TOPRIGHT", 6, 0)
  end
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
