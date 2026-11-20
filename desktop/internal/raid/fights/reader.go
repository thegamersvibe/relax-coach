package fights

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"io"
)

// Source identifies the entire raw stream, including skipped/outside-run lines.
type Source = model.Source

// Decoder is an adapter seam, not a parser. Adapt logfmt.Decode in package main;
// fights must never import logfmt. emit must be called synchronously, in file order.
type Decoder func(io.Reader, func(model.Event)) error

// Read hashes raw bytes as the caller's decoder reads them. Segmentation buffers
// only the active fight (including its configured tail). Closed index-only fights
// discard events; selected details intentionally retain events in the result.
// Run metadata, ownership facts, combatants and per-file repeat keys grow with
// fights/actors, not the number of discarded event lines. Death-scope diagnostics
// additionally hold one timestamp and membership bit per real player death until
// EOF, then release them. There is no fixed byte cap on one fight or on requested
// output; this is not a constant-memory API.
// Errors return no result: a truncated hash must never name a complete source.
func Read(raw io.Reader, decode Decoder, p Params) (Result, error) {
	if raw == nil || decode == nil {
		return Result{}, errors.New("fights: reader and decoder required")
	}
	hash := sha256.New()
	counted := &countReader{r: io.TeeReader(raw, hash)}
	var decodeErr error
	seq := func(yield func(model.Event) bool) {
		decodeErr = decode(counted, func(e model.Event) { yield(e) })
	}
	result, err := collect(seq, p)
	if err != nil {
		return Result{}, err
	}
	if decodeErr != nil {
		return Result{}, decodeErr
	}
	if _, err = io.Copy(io.Discard, counted); err != nil {
		return Result{}, err
	}
	if counted.err != nil {
		return Result{}, counted.err
	}
	result.Source = Source{SHA256: fmt.Sprintf("%x", hash.Sum(nil)), Bytes: counted.n}
	for i := range result.Runs {
		result.Runs[i].Source = result.Source
	}
	return result, nil
}

type countReader struct {
	r   io.Reader
	n   int64
	err error
}

func (c *countReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.n += int64(n)
	if err != nil && err != io.EOF && c.err == nil {
		c.err = err
	}
	return n, err
}

// RaidNight refuses dungeon/unknown runs. There is deliberately no score or
// baseline writer here: downstream code must accept the distinct model type.
func (r Run) RaidNight() (model.RaidNight, bool) {
	if r.Kind != model.RunRaid {
		return model.RaidNight{}, false
	}
	return model.RaidNight{Fights: r.Fights, Roster: r.Roster, Partial: r.Partial, Logger: r.Logger, StartMs: r.StartMs, OffsetH: r.OffsetH, Build: r.Build, LogVersion: r.LogVersion, Advanced: r.Advanced, Source: r.Source}, true
}
func (r Run) DungeonRun() (model.DungeonRun, bool) {
	if r.Kind != model.RunDungeon {
		return model.DungeonRun{}, false
	}
	return model.DungeonRun{Fights: r.Fights, Roster: r.Roster, InstanceID: r.InstanceID, Partial: r.Partial, Logger: r.Logger, StartMs: r.StartMs, OffsetH: r.OffsetH, Build: r.Build, LogVersion: r.LogVersion, Advanced: r.Advanced, Source: r.Source}, true
}
