# PC4 - Fase 5: MongoDB y Redis

## Objetivo

La quinta fase completa el bloque de almacenamiento solicitado para PC4. La API REST deja de ser únicamente un coordinador de nodos ML y ahora también registra resultados y utiliza cache.

## Componentes agregados

```text
internal/storage/types.go
internal/storage/redis.go
internal/storage/bson.go
internal/storage/mongo.go
```

## MongoDB

MongoDB se utiliza como almacenamiento persistente. Cada predicción o recomendación queda registrada con:

```text
request_id
query_type
input
result
node
cached
latency_ms
created_at
```

Endpoint agregado:

```text
GET /predictions/history
```

## Redis

Redis se utiliza como cache para predicciones individuales. La API construye una llave a partir de:

```text
district + community_area + day_of_week + hour + week_start
```

Flujo:

```text
1. Llega POST /predict.
2. API busca la llave en Redis.
3. Si existe, responde cached=true.
4. Si no existe, consulta un nodo ML por TCP.
5. Guarda el resultado en Redis y MongoDB.
```

## Docker Compose

Se agregan dos servicios:

```text
mongo:27017
redis:6379
```

La API recibe las variables:

```text
MONGO_ADDR=mongo:27017
MONGO_DB=pc4
MONGO_COLLECTION=predictions
REDIS_ADDR=redis:6379
REDIS_TTL_SECONDS=300
```

## Evidencias sugeridas

1. `docker compose up --build` mostrando API, nodos ML, MongoDB y Redis.
2. Primera llamada a `/predict` con `cached=false`.
3. Segunda llamada igual a `/predict` con `cached=true`.
4. Llamada a `/predictions/history` mostrando registros persistidos.
5. Llamada a `/metrics` mostrando `mongo_enabled=true`, `redis_enabled=true` y `cache_hits`.

## Relación con el enunciado

Esta fase cubre la implementación de bases de datos solicitada en PC4:

- Persistente: MongoDB.
- Cache/colaborativo: Redis.

Además, mantiene la arquitectura distribuida de la fase anterior: la API coordina los nodos ML por TCP y ahora almacena resultados para consulta posterior.
