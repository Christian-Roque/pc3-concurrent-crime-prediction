$ErrorActionPreference = "Stop"

Write-Host "=== PC3 Seguridad Ciudadana - Ejecucion completa ==="

Write-Host "`n[0/3] Creando carpetas necesarias..."
New-Item -ItemType Directory -Force -Path output | Out-Null
New-Item -ItemType Directory -Force -Path models | Out-Null

Write-Host "`n[1/3] Validando compilacion y pruebas..."
go test ./...

Write-Host "`n[2/3] Cargando, limpiando y preparando observaciones ML con concurrencia..."
go run ./cmd/prepare `
  -file data/dataset.csv `
  -workers 8 `
  -batch-size 20000 `
  -min-year 2022 `
  -train-until 2025 `
  -negative-ratio 0 `
  -target-before 2026-06-01 `
  -out output

Write-Host "`n[3/3] Entrenando modelo ML con calculo paralelo de gradientes..."
go run ./cmd/train `
  -train output/ml_observations_train.csv.gz `
  -test output/ml_observations_test.csv.gz `
  -metadata output/ml_observations_metadata.json `
  -workers 8 `
  -epochs 80 `
  -lambda 0.0005 `
  -out output `
  -model models/model.json

Write-Host "`n=== EJECUCION COMPLETA FINALIZADA ==="
Write-Host "Revisa output/prepare_summary.json, output/loader_stats.json, output/train_summary.json, output/risk_predictions_sample.csv y models/model.json"
