#!/usr/bin/env bash
# Pruebas rápidas PC4 para generar evidencias con curl.
# Requisito: ejecutar antes `docker compose up --build` o tener la API disponible en API_URL.

set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
STAMP="$(date +%Y%m%d_%H%M%S)"
OUT_DIR="${OUT_DIR:-evidencias/pc4_$STAMP}"
mkdir -p "$OUT_DIR"

echo "== PC4 Smoke Test =="
echo "API_URL=$API_URL"
echo "OUT_DIR=$OUT_DIR"

run_get() {
  local name="$1"
  local path="$2"
  echo "GET $path"
  curl -sS "$API_URL$path" | tee "$OUT_DIR/$name.json" >/dev/null
}

run_post() {
  local name="$1"
  local path="$2"
  local payload="$3"
  echo "POST $path"
  curl -sS -X POST "$API_URL$path" \
    -H "Content-Type: application/json" \
    -d "$payload" | tee "$OUT_DIR/$name.json" >/dev/null
}

PREDICT_PAYLOAD='{"district":11,"community_area":23,"day_of_week":2,"hour":18,"week_start":"2026-06-01"}'
TOP_PAYLOAD='{"top_n":3,"candidates":[{"district":11,"community_area":23,"day_of_week":2,"hour":18,"week_start":"2026-06-01"},{"district":7,"community_area":68,"day_of_week":5,"hour":21,"week_start":"2026-06-01"},{"district":12,"community_area":30,"day_of_week":6,"hour":22,"week_start":"2026-06-01"},{"district":4,"community_area":40,"day_of_week":1,"hour":10,"week_start":"2026-06-01"},{"district":18,"community_area":8,"day_of_week":4,"hour":23,"week_start":"2026-06-01"}]}'

run_get "01_health" "/health"
run_get "02_nodes" "/nodes"
run_get "03_model_info" "/model/info"
run_post "04_predict_1" "/predict" "$PREDICT_PAYLOAD"
run_post "05_predict_2_cache" "/predict" "$PREDICT_PAYLOAD"
run_post "06_recommendations_top" "/recommendations/top" "$TOP_PAYLOAD"
run_get "07_predictions_history" "/predictions/history?limit=10"
run_get "08_metrics" "/metrics"

cat > "$OUT_DIR/README_EVIDENCIAS.txt" <<TXT
Evidencias generadas automáticamente para PC4.

Orden sugerido para capturas:
1. docker compose up --build / docker ps
2. 01_health.json
3. 02_nodes.json
4. 04_predict_1.json y 05_predict_2_cache.json
5. 06_recommendations_top.json
6. 07_predictions_history.json
7. 08_metrics.json
8. logs de api y nodos ML
TXT

echo "Evidencias guardadas en $OUT_DIR"
