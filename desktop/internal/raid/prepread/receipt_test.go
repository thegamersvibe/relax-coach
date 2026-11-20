package prepread

import "testing"

func TestEarliestUnorderedReceipts(t *testing.T) {
	r := rootFixture(t, "dedup")
	firstNight(r)["windows"].(table)[float64(2)].(table)["reports"].(table)["alia-stone-pvp-eu"].(table)["receivedAt"] = float64(epoch - 10)
	got, d := Parse(rawRoot(r), epoch, epoch)
	if len(got) != 1 || got[0].Stamped != epoch-10 || len(d.Warnings) != 0 {
		t.Fatalf("earliest reference not selected: %+v %+v", got, d)
	}
}
