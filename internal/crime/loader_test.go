package crime

import (
	"os"
	"testing"
)

func TestClassifyCrimeTypeTargetExcludesMinorCrimes(t *testing.T) {
	minorTypes := []string{"THEFT", "CRIMINAL DAMAGE", "CRIMINAL TRESPASS"}
	for _, typ := range minorTypes {
		cls := ClassifyCrimeType(typ)
		if cls.Relevant {
			t.Fatalf("%s no debe activar el target; debe quedar como contexto", typ)
		}
	}

	relevantTypes := []string{"ROBBERY", "BATTERY", "WEAPONS VIOLATION", "BURGLARY", "MOTOR VEHICLE THEFT", "HOMICIDE"}
	for _, typ := range relevantTypes {
		cls := ClassifyCrimeType(typ)
		if !cls.Relevant {
			t.Fatalf("%s debe activar el target de delito relevante", typ)
		}
	}
}

func TestConcurrentLoaderProducesExpectedStats(t *testing.T) {
	csv := `Date,Primary Type,District,Community Area,Arrest,Domestic,Year
01/01/2026 10:00:00 AM,ROBBERY,1,1,false,false,2026
01/01/2026 10:30:00 AM,THEFT,1,1,false,false,2026
01/02/2026 11:00:00 PM,WEAPONS VIOLATION,2,2,true,false,2026
12/31/2021 10:00:00 AM,ROBBERY,1,1,false,false,2021
bad date,ROBBERY,1,1,false,false,2026
`
	path := t.TempDir() + "/sample.csv"
	if err := os.WriteFile(path, []byte(csv), 0644); err != nil {
		t.Fatal(err)
	}
	res, err := LoadAndAggregateCSV(path, 2, 2, 2022, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats.ValidRows != 3 || res.Stats.InvalidRows != 1 || res.Stats.FilteredRows != 1 || res.Stats.RelevantRows != 2 {
		t.Fatalf("estadisticas inesperadas: %+v", res.Stats)
	}
	if len(res.Aggregates) != 2 {
		t.Fatalf("se esperaban 2 agregados, se obtuvo %d", len(res.Aggregates))
	}
}
