# PC3 - Seguridad ciudadana con programación concurrente en Go

Proyecto para el entregable **PC3** del curso **CC65 Programación Concurrente y Distribuida**.

## Caso

El sistema procesa datos abiertos de delitos urbanos para construir un **score de riesgo estimado** por zona, día y hora. El objetivo no es predecir personas, sino ordenar combinaciones espacio-temporales según la posibilidad de ocurrencia futura de delitos relevantes para seguridad ciudadana.

Archivo esperado:

```text
data/dataset.csv
```

El dataset real no se versiona por tamaño.

## Alcance PC3

PC3 se enfoca en los puntos solicitados por el enunciado:

1. Presentación del caso a resolver: problema y motivación.
2. Limpieza y análisis de datos.
3. Diseño del modelo ML.
4. Paralelización del cálculo.
5. Evidencias de la implementación.
6. Reporte de participación.

No se implementa API, frontend, MongoDB ni Redis en PC3. Esos componentes corresponden a PC4/TB2.

## Flujo del proyecto

El proyecto está separado en comandos para que cada etapa sea clara:

```text
cmd/prepare  -> carga, limpia, agrega y crea observaciones ML.
cmd/train    -> entrena el modelo final con cálculo de gradientes paralelizado.
cmd/pc3      -> ejecuta solo la etapa de carga/limpieza concurrente, si se desea revisarla por separado.
```

## 1. Ejecución completa

PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\run_pc3_completo.ps1
```

Este script ejecuta:

```text
go test ./...
cmd/prepare
cmd/train
```

## 2. Preparar la data ML

```powershell
go run ./cmd/prepare -file data/dataset.csv -workers 8 -batch-size 20000 -min-year 2022 -train-until 2025 -negative-ratio 0 -target-before 2026-06-01 -out output
```

Esta etapa usa goroutines y channels para cargar, limpiar y agregar los registros reales del CSV. Luego construye las observaciones ML de entrenamiento y prueba.

Genera:

```text
output/weekly_aggregates.csv
output/loader_stats.json
output/ml_observations_train.csv.gz
output/ml_observations_test.csv.gz
output/ml_observations_metadata.json
output/prepare_summary.json
```

## 3. Entrenar el modelo final

```powershell
go run ./cmd/train -train output/ml_observations_train.csv.gz -test output/ml_observations_test.csv.gz -metadata output/ml_observations_metadata.json -workers 8 -epochs 80 -lambda 0.0005 -out output -model models/model.json
```

Esta etapa entrena una regresión logística binaria. El cálculo del gradiente se divide en bloques y se procesa con goroutines. Los gradientes parciales se combinan antes de actualizar los pesos del modelo.

Genera:

```text
models/model.json
output/train_summary.json
output/risk_predictions_sample.csv
```

## 4. Ejecutar solo carga/limpieza concurrente

```powershell
go run ./cmd/pc3 -file data/dataset.csv -workers 8 -batch-size 20000 -min-year 2022 -out output
```

Genera:

```text
output/weekly_aggregates.csv
output/loader_stats.json
```

## Modelo ML

- Modelo: regresión logística binaria.
- Unidad de análisis: `semana + distrito + community area + día de semana + hora`.
- Target: `ocurre_delito_relevante_siguiente_semana`.
- Interpretación: score de riesgo estimado para ranking, no probabilidad calibrada.
- Delitos relevantes: severos, violentos, armas y patrimoniales de alto impacto.
- Delitos menores: `THEFT`, `CRIMINAL DAMAGE`, `CRIMINAL TRESPASS` no activan el target, pero se usan como contexto predictivo.
- Evaluación: AUC, Gini, logloss, precision, recall y F1.

## Concurrencia implementada

### Carga y limpieza concurrente

```text
CSV -> batches -> goroutines workers -> agregados parciales -> reducer final
```

La etapa de carga procesa el CSV en streaming, no carga todo el archivo en memoria y conserva solo agregaciones parciales por celda espacio-temporal.

### Entrenamiento paralelo

```text
observaciones ML preparadas -> chunks -> gradientes parciales por goroutine -> suma de gradientes -> actualización de pesos
```

El entrenamiento usa una matriz compacta row-major para reducir overhead de memoria y paraleliza el cálculo de gradientes.

## Docker

```powershell
docker compose run --rm pc3-trainer
```

El contenedor ejecuta `prepare` y luego `train`.

## Estructura

```text
cmd/prepare/          Preparación inicial de observaciones ML
cmd/train/            Entrenamiento final del modelo
cmd/pc3/              Carga y limpieza concurrente independiente
internal/crime/       Carga concurrente, limpieza y agregación
internal/ml/          Features, dataset preparado, regresión logística y métricas
data/                 Dataset real y muestra mínima
output/               Resultados generados
models/               Modelo entrenado
docs/                 Informe PC3
```

## PC4 - Fase 1 y 2: base distribuida y nodo ML TCP

Esta version inicia la integracion de PC4. Se mantiene todo lo construido para PC3 y se agregan los primeros componentes distribuidos:

- `internal/cluster`: define el protocolo JSON entre la API y los nodos ML, cliente TCP y registro/round-robin de nodos.
- `internal/node`: implementa el predictor y servidor TCP de cada nodo ML.
- `cmd/mlnode`: ejecutable de un nodo ML que carga `models/model.json`, escucha por TCP y responde predicciones.

### Ejecutar un nodo ML local

```powershell
go run ./cmd/mlnode -node-id ml-node-1 -port 9001 -model models/model.json
```

El nodo queda escuchando en `:9001`. En PC4 la API se conectara a este nodo usando TCP interno y mensajes JSON.

### Mensaje esperado por el nodo

```json
{
  "request_id": "req-001",
  "command": "predict",
  "input": {
    "district": 11,
    "community_area": 23,
    "day_of_week": 2,
    "hour": 18
  }
}
```

### Respuesta esperada

```json
{
  "request_id": "req-001",
  "node_id": "ml-node-1",
  "status": "ok",
  "result": {
    "district": 11,
    "community_area": 23,
    "day_of_week": 2,
    "hour": 18,
    "risk_score": 0.1561,
    "risk_level": "Bajo",
    "node_id": "ml-node-1"
  }
}
```

### Nota tecnica

El nodo acepta dos modos de prediccion:

1. Consulta simple: `district`, `community_area`, `day_of_week`, `hour`. El nodo construye un vector base compatible con el modelo y completa features historicas con cero.
2. Consulta tecnica: `features` con el vector completo del modelo. Esta ruta sera util para pruebas exactas y para futuras integraciones con agregados historicos.


## PC4 - Fase 3: API REST coordinadora

En esta fase se agrega la API HTTP que funcionara como coordinador del cluster ML. La API no calcula directamente el score: recibe la solicitud REST, selecciona un nodo ML de la bitacora, envia la consulta por TCP usando JSON y devuelve la respuesta al usuario.

Componentes agregados:

```text
cmd/api                  Ejecutable de la API REST
internal/api             Handlers HTTP, rutas y utilidades de respuesta JSON
internal/cluster/client  Cliente TCP usado por la API para consultar nodos ML
```

### Ejecutar Fase 3 localmente

Terminal 1: levantar nodo ML TCP.

```powershell
go run ./cmd/mlnode -node-id ml-node-1 -port 9001 -model models/model.json
```

Terminal 2: levantar API REST.

```powershell
go run ./cmd/api -port 8080 -nodes localhost:9001 -model models/model.json
```

### Endpoints implementados

```text
GET  /health       Verifica que la API esta activa.
GET  /nodes        Muestra la bitacora de nodos ML configurados.
GET  /model/info   Muestra informacion del modelo entrenado.
POST /predict      Envia una consulta al nodo ML por TCP y devuelve el score.
GET  /metrics      Muestra metricas basicas de uso de la API.
```

### Pruebas con curl

```powershell
curl http://localhost:8080/health
curl http://localhost:8080/nodes
curl http://localhost:8080/model/info
```

Prediccion individual:

```powershell
curl -X POST http://localhost:8080/predict `
  -H "Content-Type: application/json" `
  -d '{"district":11,"community_area":23,"day_of_week":2,"hour":18,"week_start":"2026-06-01"}'
```

Respuesta esperada:

```json
{
  "request_id": "req-...",
  "node": {
    "node_id": "ml-node-1",
    "address": "localhost:9001",
    "status": "active"
  },
  "cached": false,
  "latency_ms": 2,
  "result": {
    "district": 11,
    "community_area": 23,
    "day_of_week": 2,
    "hour": 18,
    "risk_score": 0.1573,
    "risk_level": "Bajo",
    "node_id": "ml-node-1"
  }
}
```

### Relacion con lo visto en clase

La API reutiliza los conceptos trabajados en los miniproyectos:

```text
net/http       -> endpoints REST.
net.Dial       -> cliente TCP desde API hacia nodo ML.
JSON           -> protocolo API-nodo.
Bitacora       -> endpoint /nodes.
Round-robin    -> seleccion de nodo cuando existan varios nodos.
```

La siguiente fase agregara varios nodos en Docker Compose y el endpoint distribuido `/recommendations/top`.

## PC4 - Fase 4: Cluster de nodos ML y recomendación distribuida

Esta versión agrega la integración distribuida inicial del proyecto:

- `cmd/api`: API REST coordinadora.
- `cmd/mlnode`: nodo ML TCP.
- `internal/cluster`: protocolo JSON, cliente TCP, bitácora de nodos, round-robin y dispatcher distribuido.
- `internal/node`: servidor TCP de cada nodo ML y predictor basado en `models/model.json`.
- `docker-compose.yml`: levanta API + 3 nodos ML.

### Ejecutar localmente sin Docker

Terminal 1:

```powershell
go run ./cmd/mlnode -node-id ml-node-1 -port 9001 -model models/model.json
```

Terminal 2:

```powershell
go run ./cmd/mlnode -node-id ml-node-2 -port 9002 -model models/model.json
```

Terminal 3:

```powershell
go run ./cmd/mlnode -node-id ml-node-3 -port 9003 -model models/model.json
```

Terminal 4:

```powershell
go run ./cmd/api -port 8080 -nodes localhost:9001,localhost:9002,localhost:9003 -model models/model.json
```

### Endpoints principales

```text
GET  /health
GET  /nodes
GET  /model/info
POST /predict
POST /recommendations/top
GET  /metrics
```

### Probar round-robin con /predict

```powershell
curl -X POST http://localhost:8080/predict `
  -H "Content-Type: application/json" `
  -d '{"district":11,"community_area":23,"day_of_week":2,"hour":18,"week_start":"2026-06-01"}'
```

Ejecutando varias veces el endpoint, la respuesta debe cambiar el `node_id`, evidenciando la distribución por round-robin.

### Probar procesamiento distribuido con /recommendations/top

```powershell
curl -X POST http://localhost:8080/recommendations/top `
  -H "Content-Type: application/json" `
  -d '{"top_n":3,"candidates":[{"district":11,"community_area":23,"day_of_week":2,"hour":18,"week_start":"2026-06-01"},{"district":7,"community_area":68,"day_of_week":5,"hour":21,"week_start":"2026-06-01"},{"district":12,"community_area":30,"day_of_week":6,"hour":22,"week_start":"2026-06-01"},{"district":4,"community_area":40,"day_of_week":1,"hour":10,"week_start":"2026-06-01"},{"district":18,"community_area":8,"day_of_week":4,"hour":23,"week_start":"2026-06-01"}]}'
```

Este endpoint divide los candidatos entre nodos ML, cada nodo calcula scores parciales por TCP y la API consolida el top N global.

### Ejecutar con Docker Compose

```powershell
docker compose up --build
```

Servicios levantados:

```text
api          -> http://localhost:8080
ml-node-1    -> TCP interno ml-node-1:9001
ml-node-2    -> TCP interno ml-node-2:9001
ml-node-3    -> TCP interno ml-node-3:9001
```

La etapa offline de PC3 queda disponible como perfil opcional:

```powershell
docker compose --profile offline up pc3-trainer --build
```

## PC4 - Fase 5: MongoDB y Redis

Esta fase agrega almacenamiento y cache a la arquitectura distribuida:

- **MongoDB**: almacena el historial persistente de predicciones y recomendaciones.
- **Redis**: cachea predicciones repetidas para responder sin consultar nuevamente al cluster ML.
- **API REST**: agrega `GET /predictions/history` y mejora `/metrics` con estado de almacenamiento.

### Servicios Docker Compose de Fase 5

```text
api
ml-node-1
ml-node-2
ml-node-3
mongo
redis
```

### Ejecutar la arquitectura completa

```powershell
docker compose up --build
```

### Probar cache Redis

Primera consulta: debe responder con `cached=false`.

```powershell
curl -X POST http://localhost:8080/predict `
  -H "Content-Type: application/json" `
  -d '{"district":11,"community_area":23,"day_of_week":2,"hour":18,"week_start":"2026-06-01"}'
```

Segunda consulta igual: debe responder con `cached=true`, porque la respuesta se recupera desde Redis.

```powershell
curl -X POST http://localhost:8080/predict `
  -H "Content-Type: application/json" `
  -d '{"district":11,"community_area":23,"day_of_week":2,"hour":18,"week_start":"2026-06-01"}'
```

### Probar historial MongoDB

```powershell
curl http://localhost:8080/predictions/history
```

También se puede limitar la cantidad de registros:

```powershell
curl "http://localhost:8080/predictions/history?limit=5"
```

### Revisar métricas

```powershell
curl http://localhost:8080/metrics
```

La respuesta incluye:

```text
requests_predict
requests_recommendations_top
cache_hits
cache_errors
storage_errors
storage.mongo_enabled
storage.redis_enabled
```

### Nota técnica de dependencias

Para mantener el proyecto autocontenido y compilable sin descargar librerías externas, Redis y MongoDB se integran usando la librería estándar de Go:

- Redis: cliente RESP mínimo sobre TCP.
- MongoDB: cliente OP_MSG/BSON mínimo para insertar y consultar historial.

Esto permite evidenciar comunicación con servicios externos de almacenamiento sin depender de paquetes adicionales.


## PC4 - Fase 6: cierre técnico y evidencias

Esta fase agrega recursos para probar y sustentar la entrega PC4. No cambia el flujo principal de predicción; añade scripts y documentación para generar evidencias de funcionamiento.

### Recursos agregados

```text
scripts/pc4_smoke_test.ps1
scripts/pc4_smoke_test.sh
docs/pc4/PLAN_PRUEBAS_PC4.md
docs/pc4/CHECKLIST_ENTREGA_PC4.md
docs/pc4/POSTMAN_PC4_COLLECTION.json
docs/pc4/IMPLEMENTACION_FASE_6.md
```

### Levantar arquitectura PC4

```powershell
docker compose up --build
```

La arquitectura levanta:

```text
api
ml-node-1
ml-node-2
ml-node-3
mongo
redis
```

### Generar evidencias automáticamente

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc4_smoke_test.ps1
```

Nota: el script de Windows usa `Invoke-WebRequest` para enviar JSON correctamente y evitar problemas de comillas con `curl.exe` en PowerShell.

Linux/macOS/Git Bash:

```bash
bash scripts/pc4_smoke_test.sh
```

Los scripts generan una carpeta `evidencias/pc4_YYYYMMDD_HHMMSS` con respuestas JSON de:

```text
/health
/nodes
/model/info
/predict
/recommendations/top
/predictions/history
/metrics
```

### Colección Postman

Se incluye una colección lista para importar:

```text
docs/pc4/POSTMAN_PC4_COLLECTION.json
```

Variable principal:

```text
base_url = http://localhost:8080
```

### Evidencias recomendadas para el informe

1. `docker compose up --build`.
2. `docker ps` con API, 3 nodos, MongoDB y Redis.
3. `GET /health`.
4. `GET /nodes`.
5. `POST /predict` mostrando `node_id`.
6. Segunda llamada a `/predict` mostrando `cached=true`.
7. `POST /recommendations/top` mostrando `nodes_used`.
8. `GET /predictions/history` mostrando MongoDB.
9. `GET /metrics` mostrando requests, cache y storage.
10. Logs de API y nodos ML.

## Evidencias reales de speedup y recursos de cómputo

Para completar el informe sin inventar tiempos ni uso de CPU/RAM, se agregaron scripts de medición en `scripts/`.

### Speedup de carga/limpieza/preparación

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc3_benchmark_prepare.ps1 -Dataset data\dataset.csv -WorkersList 1,2,4,8 -BatchSize 20000 -MaxRows 0
```

Resultado:

```text
evidencias/performance/prepare/prepare_speedup_summary.csv
```

### Speedup de entrenamiento

Ejecutar después de contar con los archivos preparados en `output/`:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc3_benchmark_train.ps1 -WorkersList 1,2,4,8 -Epochs 80
```

Resultado:

```text
evidencias/performance/train/train_speedup_summary.csv
```

### Recursos de Docker / PC4

Con `docker compose up --build` activo:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc4_docker_stats_capture.ps1 -DurationSeconds 90 -IntervalSeconds 3
```

Resultado:

```text
evidencias/performance/docker/docker_stats_samples.csv
```

Guía completa: `docs/pc4/EVIDENCIAS_RENDIMIENTO_Y_RECURSOS.md`.

## Evidencias automáticas de recursos sin capturas manuales

Para no depender de capturas del Administrador de tareas durante procesos largos, se agregaron scripts que registran CPU, RAM y tiempos en archivos CSV.

1. Ejecutar benchmark de preparación:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc3_benchmark_prepare.ps1 -Dataset data\dataset.csv -WorkersList 1,2,4,8 -BatchSize 20000 -MaxRows 0
```

2. Ejecutar benchmark de entrenamiento, después de tener los archivos `output/ml_observations_*.csv.gz`:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc3_benchmark_train.ps1 -WorkersList 1,2,4,8 -Epochs 80
```

3. Con Docker Compose levantado, capturar recursos de contenedores:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc4_docker_stats_capture.ps1 -DurationSeconds 90 -IntervalSeconds 3
```

4. Generar reporte visual automático:

```powershell
python .\scripts\generate_performance_report.py --evidence-dir evidencias\performance
```

Se generará:

```text
evidencias/performance/performance_report.html
evidencias/performance/charts/*.svg
```

Estos archivos reemplazan la necesidad de estar tomando capturas manuales durante todo el procesamiento. Las capturas quedan como evidencia opcional.
