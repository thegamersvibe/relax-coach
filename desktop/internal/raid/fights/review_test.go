package fights

import (
	"fmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"testing"
)

func TestPlayerGUIDNeverOwnedOrUnowned(t *testing.T) {
	for _, flags := range []uint32{0x1248, 0x11248, 0x1218, 0x2000} {
		for _, evidence := range []string{"flags", "actor", "advanced", "summon"} {
			t.Run(fmt.Sprintf("%x/%s", flags, evidence), func(t *testing.T) {
				a := cato
				a.Flags = flags
				es := boss(7, 1000)
				e := modeltest.At(1010).Src(a).Dst(giant).Damage(9, -1).Build()
				switch evidence {
				case "actor":
					e.Src.OwnerID = bram.ID
				case "advanced":
					e.InfoUnit = a.ID
					e.InfoOwnerID = bram.ID
				case "summon":
					e.Kind = model.Summon
					e.Src = bram
					e.Dst = a
				}
				es = append(es[:4], append([]model.Event{e}, es[4:]...)...)
				f := only(t, build(t, es, params())).Fights[0]
				for _, id := range f.Unowned {
					if id == a.ID {
						t.Fatal("player classified as unowned pet")
					}
				}
				if _, ok := f.Owners[a.ID]; ok {
					t.Fatal("player owned")
				}
				found := false
				for _, p := range f.Players {
					found = found || p.ID == a.ID
				}
				if !found {
					t.Fatal("player absent")
				}
			})
		}
	}
}
func TestCharmedFriendlyDamageExcluded(t *testing.T) {
	for _, flags := range []uint32{0x1248, 0x11248, 0x1218} {
		a := cato
		a.Flags = flags
		friendly := bram
		friendly.Flags = 0x512
		es := boss(7, 1000)
		es = append(es[:4], append([]model.Event{
			modeltest.At(1010).Src(a).Dst(friendly).Damage(11, -1).Build(),
			modeltest.At(1011).Src(a).Dst(giant).Damage(17, -1).Build(),
		}, es[4:]...)...)
		d := only(t, build(t, es, params())).Details[0]
		if d.Timelines[a.ID].DamageDone != 17 || d.Timelines[bram.ID].DamageTaken != 11 {
			t.Fatal("charmed friendly damage credited or hostile damage lost")
		}
	}
}
