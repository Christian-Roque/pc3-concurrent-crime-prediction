#!/usr/bin/env sh
set -e

mkdir -p output models

go test ./...

go run ./cmd/prepare \
  -file data/dataset.csv \
  -workers 8 \
  -batch-size 20000 \
  -min-year 2022 \
  -train-until 2025 \
  -negative-ratio 0 \
  -target-before 2026-06-01 \
  -out output

go run ./cmd/train \
  -train output/ml_observations_train.csv.gz \
  -test output/ml_observations_test.csv.gz \
  -metadata output/ml_observations_metadata.json \
  -workers 8 \
  -epochs 80 \
  -lambda 0.0005 \
  -out output \
  -model models/model.json
