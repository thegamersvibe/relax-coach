# Relax Raid preparation v1 (R2)

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

(The spaces above stand for TAB bytes.) Unknown scalar is `u`. Lists use `-`
for a checked-empty list, `u` for unreadable; gear additionally uses `n` for
not checked. Consumable rows are `category,itemID,count`, gear rows are
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

## SavedVariables v1

Only these TOC identifiers exist:

```
SavedVariables: RelaxRaidDB
SavedVariablesPerCharacter: RelaxRaidCharDB
```

R2 writes only `RelaxRaidDB.version = 1` and `RelaxRaidDB.prep`:

```lua
RelaxRaidDB = {
    version = 1,
    prep = {
        version = 1,
        nights = {
            {
                day = 20832,
                windows = {
                    {
                        openedAt = 1800000000,
                        reports = {
                            ["alia-stone-pvp-eu"] = {
                                receivedAt = 1800000000,
                                report = { -- fields listed above
                                    protocol = 1,
                                    name = "Alia-Stone-PvP-EU",
                                    -- ...
                                },
                            },
                        },
                    },
                },
            },
        },
    },
}
```

`RelaxRaidCharDB` remains R1's `{version=1, logger=boolean-or-absent}`; R2 adds
no per-character fields. Existing unrelated account-root keys are retained.

`day = floor((receiverServerTime - 43200) / 86400)`: a UTC-noon storage bucket
that does not split an evening at midnight. It is **not** the app's content-
derived `nightId`. The app must match reports to fights using receiver times.
A window begins with the first accepted report, or the first report after an
encounter end. Join/ready reports in the same window replace that character's
previous report. Arrival order, with the receiver's own timestamp, wins; a
sender's clock cannot make its old report outrank a later receipt. Reload
retains the last window. Empty windows are not persisted.

Caps: 3 night buckets, 80 windows per night, 40 characters per window, 9600
reports total. Oldest inserted nights/windows are removed; a full player map
evicts its oldest receiver timestamp (whole-name lexical tie-break) before
inserting a new player. Known versions are rebuilt from validated bounded
fields at load; malformed records are discarded. Unknown future root/prep or
character versions block all Store writes and are left unchanged. Writers
recheck supported root/prep versions, not only the initialization decision.

A filled maximum-field synthetic store, including 19 `notScored` slots per
report, measured 38881331 bytes using the test's canonical compact Lua-literal
serializer; the review-fix reload measurement was 2.691 seconds under lupa.
The test fills all 9600 records, asserts a 50 MiB
compact serialization budget, reloads and compares the entire serialization.
This is **not** measured WoW file formatting, Lua heap size or native frame
latency. Tables and Blizzard serialization add overhead. Newer-save byte
preservation is tested with canonical serialization before/after all writer
paths; native client's file-order/whitespace behavior is not asserted.

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

No R2 spec disagreement was found. Explicit DEC-40 and PHASE1-BRIEF supersede
older logger-default and packaging wording. R2 does not modify the separate
app upload/raw-log policies or compute a score.
