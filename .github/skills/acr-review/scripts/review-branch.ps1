param(
    [Parameter(Mandatory=$true)]
    [string]$Branch,
    [string]$BaseRef = "main",
    [int]$Reviewers = 5,
    [string]$Agent = "copilot"
)

$ErrorActionPreference = "Stop"

$acr = "$env:USERPROFILE\.acr\acr.exe"

if (-not (Test-Path $acr)) {
    Write-Error @"
ACR not found at $acr

Run the setup script first to build ACR:
.\.github\skills\acr-review\scripts\setup-acr.ps1
"@
    exit 1
}

& $acr --branch $Branch --base $BaseRef --reviewers $Reviewers --agent $Agent
