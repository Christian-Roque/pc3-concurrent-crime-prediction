package ml

import (
	"testing"

	"pc3-seguridad-ciudadana/internal/crime"
)

func aggKey(week string, year int, month int, district int, area int, dow int, hour int) crime.AggregateKey {
	return crime.AggregateKey{WeekStart: week, Year: year, Month: month, District: district, CommunityArea: area, DayOfWeek: dow, Hour: hour}
}

func TestBuildSamplesSplitUsesTargetWeek(t *testing.T) {
	aggs := map[crime.AggregateKey]crime.AggregateValue{}
	zoneDistrict, zoneArea := 1, 1
	// Semanas base: 2025-12-15, 2025-12-22 y 2025-12-29.
	// La ultima tiene target en 2026-01-05 y, por tanto, debe ir a test con trainUntil=2025.
	aggs[aggKey("2025-12-15", 2025, 12, zoneDistrict, zoneArea, 1, 10)] = crime.AggregateValue{RelevantCount: 1, Count: 1}
	aggs[aggKey("2025-12-22", 2025, 12, zoneDistrict, zoneArea, 1, 10)] = crime.AggregateValue{RelevantCount: 1, Count: 1}
	aggs[aggKey("2025-12-29", 2025, 12, zoneDistrict, zoneArea, 1, 10)] = crime.AggregateValue{RelevantCount: 1, Count: 1}
	aggs[aggKey("2026-01-05", 2026, 1, zoneDistrict, zoneArea, 1, 10)] = crime.AggregateValue{RelevantCount: 1, Count: 1}

	train, test, _, _, _ := BuildSamples(aggs, 2025, 0, "")
	if len(train) == 0 || len(test) == 0 {
		t.Fatalf("se esperaban muestras en train y test; train=%d test=%d", len(train), len(test))
	}
	for _, s := range train {
		if s.TargetWeek >= "2026-01-01" {
			t.Fatalf("fuga temporal: muestra con target_week=%s quedo en train", s.TargetWeek)
		}
	}
	for _, s := range test {
		if s.TargetWeek < "2026-01-01" {
			t.Fatalf("muestra con target_week=%s debio quedar en train", s.TargetWeek)
		}
	}
}

func TestBuildSamplesTargetBeforeExcludesJuneTargets(t *testing.T) {
	aggs := map[crime.AggregateKey]crime.AggregateValue{}
	zoneDistrict, zoneArea := 1, 1
	aggs[aggKey("2026-05-18", 2026, 5, zoneDistrict, zoneArea, 1, 10)] = crime.AggregateValue{RelevantCount: 1, Count: 1}
	aggs[aggKey("2026-05-25", 2026, 5, zoneDistrict, zoneArea, 1, 10)] = crime.AggregateValue{RelevantCount: 1, Count: 1}
	aggs[aggKey("2026-06-01", 2026, 6, zoneDistrict, zoneArea, 1, 10)] = crime.AggregateValue{RelevantCount: 1, Count: 1}

	train, test, info, _, _ := BuildSamples(aggs, 2025, 0, "2026-06-01")
	all := append(train, test...)
	if len(all) == 0 {
		t.Fatalf("se esperaban muestras antes del corte target-before")
	}
	for _, s := range all {
		if s.TargetWeek >= "2026-06-01" {
			t.Fatalf("target_week=%s debio ser excluido por target-before", s.TargetWeek)
		}
	}
	if info.TargetBefore != "2026-06-01" {
		t.Fatalf("TargetBefore no fue registrado correctamente: %q", info.TargetBefore)
	}
}
