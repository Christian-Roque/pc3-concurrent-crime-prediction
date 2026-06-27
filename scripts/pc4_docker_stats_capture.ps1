param(
    [int]$DurationSeconds = 90,
    [int]$IntervalSeconds = 3,
    [string]$EvidenceDir = "evidencias/performance/docker"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = (Get-Location).ProviderPath
if (!([System.IO.Path]::IsPathRooted($EvidenceDir))) {
    $EvidenceDir = Join-Path $ProjectRoot $EvidenceDir
}
New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null

$csv = Join-Path $EvidenceDir "docker_stats_samples.csv"
"timestamp,name,cpu_percent,mem_usage,net_io,block_io,pids" | Out-File -FilePath $csv -Encoding utf8

Write-Host "Capturando docker stats durante $DurationSeconds segundos cada $IntervalSeconds segundos..."
Write-Host "Mantén docker compose up ejecutándose y, si deseas, lanza scripts/pc4_smoke_test.ps1 en otra terminal."

$end = (Get-Date).AddSeconds($DurationSeconds)
while ((Get-Date) -lt $end) {
    $ts = (Get-Date).ToString("s")
    $lines = docker stats --no-stream --format "{{.Name}},{{.CPUPerc}},{{.MemUsage}},{{.NetIO}},{{.BlockIO}},{{.PIDs}}"
    foreach ($line in $lines) {
        "$ts,$line" | Out-File -FilePath $csv -Encoding utf8 -Append
    }
    Start-Sleep -Seconds $IntervalSeconds
}

Write-Host "Evidencia generada en: $csv"
Write-Host "Para captura visual del informe, abre otra terminal y ejecuta: docker stats"
