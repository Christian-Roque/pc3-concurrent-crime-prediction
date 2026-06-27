# Plan de pruebas PC4

## Objetivo

Verificar que la arquitectura distribuida de PC4 funcione correctamente desde el despliegue hasta la respuesta de predicciones.

## Servicios esperados

```text
api           API REST coordinadora
ml-node-1     Nodo ML TCP 1
ml-node-2     Nodo ML TCP 2
ml-node-3     Nodo ML TCP 3
mongo         Base persistente
redis         Cache
```

## Casos de prueba

| ID | Prueba | Endpoint / comando | Resultado esperado |
|---|---|---|---|
| P01 | Levantamiento de servicios | `docker compose up --build` | API, nodos, MongoDB y Redis activos. |
| P02 | Health API | `GET /health` | `status = ok`. |
| P03 | Bitácora de nodos | `GET /nodes` | Lista de 3 nodos activos. |
| P04 | Información del modelo | `GET /model/info` | Muestra tipo de modelo, cantidad de features y pesos. |
| P05 | Predicción individual | `POST /predict` | Devuelve score, nivel de riesgo y `node_id`. |
| P06 | Round-robin | 3 llamadas seguidas a `/predict` | Cambia el nodo usado entre llamadas. |
| P07 | Cache Redis | repetir la misma consulta | Segunda respuesta con `cached = true`. |
| P08 | Recomendación distribuida | `POST /recommendations/top` | Devuelve `nodes_used` y top de riesgo. |
| P09 | Historial MongoDB | `GET /predictions/history` | Retorna predicciones guardadas. |
| P10 | Métricas | `GET /metrics` | Muestra requests, cache hits y storage. |
| P11 | Error controlado | JSON inválido en `/predict` | HTTP 400 con mensaje de error. |
| P12 | Método no permitido | `GET /predict` | HTTP 405. |

## Comando rápido

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc4_smoke_test.ps1
```

Nota: el script de Windows usa `Invoke-WebRequest` para enviar JSON correctamente y evitar problemas de comillas con `curl.exe` en PowerShell.

Linux/macOS/Git Bash:

```bash
bash scripts/pc4_smoke_test.sh
```

## Evidencias sugeridas para el informe

- Captura de `docker compose up --build`.
- Captura de `docker ps`.
- Respuesta de `/health`.
- Respuesta de `/nodes`.
- Respuesta de `/predict`.
- Segunda respuesta de `/predict` con `cached=true`.
- Respuesta de `/recommendations/top` mostrando varios nodos usados.
- Respuesta de `/predictions/history`.
- Respuesta de `/metrics`.
- Logs de API y nodos ML.
