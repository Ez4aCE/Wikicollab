# WikiCollab — Start Cloudflare Tunnels for sharing
# Run this from d:\minor project whenever you want to share with friends.
# Keep this window open while friends are using the app.

Set-Location "d:\minor project"

# Make sure Docker containers are running
docker compose up -d
Start-Sleep -Seconds 5

# Start backend tunnel and capture the public URL
Write-Host "Starting backend tunnel..." -ForegroundColor Cyan
$backendJob = Start-Job -ScriptBlock {
    Set-Location "d:\minor project"
    .\cloudflared.exe tunnel --url http://localhost:8080 2>&1
}

# Wait for backend URL to appear in output (up to 30s)
$backendUrl = $null
$timeout = 60
$elapsed = 0
while (-not $backendUrl -and $elapsed -lt $timeout) {
    Start-Sleep -Seconds 2
    $elapsed += 2
    $output = Receive-Job $backendJob -Keep | Out-String
    if ($output -match 'https://[a-z0-9\-]+\.trycloudflare\.com') {
        $backendUrl = $Matches[0]
    }
}

if (-not $backendUrl) {
    Write-Host "ERROR: Could not get backend tunnel URL. Is cloudflared.exe in d:\minor project?" -ForegroundColor Red
    exit 1
}

Write-Host "Backend tunnel: $backendUrl" -ForegroundColor Green

# Rebuild frontend with the backend URL baked in
Write-Host "Recreating frontend with backend URL..." -ForegroundColor Cyan
$env:VITE_API_URL = $backendUrl
$env:CORS_ORIGIN = "*"
docker compose -f "d:\minor project\docker-compose.yml" up -d frontend backend | Out-Null
Start-Sleep -Seconds 10
Set-Location "d:\minor project"

# Start frontend tunnel
Write-Host "Starting frontend tunnel..." -ForegroundColor Cyan
$frontendJob = Start-Job -ScriptBlock {
    Set-Location "d:\minor project"
    .\cloudflared.exe tunnel --url http://localhost:5173 2>&1
}

$frontendUrl = $null
$elapsed = 0
while (-not $frontendUrl -and $elapsed -lt $timeout) {
    Start-Sleep -Seconds 2
    $elapsed += 2
    $output = Receive-Job $frontendJob -Keep | Out-String
    if ($output -match 'https://[a-z0-9\-]+\.trycloudflare\.com') {
        $frontendUrl = $Matches[0]
    }
}

if (-not $frontendUrl) {
    Write-Host "ERROR: Could not get frontend tunnel URL." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "======================================================" -ForegroundColor Yellow
Write-Host "  SHARE THIS URL WITH YOUR FRIENDS:" -ForegroundColor Yellow
Write-Host "  $frontendUrl" -ForegroundColor Green
Write-Host "======================================================" -ForegroundColor Yellow
Write-Host ""
Write-Host "Keep this window open. Press Ctrl+C to stop sharing." -ForegroundColor Gray

# Keep script alive so jobs keep running
try {
    while ($true) { Start-Sleep -Seconds 30 }
} finally {
    Stop-Job $backendJob, $frontendJob
    Remove-Job $backendJob, $frontendJob
    Write-Host "Tunnels stopped." -ForegroundColor Red
}
