param(
    [string]$Dataset = "data/dataset.csv",
    [object[]]$WorkersList = @(1,2,4,8),
    [int]$BatchSize = 20000,
    [int]$MinYear = 2022,
    [int]$TrainUntil = 2025,
    [int]$NegativeRatio = 0,
    [string]$TargetBefore = "2026-06-01",
    [int]$MaxRows = 0,
    [string]$EvidenceDir = "evidencias/performance/prepare",
    [string]$MirrorOutputDir = "output",
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
$ProjectRoot = (Get-Location).ProviderPath

function Convert-ToAbsolutePath($Path) {
    if ([System.IO.Path]::IsPathRooted($Path)) { return $Path }
    return (Join-Path $ProjectRoot $Path)
}

function Convert-WorkersList($Items) {
    $values = @()
    foreach ($item in @($Items)) {
        if ($null -eq $item) { continue }
        $text = [string]$item
        if ($text.Contains(',')) {
            foreach ($part in $text.Split(',')) {
                if ($part.Trim().Length -gt 0) { $values += [int]$part.Trim() }
            }
        } else {
            $values += [int]$item
        }
    }
    return $values
}

$Dataset = Convert-ToAbsolutePath $Dataset
$EvidenceDir = Convert-ToAbsolutePath $EvidenceDir
if ($MirrorOutputDir -ne "") { $MirrorOutputDir = Convert-ToAbsolutePath $MirrorOutputDir }
$WorkersList = Convert-WorkersList $WorkersList

function New-Directory($Path) {
    New-Item -ItemType Directory -Force -Path $Path | Out-Null
}

function Quote-Argument($Value) {
    $text = [string]$Value
    $text = $text.Replace('"', '\"')
    return '"' + $text + '"'
}

function Build-ArgumentString([object[]]$ArgsList) {
    return ($ArgsList | ForEach-Object { Quote-Argument $_ }) -join ' '
}

function Start-ResourceMonitor {
    param(
        [int]$ProcessId,
        [string]$OutCsv,
        [int]$IntervalSeconds = 1
    )

    Start-Job -ArgumentList $ProcessId,$OutCsv,$IntervalSeconds -ScriptBlock {
        param($TargetPid,$OutCsv,$IntervalSeconds)
        $parent = Split-Path -Parent $OutCsv
        if (!(Test-Path $parent)) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }

        # No se usa Get-Counter porque en Windows en español puede devolver NA por nombres localizados.
        # La CPU se calcula con el delta de CPU acumulado del proceso / tiempo transcurrido / núcleos lógicos.
        $logicalProcessors = [Environment]::ProcessorCount
        $prevCpu = $null
        $prevTime = $null
        "timestamp,process_cpu_percent,process_cpu_seconds,process_memory_mb" | Out-File -LiteralPath $OutCsv -Encoding utf8

        while ($true) {
            $proc = Get-Process -Id $TargetPid -ErrorAction SilentlyContinue
            if ($null -eq $proc) { break }

            $now = Get-Date
            $cpuSeconds = [double]$proc.CPU
            $cpuPercent = "NA"
            if ($null -ne $prevCpu -and $null -ne $prevTime) {
                $elapsed = ($now - $prevTime).TotalSeconds
                if ($elapsed -gt 0 -and $logicalProcessors -gt 0) {
                    $deltaCpu = $cpuSeconds - [double]$prevCpu
                    if ($deltaCpu -ge 0) {
                        $cpuPercent = [Math]::Round(($deltaCpu / ($elapsed * $logicalProcessors)) * 100.0, 2)
                    }
                }
            }

            $memMb = [Math]::Round($proc.WorkingSet64 / 1MB, 2)
            $ts = $now.ToString("s")
            "$ts,$cpuPercent,$([Math]::Round($cpuSeconds,2)),$memMb" | Out-File -LiteralPath $OutCsv -Encoding utf8 -Append

            $prevCpu = $cpuSeconds
            $prevTime = $now
            Start-Sleep -Seconds $IntervalSeconds
        }
    }
}

function Get-ResourceSummary {
    param([string]$CsvPath)
    $summary = [ordered]@{
        cpu_avg_percent = "NA"
        cpu_max_percent = "NA"
        ram_peak_mb = "NA"
        samples = 0
    }
    if (!(Test-Path $CsvPath)) { return $summary }
    $rows = Import-Csv $CsvPath
    if ($null -eq $rows -or $rows.Count -eq 0) { return $summary }
    $summary.samples = $rows.Count

    # Se prioriza process_cpu_percent. Si el CSV fuera de una versión anterior, se usa cpu_total_percent como fallback.
    $cpu = @()
    if ($rows[0].PSObject.Properties.Name -contains "process_cpu_percent") {
        $cpu = @($rows | Where-Object { $_.process_cpu_percent -ne "NA" -and $_.process_cpu_percent -ne "" } | ForEach-Object { [double]$_.process_cpu_percent })
    } elseif ($rows[0].PSObject.Properties.Name -contains "cpu_total_percent") {
        $cpu = @($rows | Where-Object { $_.cpu_total_percent -ne "NA" -and $_.cpu_total_percent -ne "" } | ForEach-Object { [double]$_.cpu_total_percent })
    }

    $ram = @($rows | Where-Object { $_.process_memory_mb -ne "NA" -and $_.process_memory_mb -ne "" } | ForEach-Object { [double]$_.process_memory_mb })
    if ($cpu.Count -gt 0) {
        $summary.cpu_avg_percent = [Math]::Round(($cpu | Measure-Object -Average).Average, 2)
        $summary.cpu_max_percent = [Math]::Round(($cpu | Measure-Object -Maximum).Maximum, 2)
    }
    if ($ram.Count -gt 0) {
        $summary.ram_peak_mb = [Math]::Round(($ram | Measure-Object -Maximum).Maximum, 2)
    }
    return $summary
}

if (!(Test-Path $Dataset)) {
    Write-Error "No se encontro el dataset: $Dataset. Coloca el archivo real en data/dataset.csv o usa -Dataset."
}

New-Directory $EvidenceDir
New-Directory "$EvidenceDir/bin"
New-Directory "$EvidenceDir/runs"

$exe = Join-Path $EvidenceDir "bin/prepare_bench.exe"
if (!$SkipBuild) {
    Write-Host "Compilando cmd/prepare para medicion estable..."
    go build -o $exe ./cmd/prepare
}

$resultsPath = Join-Path $EvidenceDir "prepare_speedup_summary.csv"
"workers,batch_size,max_rows,exit_code,wall_seconds,program_total_seconds,total_rows,valid_rows,invalid_rows,aggregated_cells,train_observations,test_observations,speedup,efficiency,cpu_avg_percent,cpu_max_percent,ram_peak_mb,run_dir" | Out-File -FilePath $resultsPath -Encoding utf8

$baselineWall = $null
$lastSuccessfulOutDir = $null
foreach ($w in $WorkersList) {
    $runDir = Join-Path $EvidenceDir "runs/prepare_w$w"
    if (Test-Path $runDir) { Remove-Item -Recurse -Force $runDir }
    New-Directory $runDir
    $outDir = Join-Path $runDir "output"
    New-Directory $outDir

    $stdout = Join-Path $runDir "stdout.txt"
    $stderr = Join-Path $runDir "stderr.txt"
    $resourceCsv = Join-Path $runDir "resource_usage.csv"

    $procArgs = @(
        "-file", $Dataset,
        "-workers", "$w",
        "-batch-size", "$BatchSize",
        "-min-year", "$MinYear",
        "-train-until", "$TrainUntil",
        "-negative-ratio", "$NegativeRatio",
        "-target-before", $TargetBefore,
        "-out", $outDir
    )
    if ($MaxRows -gt 0) { $procArgs += @("-max-rows", "$MaxRows") }

    Write-Host "`nEjecutando prepare con workers=$w ..."
    $started = Get-Date
    $argString = Build-ArgumentString $procArgs
    $argLinePath = Join-Path $runDir "arguments.txt"
    $argString | Out-File -LiteralPath $argLinePath -Encoding utf8
    $proc = Start-Process -FilePath $exe -ArgumentList $argString -WorkingDirectory $ProjectRoot -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru -NoNewWindow
    $monitorJob = Start-ResourceMonitor -ProcessId $proc.Id -OutCsv $resourceCsv
    $proc.WaitForExit()
    $proc.Refresh()
    Wait-Job $monitorJob | Out-Null
    Receive-Job $monitorJob | Out-Null
    Remove-Job $monitorJob | Out-Null
    $finished = Get-Date
    $wall = [Math]::Round(($finished - $started).TotalSeconds, 2)

    $programTotal = "NA"; $totalRows = "NA"; $validRows = "NA"; $invalidRows = "NA"; $aggregatedCells = "NA"; $trainObs = "NA"; $testObs = "NA"
    $summaryJson = Join-Path $outDir "prepare_summary.json"
    if (Test-Path $summaryJson) {
        $summary = Get-Content $summaryJson -Raw | ConvertFrom-Json
        $programTotal = [Math]::Round([double]$summary.total_seconds, 2)
        $totalRows = $summary.loader_stats.total_rows
        $validRows = $summary.loader_stats.valid_rows
        $invalidRows = $summary.loader_stats.invalid_rows
        $aggregatedCells = $summary.loader_stats.aggregated_cells
        $trainObs = $summary.train_observations
        $testObs = $summary.test_observations
    }

    $exitCode = $proc.ExitCode
    if ($null -eq $exitCode -and (Test-Path $summaryJson)) { $exitCode = 0 }
    if ($exitCode -eq 0 -and (Test-Path $summaryJson)) { $lastSuccessfulOutDir = $outDir }
    if ($null -eq $baselineWall -and $exitCode -eq 0) { $baselineWall = $wall }
    $speedup = "NA"; $eff = "NA"
    if ($null -ne $baselineWall -and $wall -gt 0) {
        $speedupValue = $baselineWall / $wall
        $speedup = [Math]::Round($speedupValue, 4)
        $eff = [Math]::Round($speedupValue / $w, 4)
    }

    $rs = Get-ResourceSummary -CsvPath $resourceCsv
    "$w,$BatchSize,$MaxRows,$exitCode,$wall,$programTotal,$totalRows,$validRows,$invalidRows,$aggregatedCells,$trainObs,$testObs,$speedup,$eff,$($rs.cpu_avg_percent),$($rs.cpu_max_percent),$($rs.ram_peak_mb),$runDir" | Out-File -FilePath $resultsPath -Encoding utf8 -Append

    if ($exitCode -ne 0 -or !(Test-Path $summaryJson)) {
        Write-Warning "prepare con workers=$w termino sin resumen valido. Revisa stdout.txt y stderr.txt en: $runDir"
    }
}

if ($MirrorOutputDir -ne "" -and $null -ne $lastSuccessfulOutDir) {
    New-Directory $MirrorOutputDir
    Copy-Item -Path (Join-Path $lastSuccessfulOutDir "*") -Destination $MirrorOutputDir -Recurse -Force
    Write-Host "Salida ML copiada tambien a: $MirrorOutputDir"
}

Write-Host "`nResumen generado en: $resultsPath"
Write-Host "Archivos por corrida en: $EvidenceDir/runs"
Write-Host "Usa prepare_speedup_summary.csv para completar la tabla de speedup del informe. No inventes valores."
