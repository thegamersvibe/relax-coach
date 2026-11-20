package prepread

import (
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func TestAccentedRealSave(t *testing.T) {
	got, diagnostic, err := ReadFile("testdata/accented.lua", 1800000000, 1800001000)
	want := modeltest.Prep("Élia-Stóne-PvP-EU", "MAGE", 60, 1, "0.2.0").
		Reason("ready").Clock(1800000000, 1800000000).Build()
	if err != nil || !reflect.DeepEqual(got, []model.PrepReport{want}) ||
		len(diagnostic.Warnings) != 0 || len(diagnostic.Skipped) != 0 {
		t.Fatalf("reports=%#v, diagnostic=%+v, err=%v; want=%#v", got, diagnostic, err, want)
	}
}
