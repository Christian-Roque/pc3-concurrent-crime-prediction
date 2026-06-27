# PC4 - Fase 6: cierre técnico, evidencias y validación de entrega

## Objetivo de la fase

Esta fase consolida lo desarrollado para PC4 y agrega recursos de apoyo para la entrega: scripts de prueba, guía de evidencias, checklist técnico y colección de solicitudes. No modifica la lógica central del modelo, sino que facilita demostrar el funcionamiento end to end de la arquitectura distribuida.

## Componentes agregados

```text
scripts/pc4_smoke_test.ps1        Pruebas rápidas para Windows/PowerShell.
scripts/pc4_smoke_test.sh         Pruebas rápidas para Linux/macOS/Git Bash.
docs/pc4/PLAN_PRUEBAS_PC4.md      Plan de pruebas funcionales de PC4.
docs/pc4/CHECKLIST_ENTREGA_PC4.md Checklist final de entrega.
docs/pc4/POSTMAN_PC4_COLLECTION.json Colección para Postman.
```

## Flujo de validación recomendado

1. Levantar la arquitectura:

```powershell
docker compose up --build
```

2. Validar que estén activos los servicios:

```powershell
docker ps
```

3. Ejecutar pruebas automáticas de evidencia:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\pc4_smoke_test.ps1
```

Nota: el script de Windows usa `Invoke-WebRequest` para enviar JSON correctamente y evitar problemas de comillas con `curl.exe` en PowerShell.

4. Revisar los archivos generados en la carpeta `evidencias/`.

5. Tomar capturas para el informe PC4.

## Qué evidencia esta fase

- API REST activa.
- Bitácora de nodos ML disponible.
- Modelo cargado por la API.
- Predicción individual vía API -> TCP -> nodo ML.
- Cache Redis en consultas repetidas.
- Endpoint distribuido `/recommendations/top` usando varios nodos.
- Historial de predicciones en MongoDB.
- Métricas de API, cache, almacenamiento y nodos.

## Nota de alcance

Los scripts de esta fase son recursos de apoyo para pruebas y evidencia. No reemplazan el informe ni el video. Su propósito es reducir errores manuales al momento de demostrar la solución.
