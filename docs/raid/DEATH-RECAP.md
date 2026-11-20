# Shared death recap (Relax Raid lane)

## Contract and limits

Target: WoW Forever, source version **1.60.1.70959**, interface 16001.
Source pin: Gethe/wow-ui-source **3d8abb1da73acd38e412e24fb3f99454a6392346**.
All paths below are under `Interface/AddOns/` at that pin. These are source
observations, not game execution. **Not tested in game (alpha, owner's decision).**
Native rendering/taint and the accepted text mockup's visual comparison were not
run. Offline tests use synthetic data only; no donor addon code is loaded.

`DeathRecap.lua` and `DeathRecapUI.lua` are addon-independent source files for a
later LayerEnemies port. One `RelaxDeathRecap` global owns the window and its
plain event frame. The higher module version owns it; the first equal version
wins. Other copies only register named hooks. Both hooks receive the settled
recap once, after the final retry (or release/revive). This lane changes no
LayerEnemies code, tracking, scoring, wire format or updater.

The API supplies damage, not heals or a fixed time window. About five hits is
an expectation, **not a documented Forever count**. Copy at most twelve rows;
show five at a time with mouse-wheel/Newer/Older scrolling. Read on death and
retry at 0.3, 1 and 2 seconds. Duplicate death events before release/revive do
not create new records. Later callbacks cannot modify a different death.
No restricted combat-log listener or health sampler is installed.

Missing, inaccessible, secret or throwing fields are absent, not formatted as
zero. `critical`, `sourceGUID` and `sourceFlags` are optional observed inputs,
**not a guaranteed native schema**. A hidden/unreadable caster is not attributed.
Effective damage subtracts positive overkill; negative native overkill means
none. An unreadable amount or overkill makes shares unknown. Shares use damage
from **all** copied rows, so displayed hostile-player shares need not sum to
100 when creatures or other sources also damaged the player. Pets stay separate,
including when a readable owner GUID exists; owner resolution never guesses a
name or merges their damage into a player. No hostile player means no PvP block.

The API has no documented freshness/ready contract. This is a display of the
client's current recap snapshot, not proof that every delayed native row belongs
to a new death; no tracking or other external action is based on it in this lane.
The future LE integration still needs its own attribution/tracking safeguards.

## Hook and save interface

After the host's `ADDON_LOADED`, call
`RelaxDeathRecap.register(addonName, {settings=table, avoidable=fn, identity=fn, onRecap=fn})`.
Only the current owner supplies persisted settings. An optional
`saveSettings(table)` hook binds initially session-local defaults when a setting,
position or retained history is actually saved; default-only login creates no
character save. `avoidable(spellId)` returns
true/nil/false. `identity(guid, name)` returns class, level, guild; absent identity
falls back to **class only** from `GetPlayerInfoByGUID`. `onRecap(recap)` receives
a copied snapshot. Callbacks are guarded independently. No hook is required.

Relax Raid stores `deathRecap` inside `RelaxRaidCharDB`; no new SavedVariables.
Settings schema 1: `auto=true`, `autoclose=true`, `keep=false`, optional position
`{x,y}`. A newer root or settings version is untouched. History is session-only
unless keep is on: three recaps, twelve rows each, strings at most 160 bytes,
finite numbers bounded to 1e12. Persist only whitelisted raw fields plus ID,
zone, clock and max health; computed UI state and foreign tables never persist.
Wrong-type fields or oversized/sparse saved arrays reject the recap. Turning
keep off deletes the saved history, not the current session tabs. Data contains
player names and remains local. A replacement owner clears the previous owner's
schema-1 saved history and alone writes subsequent history.

The window is a plain frame on UIParent, MEDIUM strata, with stock textures.
X, Close and Escape hide it; release/revive auto-close is optional and on by
default. Auto-pop defaults on. Position is character-specific, restored with
screen clamping; `/recap reset` removes the position. A separate button uses
numeric UIParent-relative coordinates from the DEATH popup, never a popup anchor
or parent. It never modifies the popup, hides it or invokes a release action.
Absent/throwing popup callbacks leave the local link and slash command working.
Tab links use monotonically assigned record IDs, not shifting tab positions;
expired links cannot reopen a different death.

## Rules source

`scripts/build_rules_lua.py` now also exports `AvoidableSpells.lua` from
`rules/raid/encounters-enabled.json` -> `encounters[].mechanics[]` where
`avoidable=true` and `spell.id` has the existing `{kind,note}` source metadata.
The output is deduplicated and sorted. Name-only spells and candidate encounters
never supply guessed IDs. `--check` compares both generated outputs without
writing. The current enabled mechanics arrays are empty, so the generated table
is empty. The native `avoidable` flag still applies. No JSON schema/data version
or scoring implementation changed. Regenerating the original Rules.lua leaves
its bytes unchanged.

## Pinned API and field evidence

| Used surface | Source and lines | Contract used |
|---|---|---|
| Native getters | `Blizzard_APIDocumentationGenerated/DeathRecapDocumentation.lua:3-24,40-70` | `C_DeathRecap.HasRecapEvents()`, `GetRecapEvents()`, `GetRecapMaxHealth()`; nilable recap ID omitted; all under pcall |
| Row schema limit | Same file `:77-85` | Generated event field list is empty; treat every field as optional |
| Names/type/spell | `Blizzard_DeathRecap/Mainline/Blizzard_DeathRecap.lua:100-117` | `spellName`, `event`, `spellId`, `environmentalType` (corrected plan citation) |
| Amounts, identity visibility, flags | Same file `:161-222` | `amount`, `overkill`, `absorbed`, `resisted`, `blocked`, `currentHP`, `hideCaster`, `sourceName`, `school`, `deadly`, `avoidable`; positive-overkill subtraction at 169-172 |
| Relative time | Same file `:324-339` | largest readable timestamp minus row timestamp; unreadable timestamp prevents a guessed killing blow |
| Access guards | `Blizzard_APIDocumentationGenerated/FrameScriptDocumentation.lua:47-80,286-302` | `canaccesstable`, `canaccessvalue`, `issecretvalue`; check before indexing/operations, failures return nil |
| Death lifecycle | `Blizzard_APIDocumentationGenerated/DeathInfoDocumentation.lua:122-148` | PLAYER_DEAD, PLAYER_ALIVE, PLAYER_UNGHOST, no payload needed |
| Retry timer | `Blizzard_APIDocumentationGenerated/UITimerDocumentation.lua:10-20` | `C_Timer.After(seconds, callback)` |
| Clock/zone | `Blizzard_APIDocumentationGenerated/SystemTimeDocumentation.lua:9-18`; `ZoneScriptDocumentation.lua:51-59` | `GetGameTime()` hour/minute, `GetZoneText()` string |
| Player class | `Blizzard_APIDocumentationGenerated/PlayerScriptDocumentation.lua:836-858` | `GetPlayerInfoByGUID(guid)` first return localized class; may return nothing |
| Optional pet owner | `Blizzard_APIDocumentationGenerated/UnitDocumentation.lua:2584-2599,3295-3310` | `UnitTokenFromGUID(guid)` then `UnitOwnerGUID(unitToken)`; each output checked, identity may be secret |
| Hostile/player flags | `Blizzard_APIDocumentationGenerated/CombatLogConstantsDocumentation.lua:18-27` | ReactionHostile=64, ControlPlayer=256, TypePlayer=1024; arithmetic mask on readable integers only; no combat-reader API |
| Icons | `Blizzard_APIDocumentationGenerated/SpellDocumentation.lua:594-612`; `Blizzard_DeathRecap/Mainline/Blizzard_DeathRecap.lua:122-150` | `C_Spell.GetSpellTexture(spellId)` first return file ID; stock environmental icon files |
| Popup subscription | `Blizzard_StaticPopup/StaticPopup.lua:106-112,156-168,620,663`; `StaticPopupGamepad.lua:263-269` | PopupOpened/PopupClosed dialog, optional StaticPopup_FindVisible("DEATH") bootstrap |
| Callback order | `Blizzard_SharedXMLBase/CallbackRegistry.lua:112-141,198-213` | `RegisterCallback(event, fn, owner)` dispatches `fn(owner, ...)` |
| Local chat link | `Blizzard_UIPanels_Game/Shared/ItemRefHandlersShared.lua:279-282` | addon link route triggers EventRegistry SetItemRef; own strict prefix and retained ID only |
| Local chat output | `Blizzard_EncounterWarnings/EncounterWarningsUtil.lua:52` | DEFAULT_CHAT_FRAME:AddMessage; no server send |
| Slash binding | `Blizzard_ChatFrameBase/Shared/ChatFrameUtil.lua:809-833` | SLASH-owned key plus SlashCmdList-owned entry, never replace the table |
| Existing settings | `Blizzard_Settings_Shared/Blizzard_Settings.lua:178-180,388-395` | RegisterProxySetting/CreateCheckbox in Log's existing category |
| Frame creation | `Blizzard_PTRFeedbackGlue/Blizzard_PTRFeedback_Events_Glue.lua:2`; `Blizzard_DeathRecap/Mainline/Blizzard_DeathRecap.xml:96-100,194-222` | CreateFrame plain frame; own UIParent child, no action template |
| Escape | `Blizzard_UIParentPanelManager/Shared/UIParentPanelManager.lua:21,1107-1118` | append only our frame name to UISpecialFrames |
| Tooltip | `Blizzard_DeathRecap/Mainline/Blizzard_DeathRecap.lua:45-77`; `Blizzard_APIDocumentationGenerated/FrameAPITooltipDocumentation.lua:120-138` | GameTooltip:SetOwner/AddLine/SetText, Show/Hide |

### Widget methods (all at the same pin)

The following generated files live in `Blizzard_APIDocumentationGenerated/`.
Only addon-owned ordinary widgets receive setters. Generated protection flags
on frame methods are not a claim of in-game permission for a protected frame.
Popup coordinates may be secret and are checked before arithmetic; a failed
coordinate read hides no Blizzard UI and provides no guessed attachment.

| File | Called methods and declaration lines |
|---|---|
| `SimpleFrameAPIDocumentation.lua` | CreateFontString 78; CreateTexture 131; GetEffectiveScale 381; Hide 687; RegisterEvent 1019; RegisterForDrag 1054; SetClampedToScreen 1207; SetFrameStrata 1293; SetMovable 1405; SetShown 1484; Show 1539; StartMoving 1548; StopMovingOrSizing 1571 |
| `SimpleScriptRegionAPIDocumentation.lua` | EnableMouse 73; EnableMouseWheel 95; GetBottom 130; GetCenter 145; GetHeight 161; GetLeft 178; GetRight 211; GetTop 308; GetWidth 323; Hide 357; SetScript 712; SetShown 725; Show 751 |
| `SimpleScriptRegionResizingAPIDocumentation.lua` | ClearAllPoints 22; SetHeight 124; SetPoint 135 (explicit frame, relative point, offsets); SetSize 163 |
| `SimpleFontStringAPIDocumentation.lua` | SetJustifyH 560; SetText 664 |
| `SimpleButtonAPIDocumentation.lua` | SetText 476 |
| `SimpleTextureBaseAPIDocumentation.lua` | SetAtlas 368; SetTexture 604; file-ID usage also observed in stock recap at 150/215 |
| `SimpleStatusBarAPIDocumentation.lua` | SetMinMaxValues 229; SetStatusBarColor 282; SetStatusBarTexture 318; SetValue 355 |
| `Blizzard_SharedXML/Backdrop.lua` | SetBackdrop 336; SetBackdropColor 406; dark dialog asset preset 27-35 |

### Stock templates, fonts and assets

`Blizzard_SharedXML/Backdrop.xml:3-10` declares BackdropTemplate.
`Blizzard_SharedXML/Mainline/SharedUIPanelTemplates.xml:134-152,315-320`
declares UIPanelCloseButton/UIPanelButtonTemplate; `:936-962` supplies the active
and inactive tab-center atlases; `:2168` references the status-bar texture.
`Blizzard_DeathRecap/Mainline/Blizzard_DeathRecap.xml:15-26` references the
avoidable atlas. `Blizzard_Fonts_Shared/Shared/FontStyles.xml:50,66,92` supplies
GameFontNormal, GameFontHighlightSmall and GameFontRed. These references do not
prove the assets rendered in a native client. No media files were added.

## Offline acceptance

`test/raid/recap/module.json` registers the Lua/Python harness with raidtest;
`mutants.json` changes **DeathRecap.lua only**. CI's existing all-Lua/Python step
runs the new suite, and the Lua mutation step includes `recap`. Mocks cover late
third-retry data, all fields secret/missing/throwing, zero max health, no events,
environmental/single-row recaps, shares/pets, hook failures and load order,
retention/persistence corruption, native-popup isolation, options, combat and
all instance types, tabs/scrolling/position, link validation and generated rules.
They do not execute another addon, certify native rendering, or certify the
future LE tracking lane. The existing core frame-count assertion now separately
requires exactly one Core dispatcher and one portable recap dispatcher.
