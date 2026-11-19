package main

import (
	"strings"
	"testing"
)

func gearTestChar(t *testing.T, body string) CharView {
	t.Helper()
	vars, err := ParseSavedVariables(`ForeverCoachDB = {schema=1, chars={ ["Probe-Test"]={` + body + `} } }`)
	if err != nil {
		t.Fatal(err)
	}
	chars := buildChars(M(vars["ForeverCoachDB"]))
	if len(chars) != 1 {
		t.Fatalf("character count = %d", len(chars))
	}
	return chars[0]
}

func gearTestSlot(t *testing.T, c CharView, name string) SlotView {
	t.Helper()
	for _, slot := range c.Slots {
		if slot.Name == name {
			return slot
		}
	}
	t.Fatalf("missing slot %q", name)
	return SlotView{}
}

func TestGearViewUncachedItemKeepsIdentity(t *testing.T) {
	c := gearTestChar(t, `snapshot={name="Probe",level=20,class="WARRIOR",xpMax=10000,
		equipped={[5]={id=987654}}, equipmentState={[5]="equipped"}},
		gear={status="ok",slots={[5]={scoreStatus="unknown"}}}`)
	s := gearTestSlot(t, c, "Chest")
	if !s.Equipped || s.Item != "Item #987654 (name pending)" || s.GapPct != -1 || s.Status != "unknown" {
		t.Fatalf("uncached equipped item lost identity or was scored: %+v", s)
	}
	html, err := renderReportBody(View{Chars: []CharView{c}})
	if err != nil || !strings.Contains(html, "Item #987654 (name pending)") {
		t.Fatalf("equipped ID not shown in actual report: %v", err)
	}
	if !strings.Contains(html, "Not scored for this spec") {
		t.Fatal("unknown tooltip falsely asserts the item is absent from the provider")
	}
}

func TestGearViewUnreadableNeverClaimsEmpty(t *testing.T) {
	c := gearTestChar(t, `snapshot={name="Probe",level=20,class="WARRIOR",xpMax=10000,
		equipped={}, equipmentState={[5]="unreadable",[6]="empty"}},
		gear={status="ok",slots={[5]={scoreStatus="empty",gapPct=100,best={name="Test chest"}},[6]={scoreStatus="empty"}}}`)
	s := gearTestSlot(t, c, "Chest")
	if s.Status != "unreadable" || s.GapPct != -1 || c.GearEmpty != 0 {
		t.Fatalf("unreadable slot was classified as empty or scored: slot=%+v empty=%d", s, c.GearEmpty)
	}
	var html strings.Builder
	if err := reportTemplate.ExecuteTemplate(&html, "slot", s); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html.String(), ">Empty<") || !strings.Contains(html.String(), "Not read") {
		t.Fatalf("unreadable slot render is misleading: %s", html.String())
	}
	var emptyHTML strings.Builder
	if err := reportTemplate.ExecuteTemplate(&emptyHTML, "slot", gearTestSlot(t, c, "Waist")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(emptyHTML.String(), ">Empty<") {
		t.Fatal("confirmed empty slot no longer shows Empty")
	}
}

func TestGearViewLegacyUncapturedSlotsSuppressFalseAdvice(t *testing.T) {
	c := gearTestChar(t, `reason="logout",snapshot={name="Probe",level=17,class="WARRIOR",xpMax=0,money=0,
		equipped={},bags={},baginfo={slots=0}},
		gear={status="ok",slots={[5]={scoreStatus="empty",gapPct=100,best={name="Test chest"}}}},
		findings={{area="Gear",text="Empty Chest slot"},{area="Build",text="Keep valid build finding"}}`)
	if c.GearStatus != "unavailable" || c.GearEmpty != 0 || c.GearNote == "" {
		t.Fatalf("old incomplete save was treated as valid empty inventory: %+v", c)
	}
	for _, f := range c.Findings {
		if f.Text == "Empty Chest slot" {
			t.Fatal("old saved empty-slot advice still presented despite unknown equipment")
		}
	}
	if c.Counts["Gear"] != 0 || c.Counts["Build"] != 1 {
		t.Fatalf("finding counts weren't recalculated: %v", c.Counts)
	}
}

func TestGearViewLegacyStatEstimateIsNotComparable(t *testing.T) {
	c := gearTestChar(t, `snapshot={name="Probe",level=20,class="WARRIOR",xpMax=10000,
		equipped={[5]={id=987654},[10]={id=12345}},
		equipmentState={[1]="empty",[2]="empty",[3]="empty",[15]="empty",[5]="equipped",[9]="empty",[10]="equipped",[6]="empty",[7]="empty",[8]="empty",[11]="empty",[12]="empty",[13]="empty",[14]="empty",[16]="empty",[17]="empty",[18]="empty"}},
		gear={status="ok",slots={[5]={scoreStatus="estimated",score=2.15,gapPct=92,best={name="Test chest"}},[10]={scoreStatus="known",score=50,gapPct=50,best={name="Test gloves"}}},
		owned={{inv=5,label="Chest",name="Test bag chest",gain=100},{inv=10,label="Hands",name="Test bag gloves",gain=50}}},
		findings={{area="Gear",text="Chest: Test chest is 92% better"},{area="Gear",text="Upgrade in your bags: Test bag chest for Chest"},
		{area="Gear",text="Hands: Test gloves is 50% better"},{area="Build",text="Keep valid build finding"}}`)
	s := gearTestSlot(t, c, "Chest")
	if s.Status != "unknown" || s.GapPct != -1 || !s.Equipped || c.GearUpgrades != 1 {
		t.Fatalf("legacy stat units used for pipeline percentage: %+v upgrades=%d", s, c.GearUpgrades)
	}
	if c.Counts["Gear"] != 1 || c.Counts["Build"] != 1 || len(c.Owned) != 1 || c.Owned[0].Slot != "Hands" {
		t.Fatalf("incomparable saved advice survived or valid advice lost: findings=%+v owned=%+v", c.Findings, c.Owned)
	}
}

func TestGearViewLayoutStatusSaysFormatChanged(t *testing.T) {
	c := gearTestChar(t, `snapshot={name="Probe",level=51,class="WARRIOR",xpMax=10000, equipped={}},
		gear={status="layout",slots={}}`)
	if !strings.Contains(c.GearNote, "GearQuest changed its data format") || strings.Contains(c.GearNote, "enable") {
		t.Fatalf("layout status note = %q", c.GearNote)
	}
}

func TestGearViewHiddenPercentStillCountsUpgrade(t *testing.T) {
	c := gearTestChar(t, `snapshot={name="Probe",level=20,class="WARRIOR",xpMax=10000,
		equipped={[5]={id=1001},[7]={id=1002}}, equipmentState={[5]="equipped",[7]="equipped"}},
		gear={status="ok",slots={[5]={scoreStatus="known",score=10,gapPct=100,pctHidden=true,best={name="Test chest"}},
			[7]={scoreStatus="known",score=10,gapPct=40,best={name="Test legs"}}}}`)
	chest, legs := gearTestSlot(t, c, "Chest"), gearTestSlot(t, c, "Legs")
	if chest.GapPct != -1 || legs.GapPct != 40 || c.GearUpgrades != 2 {
		t.Fatalf("hidden percentage shown or upgrade lost: chest=%d legs=%d upgrades=%d", chest.GapPct, legs.GapPct, c.GearUpgrades)
	}
}

// One clamp rule with the addon (G.PctText): a gain above 100 reads ">100%"; a bag upgrade's percentage is
// hidden when too few slots are scored.
func TestGearViewPercentClampAndHiddenBagPercent(t *testing.T) {
	c := gearTestChar(t, `snapshot={name="Probe",level=51,class="WARRIOR",xpMax=10000,
		equipped={[18]={id=1008},[5]={id=1004}}, equipmentState={[18]="equipped",[5]="equipped",[11]="empty",[2]="empty"}},
		gear={status="ok",slots={[18]={id=1008,scoreStatus="known",score=100,gapPct=300,best={id=9001,name="Big Gun"}},
			[5]={id=1004,scoreStatus="known",score=100,gapPct=45,best={id=9002,name="Chest"}}},
		owned={{id=9003,name="Bag Ring",label="Ring 1",inv=11,gain=250},{id=9004,name="Bag Neck",label="Neck",inv=2,gain=20,pctHidden=true}}}`)
	if s := gearTestSlot(t, c, "Ranged"); s.GapText != ">100%" || s.GapPct != 100 {
		t.Fatalf("300%% gain: want >100%% text and colour 100, got %+v", s)
	}
	if s := gearTestSlot(t, c, "Chest"); s.GapText != "45%" {
		t.Fatalf("45%% gain text: %+v", s)
	}
	if len(c.Owned) != 2 || c.Owned[0].GainText != "+>100%" || c.Owned[1].GainText != "" {
		t.Fatalf("owned gain text: %+v", c.Owned)
	}
	html, err := renderReportBody(View{Chars: []CharView{c}})
	if err != nil {
		t.Fatal(err)
	}
	text := strings.NewReplacer("&#43;", "+", "&gt;", ">").Replace(html) // the template escapes + and >
	if !strings.Contains(text, "about +>100%") || !strings.Contains(text, "+>100%</div>") || strings.Contains(text, "300%") ||
		strings.Contains(text, "about +20%") {
		t.Fatal("report shows an unclamped or hidden percentage")
	}
}
