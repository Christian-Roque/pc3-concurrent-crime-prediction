package crime

import (
	"encoding/csv"
	"os"
	"sort"
	"strconv"
)

func SaveAggregatesCSV(path string, aggregates map[AggregateKey]AggregateValue) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	header := []string{
		"week_start", "year", "month", "district", "community_area", "day_of_week", "hour",
		"total_count", "relevant_count", "violent_count", "property_count", "weapons_count", "severe_count", "other_count",
		"arrest_count", "domestic_count",
	}
	if err := w.Write(header); err != nil {
		return err
	}
	keys := make([]AggregateKey, 0, len(aggregates))
	for k := range aggregates {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.WeekStart != b.WeekStart {
			return a.WeekStart < b.WeekStart
		}
		if a.District != b.District {
			return a.District < b.District
		}
		if a.CommunityArea != b.CommunityArea {
			return a.CommunityArea < b.CommunityArea
		}
		if a.DayOfWeek != b.DayOfWeek {
			return a.DayOfWeek < b.DayOfWeek
		}
		return a.Hour < b.Hour
	})
	for _, k := range keys {
		v := aggregates[k]
		record := []string{
			k.WeekStart, strconv.Itoa(k.Year), strconv.Itoa(k.Month), strconv.Itoa(k.District), strconv.Itoa(k.CommunityArea),
			strconv.Itoa(k.DayOfWeek), strconv.Itoa(k.Hour), strconv.Itoa(v.Count), strconv.Itoa(v.RelevantCount), strconv.Itoa(v.ViolentCount),
			strconv.Itoa(v.PropertyCount), strconv.Itoa(v.WeaponsCount), strconv.Itoa(v.SevereCount), strconv.Itoa(v.OtherCount),
			strconv.Itoa(v.ArrestCount), strconv.Itoa(v.DomesticCount),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return w.Error()
}
