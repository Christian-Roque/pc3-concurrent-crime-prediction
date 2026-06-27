# Evidencias reales de rendimiento y recursos de cómputo

Este documento indica cómo generar evidencias reales para completar el informe sin inventar tiempos ni uso de recursos.

## 1. Recomendación importante

No se deben colocar tiempos estimados. El speedup debe calcularse con ejecuciones equivalentes del mismo comando y la misma data.

Las fórmulas usadas en el informe son:

```text
Speedup(p) = Tiempo con 1 worker / Tiempo con p workers
Eficiencia(p) = Speedup(p) / p
```

## 2. Evidencia de speedup en carga, limpieza y preparación

Ejecutar desde la raíz del proyecto:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc3_benchmark_prepare.ps1 `
  -Dataset data\dataset.csv `
  -WorkersList 1,2,4,8 `
  -BatchSize 20000 `
  -MaxRows 0
```

Salida principal:

```text
evidencias/performance/prepare/prepare_speedup_summary.csv
```

También se genera una carpeta por corrida:

```text
evidencias/performance/prepare/runs/prepare_w1
evidencias/performance/prepare/runs/prepare_w2
evidencias/performance/prepare/runs/prepare_w4
evidencias/performance/prepare/runs/prepare_w8
```

Cada carpeta incluye:

```text
stdout.txt
stderr.txt
resource_usage.csv
output/prepare_summary.json
output/loader_stats.json
```

## 3. Evidencia de speedup en entrenamiento

Primero debe existir la salida de preparación:

```text
output/ml_observations_train.csv.gz
output/ml_observations_test.csv.gz
output/ml_observations_metadata.json
```

Luego ejecutar:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc3_benchmark_train.ps1 `
  -WorkersList 1,2,4,8 `
  -Epochs 80
```

Salida principal:

```text
evidencias/performance/train/train_speedup_summary.csv
```

Cada corrida incluye:

```text
stdout.txt
stderr.txt
resource_usage.csv
output/train_summary.json
```

Para el informe, usar el `wall_seconds` del CSV como tiempo comparable end-to-end del comando. El campo `program_total_seconds` corresponde al tiempo interno reportado por el programa y puede no incluir toda la carga previa del comando.

## 4. Evidencia de uso de CPU/RAM durante el proceso, sin capturas manuales

Los scripts anteriores generan automáticamente `resource_usage.csv` por corrida. Este archivo contiene muestras periódicas de:

```text
timestamp
cpu_total_percent
process_cpu_seconds
process_memory_mb
```

Por ello, no es obligatorio estar mirando el Administrador de tareas durante toda la ejecución. La evidencia principal puede ser:

```text
evidencias/performance/prepare/runs/prepare_w*/resource_usage.csv
evidencias/performance/train/runs/train_w*/resource_usage.csv
evidencias/performance/prepare/prepare_speedup_summary.csv
evidencias/performance/train/train_speedup_summary.csv
```

Con estos archivos se puede sustentar el uso de recursos de cómputo por cada configuración de workers. Las capturas del Administrador de tareas pasan a ser evidencia complementaria, no obligatoria.

Para generar un reporte visual automático con tablas y gráficos, ejecutar al final:

```powershell
python .\scripts\generate_performance_report.py --evidence-dir evidencias\performance
```

Salida esperada:

```text
evidencias/performance/performance_report.html
evidencias/performance/charts/*.svg
```

El HTML y los SVG pueden usarse como evidencia en el informe sin necesidad de tomar capturas manuales mientras el proceso corre.

## 5. Evidencia de recursos en arquitectura distribuida PC4

Levantar la arquitectura:

```powershell
docker compose up --build
```

En otra terminal, ejecutar:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc4_docker_stats_capture.ps1 `
  -DurationSeconds 90 `
  -IntervalSeconds 3
```

Salida:

```text
evidencias/performance/docker/docker_stats_samples.csv
```

El script genera muestras periódicas de `docker stats` en CSV. Por ello, la captura manual de `docker stats` es opcional. Si se desea una evidencia visual adicional, abrir:

```text
evidencias/performance/performance_report.html
```

Ahí se generará un gráfico de CPU por contenedor. Los contenedores esperados son:

```text
api
ml-node-1
ml-node-2
ml-node-3
mongo
redis
```

## 6. Evidencias PC4 funcionales

Con `docker compose up --build` activo, ejecutar:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc4_smoke_test.ps1
```

Esto genera evidencias JSON de API, nodos, predicción, cache, recommendations, historial y métricas.

## 7. Generar reporte automático final

Cuando terminen las pruebas de prepare, train y docker stats, ejecutar:

```powershell
python .\scripts\generate_performance_report.py --evidence-dir evidencias\performance
```

Este comando consolida las evidencias en:

```text
evidencias/performance/performance_report.html
evidencias/performance/charts/prepare_speedup.svg
evidencias/performance/charts/train_speedup.svg
evidencias/performance/charts/docker_cpu_percent.svg
evidencias/performance/charts/prepare_*_cpu_ram.svg
evidencias/performance/charts/train_*_cpu_ram.svg
```

## 8. Qué completar en el informe

Completar con valores reales:

- Tabla de speedup de carga/limpieza usando `prepare_speedup_summary.csv`.
- Tabla de speedup de entrenamiento usando `train_speedup_summary.csv`.
- Uso de CPU/RAM usando los `resource_usage.csv` y gráficos generados.
- Uso de recursos por contenedor usando `docker_stats_samples.csv` y `docker_cpu_percent.svg`.
- Archivos JSON generados por `pc4_smoke_test.ps1`.

No colocar valores si no fueron medidos.
