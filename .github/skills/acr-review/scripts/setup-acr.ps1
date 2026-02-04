# Setup ACR - one-time build from source
$ErrorActionPreference = "Stop"

$acrPath = "$env:USERPROFILE\.acr"
$acrExe = "$acrPath\acr.exe"

# Skip if already built
if (Test-Path $acrExe) {
    Write-Host "ACR already installed at $acrExe"
    exit 0
}

# Check if Go is available
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error @"
Go is not installed. ACR cannot be built.

To install Go:
1. Download from https://go.dev/dl/
2. Install and ensure 'go' is in your PATH
3. Run this script again
"@
    exit 1
}

# Clone and build
Write-Host "Cloning ACR repository..."
git clone https://github.com/richhaase/agentic-code-reviewer.git $acrPath

Write-Host "Building ACR..."
Push-Location $acrPath
go build -o acr.exe ./cmd/acr
Pop-Location

if (Test-Path $acrExe) {
    Write-Host "ACR built successfully at $acrExe"
} else {
    Write-Error "Build failed - acr.exe not created"
    exit 1
}
