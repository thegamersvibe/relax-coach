package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// CreatureSelector mirrors cfg's boss selector (ID is a creature type).
type CreatureSelector struct {
	ID           int64
	Name, Source string
}

// SpecProfile is the signature-only projection of cfg.Profile. A caller adapts
// cfg profiles and supplies inference; empty signatures are not talent evidence.
type SpecProfile struct {
	ID             string
	CastSignatures []SpellSignature
}
type SpellSignature struct {
	ID   int64
	Name string
}

// Detail carries fields absent from the frozen shared model. Byte length includes
// the final line ending. Timelines keep original events, not rewritten pet actors.
type Detail struct {
	Offset, Len            int64
	LowConfidence          bool
	UnscoredReason         string
	Specs                  map[string]string
	Timelines              map[string]*Timeline
	RemainingHP, MaximumHP model.Opt
}
type Sample struct {
	TimeMs          int64
	HP, HPMax, X, Y model.Opt
}
type Timeline struct {
	Events                                                                                []model.Event
	Samples                                                                               []Sample
	Deaths                                                                                []model.Event
	DamageDone, DamageTaken, HealingDone, HealingReceived, OverhealDone, OverhealReceived int64
}
