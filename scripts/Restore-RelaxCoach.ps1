[CmdletBinding()]
param([string]$Backup=$PSScriptRoot)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
# Installation records optional RelaxQuestPlanner and RelaxRaid slots too.
# Restore every recorded slot: replace a prior copy, or remove a fresh install; never touch WTF.
$state=Get-Content -LiteralPath (Join-Path $Backup 'installation.json') -Raw | ConvertFrom-Json
foreach($slot in $state.slots) { if($slot.existed -and !(Test-Path -LiteralPath $slot.backup)) { throw "Missing backup; restoration refused: $($slot.backup)" } }
$exe=($state.slots | Where-Object {$_.source -eq 'RelaxCoach.exe'}).target
foreach($p in Get-CimInstance Win32_Process | Where-Object {$_.Name -eq 'RelaxCoach.exe' -and $_.ExecutablePath -eq $exe}) { Stop-Process -Id $p.ProcessId -Force }
foreach($slot in $state.slots) {
  if(Test-Path -LiteralPath $slot.target) { Remove-Item -LiteralPath $slot.target -Recurse -Force }
  if($slot.existed) {
    New-Item -ItemType Directory -Path (Split-Path -Parent $slot.target) -Force | Out-Null
    Copy-Item -LiteralPath $slot.backup -Destination $slot.target -Recurse
  }
}
foreach($slot in $state.slots) {
  if(!$slot.existed) { if(Test-Path -LiteralPath $slot.target) { throw "New target survived rollback: $($slot.target)" }; continue }
  if($slot.kind -eq 'file') { $sourceFiles=@(Get-Item -LiteralPath $slot.backup) }
  else { $sourceFiles=@(Get-ChildItem -LiteralPath $slot.backup -File -Recurse) }
  foreach($f in $sourceFiles) {
    if($slot.kind -eq 'file') { $target=$slot.target } else { $target=Join-Path $slot.target $f.FullName.Substring($slot.backup.Length+1) }
    if((Get-FileHash -LiteralPath $f.FullName).Hash -ne (Get-FileHash -LiteralPath $target).Hash) { throw "Rollback verification failed: $target" }
  }
}
$state.status='restored'
[IO.File]::WriteAllText((Join-Path $Backup 'installation.json'),($state | ConvertTo-Json -Depth 6),(New-Object Text.UTF8Encoding($false)))
[ordered]@{status='restored';backup=$Backup;slots=@($state.slots).Count;savedVariables='untouched';restartRequired=$true}|ConvertTo-Json
