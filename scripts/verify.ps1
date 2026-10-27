$ErrorActionPreference = "Stop"

$ProjectDir = Split-Path -Parent $PSScriptRoot
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host " Cyberspace Refactor - Automated Verification Suite" -ForegroundColor Cyan
Write-Host " Project Directory: $ProjectDir" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan

Write-Host "`n[1/5] Running static analysis (go vet ./...)..." -ForegroundColor Yellow
& go -C $ProjectDir vet ./...
if ($LASTEXITCODE -ne 0) { throw "go vet failed" }

Write-Host "`n[2/5] Running all unit, corpus, and OWASP/NIST security tests..." -ForegroundColor Yellow
& go -C $ProjectDir test -v ./... -count=1
if ($LASTEXITCODE -ne 0) { throw "unit/security tests failed" }

Write-Host "`n[3/5] Running short fuzzing pass on legacy parsers (LTG, DEK, NPC)..." -ForegroundColor Yellow
& go -C $ProjectDir test ./internal/format -run=^$ -fuzz=^FuzzParseLTG$ -fuzztime=2s
if ($LASTEXITCODE -ne 0) { throw "FuzzParseLTG failed" }
& go -C $ProjectDir test ./internal/format -run=^$ -fuzz=^FuzzParseDeck$ -fuzztime=2s
if ($LASTEXITCODE -ne 0) { throw "FuzzParseDeck failed" }
& go -C $ProjectDir test ./internal/format -run=^$ -fuzz=^FuzzParseNPC$ -fuzztime=2s
if ($LASTEXITCODE -ne 0) { throw "FuzzParseNPC failed" }

Write-Host "`n[4/5] Building standalone Windows executable (Cyberspace Refactor.exe)..." -ForegroundColor Yellow
$ExePath = Join-Path $ProjectDir "Cyberspace Refactor.exe"
& go -C $ProjectDir build -trimpath -ldflags "-s -w -H=windowsgui" -o $ExePath ./cmd/cyberspace
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

Write-Host "`n[5/5] Running executable headless self-check (--headless-check)..." -ForegroundColor Yellow
& $ExePath --headless-check
if ($LASTEXITCODE -ne 0) { throw "executable --headless-check failed" }

$Item = Get-Item $ExePath
Write-Host "`n============================================================" -ForegroundColor Green
Write-Host (" ALL CHECKS PASSED! Executable ready: {0} ({1:N2} MB)" -f $Item.FullName, ($Item.Length / 1MB)) -ForegroundColor Green
Write-Host "============================================================" -ForegroundColor Green
