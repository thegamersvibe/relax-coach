-- Strict synthetic frame model: no fallback for unknown client methods.
UIFrames, UISpecialFrames = {}, {}
function ExtendFrame(f, kind, name, parent, template)
    f.kind, f.name, f.parent, f.template = kind, name, parent, template
    f.scripts, f.shown, f.width, f.height, f.left, f.top = {}, false, 0, 0, 0, 700
    UIFrames[#UIFrames+1] = f
    if name then _G[name] = f end
    local original = f.SetScript
    function f:SetScript(event, fn) self.scripts[event]=fn; if event=='OnEvent' and original then original(self,event,fn) end end
    function f:Show() self.shown=true; if self.scripts.OnShow then self.scripts.OnShow(self) end end
    function f:Hide() self.shown=false; if self.scripts.OnHide then self.scripts.OnHide(self) end end
    function f:IsShown() return self.shown end
    function f:SetShown(on) if on then self:Show() else self:Hide() end end
    function f:SetSize(w,h) self.width,self.height=w,h end
    function f:SetWidth(w) self.width=w end
    function f:SetHeight(h) self.height=h end
    function f:GetWidth() return self.width end
    function f:GetHeight() return self.height end
    function f:SetPoint(point, relative, other, x,y)
        self.point={point,relative,other,x,y}
        if point=='TOPLEFT' and relative==UIParent and other=='BOTTOMLEFT' then self.left,self.top=x,y end
    end
    function f:ClearAllPoints() self.point=nil end
    function f:GetLeft() return self.left end
    function f:GetTop() return self.top end
    function f:GetRight() return self.left+self.width end
    function f:GetBottom() return self.top-self.height end
    function f:GetCenter() return self.left+self.width/2,self.top-self.height/2 end
    function f:GetEffectiveScale() return 1 end
    function f:SetFrameStrata(v) self.strata=v end
    function f:SetMovable(v) self.movable=v end
    function f:SetClampedToScreen(v) self.clamped=v end
    function f:EnableMouse(v) self.mouse=v end
    function f:EnableMouseWheel(v) self.wheel=v end
    function f:RegisterForDrag(v) self.drag=v end
    function f:StartMoving() self.moving=true end
    function f:StopMovingOrSizing() self.moving=false end
    function f:SetBackdrop(v) self.backdrop=v end
    function f:SetBackdropColor(...) self.backdropColor={...} end
    function f:SetText(v) assert(type(v)=='string'); self.text=v end
    function f:SetJustifyH(v) self.justify=v end
    function f:SetTexture(v) self.texture=v end
    function f:SetAtlas(v) self.atlas=v end
    function f:SetStatusBarTexture(v) self.barTexture=v end
    function f:SetStatusBarColor(...) self.barColor={...} end
    function f:SetMinMaxValues(a,b) self.min,self.max=a,b end
    function f:SetValue(v) self.value=v end
    function f:SetFontObject(v) self.font=v end
    function f:SetNormalTexture(v) self.normalTexture=v end
    function f:SetHighlightTexture(v) self.highlight=v end
    function f:CreateFontString(name,layer,font) local v=ExtendFrame({},'FontString',name,self); v.font=font; return v end
    function f:CreateTexture(name,layer) return ExtendFrame({},'Texture',name,self) end
    return f
end
UIParent=ExtendFrame({},'Frame','UIParent'); UIParent:SetSize(1080,800)
GameTooltip=ExtendFrame({},'GameTooltip')
function GameTooltip:SetText(line,r,g,b,a,wrap)
    assert(type(r)=='number' and type(g)=='number' and type(b)=='number', 'tooltip requires RGB')
    self.text=line
end
function GameTooltip:SetOwner(f,anchor) self.owner=f end
function GameTooltip:AddLine(line) self.text=(self.text or '')..'\n'..line end
local function registry()
    return {callbacks={}, RegisterCallback=function(self,event,fn,owner)
        self.callbacks[event]=self.callbacks[event] or {}; self.callbacks[event][owner]=fn
    end, TriggerEvent=function(self,event,...)
        for owner,fn in pairs(self.callbacks[event] or {}) do fn(owner,...) end
    end}
end
EventRegistry=registry(); PopupEventManager=registry()
C_Spell={GetSpellTexture=function(id) return 12345 end}
releaseCalls=0; RepopMe=function() releaseCalls=releaseCalls+1 end
function Escape() for _,name in ipairs(UISpecialFrames) do _G[name]:Hide() end end
