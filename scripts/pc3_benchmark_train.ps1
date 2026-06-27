param(
    [string]$TrainPath = "output/ml_observations_train.csv.gz",
    [string]$TestPath = "output/ml_observations_test.csv.gz",
    [string]$MetadataPath = "output/ml_observations_metadata.json",
    [object[]]$WorkersList = @(1,2,4,8),
    [int]$Epochs = 80,
    [double]$LearningRate = 0.05,
    [double]$Lambda = 0.0005,
    [string]$EvidenceDir = "evidencias/performance/train",
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

$TrainPath = Convert-ToAbsolutePath $TrainPath
$TestPath = Convert-ToAbsolutePath $TestPath
$MetadataPath = Convert-ToAbsolutePath $MetadataPath
$EvidenceDir = Convert-ToAbsolutePath $EvidenceDir
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

if (!(Test-Path $TrainPath) -or !(Test-Path $TestPath) -or !(Test-Path $MetadataPath)) {
    $prepareRuns = Join-Path $ProjectRoot "evidencias/performance/prepare/runs"
    if (Test-Path $prepareRuns) {
        $candidate = Get-ChildItem -Path $prepareRuns -Directory -Filter "prepare_w*" |
            Sort-Object { [int](($_.Name -replace "[^0-9]", "")) } -Descending |
            Select-Object -First 1
        if ($null -ne $candidate) {
            $candidateOut = Join-Path $candidate.FullName "output"
            $candidateTrain = Join-Path $candidateOut "ml_observations_train.csv.gz"
            $candidateTest = Join-Path $candidateOut "ml_observations_test.csv.gz"
            $candidateMeta = Join-Path $candidateOut "ml_observations_metadata.json"
            if ((Test-Path $candidateTrain) -and (Test-Path $candidateTest) -and (Test-Path $candidateMeta)) {
                Write-Host "No se encontro output/ completo. Usando salida del benchmark prepare: $candidateOut"
                $TrainPath = $candidateTrain
                $TestPath = $candidateTest
                $MetadataPath = $candidateMeta
            }
        }
    }
}

foreach ($path in @($TrainPath,$TestPath,$MetadataPath)) {
    if (!(Test-Path $path)) { Write-Error "No se encontro archivo requerido: $path. Ejecuta primero cmd/prepare o revisa stderr.txt de prepare." }
}

New-Directory $EvidenceDir
New-Directory "$EvidenceDir/bin"
New-Directory "$EvidenceDir/runs"
New-Directory "$EvidenceDir/models"

$exe = Join-Path $EvidenceDir "bin/train_bench.exe"
if (!$SkipBuild) {
    Write-Host "Compilando cmd/train para medicion estable..."
    go build -o $exe ./cmd/train
}

$resultsPath = Join-Path $EvidenceDir "train_speedup_summary.csv"
"workers,epochs,exit_code,wall_seconds,program_total_seconds,train_observations,test_observations,test_auc,test_gini,test_logloss,speedup,efficiency,cpu_avg_percent,cpu_max_percent,ram_peak_mb,run_dir" | Out-File -FilePath $resultsPath -Encoding utf8

$baselineWall = $null
foreach ($w in $WorkersList) {
    $runDir = Join-Path $EvidenceDir "runs/train_w$w"
    if (Test-Path $runDir) { Remove-Item -Recurse -Force $runDir }
    New-Directory $runDir
    $outDir = Join-Path $runDir "output"
    New-Directory $outDir
    $modelPath = Join-Path $EvidenceDir "models/model_w$w.json"

    $stdout = Join-Path $runDir "stdout.txt"
    $stderr = Join-Path $runDir "stderr.txt"
    $resourceCsv = Join-Path $runDir "resource_usage.csv"

    $procArgs = @(
        "-train", $TrainPath,
        "-test", $TestPath,
        "-metadata", $MetadataPath,
        "-workers", "$w",
        "-epochs", "$Epochs",
        "-lr", "$LearningRate",
        "-lambda", "$Lambda",
        "-out", $outDir,
        "-model", $modelPath
    )

    Write-Host "`nEjecutando train con workers=$w epochs=$Epochs ..."
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

    $programTotal = "NA"; $trainObs = "NA"; $testObs = "NA"; $auc = "NA"; $gini = "NA"; $logloss = "NA"
    $summaryJson = Join-Path $outDir "train_summary.json"
    if (Test-Path $summaryJson) {
        $summary = Get-Content $summaryJson -Raw | ConvertFrom-Json
        $programTotal = [Math]::Round([double]$summary.total_seconds, 2)
        $trainObs = $summary.train_observations
        $testObs = $summary.test_observations
        $auc = [Math]::Round([double]$summary.test_metrics.auc, 4)
        $gini = [Math]::Round([double]$summary.test_metrics.gini, 4)
        $logloss = [Math]::Round([double]$summary.test_metrics.logloss, 4)
    }

    $exitCode = $proc.ExitCode
    if ($null -eq $exitCode -and (Test-Path $summaryJson)) { $exitCode = 0 }
    if ($null -eq $baselineWall -and $exitCode -eq 0) { $baselineWall = $wall }
    $speedup = "NA"; $eff = "NA"
    if ($null -ne $baselineWall -and $wall -gt 0) {
        $speedupValue = $baselineWall / $wall
        $speedup = [Math]::Round($speedupValue, 4)
        $eff = [Math]::Round($speedupValue / $w, 4)
    }

    $rs = Get-ResourceSummary -CsvPath $resourceCsv
    "$w,$Epochs,$exitCode,$wall,$programTotal,$trainObs,$testObs,$auc,$gini,$logloss,$speedup,$eff,$($rs.cpu_avg_percent),$($rs.cpu_max_percent),$($rs.ram_peak_mb),$runDir" | Out-File -FilePath $resultsPath -Encoding utf8 -Append

    if ($exitCode -ne 0 -or !(Test-Path $summaryJson)) {
        Write-Warning "train con workers=$w termino sin resumen valido. Revisa stdout.txt y stderr.txt en: $runDir"
    }
}

Write-Host "`nResumen generado en: $resultsPath"
Write-Host "Archivos por corrida en: $EvidenceDir/runs"
Write-Host "Usa train_speedup_summary.csv para completar la tabla de speedup del informe. No inventes valores."
