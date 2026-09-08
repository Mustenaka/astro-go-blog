# Development release script: only builds the Astro site.
# Production (phase 7) replaces this with git pull + build + atomic swap of the Nginx root.
$ErrorActionPreference = "Stop"
$repo = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Write-Host "[release.dev] repo=$repo"
Set-Location $repo
pnpm -C site build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "[release.dev] done"
