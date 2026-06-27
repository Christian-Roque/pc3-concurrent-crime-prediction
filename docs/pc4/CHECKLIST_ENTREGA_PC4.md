# Checklist final de entrega PC4

## Código fuente

- [ ] El ZIP no incluye `data/` ni `output/` pesados.
- [ ] Existe `models/model.json`.
- [ ] Existe `cmd/api`.
- [ ] Existe `cmd/mlnode`.
- [ ] Existe `internal/cluster`.
- [ ] Existe `internal/node`.
- [ ] Existe `internal/storage`.
- [ ] Existe `docker-compose.yml` con API, 3 nodos ML, MongoDB y Redis.

## Validaciones técnicas

- [ ] `go test ./...` pasa correctamente.
- [ ] `go vet ./...` no reporta problemas relevantes.
- [ ] `docker compose up --build` levanta los servicios.
- [ ] `/health` responde correctamente.
- [ ] `/nodes` muestra 3 nodos ML.
- [ ] `/predict` devuelve score y nodo usado.
- [ ] Varias llamadas a `/predict` muestran round-robin.
- [ ] La segunda consulta repetida muestra `cached=true`.
- [ ] `/recommendations/top` usa varios nodos y devuelve top N.
- [ ] `/predictions/history` muestra historial desde MongoDB.
- [ ] `/metrics` muestra métricas del sistema.

## Informe PC4

- [ ] Explica continuidad desde PC3.
- [ ] Explica arquitectura distribuida PC4.
- [ ] Incluye diagrama API -> cluster ML -> MongoDB/Redis.
- [ ] Describe API REST.
- [ ] Describe comunicación TCP interna.
- [ ] Describe protocolo JSON.
- [ ] Describe MongoDB y Redis.
- [ ] Incluye evidencias de ejecución.
- [ ] Incluye reporte de participación.
- [ ] Incluye GitHub y Git Flow en anexos.

## Video

- [ ] Duración máxima 6 minutos.
- [ ] Participan todos los integrantes.
- [ ] Se muestra funcionamiento end to end.
- [ ] Se explica PC3 como núcleo y PC4 como integración distribuida.
- [ ] Se demuestra API, nodos, MongoDB, Redis y Docker Compose.
