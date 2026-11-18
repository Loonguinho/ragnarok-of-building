# Builds the desktop app: one .exe with the UI embedded.
#   powershell -ExecutionPolicy Bypass -File scripts\build-desktop.ps1
# Output: dist\RagnarokOfBuilding.exe
$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)
go build -trimpath -ldflags "-H=windowsgui -s -w" -o dist\RagnarokOfBuilding.exe .\cmd\desktop
Write-Host "dist\RagnarokOfBuilding.exe"
