#!/usr/bin/env python3
"""
Genera un reporte HTML y gráficos SVG a partir de las evidencias de rendimiento.
No requiere librerías externas: usa solo la biblioteca estándar de Python.

Uso:
  python scripts/generate_performance_report.py --evidence-dir evidencias/performance

Entradas esperadas, si existen:
  evidencias/performance/prepare/prepare_speedup_summary.csv
  evidencias/performance/prepare/runs/*/resource_usage.csv
  evidencias/performance/train/train_speedup_summary.csv
  evidencias/performance/train/runs/*/resource_usage.csv
  evidencias/performance/docker/docker_stats_samples.csv

Salidas:
  evidencias/performance/performance_report.html
  evidencias/performance/charts/*.svg
"""
from __future__ import annotations

import argparse
import csv
import html
import math
import os
from pathlib import Path
from typing import Dict, Iterable, List, Optional, Tuple


def read_csv(path: Path) -> List[Dict[str, str]]:
    if not path.exists():
        return []
    with path.open("r", encoding="utf-8-sig", newline="") as f:
        return list(csv.DictReader(f))


def to_float(value: object) -> Optional[float]:
    if value is None:
        return None
    s = str(value).strip().replace("%", "")
    if not s or s.upper() == "NA":
        return None
    try:
        return float(s)
    except ValueError:
        return None


def parse_mem_to_mb(value: str) -> Optional[float]:
    if not value:
        return None
    # Docker stats puede devolver "56.2MiB / 7.6GiB". Tomamos el uso antes de "/".
    token = value.split("/")[0].strip().replace("B", "")
    units = [("Gi", 1024.0), ("Mi", 1.0), ("Ki", 1.0 / 1024.0), ("G", 1000.0), ("M", 1.0), ("K", 1.0 / 1000.0)]
    for suffix, mult in units:
        if token.endswith(suffix):
            return to_float(token[: -len(suffix)]) * mult if to_float(token[: -len(suffix)]) is not None else None
    return to_float(token)


def ensure_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)


def points(values: List[Optional[float]], width: int, height: int, pad: int) -> Tuple[str, float, float]:
    clean = [v for v in values if v is not None]
    if not clean:
        return "", 0.0, 0.0
    vmin = min(clean)
    vmax = max(clean)
    if math.isclose(vmin, vmax):
        vmin = 0.0
        vmax = max(1.0, vmax)
    pts = []
    n = max(1, len(values) - 1)
    for i, v in enumerate(values):
        if v is None:
            continue
        x = pad + (width - 2 * pad) * i / n
        y = height - pad - (height - 2 * pad) * (v - vmin) / (vmax - vmin)
        pts.append(f"{x:.1f},{y:.1f}")
    return " ".join(pts), vmin, vmax


def svg_line_chart(
    title: str,
    series: List[Tuple[str, List[Optional[float]]]],
    output: Path,
    y_label: str = "valor",
) -> None:
    width, height, pad = 900, 420, 55
    colors = ["#1f77b4", "#d62728", "#2ca02c", "#9467bd", "#ff7f0e"]
    all_values = [v for _, vals in series for v in vals if v is not None]
    if not all_values:
        output.write_text("<svg xmlns='http://www.w3.org/2000/svg'></svg>", encoding="utf-8")
        return
    vmin = min(all_values)
    vmax = max(all_values)
    if math.isclose(vmin, vmax):
        vmin = 0.0
        vmax = max(1.0, vmax)
    max_len = max((len(vals) for _, vals in series), default=1)
    n = max(1, max_len - 1)

    def xy(i: int, v: float) -> Tuple[float, float]:
        x = pad + (width - 2 * pad) * i / n
        y = height - pad - (height - 2 * pad) * (v - vmin) / (vmax - vmin)
        return x, y

    parts = [
        f"<svg xmlns='http://www.w3.org/2000/svg' width='{width}' height='{height}' viewBox='0 0 {width} {height}'>",
        "<style>text{font-family:Arial, sans-serif;font-size:13px}.title{font-size:18px;font-weight:bold}.axis{stroke:#333;stroke-width:1}.grid{stroke:#ddd;stroke-width:1}.line{fill:none;stroke-width:2.5}</style>",
        f"<text class='title' x='{width/2}' y='28' text-anchor='middle'>{html.escape(title)}</text>",
        f"<line class='axis' x1='{pad}' y1='{height-pad}' x2='{width-pad}' y2='{height-pad}'/>",
        f"<line class='axis' x1='{pad}' y1='{pad}' x2='{pad}' y2='{height-pad}'/>",
        f"<text x='12' y='{height/2}' transform='rotate(-90 12,{height/2})' text-anchor='middle'>{html.escape(y_label)}</text>",
    ]
    for k in range(5):
        y = pad + (height - 2 * pad) * k / 4
        value = vmax - (vmax - vmin) * k / 4
        parts.append(f"<line class='grid' x1='{pad}' y1='{y:.1f}' x2='{width-pad}' y2='{y:.1f}'/>")
        parts.append(f"<text x='{pad-8}' y='{y+4:.1f}' text-anchor='end'>{value:.2f}</text>")
    legend_x = pad + 10
    for idx, (name, vals) in enumerate(series):
        color = colors[idx % len(colors)]
        pts = []
        for i, v in enumerate(vals):
            if v is None:
                continue
            x, y = xy(i, v)
            pts.append(f"{x:.1f},{y:.1f}")
        if pts:
            parts.append(f"<polyline class='line' stroke='{color}' points='{' '.join(pts)}'/>")
        ly = 55 + idx * 20
        parts.append(f"<line x1='{legend_x}' y1='{ly}' x2='{legend_x+25}' y2='{ly}' stroke='{color}' stroke-width='3'/>")
        parts.append(f"<text x='{legend_x+32}' y='{ly+4}'>{html.escape(name)}</text>")
    parts.append("</svg>")
    output.write_text("\n".join(parts), encoding="utf-8")


def svg_bar_line_summary(title: str, rows: List[Dict[str, str]], output: Path, x_field: str = "workers") -> None:
    width, height, pad = 900, 460, 70
    workers = [str(r.get(x_field, "")) for r in rows]
    wall = [to_float(r.get("wall_seconds")) for r in rows]
    speed = [to_float(r.get("speedup")) for r in rows]
    max_wall = max([v for v in wall if v is not None] or [1.0])
    max_speed = max([v for v in speed if v is not None] or [1.0])
    n = max(1, len(rows))
    bar_w = (width - 2 * pad) / max(1, n) * 0.42
    parts = [
        f"<svg xmlns='http://www.w3.org/2000/svg' width='{width}' height='{height}' viewBox='0 0 {width} {height}'>",
        "<style>text{font-family:Arial, sans-serif;font-size:13px}.title{font-size:18px;font-weight:bold}.axis{stroke:#333;stroke-width:1}.grid{stroke:#ddd;stroke-width:1}.bar{fill:#1f77b4}.line{fill:none;stroke:#d62728;stroke-width:2.5}</style>",
        f"<text class='title' x='{width/2}' y='28' text-anchor='middle'>{html.escape(title)}</text>",
        f"<line class='axis' x1='{pad}' y1='{height-pad}' x2='{width-pad}' y2='{height-pad}'/>",
        f"<line class='axis' x1='{pad}' y1='{pad}' x2='{pad}' y2='{height-pad}'/>",
        f"<line class='axis' x1='{width-pad}' y1='{pad}' x2='{width-pad}' y2='{height-pad}'/>",
        f"<text x='{pad}' y='{height-20}' text-anchor='start'>Workers</text>",
        f"<text x='14' y='{height/2}' transform='rotate(-90 14,{height/2})' text-anchor='middle'>Tiempo wall_seconds</text>",
        f"<text x='{width-16}' y='{height/2}' transform='rotate(90 {width-16},{height/2})' text-anchor='middle'>Speedup</text>",
        "<rect x='665' y='48' width='16' height='12' class='bar'/><text x='688' y='59'>Tiempo</text>",
        "<line x1='760' y1='54' x2='790' y2='54' stroke='#d62728' stroke-width='3'/><text x='797' y='59'>Speedup</text>",
    ]
    speed_pts = []
    for i, label in enumerate(workers):
        cx = pad + (width - 2 * pad) * (i + 0.5) / n
        wt = wall[i] if i < len(wall) and wall[i] is not None else 0
        h = (height - 2 * pad) * wt / max_wall
        x = cx - bar_w / 2
        y = height - pad - h
        parts.append(f"<rect class='bar' x='{x:.1f}' y='{y:.1f}' width='{bar_w:.1f}' height='{h:.1f}'/>")
        parts.append(f"<text x='{cx:.1f}' y='{height-pad+20}' text-anchor='middle'>{html.escape(label)}</text>")
        parts.append(f"<text x='{cx:.1f}' y='{y-5:.1f}' text-anchor='middle'>{wt:.1f}s</text>")
        sp = speed[i] if i < len(speed) and speed[i] is not None else None
        if sp is not None:
            sy = height - pad - (height - 2 * pad) * sp / max_speed
            speed_pts.append(f"{cx:.1f},{sy:.1f}")
            parts.append(f"<circle cx='{cx:.1f}' cy='{sy:.1f}' r='4' fill='#d62728'/>")
            parts.append(f"<text x='{cx:.1f}' y='{sy-8:.1f}' text-anchor='middle'>{sp:.2f}x</text>")
    if speed_pts:
        parts.append(f"<polyline class='line' points='{' '.join(speed_pts)}'/>")
    parts.append("</svg>")
    output.write_text("\n".join(parts), encoding="utf-8")


def table_html(rows: List[Dict[str, str]], max_rows: int = 50) -> str:
    if not rows:
        return "<p>No se encontraron datos.</p>"
    fields = list(rows[0].keys())
    out = ["<table>", "<thead><tr>" + "".join(f"<th>{html.escape(f)}</th>" for f in fields) + "</tr></thead><tbody>"]
    for r in rows[:max_rows]:
        out.append("<tr>" + "".join(f"<td>{html.escape(str(r.get(f, '')))}</td>" for f in fields) + "</tr>")
    out.append("</tbody></table>")
    if len(rows) > max_rows:
        out.append(f"<p>Se muestran {max_rows} de {len(rows)} filas.</p>")
    return "\n".join(out)


def create_resource_charts(base: Path, charts: Path, section: str) -> List[Tuple[str, Path]]:
    result = []
    runs_dir = base / section / "runs"
    if not runs_dir.exists():
        return result
    for run in sorted(runs_dir.iterdir()):
        csv_path = run / "resource_usage.csv"
        rows = read_csv(csv_path)
        if not rows:
            continue
        cpu = [to_float(r.get("process_cpu_percent", r.get("cpu_total_percent"))) for r in rows]
        ram = [to_float(r.get("process_memory_mb")) for r in rows]
        out = charts / f"{section}_{run.name}_cpu_ram.svg"
        svg_line_chart(f"{section}: CPU del proceso y RAM - {run.name}", [("CPU proceso %", cpu), ("RAM proceso MB", ram)], out, "CPU % / RAM MB")
        result.append((f"{section} {run.name}", out))
    return result


def create_docker_chart(base: Path, charts: Path) -> Optional[Path]:
    rows = read_csv(base / "docker" / "docker_stats_samples.csv")
    if not rows:
        return None
    names = sorted(set(r.get("name", "") for r in rows if r.get("name")))
    series = []
    for name in names:
        vals = [to_float(r.get("cpu_percent")) for r in rows if r.get("name") == name]
        series.append((name, vals))
    out = charts / "docker_cpu_percent.svg"
    svg_line_chart("Docker stats: CPU por contenedor", series, out, "CPU %")
    return out


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--evidence-dir", default="evidencias/performance", help="Directorio base de evidencias de performance")
    args = parser.parse_args()

    base = Path(args.evidence_dir)
    charts = base / "charts"
    ensure_dir(charts)

    prepare_rows = read_csv(base / "prepare" / "prepare_speedup_summary.csv")
    train_rows = read_csv(base / "train" / "train_speedup_summary.csv")
    docker_rows = read_csv(base / "docker" / "docker_stats_samples.csv")

    chart_refs: List[Tuple[str, Path]] = []
    if prepare_rows:
        p = charts / "prepare_speedup.svg"
        svg_bar_line_summary("Speedup de carga, limpieza y preparación", prepare_rows, p)
        chart_refs.append(("Speedup prepare", p))
    if train_rows:
        p = charts / "train_speedup.svg"
        svg_bar_line_summary("Speedup de entrenamiento", train_rows, p)
        chart_refs.append(("Speedup train", p))
    chart_refs.extend(create_resource_charts(base, charts, "prepare"))
    chart_refs.extend(create_resource_charts(base, charts, "train"))
    docker_chart = create_docker_chart(base, charts)
    if docker_chart:
        chart_refs.append(("Docker CPU", docker_chart))

    html_parts = [
        "<!doctype html><html lang='es'><head><meta charset='utf-8'>",
        "<title>Reporte de rendimiento y recursos PC4</title>",
        "<style>body{font-family:Arial,sans-serif;margin:28px;line-height:1.4}table{border-collapse:collapse;width:100%;font-size:12px;margin:12px 0}th,td{border:1px solid #ccc;padding:5px;text-align:left}th{background:#f0f0f0}img{max-width:100%;border:1px solid #ddd;margin:10px 0 25px}code{background:#f5f5f5;padding:2px 4px}</style>",
        "</head><body>",
        "<h1>Reporte automático de rendimiento y recursos de cómputo</h1>",
        "<p>Este reporte se genera a partir de archivos CSV producidos por los scripts de benchmark. No requiere capturas manuales del Administrador de tareas.</p>",
        "<h2>1. Speedup de carga, limpieza y preparación</h2>",
        table_html(prepare_rows),
        "<h2>2. Speedup de entrenamiento</h2>",
        table_html(train_rows),
        "<h2>3. Docker stats de la arquitectura distribuida</h2>",
        table_html(docker_rows[:80] if docker_rows else []),
        "<h2>4. Gráficos generados</h2>",
    ]
    for title, path in chart_refs:
        rel = os.path.relpath(path, base)
        html_parts.append(f"<h3>{html.escape(title)}</h3><img src='{html.escape(rel)}' alt='{html.escape(title)}'>")
    html_parts.append("<h2>5. Uso en el informe</h2>")
    html_parts.append("<p>Para el informe, usar las tablas de resumen CSV y los gráficos SVG/HTML generados. Si se desea, se puede tomar captura del HTML final, pero ya no es necesario estar pendiente del Administrador de tareas durante la ejecución.</p>")
    html_parts.append("</body></html>")

    out_html = base / "performance_report.html"
    out_html.write_text("\n".join(html_parts), encoding="utf-8")
    print(f"Reporte generado: {out_html}")
    print(f"Graficos generados en: {charts}")


if __name__ == "__main__":
    main()
