<#
.SYNOPSIS
    Aether Network - 5-Minute Local Node Quickstart Script (Windows PowerShell)
.DESCRIPTION
    Compiles the native simd binary, initializes genesis state with ucurts base denom,
    creates validator keys, and boots local CometBFT block mining.
#>

$ErrorActionPreference = "Stop"

$ChainId = "aether-1"
$Moniker = "local-validator"
$Denom = "ucurts"
$BuildDir = ".\build"
$Binary = "$BuildDir\simd.exe"
$SimappHome = "$env:USERPROFILE\.simapp"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  AETHER NETWORK: 5-Minute Local Testnet Initialization" -ForegroundColor Cyan
Write-Host "  Chain ID:    $ChainId" -ForegroundColor Cyan
Write-Host "  Base Denom:  $Denom (Curtis atomic unit, 6 decimals)" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Compile Binary
Write-Host "[1/5] Compiling native 'simd' blockchain binary..." -ForegroundColor Green
if (-not (Test-Path $BuildDir)) { New-Item -ItemType Directory -Path $BuildDir | Out-Null }
go build -o $Binary .\cmd\simd
Write-Host "      Binary compiled successfully at $Binary" -ForegroundColor Gray

# 2. Reset Home Directory
Write-Host "[2/5] Initializing chain configuration at $SimappHome..." -ForegroundColor Green
if (Test-Path $SimappHome) { Remove-Item -Recurse -Force $SimappHome }
& $Binary init $Moniker --chain-id $ChainId --default-denom $Denom

# 3. Create Validator Key
Write-Host "[3/5] Generating local validator test key..." -ForegroundColor Green
& $Binary keys add validator --keyring-backend test

# 4. Allocate Genesis Balances & Generate GenTx
Write-Host "[4/5] Injecting genesis balances and forging genesis validator transaction..." -ForegroundColor Green
& $Binary genesis add-genesis-account validator "1000000000000$Denom" --keyring-backend test
& $Binary genesis gentx validator "100000000000$Denom" --chain-id $ChainId --keyring-backend test
& $Binary genesis collect-gentxs

# 5. Configure REST API and CORS
$AppToml = "$SimappHome\config\app.toml"
if (Test-Path $AppToml) {
    (Get-Content $AppToml) `
        -replace 'enable = false', 'enable = true' `
        -replace 'enabled-unsafe-cors = false', 'enabled-unsafe-cors = true' |
        Set-Content $AppToml
}
$ConfigToml = "$SimappHome\config\config.toml"
if (Test-Path $ConfigToml) {
    (Get-Content $ConfigToml) `
        -replace 'cors_allowed_origins = \[\]', 'cors_allowed_origins = ["*"]' |
        Set-Content $ConfigToml
}

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  AETHER NODE READY TO START!" -ForegroundColor Green
Write-Host "  CometBFT RPC:   http://localhost:26657" -ForegroundColor Yellow
Write-Host "  Cosmos REST:    http://localhost:1317" -ForegroundColor Yellow
Write-Host "  Starting block mining now... (Press Ctrl+C to stop)" -ForegroundColor Gray
Write-Host "==========================================================" -ForegroundColor Cyan

& $Binary start
