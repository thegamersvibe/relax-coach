[CmdletBinding()]
param(
  [Parameter(Mandatory=$true)][string]$Game,
  [string]$SourceRoot=$PSScriptRoot,
  [string]$AppRoot=(Join-Path $env:USERPROFILE 'RelaxCoach'),
  [string]$DesktopRoot=([Environment]::GetFolderPath('Desktop')),
  [string]$BackupRoot=(Join-Path $env:USERPROFILE 'Relax Coach Backups'),
  [string]$ExpectedBaseline=''
)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$Game=[IO.Path]::GetFullPath($Game).TrimEnd('\')
$SourceRoot=[IO.Path]::GetFullPath($SourceRoot).TrimEnd('\')
$AppRoot=[IO.Path]::GetFullPath($AppRoot).TrimEnd('\')
$DesktopRoot=[IO.Path]::GetFullPath($DesktopRoot).TrimEnd('\')
$BackupRoot=[IO.Path]::GetFullPath($BackupRoot).TrimEnd('\')
if (!(Test-Path (Join-Path $Game 'Interface\AddOns')) -or !(Test-Path (Join-Path $Game 'WTF'))) { throw 'Not a game folder: Interface\AddOns and WTF are required.' }
function Assert-GameStopped {
  $processes=@(Get-CimInstance Win32_Process -ErrorAction Stop | Where-Object {$_.Name -match '^(Wow|WowClassic|WowClassicT)\.exe$'})
  foreach($process in $processes) {
    $path=[string]$process.ExecutablePath
    if(!$path -or $path.StartsWith($Game+'\',[StringComparison]::OrdinalIgnoreCase)) {
      throw 'Close WoW before installing Relax Coach. No game files were changed.'
    }
  }
}
Assert-GameStopped
function Get-Sha([string]$Path) { (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() }
$manifest=Get-Content -LiteralPath (Join-Path $SourceRoot 'manifest.json') -Raw | ConvertFrom-Json
$seen=@{}
foreach($row in $manifest.files) {
  $rel=[string]$row.path
  if ([IO.Path]::IsPathRooted($rel) -or $rel.Contains('\') -or $rel.Contains(':') -or ($rel.Split('/') -contains '..') -or ($rel.Split('/') -contains '.') -or $seen.ContainsKey($rel)) { throw "Unsafe/duplicate manifest path: $rel" }
  if ($rel -notmatch '^(Interface/(AddOns/(ForeverCoach|Journey|RelaxQuestPlanner)/|Content/Journey-Content/)|RelaxCoach\.exe$|README\.md$|LICENSE-Coach$|Install-RelaxCoach\.ps1$|Restore-RelaxCoach\.ps1$)') { throw "Unexpected payload path: $rel" }
  $seen[$rel]=$true
  $p=Join-Path $SourceRoot $rel
  if (!(Test-Path -LiteralPath $p -PathType Leaf) -or (Get-Sha $p) -ne $row.sha256) { throw "Payload checksum mismatch: $rel" }
}
$actual=@(Get-ChildItem -LiteralPath $SourceRoot -File -Recurse | Where-Object { $_.FullName -ne (Join-Path $SourceRoot 'manifest.json') })
if($actual.Count -ne $seen.Count) { throw 'Payload contains missing or unlisted files.' }
foreach($mandatory in @('RelaxCoach.exe','Restore-RelaxCoach.ps1','Interface/AddOns/ForeverCoach/ForeverCoach.toc','Interface/AddOns/Journey/Journey.toc')) { if(!$seen.ContainsKey($mandatory)) { throw "Missing required payload: $mandatory" } }
# Bundles from 1.5.0 also carry Relax Quest Planner; an existing (for example manually installed) planner is backed up like the other folders.
$planner=@($seen.Keys | Where-Object { $_.StartsWith('Interface/AddOns/RelaxQuestPlanner/') }).Count -gt 0
if($planner -and !$seen.ContainsKey('Interface/AddOns/RelaxQuestPlanner/RelaxQuestPlanner.toc')) { throw 'Missing required payload: Interface/AddOns/RelaxQuestPlanner/RelaxQuestPlanner.toc' }
if($ExpectedBaseline) {
  $baseline=Get-Content -LiteralPath $ExpectedBaseline -Raw | ConvertFrom-Json
  foreach($row in $baseline.files) {
    $p=Join-Path $Game ([string]$row.path)
    if (!(Test-Path -LiteralPath $p -PathType Leaf) -or (Get-Sha $p) -ne $row.sha256) { throw "Installed file changed since inspection: $($row.path)" }
  }
}
$slots=@(
  [ordered]@{target=(Join-Path $Game 'Interface\AddOns\ForeverCoach');source='Interface\AddOns\ForeverCoach';kind='directory';action='replace'},
  [ordered]@{target=(Join-Path $Game 'Interface\AddOns\Journey');source='Interface\AddOns\Journey';kind='directory';action='replace'},
  [ordered]@{target=(Join-Path $Game 'Interface\Content\Journey-Content');source='Interface\Content\Journey-Content';kind='directory';action='replace'},
  [ordered]@{target=(Join-Path $AppRoot 'RelaxCoach.exe');source='RelaxCoach.exe';kind='file';action='replace'},
  [ordered]@{target=(Join-Path $DesktopRoot 'Relax Coach.lnk');source='';kind='file';action='shortcut'},
  [ordered]@{target=(Join-Path $DesktopRoot 'Forever Coach.lnk');source='';kind='file';action='remove'}
)
if($planner) {
  $plannerSlot=[ordered]@{target=(Join-Path $Game 'Interface\AddOns\RelaxQuestPlanner');source='Interface\AddOns\RelaxQuestPlanner';kind='directory';action='replace'}
  $slots=@($slots[0..2]) + @($plannerSlot) + @($slots[3..($slots.Count-1)])
}
foreach($s in $slots) {
  foreach($protected in @($SourceRoot,$BackupRoot)) {
    if($protected -eq $s.target -or $protected.StartsWith($s.target+'\',[StringComparison]::OrdinalIgnoreCase) -or $s.target.StartsWith($protected+'\',[StringComparison]::OrdinalIgnoreCase)) { throw 'Backup/source overlaps an installation target.' }
  }
}
# Only stop our existing report helpers, never WoW or unrelated processes.
$knownExecutables=@((Join-Path $AppRoot 'RelaxCoach.exe'))
if($AppRoot -eq (Join-Path $env:USERPROFILE 'RelaxCoach')) { $knownExecutables += (Join-Path $env:USERPROFILE 'ForeverCoach\ForeverCoach.exe') }
foreach($p in Get-CimInstance Win32_Process | Where-Object {$_.Name -in @('RelaxCoach.exe','ForeverCoach.exe')}) {
  if($p.ExecutablePath -in $knownExecutables) { Stop-Process -Id $p.ProcessId -Force }
}
$backup=Join-Path $BackupRoot ((Get-Date -Format 'yyyyMMdd-HHmmss')+'-'+[guid]::NewGuid().ToString('N').Substring(0,8))
New-Item -ItemType Directory -Path $backup -Force | Out-Null
$i=0
foreach($s in $slots) {
  $s.backup=Join-Path $backup ('original-'+$i)
  $s.existed=Test-Path -LiteralPath $s.target
  if($s.existed) { Copy-Item -LiteralPath $s.target -Destination $s.backup -Recurse }
  $i++
}
Copy-Item -LiteralPath (Join-Path $SourceRoot 'Restore-RelaxCoach.ps1') -Destination (Join-Path $backup 'Restore-RelaxCoach.ps1')
$state=[ordered]@{version=([string]$manifest.version+'-guild');game=$Game;source=$SourceRoot;created=(Get-Date -Format o);slots=$slots;status='backed-up'}
$statePath=Join-Path $backup 'installation.json'
[IO.File]::WriteAllText($statePath,($state | ConvertTo-Json -Depth 6),(New-Object Text.UTF8Encoding($false)))
$changed=$false
try {
  Assert-GameStopped
  foreach($s in $slots) {
    $changed=$true
    $parent=Split-Path -Parent $s.target
    New-Item -ItemType Directory -Path $parent -Force | Out-Null
    if($s.action -eq 'replace') {
      $stage=$s.target+'.relax-stage-'+[guid]::NewGuid().ToString('N')
      try {
        Copy-Item -LiteralPath (Join-Path $SourceRoot $s.source) -Destination $stage -Recurse
        if(Test-Path -LiteralPath $s.target) { Remove-Item -LiteralPath $s.target -Recurse -Force }
        Move-Item -LiteralPath $stage -Destination $s.target
      } finally { if(Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force } }
    } elseif($s.action -eq 'shortcut') {
      $shell=New-Object -ComObject WScript.Shell
      $link=$shell.CreateShortcut($s.target)
      $link.TargetPath=Join-Path $AppRoot 'RelaxCoach.exe'
      $link.WorkingDirectory=$AppRoot
      # Let the app use its remembered folder; a fixed -game argument overrides it.
      $link.Arguments=''
      $link.Description='Relax Coach - leveling report'
      $link.IconLocation=(Join-Path $AppRoot 'RelaxCoach.exe')+',0'
      $link.Save()
    } elseif(Test-Path -LiteralPath $s.target) { Remove-Item -LiteralPath $s.target -Force }
  }
  $verified=0
  foreach($row in $manifest.files) {
    $rel=[string]$row.path
    if($rel.StartsWith('Interface/')) { $target=Join-Path $Game $rel }
    elseif($rel -eq 'RelaxCoach.exe') { $target=Join-Path $AppRoot 'RelaxCoach.exe' }
    else { continue }
    if((Get-Sha $target) -ne $row.sha256) { throw "Installed checksum mismatch: $rel" }
    $verified++
  }
  $link=(New-Object -ComObject WScript.Shell).CreateShortcut((Join-Path $DesktopRoot 'Relax Coach.lnk'))
  if($link.TargetPath -ne (Join-Path $AppRoot 'RelaxCoach.exe') -or !(Test-Path -LiteralPath $link.TargetPath)) { throw 'Shortcut readback failed.' }
  $state.status='installed'
  $state.verifiedFiles=$verified
  [IO.File]::WriteAllText($statePath,($state | ConvertTo-Json -Depth 6),(New-Object Text.UTF8Encoding($false)))
  [ordered]@{status='installed';version=([string]$manifest.version+'-guild');verifiedFiles=$verified;backup=$backup;exe=(Join-Path $AppRoot 'RelaxCoach.exe');shortcut=(Join-Path $DesktopRoot 'Relax Coach.lnk');gameRunning=[bool](Get-Process Wow -ErrorAction SilentlyContinue);savedVariables='untouched';restartRequired=$true} | ConvertTo-Json
} catch {
  $failure=$_
  if($changed) { & (Join-Path $backup 'Restore-RelaxCoach.ps1') -Backup $backup }
  throw $failure
}
