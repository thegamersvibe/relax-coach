param([Parameter(Mandatory=$true)][string]$SourceRoot)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
foreach($name in @('Install-RelaxCoach.ps1','Restore-RelaxCoach.ps1')) {
  $tokens=$null;$errors=$null
  [void][System.Management.Automation.Language.Parser]::ParseFile((Join-Path $SourceRoot $name),[ref]$tokens,[ref]$errors)
  if($errors.Count) { throw ($errors | Out-String) }
}
$root=Join-Path $env:TEMP ('relax-installer-test-'+[guid]::NewGuid().ToString('N'))
$game=Join-Path $root 'game';$app=Join-Path $root 'app';$desktop=Join-Path $root 'desktop';$backups=Join-Path $root 'backups'
foreach($rel in @('Interface\AddOns\ForeverCoach','Interface\AddOns\Journey','Interface\Content\Journey-Content','WTF\Account\TEST\SavedVariables')) { New-Item -ItemType Directory -Path (Join-Path $game $rel) -Force | Out-Null }
New-Item -ItemType Directory -Path $app,$desktop -Force | Out-Null
$sentinels=@{
  (Join-Path $game 'Interface\AddOns\ForeverCoach\before.txt')='old coach';
  (Join-Path $game 'Interface\AddOns\Journey\before.txt')='old journey';
  (Join-Path $game 'Interface\Content\Journey-Content\before.txt')='old guides';
  (Join-Path $game 'WTF\Account\TEST\SavedVariables\ForeverCoach.lua')='ForeverCoachDB = { untouched=true }';
  (Join-Path $app 'RelaxCoach.exe')='old helper';
  (Join-Path $desktop 'Forever Coach.lnk')='old shortcut';
  (Join-Path $desktop 'Relax Coach.lnk')='prior relax shortcut'
}
foreach($p in $sentinels.Keys) { [IO.File]::WriteAllText($p,$sentinels[$p]) }
$save=Join-Path $game 'WTF\Account\TEST\SavedVariables\ForeverCoach.lua'
$saveHash=(Get-FileHash $save).Hash
$raw=& (Join-Path $SourceRoot 'Install-RelaxCoach.ps1') -Game $game -SourceRoot $SourceRoot -AppRoot $app -DesktopRoot $desktop -BackupRoot $backups
$result=($raw -join "`n") | ConvertFrom-Json
if($result.status -ne 'installed') { throw 'Installation did not report verified success' }
$manifest=[IO.File]::ReadAllText((Join-Path $SourceRoot 'manifest.json'))|ConvertFrom-Json
$receipt=[IO.File]::ReadAllText((Join-Path $result.backup 'installation.json'))|ConvertFrom-Json
if($receipt.version -ne ([string]$manifest.version+'-guild')) { throw 'Installation receipt version is stale' }
if((Get-FileHash $save).Hash -ne $saveHash) { throw 'SavedVariables changed' }
if(Test-Path (Join-Path $desktop 'Forever Coach.lnk')) { throw 'Old shortcut was not retired' }
$shortcut=(New-Object -ComObject WScript.Shell).CreateShortcut((Join-Path $desktop 'Relax Coach.lnk'))
if($shortcut.TargetPath -ne (Join-Path $app 'RelaxCoach.exe')) { throw 'New shortcut targets wrong exe' }
if($shortcut.Arguments -ne '') { throw 'Shortcut overrides the game folder remembered by the native app' }
# Bundles from 1.5.0 carry Relax Quest Planner as a fourth folder.
$hasPlanner=@($manifest.files | Where-Object { ([string]$_.path).StartsWith('Interface/AddOns/RelaxQuestPlanner/',[StringComparison]::OrdinalIgnoreCase) }).Count -gt 0
$plannerTarget=Join-Path $game 'Interface\AddOns\RelaxQuestPlanner'
if($hasPlanner) {
  # Fresh client: the planner is installed and no backup is invented for it.
  if(!(Test-Path -LiteralPath (Join-Path $plannerTarget 'RelaxQuestPlanner.toc'))) { throw 'Planner not installed on a fresh client' }
  $slot=@($receipt.slots | Where-Object { $_.target -eq $plannerTarget })
  if($slot.Count -ne 1 -or $slot[0].existed) { throw 'Fresh-client planner slot missing or marked as existing' }
}
$restore=& (Join-Path $result.backup 'Restore-RelaxCoach.ps1') -Backup $result.backup
foreach($p in $sentinels.Keys) { if([IO.File]::ReadAllText($p) -ne $sentinels[$p]) { throw "Rollback mismatch: $p" } }
if(Test-Path (Join-Path $game 'Interface\AddOns\Journey\Main.xml')) { throw 'New files survived rollback' }
if($hasPlanner -and (Test-Path -LiteralPath $plannerTarget)) { throw 'Restore left a planner folder on a client that had none' }
if($hasPlanner) {
  # Client with a manually installed planner (older test version): it is backed up,
  # replaced (obsolete files gone) and brought back byte for byte by Restore.
  New-Item -ItemType Directory -Path $plannerTarget -Force | Out-Null
  $manual=@{
    (Join-Path $plannerTarget 'RelaxQuestPlanner.toc')="## Title: Relax Quest Planner`n## Version: 0.1.2-test`n";
    (Join-Path $plannerTarget 'OnlyInOldPatch.lua')='-- only in the manual test version'
  }
  foreach($p in $manual.Keys) { [IO.File]::WriteAllText($p,$manual[$p]) }
  $raw2=& (Join-Path $SourceRoot 'Install-RelaxCoach.ps1') -Game $game -SourceRoot $SourceRoot -AppRoot $app -DesktopRoot $desktop -BackupRoot (Join-Path $root 'planner-backups')
  $result2=($raw2 -join "`n") | ConvertFrom-Json
  if($result2.status -ne 'installed') { throw 'Install over a manual planner did not report success' }
  if(Test-Path -LiteralPath (Join-Path $plannerTarget 'OnlyInOldPatch.lua')) { throw 'Obsolete manual planner file survived the install' }
  $receipt2=[IO.File]::ReadAllText((Join-Path $result2.backup 'installation.json'))|ConvertFrom-Json
  $slot2=@($receipt2.slots | Where-Object { $_.target -eq $plannerTarget })
  if($slot2.Count -ne 1 -or !$slot2[0].existed) { throw 'Manual planner was not recorded as an existing, backed-up folder' }
  foreach($p in $manual.Keys) {
    $copy=Join-Path $slot2[0].backup ([IO.Path]::GetFileName($p))
    if(!(Test-Path -LiteralPath $copy) -or [IO.File]::ReadAllText($copy) -ne $manual[$p]) { throw "Manual planner file not in backup: $p" }
  }
  & (Join-Path $result2.backup 'Restore-RelaxCoach.ps1') -Backup $result2.backup | Out-Null
  foreach($p in $manual.Keys) { if(!(Test-Path -LiteralPath $p) -or [IO.File]::ReadAllText($p) -ne $manual[$p]) { throw "Restore did not bring back the manual planner: $p" } }
  if(@(Get-ChildItem -LiteralPath $plannerTarget -File -Recurse).Count -ne $manual.Count) { throw 'Restore left new planner files next to the manual version' }
  Remove-Item -LiteralPath $plannerTarget -Recurse -Force
  foreach($p in $sentinels.Keys) { if([IO.File]::ReadAllText($p) -ne $sentinels[$p]) { throw "Planner scenario changed another target: $p" } }
}
# A filesystem failure after the three game directories activates must auto-restore them.
$blockedApp=Join-Path $root 'blocked-app'
[IO.File]::WriteAllText($blockedApp,'not a directory')
$autoRejected=$false
try { & (Join-Path $SourceRoot 'Install-RelaxCoach.ps1') -Game $game -SourceRoot $SourceRoot -AppRoot $blockedApp -DesktopRoot $desktop -BackupRoot (Join-Path $root 'auto-rollback') | Out-Null } catch { $autoRejected=$true }
if(!$autoRejected) { throw 'Filesystem failure was not raised' }
$autoState=Get-ChildItem -LiteralPath (Join-Path $root 'auto-rollback') -Filter installation.json -Recurse | Select-Object -First 1
if(!$autoState -or ((Get-Content -LiteralPath $autoState.FullName -Raw | ConvertFrom-Json).status -ne 'restored')) { throw 'Mid-install failure did not execute rollback' }
foreach($p in $sentinels.Keys) { if([IO.File]::ReadAllText($p) -ne $sentinels[$p]) { throw "Automatic rollback mismatch: $p" } }
# Corrupt a copied payload. Refusal must occur before any backup/apply.
$bad=Join-Path $root 'corrupt-payload'
Copy-Item -LiteralPath $SourceRoot -Destination $bad -Recurse
Add-Content -LiteralPath (Join-Path $bad 'README.md') -Value 'tamper'
$rejected=$false
try { & (Join-Path $bad 'Install-RelaxCoach.ps1') -Game $game -SourceRoot $bad -AppRoot $app -DesktopRoot $desktop -BackupRoot (Join-Path $root 'bad-backups') | Out-Null } catch { if($_ -match 'checksum mismatch') { $rejected=$true } else { throw } }
if(!$rejected) { throw 'Corrupt payload accepted' }
if(Test-Path (Join-Path $root 'bad-backups')) { throw 'Corrupt payload caused backup or writes' }
foreach($p in $sentinels.Keys) { if([IO.File]::ReadAllText($p) -ne $sentinels[$p]) { throw "Rejected install changed target: $p" } }
[ordered]@{status='passed';checks=@('PowerShell syntax','manifest-verified installation','two addon folders','planner fresh install + restore (bundles from 1.5.0)','planner manual version backed up + restored (bundles from 1.5.0)','actual shortcut target','old shortcut retired','SavedVariables unchanged','rollback byte equality','rollback removes new files','corrupt payload rejected before changes');verifiedFiles=$result.verifiedFiles;plannerCovered=$hasPlanner;testRoot=$root}|ConvertTo-Json
