-- Separate read-only review surface. No route, Journey or bag actions.
local RQP=RelaxQuestPlanner
local C,D,U=RQP.Cleanup,RQP.CleanupData,RQP.UI
if not (C and D) then return end
local V={tab='item',page=1,rows={}}
RQP.CleanupUI=V

function V.ProfessionText(row)
    local names={}
    for _,id in ipairs(row.professions or {}) do
        -- Forever bebcb6bf TradeSkillUIDocumentation: skillLineID -> display name.
        local name,state=D.Call(D.Function(C_TradeSkillUI,"GetTradeSkillDisplayName"),id)
        names[#names+1]=state=='known' and type(name)=='string' and name~='' and name or ('profession #'..id..' (name unavailable)')
    end
    return #names>0 and ('Professions: '..table.concat(names,', ')) or ''
end
local function selectedRow(report)
    if not V.selected or not report then return end
    for _,row in ipairs(V.selected.kind=='quest' and report.questRows or report.itemRows) do
        if row.exactID==V.selected.id then return row end
    end
end
local function pinned(row)
    local db=type(RelaxQuestPlannerDB)=='table' and RelaxQuestPlannerDB.cleanup
    return type(db)=='table' and db.schema==1 and type(db.pins)=='table' and type(db.pins[row.kind])=='table'
        and db.pins[row.kind][row.exactID]==true
end
local function details(row)
    local lines={row.name..' ['..row.exactID..']',row.disposition..': '..table.concat(row.reasons,'; '),row.detail,
        'Linked quests / chains: '..(#row.relatedIDs>0 and table.concat(row.relatedIDs,', ') or 'none represented'),
        'Scanned: '..tostring(row.capturedAt or 'unknown')..' (session snapshot)'}
    for _,slot in ipairs(row.slots) do
        lines[#lines+1]='bag '..slot.bag..', slot '..slot.slot..'; count '..tostring(slot.count or 'unknown')
    end
    local professions=V.ProfessionText(row); if professions~='' then lines[#lines+1]=professions end
    for _,e in ipairs(row.evidence) do
        local id=e.questID or e.to
        lines[#lines+1]=(e.role or e.kind or 'association')..(id and (' -> quest '..id) or '')
            ..' ('..(e.source or 'unknown')..' '..(e.version or 'unknown')..')'
    end
    return table.concat(lines,'\n'),#lines
end
function V.Render()
    if not V.frame then return end
    local report,state=C.GetReport()
    V.status:SetText(state..' | '..(report and report.status or 'No snapshot')
        ..' Character: '..(report and report.identity or 'unknown')..'; client '..(report and report.build or 'unknown'))
    V.statusFull=V.status:GetText()
    local rows=report and (V.tab=='quest' and report.questRows or report.itemRows) or {}
    local pages=math.max(1,math.ceil(#rows/4)); V.page=math.min(V.page,pages)
    local counts=report and report.counts or {KEEP=0,['UNCERTAIN-KEEP']=0}
    V.counts:SetText('KEEP '..counts.KEEP..' | UNCERTAIN-KEEP '..counts['UNCERTAIN-KEEP']..' | Page '..V.page..'/'..pages)
    if V.page>1 then V.prev:Enable() else V.prev:Disable() end
    if V.page<pages then V.next:Enable() else V.next:Disable() end
    for i,button in ipairs(V.rows) do
        local row=rows[(V.page-1)*4+i]; button.record=row
        if row then
            button:SetText('Details: '..row.name..' ['..row.exactID..']\n'..row.disposition..': '..table.concat(row.reasons,'; '))
            button:Show()
        else button:Hide() end
    end
    local row=selectedRow(report)
    if row then
        local text,lines=details(row)
        V.details:SetText(text)
        -- Conservative wrapped height; scrolling exposes long evidence lists.
        V.detailChild:SetHeight(math.max(110,lines*20+math.ceil(#text/65)*16))
        V.details:SetHeight(V.detailChild:GetHeight())
        V.pin:SetText(pinned(row) and 'Unpin' or 'Keep pinned'); V.pin:Enable()
    else
        V.details:SetText(#rows==0 and 'No rows in this view. Non-listed items were not assessed.' or 'Click a row for Details and a Keep pin.')
        V.pin:Disable()
    end
end
function V.Build()
    if V.frame then return end
    local f=U.Frame('RelaxQuestReviewFrame',600,570); V.frame=f; f:Hide()
    V.title=U.Header(f,'Quest and item review',450)
    V.close=U.CloseButton(f,function() f:Hide() end)
    f:SetMovable(true); f:RegisterForDrag('LeftButton')
    local prefs=C.Preferences()
    if prefs and prefs.position then
        local p=prefs.position
        f:ClearAllPoints()
        local ok=pcall(f.SetPoint,f,p.point,UIParent,p.relativePoint,p.x,p.y)
        if not ok then f:ClearAllPoints(); f:SetPoint('CENTER',UIParent,'CENTER',0,0) end
    end
    f:SetScript('OnDragStart',function() if C.Combat()==false then f:StartMoving() end end)
    f:SetScript('OnDragStop',function()
        f:StopMovingOrSizing()
        local ok,point,_,relativePoint,x,y=pcall(f.GetPoint,f,1)
        local p=C.Preferences()
        if ok and p and type(point)=='string' and type(relativePoint)=='string' and D.Number(x) and D.Number(y) then
            p.position={point=point,relativePoint=relativePoint,x=x,y=y}
            C.Preferences() -- validate and cap saved coordinates
        end
    end)
    V.questTab=U.Button(f,'Quests',100,12,-42,function() V.tab='quest'; V.page=1; V.Render() end)
    V.itemTab=U.Button(f,'Quest items',120,120,-42,function() V.tab='item'; V.page=1; V.Render() end)
    V.refresh=U.Button(f,'Refresh',90,498,-42,function() C.Refresh(true) end)
    V.statusHover=CreateFrame('Frame',nil,f)
    V.statusHover:SetSize(576,44); V.statusHover:SetPoint('TOPLEFT',f,'TOPLEFT',12,-72)
    V.statusHover:EnableMouse(true)
    V.status=U.Text(V.statusHover,'GameFontHighlightSmall',0,0,576,44)
    U.Tooltip(V.statusHover,function() return V.statusFull or 'Not scanned' end)
    V.coverage=U.Text(f,'GameFontHighlightSmall',12,-122,576,36)
    V.coverage:SetText('Backpack + 4 bags: quest-class / quest-associated items, and grey or unresolved quests. Other items, keyring, reagent bag, bank, equipment, mail and alts were not assessed.')
    for i=1,4 do
        local button
        button=U.Row(f,576,12,-164-(i-1)*52,function()
            if button.record then
                V.selected={kind=button.record.kind,id=button.record.exactID}
                V.detailScroll:SetVerticalScroll(0); V.Render()
            end
        end)
        button:SetHeight(44)
        button.label:SetHeight(40); button.label:SetWordWrap(true)
        U.Tooltip(button,function() return button:GetText() end)
        V.rows[i]=button
    end
    V.prev=U.Button(f,'Previous',80,12,-378,function() V.page=math.max(1,V.page-1); V.Render() end)
    V.next=U.Button(f,'Next',70,100,-378,function() V.page=V.page+1; V.Render() end)
    V.counts=U.Text(f,'GameFontHighlightSmall',180,-382,408,20)
    V.detailScroll=CreateFrame('ScrollFrame',nil,f)
    V.detailScroll:SetSize(576,110); V.detailScroll:SetPoint('TOPLEFT',f,'TOPLEFT',12,-412)
    V.detailChild=CreateFrame('Frame',nil,V.detailScroll); V.detailChild:SetSize(556,110)
    V.detailScroll:SetScrollChild(V.detailChild)
    V.details=U.Text(V.detailChild,'GameFontHighlightSmall',0,0,556,110)
    V.detailScroll:EnableMouseWheel(true)
    V.detailScroll:SetScript('OnMouseWheel',function(self,delta)
        self:SetVerticalScroll(math.max(0,math.min(self:GetVerticalScrollRange(),self:GetVerticalScroll()-delta*32)))
    end)
    V.pin=U.Button(f,'Keep pinned',140,12,-536,function()
        local row=selectedRow(C.GetReport())
        if row and not C.SetPin(row.kind,row.exactID,not pinned(row)) then
            V.status:SetText('Limited data: pin limit reached or newer preferences; not changed.')
        end
    end)
    V.instructions=U.Text(f,'GameFontHighlightSmall',164,-534,424,32)
    V.instructions:SetText('Review the native quest log or bags manually. No item or quest is certified disposable. Scroll Details for evidence.')
    f:SetScript('OnHide',function() C.Close() end)
    f:SetScript('OnShow',function() C.Open() end)
end
function C.Show()
    V.Build()
    if V.frame:IsShown() then C.Open() else V.frame:Show() end
end
