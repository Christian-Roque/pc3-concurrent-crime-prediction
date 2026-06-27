# PC4 - Implementacion Fase 3: API REST coordinadora

## Objetivo

Integrar el nodo ML TCP de la Fase 2 con una API REST en Go. La API actua como coordinador: recibe solicitudes HTTP, selecciona un nodo ML, envia la consulta por TCP con JSON y devuelve el score de riesgo.

## Archivos agregados

| Archivo | Funcion |
|---|---|
| `cmd/api/main.go` | Ejecuta la API HTTP. Lee puerto, modelo y nodos por flags o variables de entorno. |
| `internal/api/server.go` | Define rutas, handlers, respuestas JSON y llamada al nodo ML. |
| `internal/api/server_test.go` | Prueba de parseo de bitacora de nodos. |

## Endpoints

| Metodo | Endpoint | Descripcion |
|---|---|---|
| GET | `/health` | Estado de la API. |
| GET | `/nodes` | Bitacora de nodos ML configurados. |
| GET | `/model/info` | Informacion del modelo entrenado. |
| POST | `/predict` | Prediccion individual usando un nodo ML por TCP. |
| GET | `/metrics` | Metricas basicas de uso de la API. |

## Flujo de `/predict`

1. Usuario envia una consulta HTTP con `district`, `community_area`, `day_of_week` y `hour`.
2. La API valida el JSON.
3. La API selecciona el siguiente nodo ML desde la bitacora usando round-robin.
4. La API construye un `PredictionRequest`.
5. La API se conecta al nodo con `net.DialTimeout`.
6. El request se envia como JSON.
7. El nodo ML calcula el score usando `models/model.json`.
8. La API recibe la respuesta y la devuelve al usuario.

## Comandos de prueba

Terminal 1:

```powershell
go run ./cmd/mlnode -node-id ml-node-1 -port 9001 -model models/model.json
```

Terminal 2:

```powershell
go run ./cmd/api -port 8080 -nodes localhost:9001 -model models/model.json
```

Prueba:

```powershell
curl -X POST http://localhost:8080/predict -H "Content-Type: application/json" -d '{"district":11,"community_area":23,"day_of_week":2,"hour":18}'
```

## Evidencia esperada

La respuesta debe incluir:

- `request_id`
- nodo seleccionado
- `risk_score`
- `risk_level`
- `latency_ms`

## Siguiente fase

La Fase 4 agregara:

- varios nodos ML;
- Docker Compose con API y nodos;
- endpoint `/recommendations/top` para distribuir candidatos entre nodos;
- preparacion de MongoDB y Redis para las fases siguientes.
