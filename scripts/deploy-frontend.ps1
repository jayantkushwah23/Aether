<#
.SYNOPSIS
    Aether Network - Frontend Portal Automated Deployment Pipeline (Windows PowerShell)
.DESCRIPTION
    Packages and deploys the contents of the /frontend directory directly onto the
    live AWS EC2 production web server at 34.207.176.57.
#>

param (
    [string]$Ec2Host = "34.207.176.57",
    [string]$Ec2User = "ubuntu",
    [string]$KeyPath = "..\aether-key.pem"
)

$ErrorActionPreference = "Stop"

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "  Aether Portal Automated Cloud Deployment Pipeline" -ForegroundColor Cyan
Write-Host "  Target Host: $Ec2Host" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

# 1. Resolve Key Path
$ResolvedKey = $KeyPath
if (-not (Test-Path $ResolvedKey)) {
    if (Test-Path ".\aether-key.pem") { $ResolvedKey = ".\aether-key.pem" }
    elseif (Test-Path "..\..\aether-key.pem") { $ResolvedKey = "..\..\aether-key.pem" }
    elseif (Test-Path "C:\Users\zpkgn\Downloads\cosmos-sdk-main\aether-key.pem") { $ResolvedKey = "C:\Users\zpkgn\Downloads\cosmos-sdk-main\aether-key.pem" }
    else {
        Write-Error "Could not locate aether-key.pem. Please specify with -KeyPath."
        exit 1
    }
}
Write-Host "[1/4] Using private key: $ResolvedKey" -ForegroundColor Green

# 2. Resolve Frontend Path
$FrontendPath = ".\frontend"
if (-not (Test-Path "$FrontendPath\index.html")) {
    if (Test-Path "..\frontend\index.html") { $FrontendPath = "..\frontend" }
    elseif (Test-Path "C:\Users\zpkgn\Downloads\cosmos-sdk-main\frontend\index.html") { $FrontendPath = "C:\Users\zpkgn\Downloads\cosmos-sdk-main\frontend" }
    else {
        Write-Error "Could not locate /frontend directory."
        exit 1
    }
}

Write-Host "[2/4] Validating source assets in $FrontendPath..." -ForegroundColor Green
$Files = Get-ChildItem -Path $FrontendPath -File
foreach ($file in $Files) {
    Write-Host "      Found: $($file.Name) ($($file.Length) bytes)" -ForegroundColor Gray
}

# 3. SCP Transfer
Write-Host "[3/4] Uploading frontend files to production web server (${Ec2Host}:/var/www/html)..." -ForegroundColor Green
scp.exe -i $ResolvedKey -o StrictHostKeyChecking=no -r "$FrontendPath\*" "${Ec2User}@${Ec2Host}:/var/www/html/"

# 4. Reload Nginx & Set Permissions
Write-Host "      Updating permissions and reloading Nginx..." -ForegroundColor Gray
ssh.exe -i $ResolvedKey -o StrictHostKeyChecking=no "${Ec2User}@${Ec2Host}" "sudo chown -R www-data:www-data /var/www/html && sudo systemctl reload nginx"

# 5. Verify HTTP Response
Write-Host "[4/4] Verifying live public production URL..." -ForegroundColor Green
try {
    $resp = Invoke-WebRequest -Uri "http://$Ec2Host/" -Method Head -TimeoutSec 10 -UseBasicParsing
    Write-Host "==================================================" -ForegroundColor Green
    Write-Host "  DEPLOYMENT COMPLETE & LIVE!" -ForegroundColor Green
    Write-Host "  Public Web Portal:  http://$Ec2Host/" -ForegroundColor Yellow
    Write-Host "  RPC API:            http://${Ec2Host}:26657" -ForegroundColor Yellow
    Write-Host "  REST API:           http://${Ec2Host}:1317" -ForegroundColor Yellow
    Write-Host "  HTTP Status:        $($resp.StatusCode)" -ForegroundColor Green
    Write-Host "==================================================" -ForegroundColor Green
} catch {
    Write-Host "Warning: Public probe returned error: $_" -ForegroundColor Yellow
    Write-Host "Check http://$Ec2Host/ directly in your browser." -ForegroundColor Gray
}
