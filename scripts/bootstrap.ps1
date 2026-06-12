$ErrorActionPreference = "Stop"

Set-Location (Join-Path $PSScriptRoot "..")

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
  throw "docker is required"
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  throw "go is required"
}

docker compose version | Out-Null

if (-not (Test-Path ".env")) {
  Copy-Item ".env.example" ".env"
  Write-Host "created .env from .env.example"
}

docker compose up -d

Write-Host "waiting for postgres..."
for ($i = 0; $i -lt 60; $i++) {
  docker compose exec -T postgres pg_isready -U ai_interview -d ai_interview | Out-Null
  if ($LASTEXITCODE -eq 0) { break }
  Start-Sleep -Seconds 1
}

Get-ChildItem "migrations/*.sql" | Sort-Object Name | ForEach-Object {
  Write-Host "applying $($_.FullName)"
  Get-Content $_.FullName | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U ai_interview -d ai_interview
}

New-Item -ItemType Directory -Force ".cache/go-build" | Out-Null
$env:GOCACHE = (Resolve-Path ".cache/go-build").Path
go test ./...

Write-Host "bootstrap completed"
Write-Host "run: go run ./cmd/api"
