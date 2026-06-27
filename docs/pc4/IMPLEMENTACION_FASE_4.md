# PC4 - Fase 4: Cluster ML y endpoint distribuido

## Objetivo

Integrar la API REST de la fase 3 con varios nodos ML TCP y agregar un endpoint que distribuya el cálculo de recomendaciones entre nodos.

## Componentes implementados

- `internal/cluster/dispatcher.go`: reparte candidatos entre nodos activos y consolida resultados.
- `POST /recommendations/top`: endpoint distribuido de recomendación.
- `docker-compose.yml`: despliegue de API y tres nodos ML.
- `Dockerfile`: ahora compila también `api` y `mlnode`.

## Flujo distribuido

1. El usuario envía un conjunto de celdas candidatas a `/recommendations/top`.
2. La API obtiene la lista de nodos activos desde la bitácora interna.
3. La API divide los candidatos entre los nodos.
4. Cada nodo calcula scores parciales usando `models/model.json`.
5. La API consolida los resultados y devuelve el top N global.

## Evidencia esperada

- `GET /nodes` muestra tres nodos ML.
- Varias llamadas a `POST /predict` cambian el nodo por round-robin.
- `POST /recommendations/top` devuelve `nodes_used` con los nodos participantes.
- `go test ./...` pasa correctamente.
