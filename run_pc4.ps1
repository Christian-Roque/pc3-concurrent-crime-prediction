$ErrorActionPreference = "Stop"

Write-Host "=== 1. Validando proyecto ==="
go test ./...
go vet ./...

Write-Host "=== 2. Benchmark carga/limpieza/preparacion ==="
& .\scripts\pc3_benchmark_prepare.ps1 `
  -Dataset "data\dataset.csv" `
  -WorkersList @(1,2,4,8) `
  -BatchSize 20000 `
  -MaxRows 0 `
  -MirrorOutputDir "output"

Write-Host "=== 3. Benchmark entrenamiento ==="
& .\scripts\pc3_benchmark_train.ps1 `
  -WorkersList @(1,2,4,8) `
  -Epochs 80

Write-Host "=== 4. Levantando arquitectura distribuida PC4 ==="
docker compose up --build -d

Write-Host "Esperando 20 segundos para que API, nodos, Mongo y Redis inicien..."
Start-Sleep -Seconds 20

Write-Host "=== 5. Ejecutando smoke test PC4 ==="
& .\scripts\pc4_smoke_test.ps1

Write-Host "=== 6. Capturando recursos Docker ==="
& .\scripts\pc4_docker_stats_capture.ps1 `
  -DurationSeconds 90 `
  -IntervalSeconds 3

Write-Host "=== 7. Guardando logs Docker ==="
New-Item -ItemType Directory -Force -Path evidencias\performance\docker | Out-Null
docker compose ps > evidencias\performance\docker\docker_ps.txt
docker compose logs --no-color > evidencias\performance\docker\docker_compose_logs.txt

Write-Host "=== 8. Generando reporte visual automatico ==="
python .\scripts\generate_performance_report.py --evidence-dir evidencias\performance

Write-Host "=== PROCESO TERMINADO ==="
Write-Host "Revisa la carpeta: evidencias\performance"
Write-Host "Abre el reporte: evidencias\performance\performance_report.html"
