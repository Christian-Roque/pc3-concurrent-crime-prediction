package ml

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"

	"pc3-seguridad-ciudadana/internal/crime"
)

const (
	maxDistrict      = 25
	maxCommunityArea = 77
)

// EnhancedFeatureNames contiene solo el modelo final usado en PC3.
// Se retira la evaluación con modelo base para mantener el entregable enfocado.
var EnhancedFeatureNames = buildEnhancedFeatureNames()

// FeatureNames se mantiene como alias del set final para exportaciones y reportes.
var FeatureNames = EnhancedFeatureNames

type zoneKey struct {
	District      int
	CommunityArea int
}

type zoneWeekKey struct {
	WeekStart     string
	District      int
	CommunityArea int
}

type districtWeekKey struct {
	WeekStart string
	District  int
}

type dowHourWeekKey struct {
	WeekStart string
	DayOfWeek int
	Hour      int
}

type featureContext struct {
	ZoneTotals     map[zoneWeekKey]crime.AggregateValue
	DistrictTotals map[districtWeekKey]crime.AggregateValue
	DowHourTotals  map[dowHourWeekKey]crime.AggregateValue
}

func buildEnhancedFeatureNames() []string {
	names := []string{
		"district_norm",
		"community_area_norm",
		"hour_sin",
		"hour_cos",
		"day_sin",
		"day_cos",
		"month_sin",
		"month_cos",
		"is_weekend",
		"is_night",
		"is_morning",
		"is_afternoon",
		"is_evening",
		"is_friday_saturday_night",
	}
	for i := 1; i <= maxDistrict; i++ {
		names = append(names, "district_onehot_"+twoDigits(i))
	}
	for i := 1; i <= maxCommunityArea; i++ {
		names = append(names, "community_area_onehot_"+twoDigits(i))
	}
	names = append(names,
		"log_relevant_last_week",
		"log_relevant_avg_2w",
		"log_relevant_avg_4w",
		"log_relevant_avg_8w",
		"log_relevant_avg_12w",
		"trend_relevant_2w_12w",
		"trend_relevant_4w_12w",
		"burst_relevant_last_vs_avg4",
		"any_relevant_last_week",
		"log_violent_last_week",
		"log_violent_avg_4w",
		"log_property_last_week",
		"log_property_avg_4w",
		"log_weapons_last_week",
		"log_weapons_avg_12w",
		"log_severe_last_week",
		"log_severe_avg_12w",
		"log_minor_last_week",
		"log_minor_avg_2w",
		"log_minor_avg_4w",
		"log_minor_avg_12w",
		"trend_minor_4w_12w",
		"burst_minor_last_vs_avg4",
		"any_minor_last_week",
		"log_total_incidents_last_week",
		"log_total_incidents_avg_4w",
		"log_total_incidents_avg_12w",
		"trend_total_4w_12w",
		"relevant_ratio_4w",
		"minor_ratio_4w",
		"log_zone_relevant_last_week",
		"log_zone_relevant_avg_4w",
		"log_zone_minor_last_week",
		"log_zone_minor_avg_4w",
		"log_zone_total_last_week",
		"log_zone_total_avg_4w",
		"zone_relevant_ratio_4w",
		"log_district_relevant_last_week",
		"log_district_relevant_avg_4w",
		"log_district_total_avg_4w",
		"district_relevant_ratio_4w",
		"log_city_dow_hour_relevant_last_week",
		"log_city_dow_hour_relevant_avg_4w",
		"log_city_dow_hour_total_avg_4w",
		"city_dow_hour_relevant_ratio_4w",
	)
	return names
}

func twoDigits(v int) string {
	if v < 10 {
		return "0" + string(rune('0'+v))
	}
	tens := v / 10
	ones := v % 10
	return string([]rune{rune('0' + tens), rune('0' + ones)})
}

// BuildSamples convierte agregaciones semanales en observaciones ML.
// El target es ocurrencia futura: 1 si en la siguiente semana ocurre al menos un delito relevante
// para la misma zona, dia de semana y hora; 0 en caso contrario.
// Para PC3 se usan todos los negativos generados por la grilla zona-semana-dia-hora.
// La separacion train/test se hace segun la semana objetivo (t+1), no por la semana base,
// para evitar fuga temporal de etiquetas futuras.
func BuildSamples(aggregates map[crime.AggregateKey]crime.AggregateValue, trainUntilYear int, negativeRatio int, targetBefore string) ([]Sample, []Sample, TargetInfo, []float64, []float64) {
	useAllNegatives := negativeRatio <= 0
	zones, weeks := discoverZonesAndWeeks(aggregates)
	ctx := buildFeatureContext(aggregates)
	prevWeeks := buildPreviousWeeks(weeks, 12)
	info := TargetInfo{
		TargetName:              "ocurre_delito_relevante_siguiente_semana",
		Definition:              "1 si en la siguiente semana ocurre al menos un delito relevante de mayor impacto en la misma zona, dia de semana y hora; 0 si no ocurre.",
		NegativeSamplingRatio:   negativeRatio,
		UsesAllNegatives:        useAllNegatives,
		TargetBefore:            targetBefore,
		RelevantCrimeDefinition: "Solo delitos de mayor impacto activan el target: severos, violentos, armas y patrimoniales de alto impacto como burglary, motor vehicle theft y arson. Delitos menores o de menor impacto, como theft, criminal damage o criminal trespass, se usan solo como contexto predictivo.",
	}

	if len(zones) == 0 || len(weeks) < 2 {
		means := make([]float64, len(FeatureNames))
		stds := fillOnes(make([]float64, len(FeatureNames)))
		return nil, nil, info, means, stds
	}

	validTarget := func(targetWeek string) bool {
		if strings.TrimSpace(targetBefore) == "" {
			return true
		}
		targetTime, ok := crime.ParseWeekStart(targetWeek)
		limitTime, okLimit := crime.ParseWeekStart(targetBefore)
		if !ok || !okLimit {
			return true
		}
		return targetTime.Before(limitTime)
	}

	for _, ws := range weeks[:len(weeks)-1] {
		targetWeek := crime.AddWeeks(ws, 1)
		if !validTarget(targetWeek) {
			continue
		}
		for _, z := range zones {
			for dow := 0; dow < 7; dow++ {
				for hour := 0; hour < 24; hour++ {
					next := makeKey(targetWeek, z, dow, hour)
					info.CandidateCells++
					if aggregates[next].RelevantCount > 0 {
						info.PositiveCells++
					} else {
						info.NegativeCells++
					}
				}
			}
		}
	}

	targetNegatives := info.PositiveCells * negativeRatio
	keepAllNegatives := useAllNegatives || targetNegatives <= 0 || targetNegatives >= info.NegativeCells
	negModulo := 1
	if !keepAllNegatives && targetNegatives > 0 {
		negModulo = int(math.Ceil(float64(info.NegativeCells) / float64(targetNegatives)))
		if negModulo < 1 {
			negModulo = 1
		}
	}

	rawTrain := []Sample{}
	rawTest := []Sample{}
	for _, ws := range weeks[:len(weeks)-1] {
		weekTime, _ := crime.ParseWeekStart(ws)
		targetWeek := crime.AddWeeks(ws, 1)
		if !validTarget(targetWeek) {
			continue
		}
		targetTime, targetOK := crime.ParseWeekStart(targetWeek)
		for _, z := range zones {
			for dow := 0; dow < 7; dow++ {
				for hour := 0; hour < 24; hour++ {
					k := makeKey(ws, z, dow, hour)
					next := makeKey(targetWeek, z, dow, hour)
					current := aggregates[k]
					nextRelevant := aggregates[next].RelevantCount
					label := 0.0
					if nextRelevant > 0 {
						label = 1.0
					} else {
						if !keepAllNegatives && (stableHash(k)%uint32(negModulo) != 0) {
							continue
						}
						info.KeptNegativeCells++
					}
					features := rawFeatures(k, aggregates, ctx, prevWeeks)
					s := Sample{
						Features:      features,
						Label:         label,
						Year:          weekTime.Year(),
						WeekStart:     ws,
						TargetWeek:    targetWeek,
						District:      z.District,
						CommunityArea: z.CommunityArea,
						DayOfWeek:     dow,
						Hour:          hour,
						RelevantCount: current.RelevantCount,
						OtherCount:    current.OtherCount,
						TotalCount:    current.Count,
						NextRelevant:  nextRelevant,
					}
					if targetOK && targetTime.Year() <= trainUntilYear {
						rawTrain = append(rawTrain, s)
					} else {
						rawTest = append(rawTest, s)
					}
				}
			}
		}
	}

	totalRaw := len(rawTrain) + len(rawTest)
	trainTooSmall := totalRaw > 10 && len(rawTrain) < int(0.20*float64(totalRaw))
	testEmpty := totalRaw > 10 && len(rawTest) == 0
	if trainTooSmall || testEmpty {
		all := append(append([]Sample{}, rawTrain...), rawTest...)
		rawTrain, rawTest = splitByTargetWeek(all, 0.80)
	}

	means, stds := computeStandardizer(rawTrain, len(FeatureNames))
	standardizeSamples(rawTrain, means, stds)
	standardizeSamples(rawTest, means, stds)
	return rawTrain, rawTest, info, means, stds
}

func splitByTargetWeek(samples []Sample, trainFraction float64) ([]Sample, []Sample) {
	if len(samples) == 0 {
		return nil, nil
	}
	weekMap := map[string]bool{}
	for _, s := range samples {
		w := s.TargetWeek
		if w == "" {
			w = s.WeekStart
		}
		weekMap[w] = true
	}
	weeks := make([]string, 0, len(weekMap))
	for w := range weekMap {
		weeks = append(weeks, w)
	}
	sort.Strings(weeks)
	cutIdx := int(float64(len(weeks)) * trainFraction)
	if cutIdx < 1 {
		cutIdx = 1
	}
	if cutIdx >= len(weeks) {
		cutIdx = len(weeks) - 1
	}
	cutWeek := weeks[cutIdx]
	train := []Sample{}
	test := []Sample{}
	for _, s := range samples {
		w := s.TargetWeek
		if w == "" {
			w = s.WeekStart
		}
		if w < cutWeek {
			train = append(train, s)
		} else {
			test = append(test, s)
		}
	}
	return train, test
}

func discoverZonesAndWeeks(aggregates map[crime.AggregateKey]crime.AggregateValue) ([]zoneKey, []string) {
	zoneMap := map[zoneKey]bool{}
	weekMap := map[string]bool{}
	for k := range aggregates {
		zoneMap[zoneKey{District: k.District, CommunityArea: k.CommunityArea}] = true
		weekMap[k.WeekStart] = true
	}
	zones := make([]zoneKey, 0, len(zoneMap))
	for z := range zoneMap {
		zones = append(zones, z)
	}
	sort.Slice(zones, func(i, j int) bool {
		if zones[i].District != zones[j].District {
			return zones[i].District < zones[j].District
		}
		return zones[i].CommunityArea < zones[j].CommunityArea
	})
	weeks := make([]string, 0, len(weekMap))
	for w := range weekMap {
		weeks = append(weeks, w)
	}
	sort.Strings(weeks)
	return zones, weeks
}

func buildFeatureContext(aggregates map[crime.AggregateKey]crime.AggregateValue) featureContext {
	ctx := featureContext{
		ZoneTotals:     map[zoneWeekKey]crime.AggregateValue{},
		DistrictTotals: map[districtWeekKey]crime.AggregateValue{},
		DowHourTotals:  map[dowHourWeekKey]crime.AggregateValue{},
	}
	for k, v := range aggregates {
		zk := zoneWeekKey{WeekStart: k.WeekStart, District: k.District, CommunityArea: k.CommunityArea}
		ctx.ZoneTotals[zk] = addAggregate(ctx.ZoneTotals[zk], v)

		dk := districtWeekKey{WeekStart: k.WeekStart, District: k.District}
		ctx.DistrictTotals[dk] = addAggregate(ctx.DistrictTotals[dk], v)

		hk := dowHourWeekKey{WeekStart: k.WeekStart, DayOfWeek: k.DayOfWeek, Hour: k.Hour}
		ctx.DowHourTotals[hk] = addAggregate(ctx.DowHourTotals[hk], v)
	}
	return ctx
}

func addAggregate(a crime.AggregateValue, b crime.AggregateValue) crime.AggregateValue {
	a.Count += b.Count
	a.RelevantCount += b.RelevantCount
	a.ViolentCount += b.ViolentCount
	a.PropertyCount += b.PropertyCount
	a.WeaponsCount += b.WeaponsCount
	a.SevereCount += b.SevereCount
	a.OtherCount += b.OtherCount
	a.ArrestCount += b.ArrestCount
	a.DomesticCount += b.DomesticCount
	return a
}

func makeKey(weekStart string, z zoneKey, dow int, hour int) crime.AggregateKey {
	t, ok := crime.ParseWeekStart(weekStart)
	month := 1
	year := 0
	if ok {
		month = int(t.Month())
		year = t.Year()
	}
	return crime.AggregateKey{WeekStart: weekStart, Year: year, Month: month, District: z.District, CommunityArea: z.CommunityArea, DayOfWeek: dow, Hour: hour}
}

func rawFeatures(k crime.AggregateKey, aggregates map[crime.AggregateKey]crime.AggregateValue, ctx featureContext, prevWeeks map[string][]string) []float64 {
	isWeekend := boolFloat(k.DayOfWeek == int(time.Saturday) || k.DayOfWeek == int(time.Sunday))
	isNight := boolFloat(k.Hour >= 20 || k.Hour <= 5)
	isMorning := boolFloat(k.Hour >= 6 && k.Hour <= 11)
	isAfternoon := boolFloat(k.Hour >= 12 && k.Hour <= 17)
	isEvening := boolFloat(k.Hour >= 18 && k.Hour <= 23)
	isFridaySaturdayNight := boolFloat((k.DayOfWeek == int(time.Friday) || k.DayOfWeek == int(time.Saturday)) && (k.Hour >= 18 || k.Hour <= 3))

	relevantLastWeek := float64(aggregates[k].RelevantCount)
	relevantAvg2 := avgCount(k, aggregates, prevWeeks, 2, "relevant")
	relevantAvg4 := avgCount(k, aggregates, prevWeeks, 4, "relevant")
	relevantAvg8 := avgCount(k, aggregates, prevWeeks, 8, "relevant")
	relevantAvg12 := avgCount(k, aggregates, prevWeeks, 12, "relevant")
	relevantTrend2 := relevantAvg2 - relevantAvg12
	relevantTrend4 := relevantAvg4 - relevantAvg12
	relevantBurst := safeRatio(relevantLastWeek, relevantAvg4+0.01)

	violentLastWeek := float64(aggregates[k].ViolentCount)
	propertyLastWeek := float64(aggregates[k].PropertyCount)
	weaponsLastWeek := float64(aggregates[k].WeaponsCount)
	severeLastWeek := float64(aggregates[k].SevereCount)
	violentAvg4 := avgCount(k, aggregates, prevWeeks, 4, "violent")
	propertyAvg4 := avgCount(k, aggregates, prevWeeks, 4, "property")
	weaponsAvg12 := avgCount(k, aggregates, prevWeeks, 12, "weapons")
	severeAvg12 := avgCount(k, aggregates, prevWeeks, 12, "severe")

	minorLastWeek := float64(aggregates[k].OtherCount)
	minorAvg2 := avgCount(k, aggregates, prevWeeks, 2, "minor")
	minorAvg4 := avgCount(k, aggregates, prevWeeks, 4, "minor")
	minorAvg12 := avgCount(k, aggregates, prevWeeks, 12, "minor")
	minorTrend := minorAvg4 - minorAvg12
	minorBurst := safeRatio(minorLastWeek, minorAvg4+0.01)

	totalLastWeek := float64(aggregates[k].Count)
	totalAvg4 := avgCount(k, aggregates, prevWeeks, 4, "total")
	totalAvg12 := avgCount(k, aggregates, prevWeeks, 12, "total")
	totalTrend := totalAvg4 - totalAvg12
	relevantRatio4 := safeRatio(relevantAvg4, totalAvg4)
	minorRatio4 := safeRatio(minorAvg4, totalAvg4)

	zoneRelevantLast := contextCount(k, ctx, prevWeeks, 1, "zone", "relevant")
	zoneRelevantAvg4 := contextCount(k, ctx, prevWeeks, 4, "zone", "relevant")
	zoneMinorLast := contextCount(k, ctx, prevWeeks, 1, "zone", "minor")
	zoneMinorAvg4 := contextCount(k, ctx, prevWeeks, 4, "zone", "minor")
	zoneTotalLast := contextCount(k, ctx, prevWeeks, 1, "zone", "total")
	zoneTotalAvg4 := contextCount(k, ctx, prevWeeks, 4, "zone", "total")
	zoneRelevantRatio4 := safeRatio(zoneRelevantAvg4, zoneTotalAvg4)

	districtRelevantLast := contextCount(k, ctx, prevWeeks, 1, "district", "relevant")
	districtRelevantAvg4 := contextCount(k, ctx, prevWeeks, 4, "district", "relevant")
	districtTotalAvg4 := contextCount(k, ctx, prevWeeks, 4, "district", "total")
	districtRelevantRatio4 := safeRatio(districtRelevantAvg4, districtTotalAvg4)

	cityDowHourRelevantLast := contextCount(k, ctx, prevWeeks, 1, "dowhour", "relevant")
	cityDowHourRelevantAvg4 := contextCount(k, ctx, prevWeeks, 4, "dowhour", "relevant")
	cityDowHourTotalAvg4 := contextCount(k, ctx, prevWeeks, 4, "dowhour", "total")
	cityDowHourRelevantRatio4 := safeRatio(cityDowHourRelevantAvg4, cityDowHourTotalAvg4)

	out := []float64{
		float64(k.District) / 25.0,
		float64(k.CommunityArea) / 77.0,
		math.Sin(2.0 * math.Pi * float64(k.Hour) / 24.0),
		math.Cos(2.0 * math.Pi * float64(k.Hour) / 24.0),
		math.Sin(2.0 * math.Pi * float64(k.DayOfWeek) / 7.0),
		math.Cos(2.0 * math.Pi * float64(k.DayOfWeek) / 7.0),
		math.Sin(2.0 * math.Pi * float64(k.Month) / 12.0),
		math.Cos(2.0 * math.Pi * float64(k.Month) / 12.0),
		isWeekend,
		isNight,
		isMorning,
		isAfternoon,
		isEvening,
		isFridaySaturdayNight,
	}
	out = append(out, oneHot(k.District, maxDistrict)...)
	out = append(out, oneHot(k.CommunityArea, maxCommunityArea)...)
	out = append(out,
		math.Log1p(relevantLastWeek),
		math.Log1p(relevantAvg2),
		math.Log1p(relevantAvg4),
		math.Log1p(relevantAvg8),
		math.Log1p(relevantAvg12),
		relevantTrend2,
		relevantTrend4,
		relevantBurst,
		boolFloat(relevantLastWeek > 0),
		math.Log1p(violentLastWeek),
		math.Log1p(violentAvg4),
		math.Log1p(propertyLastWeek),
		math.Log1p(propertyAvg4),
		math.Log1p(weaponsLastWeek),
		math.Log1p(weaponsAvg12),
		math.Log1p(severeLastWeek),
		math.Log1p(severeAvg12),
		math.Log1p(minorLastWeek),
		math.Log1p(minorAvg2),
		math.Log1p(minorAvg4),
		math.Log1p(minorAvg12),
		minorTrend,
		minorBurst,
		boolFloat(minorLastWeek > 0),
		math.Log1p(totalLastWeek),
		math.Log1p(totalAvg4),
		math.Log1p(totalAvg12),
		totalTrend,
		relevantRatio4,
		minorRatio4,
		math.Log1p(zoneRelevantLast),
		math.Log1p(zoneRelevantAvg4),
		math.Log1p(zoneMinorLast),
		math.Log1p(zoneMinorAvg4),
		math.Log1p(zoneTotalLast),
		math.Log1p(zoneTotalAvg4),
		zoneRelevantRatio4,
		math.Log1p(districtRelevantLast),
		math.Log1p(districtRelevantAvg4),
		math.Log1p(districtTotalAvg4),
		districtRelevantRatio4,
		math.Log1p(cityDowHourRelevantLast),
		math.Log1p(cityDowHourRelevantAvg4),
		math.Log1p(cityDowHourTotalAvg4),
		cityDowHourRelevantRatio4,
	)
	return out
}

func oneHot(value int, size int) []float64 {
	out := make([]float64, size)
	if value >= 1 && value <= size {
		out[value-1] = 1.0
	}
	return out
}

func boolFloat(ok bool) float64 {
	if ok {
		return 1.0
	}
	return 0.0
}

func buildPreviousWeeks(weeks []string, maxBack int) map[string][]string {
	out := make(map[string][]string, len(weeks))
	weekSet := map[string]bool{}
	for _, w := range weeks {
		weekSet[w] = true
	}
	for _, w := range weeks {
		prev := make([]string, maxBack)
		for i := 0; i < maxBack; i++ {
			candidate := crime.AddWeeks(w, -i)
			if weekSet[candidate] {
				prev[i] = candidate
			}
		}
		out[w] = prev
	}
	return out
}

func weekBack(k crime.AggregateKey, prevWeeks map[string][]string, offset int) string {
	if offset < 0 {
		return ""
	}
	prev := prevWeeks[k.WeekStart]
	if offset >= len(prev) {
		return ""
	}
	return prev[offset]
}

func avgCount(k crime.AggregateKey, aggregates map[crime.AggregateKey]crime.AggregateValue, prevWeeks map[string][]string, weeksBack int, group string) float64 {
	if weeksBack <= 0 {
		return 0
	}
	total := 0.0
	for i := 0; i < weeksBack; i++ {
		wk := weekBack(k, prevWeeks, i)
		if wk == "" {
			continue
		}
		prev := k
		prev.WeekStart = wk
		if t, ok := crime.ParseWeekStart(wk); ok {
			prev.Year = t.Year()
			prev.Month = int(t.Month())
		}
		total += valueForGroup(aggregates[prev], group)
	}
	return total / float64(weeksBack)
}

func contextCount(k crime.AggregateKey, ctx featureContext, prevWeeks map[string][]string, weeksBack int, level string, group string) float64 {
	if weeksBack <= 0 {
		return 0
	}
	total := 0.0
	for i := 0; i < weeksBack; i++ {
		wk := weekBack(k, prevWeeks, i)
		if wk == "" {
			continue
		}
		var v crime.AggregateValue
		switch level {
		case "zone":
			v = ctx.ZoneTotals[zoneWeekKey{WeekStart: wk, District: k.District, CommunityArea: k.CommunityArea}]
		case "district":
			v = ctx.DistrictTotals[districtWeekKey{WeekStart: wk, District: k.District}]
		case "dowhour":
			v = ctx.DowHourTotals[dowHourWeekKey{WeekStart: wk, DayOfWeek: k.DayOfWeek, Hour: k.Hour}]
		}
		total += valueForGroup(v, group)
	}
	return total / float64(weeksBack)
}

func valueForGroup(v crime.AggregateValue, group string) float64 {
	switch group {
	case "violent":
		return float64(v.ViolentCount)
	case "property":
		return float64(v.PropertyCount)
	case "weapons":
		return float64(v.WeaponsCount)
	case "severe":
		return float64(v.SevereCount)
	case "minor":
		return float64(v.OtherCount)
	case "total":
		return float64(v.Count)
	default:
		return float64(v.RelevantCount)
	}
}

func stableHash(k crime.AggregateKey) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(k.WeekStart))
	_, _ = h.Write([]byte{byte(k.District), byte(k.CommunityArea), byte(k.DayOfWeek), byte(k.Hour)})
	return h.Sum32()
}

func computeStandardizer(samples []Sample, nFeatures int) ([]float64, []float64) {
	means := make([]float64, nFeatures)
	stds := make([]float64, nFeatures)
	if len(samples) == 0 {
		return means, fillOnes(stds)
	}
	for _, s := range samples {
		for j := 0; j < nFeatures && j < len(s.Features); j++ {
			means[j] += s.Features[j]
		}
	}
	for j := range means {
		means[j] /= float64(len(samples))
	}
	for _, s := range samples {
		for j := 0; j < nFeatures && j < len(s.Features); j++ {
			diff := s.Features[j] - means[j]
			stds[j] += diff * diff
		}
	}
	for j := range stds {
		stds[j] = math.Sqrt(stds[j] / float64(len(samples)))
		if stds[j] < 1e-9 {
			stds[j] = 1.0
		}
	}
	return means, stds
}

func fillOnes(values []float64) []float64 {
	for i := range values {
		values[i] = 1.0
	}
	return values
}

func standardizeSamples(samples []Sample, means []float64, stds []float64) {
	for i := range samples {
		for j := range samples[i].Features {
			std := 1.0
			if j < len(stds) && stds[j] != 0 {
				std = stds[j]
			}
			mean := 0.0
			if j < len(means) {
				mean = means[j]
			}
			samples[i].Features[j] = (samples[i].Features[j] - mean) / std
		}
	}
}

func StandardizeRawFeatures(raw []float64, means []float64, stds []float64) []float64 {
	out := make([]float64, len(raw))
	for i := range raw {
		std := 1.0
		if i < len(stds) && stds[i] != 0 {
			std = stds[i]
		}
		mean := 0.0
		if i < len(means) {
			mean = means[i]
		}
		out[i] = (raw[i] - mean) / std
	}
	return out
}

func safeRatio(a float64, b float64) float64 {
	if math.Abs(b) < 1e-12 {
		return 0
	}
	return a / b
}

// SelectFeatureSubset se conserva por compatibilidad con versiones previas del proyecto.
func SelectFeatureSubset(samples []Sample, indices []int) []Sample {
	out := make([]Sample, len(samples))
	for i, s := range samples {
		out[i] = s
		out[i].Features = make([]float64, 0, len(indices))
		for _, idx := range indices {
			if idx >= 0 && idx < len(s.Features) {
				out[i].Features = append(out[i].Features, s.Features[idx])
			}
		}
	}
	return out
}

func SelectFloatSubset(values []float64, indices []int) []float64 {
	out := make([]float64, 0, len(indices))
	for _, idx := range indices {
		if idx >= 0 && idx < len(values) {
			out = append(out, values[idx])
		}
	}
	return out
}
