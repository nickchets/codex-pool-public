$ErrorActionPreference = "Stop"

$AppDir = if ($env:CODEX_POOL_APP_DIR) { $env:CODEX_POOL_APP_DIR } else { Join-Path $env:LOCALAPPDATA "codex-pool" }
$Bin = Join-Path $AppDir "codex-pool.exe"
$Config = Join-Path $AppDir "config.toml"

New-Item -ItemType Directory -Force -Path $AppDir | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $AppDir "pool\codex") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $AppDir "pool\openai_api") | Out-Null

go build -trimpath -o $Bin .

if (-not (Test-Path $Config)) {
  Copy-Item "config.toml.example" $Config
}

$Action = New-ScheduledTaskAction -Execute $Bin -WorkingDirectory $AppDir
$Trigger = New-ScheduledTaskTrigger -AtLogOn
$Settings = New-ScheduledTaskSettingsSet -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName "codex-pool" -Action $Action -Trigger $Trigger -Settings $Settings -Description "Codex CLI OpenAI account pool" -Force | Out-Null
Start-ScheduledTask -TaskName "codex-pool"
Write-Host "codex-pool installed. Status: Get-ScheduledTask -TaskName codex-pool"
