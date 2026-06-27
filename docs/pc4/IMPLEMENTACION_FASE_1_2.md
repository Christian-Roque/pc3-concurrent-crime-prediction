# PC4 - Implementacion Fase 1 y 2

## Fase 1: estructura base PC4

Se agregaron los paquetes base para la arquitectura distribuida:

| Ruta | Proposito |
|---|---|
| `internal/cluster` | Protocolo JSON, cliente TCP y registro de nodos. |
| `internal/node` | Servidor TCP y predictor del nodo ML. |
| `cmd/mlnode` | Ejecutable de nodo ML. |

## Fase 2: nodo ML TCP

El nodo ML carga `models/model.json` al iniciar y escucha conexiones TCP. Cada conexion entrante se atiende en una goroutine, siguiendo el patron trabajado en clase con `net.Listen`, `Accept` y `go handle(conn)`.

Flujo:

1. Nodo inicia con `NODE_ID`, `NODE_PORT` y `MODEL_PATH`.
2. Carga el modelo entrenado en PC3.
3. Escucha por TCP.
4. Recibe request JSON.
5. Calcula score con el modelo.
6. Devuelve response JSON.

## Comando local

```powershell
go run ./cmd/mlnode -node-id ml-node-1 -port 9001 -model models/model.json
```

## Validaciones realizadas

- `go test ./...` finaliza correctamente.
- El nodo responde una solicitud TCP JSON de prueba.
- La respuesta incluye `request_id`, `node_id`, `status`, `risk_score` y `risk_level`.

## Siguiente fase

La siguiente fase es crear `cmd/api` e `internal/api`, para que la API REST funcione como coordinador y se comunique con este nodo ML usando `internal/cluster/client.go`.
