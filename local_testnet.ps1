# ----------------------------------------------------------------------
# Local Single-Validator Testnet Launch Script (PowerShell for Windows)
# Chain ID: sovereign-1
# Moniker:  my-test-chain
# Founder:  10,000,000,000 unative
# Feeshare: 2000 bps (20%) -> Founder Treasury
# ----------------------------------------------------------------------

$ErrorActionPreference = "Stop"

$ChainId = "sovereign-1"
$Moniker = "my-test-chain"
$KeyringBackend = "test"
$FounderKey = "founder"
$TenantKey = "tenant"
$StakeDenom = "unative"
$InitialBalance = "10000000000unative"
$StakeAmount = "1000000000unative"
$FounderBps = 2000

# Locate repo directory
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
if (Test-Path (Join-Path $ScriptDir "cosmos-sdk-main")) {
    $SrcDir = Join-Path $ScriptDir "cosmos-sdk-main"
} else {
    $SrcDir = $ScriptDir
}

Set-Location $SrcDir
$NodeHome = Join-Path $env:USERPROFILE ".simapp"
$BuildDir = Join-Path $SrcDir "build"
$SimdBin = Join-Path $BuildDir "simd.exe"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " Starting Local Testnet Automated Build & Launch (Windows)" -ForegroundColor Cyan
Write-Host " Chain ID:  $ChainId" -ForegroundColor Yellow
Write-Host " Moniker:   $Moniker" -ForegroundColor Yellow
Write-Host " Node Home: $NodeHome" -ForegroundColor Yellow
Write-Host "==========================================================" -ForegroundColor Cyan

# 0. Check Go installation
$goCheck = Get-Command "go" -ErrorAction SilentlyContinue
if (-not $goCheck) {
    Write-Host "`n[ERROR] Go compiler is not found in your system PATH!" -ForegroundColor Red
    Write-Host "Please install Go 1.22+ from https://go.dev/dl/ (Windows 64-bit MSI installer)." -ForegroundColor Yellow
    Write-Host "After installing, restart your VS Code terminal and re-run this script.`n" -ForegroundColor Yellow
    exit 1
}

# 1. Clean any old test data
Write-Host "`n[1/7] Cleaning old test data ($NodeHome)..." -ForegroundColor Green
if (Test-Path $NodeHome) {
    Remove-Item -Recurse -Force $NodeHome
}

# 2. Compile project binary
Write-Host "[2/7] Compiling project binary (go build -o build/simd.exe ./cmd/simd)..." -ForegroundColor Green
if (-not (Test-Path $BuildDir)) {
    New-Item -ItemType Directory -Path $BuildDir | Out-Null
}

go build -o $SimdBin ./cmd/simd
if (-not (Test-Path $SimdBin)) {
    Write-Host "[ERROR] Binary compilation failed; $SimdBin not found!" -ForegroundColor Red
    exit 1
}
Write-Host "Compiled binary successfully: $SimdBin" -ForegroundColor Green

# 3. Initialize genesis state
Write-Host "[3/7] Initializing genesis state..." -ForegroundColor Green
& $SimdBin init $Moniker --chain-id $ChainId --home $NodeHome

$GenesisFile = Join-Path $NodeHome "config\genesis.json"
$AppToml = Join-Path $NodeHome "config\app.toml"
$ConfigToml = Join-Path $NodeHome "config\config.toml"

# Replace 'stake' with 'unative' in genesis
(Get-Content $GenesisFile -Raw) -replace '"stake"', "`"$StakeDenom`"" | Set-Content $GenesisFile

# 4. Generate local keys
Write-Host "[4/7] Generating local keys ('$FounderKey' and '$TenantKey')..." -ForegroundColor Green
& $SimdBin keys add $FounderKey --keyring-backend $KeyringBackend --home $NodeHome
& $SimdBin keys add $TenantKey --keyring-backend $KeyringBackend --home $NodeHome

$FounderAddr = (& $SimdBin keys show $FounderKey -a --keyring-backend $KeyringBackend --home $NodeHome).Trim()
$TenantAddr = (& $SimdBin keys show $TenantKey -a --keyring-backend $KeyringBackend --home $NodeHome).Trim()

Write-Host "Founder Address: $FounderAddr" -ForegroundColor Cyan
Write-Host "Tenant Address:  $TenantAddr" -ForegroundColor Cyan

# 5. Inject genesis accounts
Write-Host "[5/7] Injecting genesis accounts..." -ForegroundColor Green
& $SimdBin genesis add-genesis-account $FounderAddr $InitialBalance --keyring-backend $KeyringBackend --home $NodeHome
& $SimdBin genesis add-genesis-account $TenantAddr "100000000unative" --keyring-backend $KeyringBackend --home $NodeHome

# Generate gentx
Write-Host "Generating gentx for founder validator ($StakeAmount)..." -ForegroundColor Green
& $SimdBin genesis gentx $FounderKey $StakeAmount --chain-id $ChainId --keyring-backend $KeyringBackend --home $NodeHome

Write-Host "Collecting genesis transactions..." -ForegroundColor Green
& $SimdBin genesis collect-gentxs --home $NodeHome

# 6. Configure feeshare genesis params
Write-Host "[6/7] Configuring feeshare genesis parameters (20% founder split)..." -ForegroundColor Green
$genesisJson = Get-Content $GenesisFile -Raw | ConvertFrom-Json
if (-not $genesisJson.app_state.feeshare) {
    $genesisJson.app_state | Add-Member -NotePropertyName "feeshare" -NotePropertyValue @{}
}
$genesisJson.app_state.feeshare = @{
    params = @{
        founder_bps = [int64]$FounderBps
        treasury_address = "$FounderAddr"
    }
}
$genesisJson | ConvertTo-Json -Depth 100 | Set-Content $GenesisFile

Write-Host "Validating genesis state..." -ForegroundColor Green
& $SimdBin genesis validate-genesis --home $NodeHome

# Configure app.toml and config.toml
if (Test-Path $AppToml) {
    (Get-Content $AppToml -Raw) -replace 'enable = false', 'enable = true' `
                               -replace 'swagger = false', 'swagger = true' `
                               -replace 'minimum-gas-prices = ""', "minimum-gas-prices = `"0$StakeDenom`"" | Set-Content $AppToml
}
if (Test-Path $ConfigToml) {
    (Get-Content $ConfigToml -Raw) -replace 'timeout_commit = "5s"', 'timeout_commit = "1s"' | Set-Content $ConfigToml
}

# 7. Start the node
Write-Host "[7/7] Launching network validator ($SimdBin start)..." -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " Chain ID:          $ChainId" -ForegroundColor Yellow
Write-Host " Founder Address:   $FounderAddr" -ForegroundColor Yellow
Write-Host " Tenant Address:    $TenantAddr" -ForegroundColor Yellow
Write-Host " Treasury Routing:  20% gas share -> $FounderAddr" -ForegroundColor Yellow
Write-Host " RPC Server:        http://localhost:26657" -ForegroundColor Yellow
Write-Host " REST/Swagger:      http://localhost:1317" -ForegroundColor Yellow
Write-Host "==========================================================" -ForegroundColor Cyan

& $SimdBin start --home $NodeHome
