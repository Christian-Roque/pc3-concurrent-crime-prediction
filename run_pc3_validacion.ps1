$ErrorActionPreference = "Stop"

Write-Host "=== Validacion PC3 sin reprocesar todo ==="

Write-Host "`n[1/2] Validando compilacion..."
go test ./...

Write-Host "`n[2/2] Verificando archivos generados..."
Get-Item output\ml_observations_train.csv.gz,
         output\ml_observations_test.csv.gz,
         output\ml_observations_metadata.json,
         output\prepare_summary.json,
         output\loader_stats.json,
         output\train_summary.json,
         models\model.json |
Select-Object Name, Length, LastWriteTime

Write-Host "`nValidacion terminada. No se reproceso la data."