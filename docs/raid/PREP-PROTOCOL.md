# Relax Raid preparation (R2): wire v1, prep store v2

Target: Forever, Interface 16001. Addon version 0.2.0. This is preparation data,
not scores, a chat announcement, an equipment action, a combat-log reader, or an
upload client. DEC-40's logger defaults and explicit per-character choices are
unchanged. No dependency or SavedVariables name was added.

## Collection and optional providers

`Prep.lua` reads only the local character. Each report contains:

- `protocol = 1`, `addonVersion`, `name`, `class`, `level`;
- `playedLevel`: seconds at level 60, or `unreadable`;
- `consumables`: at most 32 `{category, itemID, count}` rows, or `unreadable`;
- `gear`: at most 19 `{slot, wornID, bagID}` rows, `not checked`, or `unreadable`;
- `notScored`: at most 19 unique slot numbers 1..19 when gear was checked
  (an empty list means none); absent when gear is `not checked` or `unreadable`;
- `reason`: `join`, `ready`, or `encounter`; `sentAt`: sender's server clock.

The bag scan is bounded to bags 0 through 4, at most 100 slots each. Zero
backpack capacity, unavailable item metadata, any secret/unreadable field, an
unknown consumable subclass, or overflow makes the entire consumables field
`unreadable`, never an apparently complete empty list. A confirmed empty list
is allowed. Item IDs are 1..999999999; counts are 1..999999 and stacks of the
same item are aggregated. No item IDs or inferred stacking rules are shipped.

Categories follow the pinned client's consumable subclasses, not item names:
`generic`, `potion`, `elixir`, `flask`, `scroll`, `food`, `enhancement`, `bandage`,
`other`, `vantus`, `utility`, `combat`, `relic`, `draught`, `device`, `temporary`.
These are observations, not the officer's required-consumables scoring policy.
R5's versioned policy can interpret these rows; R2 does not assign points.

For better gear, R2 calls `ForeverCoach:Snapshot()` and
`ForeverCoach.Gear:Audit(snapshot)` under `pcall`, at use time. It consumes the
existing `owned` and `slots` result. Source contract:
`addon/ForeverCoach/Core.lua:384-414`, `Gear.lua:230-239,335-413`. It does not
read GearQuest, Journey, or the item database itself, does not reproduce Coach's
ranking, and has no load-order dependency. Missing Coach/audit/provider data
means `not checked`; throwing, secret, malformed or partially unreadable data
means `unreadable`. In particular, a worn slot with `scoreStatus = "unreadable"`
keeps the entire gear field unreadable; an unreadable worn ID never becomes an
empty slot. A slot with `scoreStatus = "unknown"` instead enters `notScored`, in
ascending slot order, without hiding the other known `audit.owned` upgrades.
Coach already skips unscored slots for owned upgrades; its ranking is unchanged.
The provider is queried again on the next collection, so a provider loaded
later can work without restarting Relax Raid.

`RequestTimePlayed()` is requested once per observed raid join, not at addon
initialization, login, every world entry, or level-up. Group/world events observe
a readable false-to-true membership transition (the first true this session is
a join); unreadable membership does not rearm the request. A readable leave
allows one request at the next join. Level-up invalidates the cache only.
No Blizzard chat frame is hooked, edited, hidden, or unregistered from
`TIME_PLAYED_MSG`; any default `/played` output is left to the client. A readable
`TIME_PLAYED_MSG` is cached only in memory and aged by readable elapsed time;
a level mismatch or unreadable clock gives `unreadable`. It is **label
input only**. No threshold or score is computed by this addon. ADD-05's
configurable `newAt60` label and unchanged-score acceptance belong to the app's
R5/R7 scoring work; the addon tests prove no points field is produced.

## Identity

The own full name is `UnitFullName("player")`'s two documented strings joined
with a hyphen. A readable nil realm uses `GetNormalizedRealmName()`; a secret,
throwing or malformed identity is not repaired. On receive, a hyphen-free sender
or claimed name gets `-<GetNormalizedRealmName()>` before the forged-name check.
A missing, secret, throwing, empty or malformed normalized realm fails closed.
An already-qualified name is never split or realm-stripped. The same rule is
used by validation and Store's independent forged-name check. Original spelling
is retained, including the explicitly appended realm. Different realms stay
separate; this is not a combat-log actor-name conversion.

Name limit: 96 bytes; ASCII letters/digits, single spaces, apostrophes, hyphens,
and bytes 0x80..0xFF (including UTF-8 accented names). Bytes below 0x20, DEL 0x7F,
TAB, pipe, comma, semicolon, and leading/trailing/repeated spaces remain rejected.
Keys lowercase the whole name with Lua's ASCII folding; UTF-8 bytes are retained,
not Unicode case-folded or transliterated. Missing/secret own identity cannot be
encoded. Unusable receive sender/claimed names increment `Wire.stats.badName`;
other malformed envelopes/field counts increment the existing invalid counter
when parsed. Valid but different normalized identities increment `forged`.

Exact live Forever sender spelling remains unverified beyond this defensive
same-realm normalization; other mismatches are dropped and counted.

## Wire grammar

One prefix: `RLXR1`. One message kind: `P` (preparation), version byte `1`.
No hello, acknowledgement, reserve, score, whisper or relay messages exist.
Every raider broadcasts **only its own** report on `RAID`, without a target.
Only currently enabled loggers retain received reports. A logger also retains
its own collected report without depending on a client self-echo.

A fragment is:

```
P1|id|part|total|chunk
```

`id` is a local session counter 1..999999999, `part` and `total` are 1..21,
`part <= total`. Decimals are canonical (no signs, exponents or leading zeros).
The first byte is the kind, the second its version, as PLAN contract 5 requires.
Unknown kinds/versions are rejected. Maximum incoming message: 240 bytes;
maximum chunk: 200 bytes; maximum assembled body: 4096 bytes. The sender's
maximum header plus chunk is 219 bytes. No compression, code evaluation or
custom escaping is used.

Body: exactly eleven TAB-delimited fields. R2 is unreleased: the appended
`notScored` field deliberately keeps protocol/body version **1** and prefix
`RLXR1`/kind `P1`. This is a pre-release grammar revision, not compatibility with
a shipped ten-field body. The decoder rejects **every other field count**,
including ten and twelve, and still rejects every unknown version.

```
1  addonVersion  name  class  level  playedLevel  reason  sentAt  consumables  gear  notScored
```

The spaces above stand for TAB bytes. The wire markers are field-specific:

| Field | Unknown/unreadable marker | Checked empty / not checked |
|---|---|---|
| `addonVersion` | `unreadable` | neither applies; literal text, not `u` |
| `class` | `unreadable` | neither applies; literal text, not `u` |
| `level` | `u` | neither applies |
| `playedLevel` | `u` | zero is a readable zero, not unknown |
| `consumables` | `u` | `-` means checked empty; no not-checked marker |
| `gear` | `u` | `-` means checked empty; `n` means not checked |
| `notScored` | `u` | `-` means checked empty; `n` means not checked |
| `protocol`, `name`, `reason`, `sentAt` | no marker; invalid/unreadable rejects the report | required valid fields |

In decoded reports, `u` becomes `unreadable`, and gear `n` becomes
`not checked`. With unreadable/unchecked gear, `notScored` is absent in the
decoded report, but its wire field is still `u`/`n` respectively. Consumable
rows are `category,itemID,count`, gear rows are
`slot,wornID,bagID`; rows are separated by semicolons. Free-text fields reject
control characters and pipes; numerical/category fields have a closed grammar.
Worn ID 0 means Coach explicitly confirmed an empty slot, never an unreadable
slot. `notScored` is a comma-separated list of unique canonical slot numbers
1..19, `-` for a checked-empty list, `n` with unchecked gear, and `u` with
unreadable gear. It is always the eleventh wire field, even when absent from the
saved report. Mismatched gear/list markers, duplicates, holes, out-of-range
slots and lists over 19 entries are rejected. The same `Prep.validate` copies
and validates wire and saved reports. Pre-release checked-gear saved rows without
`notScored` do not meet the revised shape and are discarded on validation.

### Scheduling and rates (builder-selected limits)

- Raid join: false-to-true `IsInRaid` at group/world events. Ready check and
  encounter end also request collection. No party, guild, whisper or instance
  chat route is used.
- Three coalesced pending reasons and one active report, a single scheduled
  pump, first delay 1 second. Repeated triggers do not extend the original
  120-second deadline. Pending/active work expires at that boundary.
- Each reason: at least 30 seconds between attempted reports. All reasons
  together: at least 10 seconds between attempted report starts.
- Fragments: at least 0.5 second apart; at most 60 attempts in any rolling
  60-second interval, including failures. No retry of a client-refused fragment;
  the next ordinary trigger may try a new report.
- At collection and **immediately before each client send**, require readable
  `IsInRaid == true`, readable `IsInInstance()` boolean and type `none`, `raid`,
  or `party` (never `pvp` or `arena`; unknown types fail closed), both combat
  booleans false, both chat-restriction booleans
  false, all six `GetAddOnRestrictionState` results Inactive (0), and no locally
  observed active encounter. Absent/throwing/secret/unknown answers fail closed.
  In particular Activating (1) is not safe. No restriction is bypassed.
- Blocked work waits on the bounded pump. Leaving/unknown raid membership or
  logout cancels it by generation. Encounter start discards any partially sent
  snapshot, retaining the reason for a fresh collection after restrictions end.

### Receive bounds and trust

Only the exact prefix, kind/version and `RAID` distribution while in a raid are
admitted. Each sender has at most one partial assembly. Parts may arrive out of
order. Identical duplicates do not extend its original 30-second lifetime;
conflicting parts/totals discard the assembly. Incomplete reports are never
stored. A completed report must name its sender under the whole-name rule;
forgeries are dropped and counted independently in Wire and Store.

At most 40 peers, 60 admitted fragments per peer per rolling minute, 2400
admitted packets globally per rolling minute, and 8 retired IDs per peer per
60 seconds. The retirement limit also counts invalid/expired assemblies.
Peer memory expires lazily after 60 seconds of inactivity; leave/logout clears
it. Partial payload storage is at most 40 * 4096 = 163840 bytes, plus bounded
headers, timestamps, rate ledgers and table overhead. Counts saturate at 999999
and are session-only. Hostile text is parsed as data, never executed.

This is not authentication or anti-cheat: the client supplies the transport
sender, but that sender can lie about its own bags. Reports are observations
claimed by their owner. Replay suppression is bounded, not a durable sequence
protocol. A sender reloading and reusing a recent ID may have that report
ignored until its retirement expires; replay after expiry may be admitted.

## SavedVariables: account v1, prep v2

Only these TOC identifiers exist (unchanged):

```
SavedVariables: RelaxRaidDB
SavedVariablesPerCharacter: RelaxRaidCharDB
```

R2 writes `RelaxRaidDB.version = 1` and `RelaxRaidDB.prep.version = 2`.
The prep layout is **not** the account/character schema version or the wire
protocol version. R2 is unreleased: a prep version-1 store is **discarded**, not
migrated. A root/character version above 1, or prep version above 2, latches
Store read-only and is left byte-for-byte untouched. Every put also rechecks
these versions. Resetting a version externally cannot clear the load-time latch.
`RelaxRaidCharDB` remains R1's `{version=1, logger=boolean-or-absent}`. No new
save name or per-character field exists; unrelated account-root keys survive.

The exact layout (the string contains TAB bytes, shown as Lua `\t` escapes):

```lua
RelaxRaidDB = {
    version = 1,
    prep = {
        version = 2,
        nights = { -- dense 1-based array, insertion order
            {
                day = 20832,
                reports = { -- canonical whole-name key -> dense list of bodies
                    ["alia-stone-pvp-eu"] = {
                        "1\t0.2.0\tAlia-Stone-PvP-EU\tMAGE\t60\tu\tready\t1800000000\t-\tn\tn",
                    },
                },
                windows = { -- dense 1-based array, insertion order
                    {
                        openedAt = 1800000000,
                        reports = {
                            ["alia-stone-pvp-eu"] = {
                                receivedAt = 1800000000,
                                report = 1, -- index in this night's reports[key]
                            },
                        },
                    },
                },
            },
        },
    },
}
```

A report body is a **string in the eleven-field wire grammar above**, not an
expanded Lua field table. It has no `P1` fragment header, compression, hash,
executable text, or second stored copy. The later app reader resolves
`night.reports[key][window.reports[key].report]`, parses that body as data, and
uses the window's `receivedAt` for matching to fights. `model.PrepReport` does
not change; implementing that Go reader belongs to the later prep lane.
App 1.8.0 checks the TOC title, manifest-matching addon Version, save names,
load files and dependency prohibition, not the inner prep version.

For each character in each night, only the **latest** retained body participates
in deduplication. Equality covers every `Prep.validate` field except `sentAt`,
including `reason`, original name spelling and list order. Canonical encoding
makes that comparison deterministic: omit exactly field 8 for equality, not
for storage. Unknown extra input keys are not validated fields and are dropped.
Consecutive equal reports reuse the same index, even across 80 windows. A-B-A
is three stored entries, not a lookup of the first A. An equal report retains
the first body's `sentAt`; each receipt still updates its own window timestamp.

**Byte accounting:** `Store.put` re-encodes the validated report through
`Wire.encode` and counts the Lua string length of that canonical body. The
same computation applies to received reports, the own-copy send path and load.
There is no caller-supplied length to trust and no persistent byte counter.
Realm-less names are normalized before encoding, so the stored canonical size
includes the appended realm, rather than the shorter incoming name spelling.
Only distinct appended bodies spend bytes; timestamps on reused window rows
are not charged. The inclusive budget is **100,000 body bytes per character
per night**, not serialized table bytes and not a shared raid budget.

When appending exceeds the budget, remove that character's oldest body(s) until
it fits. Remove that character from every window referring to each dropped
body; decrement its higher indices as the dense list shifts. Other characters'
bodies, indices and receive times do not change. Empty windows are removed.
When window/player-cap eviction leaves a character with no window references,
remove its entire body list too. Older bodies for a still-referenced character
remain until its own byte budget evicts them, even if its latest receipt
replaced their last individual reference. No sender-guild check is added: a
non-guild raid member's report is retained by an enabled logger as before.

`day = floor((receiverServerTime - 43200) / 86400)` is the unchanged UTC-noon
bucket, not the app's content-derived `nightId`. A window starts on the first
accepted report or first report after an encounter end. Within a window, the
last accepted receipt for a character wins regardless of sender time. Reload
retains the last window. Caps remain **3 nights, 80 windows/night, 40
characters/window** (at most 9600 references), not a new cap on distinct bodies.
Oldest inserted nights/windows are removed. A full window evicts its oldest
receiver timestamp, with the whole-name lexical tie-break, before inserting a
new character. These policies are independent of per-character budget eviction.

Load rebuilds only version 2, validates the bounded windows and their numeric
indices, and decodes/re-encodes referenced characters' body lists. Lists must be
dense and bounded; malformed bodies/forged identities are dropped with index
remapping. Budget admission walks newest-first, then restores oldest-first
list order. Invalid references, empty windows/nights and orphan character lists
are discarded. No received or saved body is executed.

### Measured synthetic fill and its scope

The tightened fill saturates each of 40 characters' budgets on each of 3 nights
with distinct maximum-field reports (including 19 `notScored` slots), then fills
all 80 windows with references. Measured canonical compact serialization:
**13,528,853 bytes**, **7,560 bodies**, **11,816,280 body bytes**, **9,600 window
references**. The test asserts **16 MiB**, not the former 50 MiB, reloads and
compares the entire serialization exactly. The final detailed run measured
4.866 seconds to reload under lupa. See `evidence/R2.md` for the command output. Body strings avoid the expanded-field table overhead that
would otherwise make a 100,000-wire-byte budget much larger on disk.

This is the architect's fixed-40-character fill, **not a universal 16 MiB cap
under roster churn**: 40 is a per-window cap, not a per-night identity cap.
Changing the character roster between windows can retain more than 40 distinct
characters per night. A separate rotating-roster probe, with only one maximum-
field body for each of 3200 identities/night, measured 17,761,373 bytes across
3 nights (9600 references and bodies, 15,004,800 body bytes). That exceeds
16 MiB even without filling those characters' individual budgets. No unapproved
nightly character cap or shared byte budget was added. The distinction is an
open sizing-policy question, not silently resolved by dropping other senders'
history.

The measurement is not native WoW file formatting, Lua heap size or frame
latency. Newer-save preservation uses canonical serialization before/after all
writer paths; native file-order/whitespace behavior is not asserted.

## Client API evidence

All paths below are relative to `Interface/AddOns/` in
`Gethe/wow-ui-source@eac30af597e5aaa43a60f6675ef5fdcea100375f` (Forever build
70790), the supplied local source snapshot. `A/` abbreviates
`Blizzard_APIDocumentationGenerated/`. Declarations/source uses are verified;
operation in a live client is not. No API is inferred from Retail memory.

| Direct API/event used by R2 | Pinned file:line | Contract used |
|---|---|---|
| `UnitFullName` | `A/UnitDocumentation.lua:1224-1237` | unit name and server; identity may be secret |
| `GetNormalizedRealmName` (new in review fix) | `A/PlayerScriptDocumentation.lua:651-657` | no arguments; non-nil cstring; guarded despite declaration |
| `UnitClass` | `A/UnitDocumentation.lua:911` | class token is second return |
| `UnitLevel` | `A/UnitDocumentation.lua:2474` | local level, guarded |
| `C_AddOns.GetAddOnMetadata` | `A/AddOnsDocumentation.lua:165` | Version metadata |
| `C_Container.GetContainerNumSlots` | `A/ContainerDocumentation.lua:313-324` | bag slot count |
| `C_Container.GetContainerItemInfo` | `A/ContainerDocumentation.lua:192-205,766-780` | itemID and stackCount; may return nothing |
| `C_Item.GetItemInfo` | `A/ItemDocumentation.lua:626-655` | returns 12/13 are class/subclass; may return nothing |
| Consumable class/subclass values | `A/ItemConstantsDocumentation.lua:276,300-323` | enum IDs, no item IDs |
| Equipped bag traversal | `Blizzard_CatalogShop/Blizzard_CatalogShop.lua:826`; `Blizzard_FrameXMLBase/Constants.lua:168` | backpack through equipped bags; R2 uses Coach's established 0..4 range |
| `RequestTimePlayed` | `A/PlayerScriptDocumentation.lua:1614-1616` | no fabricated arguments |
| `TIME_PLAYED_MSG` | `A/SystemDocumentation.lua:216-224` | total seconds then seconds at this level |
| `PLAYER_LEVEL_UP` | `A/UnitDocumentation.lua:3830-3832` | invalidate played cache |
| `GetServerTime` | `A/SystemTimeDocumentation.lua:30` | sender/receiver wall timestamps |
| `GetTime` (via RR and Prep) | `Blizzard_FrameXML/Shared/LowHealthFrame.lua:161` | elapsed scheduling/aging clock |
| `issecretvalue` | `Blizzard_GamepadSmartNavigation/SecureHelper.lua:6` | detect before comparing/indexing/formatting |
| `IsInRaid` | `Blizzard_UnitFrame/Shared/GroupFrameVisibility.lua:6,10` | raid membership; source use, fail closed on unreadable |
| `IsInInstance` (new in R2 send gate; already used by Log) | `A/InstanceDocumentation.lua:170-177` | no arguments; bool plus instance-type cstring; check both returns |
| `InCombatLockdown` | `A/RestrictedActionsDocumentation.lua:45-51` | explicit boolean false required |
| `UnitAffectingCombat` | `A/UnitDocumentation.lua:614` | player combat; explicit false required |
| `C_ChatInfo.InChatMessagingLockdown` | `A/ChatInfoDocumentation.lua:293-299` | namespace API, not guessed global |
| `C_ChatInfo.AreOutgoingAddonChatMessagesRestricted` | `A/ChatInfoDocumentation.lua:11-17` | realm-controlled outgoing restriction |
| `C_RestrictedActions.GetAddOnRestrictionState` | `A/RestrictedActionsDocumentation.lua:29-41` | exact restriction state, including activation |
| Restriction state/type enums | `A/RestrictedActionsConstantsDocumentation.lua:13-15,26-31` | Inactive 0; types 0 through 5 |
| `C_ChatInfo.RegisterAddonMessagePrefix` | `A/ChatInfoDocumentation.lua:469-481`; `A/ChatConstantsDocumentation.lua:140-143` | numeric Success 0 / DuplicatePrefix 1, not boolean |
| `C_ChatInfo.SendAddonMessage` | `A/ChatInfoDocumentation.lua:516-531`; `A/ChatConstantsDocumentation.lua:154-166` | explicit RAID route, numeric Success 0, no secret arguments |
| `CHAT_MSG_ADDON` | `A/ChatInfoDocumentation.lua:803-815` | prefix, text, distribution, sender first |
| `GROUP_ROSTER_UPDATE` | `A/PartyInfoDocumentation.lua:678-681` | detect joining/leaving |
| `READY_CHECK` | `A/PartyInfoDocumentation.lua:838-842` | trigger only; no secret payload inspected |
| `ENCOUNTER_START`, `ENCOUNTER_END` | `A/EncounterInfoDocumentation.lua:53-55,38-40` | active-encounter guard, fresh snapshot/window |
| `PLAYER_ENTERING_WORLD` | `A/SystemDocumentation.lua:136` | group refresh through existing Core |
| `PLAYER_LOGOUT` | `A/SystemDocumentation.lua:157-160` | cancel owed sends; never flush them |
| `C_Timer.After` (RR.after) | `A/UITimerDocumentation.lua:11` | uncancellable callback, generation invalidation |

R2 registers through existing `RR.on` and uses its guarded callbacks.
Existing Core uses `CreateFrame` (`Blizzard_PTRFeedbackGlue/Blizzard_PTRFeedback_Events_Glue.lua:2`),
`RegisterEvent`/`SetScript` on that frame, `ADDON_LOADED`
(`A/AddOnsDocumentation.lua:391`) and the existing error reporter
`DEFAULT_CHAT_FRAME:AddMessage` (`Blizzard_EncounterWarnings/EncounterWarningsUtil.lua:52`).
R1's logging/settings APIs are unchanged and cited in `Log.lua:13-23`.
No aura API is used: prep reports contain bag contents; buffs at pull, worn gear
and enchants are assigned to the combat-log/app lanes by the approved contract.

## Verification and remaining native uncertainty

Module descriptors under `test/raid/{prep,wire,store}` register with the existing
discovery runner, so `tools/raidtest.py` itself requires no change. CI's existing
`--all --kind lua,py` includes them; the added mutation step covers all five Lua
modules. No new dependency is used. See `evidence/R2.md` for observed red/green
runs and command results.

Native checks are not run (alpha, owner's decision); no player test is requested.
Unverified: live sender spelling, availability/restrictions on bag/item/played
reads, prefix registration/send returns, actual raid delivery/self-echo,
RequestTimePlayed's possible default chat output, Coach audit performance and
Blizzard SavedVariables formatting/flush. Mocks do not certify these. Restricted
or unknown clients remain silent and unknown preparation is never scored as zero.

Explicit DEC-40 and PHASE1-BRIEF supersede older logger-default and packaging
wording. The rotating-roster size boundary above remains open for the architect;
the requested per-character budget and unchanged per-window cap are implemented
without inventing a global cap. R2 does not modify app upload/raw-log policies
or compute a score.
