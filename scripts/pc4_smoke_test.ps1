# Pruebas rapidas PC4 para generar evidencias.
# Requisito: ejecutar antes `docker compose up --build` o tener la API disponible en API_URL.
# Nota: se usa Invoke-WebRequest en lugar de curl.exe para evitar problemas de comillas
# al enviar JSON desde Windows PowerShell.

param(
    [string]$ApiUrl = $(if ($env:API_URL) { $env:API_URL } else { "http://localhost:8080" }),
    [string]$OutDir = ""
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($OutDir)) {
    $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $OutDir = "evidencias\pc4_$stamp"
}
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

Write-Host "== PC4 Smoke Test =="
Write-Host "API_URL=$ApiUrl"
Write-Host "OUT_DIR=$OutDir"

function Save-TextUtf8 {
    param([string]$Path, [string]$Content)
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    $fullPath = [System.IO.Path]::GetFullPath($Path)
    [System.IO.File]::WriteAllText($fullPath, $Content, $utf8)
}

function Invoke-GetEvidence {
    param([string]$Name, [string]$Path)
    Write-Host "GET $Path"
    try {
        $resp = Invoke-WebRequest -Uri "$ApiUrl$Path" -Method GET -UseBasicParsing
        Save-TextUtf8 "$OutDir\$Name.json" $resp.Content
    } catch {
        $content = if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $body = $reader.ReadToEnd()
            if ([string]::IsNullOrWhiteSpace($body)) {
                '{"status":"error","error":"' + ($_.Exception.Message -replace '"','\"') + '"}'
            } else {
                $body
            }
        } else {
            '{"status":"error","error":"' + ($_.Exception.Message -replace '"','\"') + '"}'
        }
        Save-TextUtf8 "$OutDir\$Name.json" $content
    }
}

function Invoke-PostEvidence {
    param([string]$Name, [string]$Path, [string]$Payload)
    Write-Host "POST $Path"
    try {
        $resp = Invoke-WebRequest -Uri "$ApiUrl$Path" -Method POST -ContentType "application/json" -Body $Payload -UseBasicParsing
        Save-TextUtf8 "$OutDir\$Name.json" $resp.Content
    } catch {
        $content = if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $body = $reader.ReadToEnd()
            if ([string]::IsNullOrWhiteSpace($body)) {
                '{"status":"error","error":"' + ($_.Exception.Message -replace '"','\"') + '"}'
            } else {
                $body
            }
        } else {
            '{"status":"error","error":"' + ($_.Exception.Message -replace '"','\"') + '"}'
        }
        Save-TextUtf8 "$OutDir\$Name.json" $content
    }
}

$predictPayload = '{"district":11,"community_area":23,"day_of_week":2,"hour":18,"week_start":"2026-06-01"}'
$topPayload = '{"top_n":3,"candidates":[{"district":11,"community_area":23,"day_of_week":2,"hour":18,"week_start":"2026-06-01"},{"district":7,"community_area":68,"day_of_week":5,"hour":21,"week_start":"2026-06-01"},{"district":12,"community_area":30,"day_of_week":6,"hour":22,"week_start":"2026-06-01"},{"district":4,"community_area":40,"day_of_week":1,"hour":10,"week_start":"2026-06-01"},{"district":18,"community_area":8,"day_of_week":4,"hour":23,"week_start":"2026-06-01"}]}'

Invoke-GetEvidence "01_health" "/health"
Invoke-GetEvidence "02_nodes" "/nodes"
Invoke-GetEvidence "03_model_info" "/model/info"
Invoke-PostEvidence "04_predict_1" "/predict" $predictPayload
Invoke-PostEvidence "05_predict_2_cache" "/predict" $predictPayload
Invoke-PostEvidence "06_recommendations_top" "/recommendations/top" $topPayload
Invoke-GetEvidence "07_predictions_history" "/predictions/history?limit=10"
Invoke-GetEvidence "08_metrics" "/metrics"

$readme = @"
Evidencias generadas automaticamente para PC4.

Orden sugerido para capturas:
1. docker compose up --build / docker ps
2. 01_health.json
3. 02_nodes.json
4. 04_predict_1.json y 05_predict_2_cache.json
5. 06_recommendations_top.json
6. 07_predictions_history.json
7. 08_metrics.json
8. logs de api y nodos ML
"@
Save-TextUtf8 "$OutDir\README_EVIDENCIAS.txt" $readme

Write-Host "Evidencias guardadas en $OutDir"
