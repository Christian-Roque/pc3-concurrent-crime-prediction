$ErrorActionPreference = "Stop"
$ProjectRoot = (Get-Location).ProviderPath
$Exe = Join-Path $ProjectRoot "evidencias\performance\prepare\bin\prepare_bench.exe"
if (!(Test-Path $Exe)) {
  Write-Host "No existe prepare_bench.exe. Compilando..."
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Exe) | Out-Null
  go build -o $Exe ./cmd/prepare
}
Write-Host "Probando prepare directo con workers=1 y output_debug..."
& $Exe -file (Join-Path $ProjectRoot "data\dataset.csv") -workers 1 -batch-size 20000 -min-year 2022 -train-until 2025 -negative-ratio 0 -target-before "2026-06-01" -out (Join-Path $ProjectRoot "output_debug")
Write-Host "ExitCode directo: $LASTEXITCODE"
