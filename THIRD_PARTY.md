# Third-party media and data

| Component | Author | Imported material | Licence / source |
|---|---|---|---|
| Keep or Toss: Forever 1.7.0 | Aquary | data tables only, no code; positive KEEP evidence in `addon/RelaxQuestPlanner/Core/CleanupUses.lua` | No licence file in the package; included under the owner's ruling. [Pinned package](https://www.curseforge.com/wow/addons/keep-or-toss-forever/files/9575258). |

Keep or Toss archive SHA-256: `03910d502a00f6912cb372f946873908ed7c9ed6f6261c5c645a49991a6fb33d`.
Each generated block names the data file, file hash and build. The archive,
verdict logic and English reason strings are not bundled. Regenerate/check with
`python -I -B tools/gen_cleanup_uses.py /path/KeepOrToss-1.7.0.zip --check`.

## Travel supplication

- File: `addon/ForeverCoach/Media/Reactions/flight.mp3`
- Description: Travel supplication (dua al-safar), Saudia pre-take-off recording.
- Provenance: supplied by the owner on 2026-11-16.
- Source SHA-256: `9f9b9fa41300d060dfaa031c545932277fbad41d6900f72591c034640e40e02f`.
- Processing: whole clip, +3.6 dB gain; metadata stripped; MP3, 44.1 kHz stereo, 192 kbps. No cut, limiter or normalization.
- Licence: licence not established; included by the owner's ruling.

## Forever Logs Companion (timings and client observations only)

- Used in: `addon/RelaxRaid/Log.lua` (the combat-logging retry schedule).
- Author: FangYuanWoW. Version 0.2.3, MIT. Archive SHA-256:
  `8dfe737a4eefdd772474fa4e2c4a09690cfd75447957850fdcd3b7d00ef31cc1`
  ([CurseForge file 9536234](https://www.curseforge.com/wow/addons/forever-logs-companion/files/all)).
- Taken: the retry times 3, 6, 10, 15, 20 and 30 seconds, the 35-second trust of an accepted start, and
  the author's reports about the client (a start ignored for about 10 s after a loading screen, a lagging
  state read, a header line per repeated start). No code or text is copied.
