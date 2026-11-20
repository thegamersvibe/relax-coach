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

// Detail holds per-player projections beside the model fights. Offset/Len and
// Specs mirror the corresponding model fields; byte length includes the final
// line ending. Timelines keep original events, not rewritten pet actors.
type Detail struct {
	// RolesComputed is false in index mode. True with an empty Fight.Roles
	// means inference ran but evidence was insufficient; no role is guessed.
	RolesComputed          bool
	Offset, Len            int64
	LowConfidence          bool
	UnscoredReason         string
	Specs                  map[string]string
	Timelines              map[string]*Timeline
	RemainingHP, MaximumHP model.Opt
}
type Sample struct {
	Power, PowerMax, PowerType model.Opt
	TimeMs                     int64
	HP, HPMax, X, Y            model.Opt
}
type Timeline struct {
	Events                                                                                []model.Event
	Samples                                                                               []Sample
	Deaths                                                                                []model.Event
	DamageDone, DamageTaken, HealingDone, HealingReceived, OverhealDone, OverhealReceived int64
}
