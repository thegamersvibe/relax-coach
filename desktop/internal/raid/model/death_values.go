package model

// Death causes are descriptive model values. NIGHT-FILE.md defines their wire keys.
const (
	CauseAvoidable   = "avoidable ability"
	CauseUnavoidable = "unavoidable or one-shot"
	CauseUnhealed    = "unhealed over time"
	CausePulledAggro = "pulled aggro"
	CauseLostBoss    = "tank lost the boss"
	CauseAfterWipe   = "after the wipe call"
	CauseUnresolved  = "unresolved"
)

// Death ownership keeps names separate from the kind of responsibility.
const (
	OwnerPlayer    = "player"
	OwnerHealers   = "healers"
	OwnerNobody    = "nobody"
	OwnerUndecided = "undecided"
)
