# Decoder contract and test data

`logfmt.Decode(io.Reader, *ParseReport)` yields owned `model.Event` values lazily.
It never opens a path, closes the reader, uploads data or changes the model.
Use `fixture.Write`, `Actor`, `Advanced` and `Combatant` to build invented log
rows. Fixture production code imports no decoder code; decoder production code
imports no fixture code. The test fixtures use invented names and identifiers.

## Accounting and boundaries

- `LinesRead = Parsed + HeadersSeen + sum(Skipped)`. Unknown names are yielded as
  `Other` and included in `Parsed`; `UnknownEvents` is a separate breakdown, not
  an additional term in that equation. Every known name has an explicit width
  table; an unsupported width is a skip, not an `Other` fallback.
- A header is a boundary event. The latest accepted header is in `report.Header`
  **before** its boundary is yielded. It includes the line number so glue can
  hand the right metadata to a segment consumer. No history of all headers is
  retained. `Header.Raw` contains the fields after the event name: the version,
  then key/value pairs.
- Only `Other`, `CombatantInfo` and `Header` retain `Raw`. Typed strings are copied so an
  actor identifier cannot accidentally retain the entire source line.
- Byte offsets are zero-based; line numbers are one-based; length includes LF
  or CRLF. Skipped input still advances the byte position. A final unterminated
  line is skipped even when its tokens happen to look complete; parse a fresh
  snapshot after more bytes arrive. Read failures are in `report.ReadError`.
- `First` and `Last` are valid timestamp observations in file order. `TimeMs`
  is the written wall time, not UTC; `OffsetH` is separate. Advanced-on/off
  evidence comes only from accepted widths that support the advanced block,
  not header claims. A header-only file therefore has unknown evidence.
- `limits.json` caps one physical line, including its line ending. Oversized
  lines are drained without accumulating them, counted and skipped; reading
  continues. Report names own their storage; unknown names must match
  `^[A-Z_]+$`, be at most 64 bytes long, and fit a shared 64-name budget
  (including skipped lines). Unsafe or excess names share the `other` bucket
  in `EventCounts`, `UnknownEvents` and `SkippedLayouts`. Known event names
  do not consume this budget. Raw skipped input is not retained.

## Typed field conventions and unresolved seams

- Stats are COMBATANT_INFO fields 2–25, in order. Talents, gear and auras are
  decoded; unknown gear extras and the complete combatant fields remain raw.
  No enchant meaning, class/spec mapping or item database is invented.
- Optional invalid/unavailable numbers stay absent, distinct from a present
  zero. Integral decimal spellings in energize are accepted; a fractional power
  amount is absent because `model.Opt` cannot represent fractions. Position
  samples are hundredths of a yard, rounded to the nearest hundredth. Advanced
  power type/current/maximum are separate optional values of `InfoUnit`, from
  block offsets 10/11/12; present zero mana is not absent. Power cost is unused.
  The offset reference is [Advanced Combat Logging, WowCoach.gg](https://wowcoach.gg/docs/combat-log/advanced-logging), also cited in PLAN M2.
- Absorb `Src` and `Dst` preserve the logged common actors. `Caster` is the shield caster
  (identifier, name and flags), `ExtraSpellID` the shield spell, and `Amount`
  the absorbed amount, for both widths. The swing form has `SpellID == 0`;
  the spell form keeps the attack spell separately. `InfoOwnerID` is not reused.
- Invalid actor flags (including uint32 overflow) skip the line under
  `InvalidFlags`; valid logged hexadecimal flags remain accepted.
- `NameNULBytes` counts NUL bytes and `InvalidUTF8Names` counts invalid-UTF-8
  name occurrences in recognized actor layouts, including rows later skipped
  for invalid values. Source, destination and absorb caster fields are checked;
  unsupported layouts and non-actor text are not guessed. Names are preserved
  byte-for-byte after wire unescaping, never sanitized or rejected for these bytes.

## Evidence

Format layouts were implemented from the lane's PHASE1-BRIEF R3 and PROBES
P1-full/P1-second descriptions, not copied parser code. Escape-heavy names,
repeated live headers, nonadvanced layouts, a wipe and forty-player encounters
are synthetic format-reported cases, not claims of native observation.

The tests include every-byte cuts, mixed line endings, reader failure and early
cancellation, oversized-line recovery, retained-event memory, and a generated
200 MiB file under a fixed heap ceiling. Module registration enables the normal
raid runner; mutants are run separately by CI. The real-log test is skipped
unless `RELAX_RAID_REAL_LOGS` names a local folder containing the two reference
files. It emits only aggregate counts and event-name/field-count diagnostics.
Never set that variable in CI or commit private input or output with actors.

No native game or Windows acceptance was run (alpha, owner's decision).
