package pdrest

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeProgressionRequest_FromKeywordValues(t *testing.T) {
	exp := 10
	technologyPoints := 3
	ancientTechnologyPoints := 4

	request, err := normalizeProgressionRequest(nil, &exp, &technologyPoints, &ancientTechnologyPoints, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if request.EXP != exp || request.TechnologyPoints != technologyPoints {
		t.Fatalf("unexpected request: %+v", request)
	}
	if request.AncientTechnologyPoints != ancientTechnologyPoints || request.Relics != nil {
		t.Fatalf("unexpected zero-value fields: %+v", request)
	}
}

func TestNormalizeProgressionRequest_RejectsNonPositiveKeywordValues(t *testing.T) {
	exp := 0

	_, err := normalizeProgressionRequest(nil, &exp, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for non-positive exp")
	}
	if err.Error() != "exp must be a positive integer" {
		t.Fatalf("unexpected error: %v", err)
	}

	technologyPoints := 0
	if _, err := normalizeProgressionRequest(nil, nil, &technologyPoints, nil, nil); err == nil {
		t.Fatal("expected error for non-positive technology points")
	}

	ancientTechnologyPoints := 0
	if _, err := normalizeProgressionRequest(nil, nil, nil, &ancientTechnologyPoints, nil); err == nil {
		t.Fatal("expected error for non-positive ancient technology points")
	}
}

func TestNormalizeProgressionRequest_FromRelics(t *testing.T) {
	relics := map[string]int{"CapturePower": 5, "MoveSpeed": 2}

	request, err := normalizeProgressionRequest(nil, nil, nil, nil, relics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(request.Relics) != 2 || request.Relics["CapturePower"] != 5 {
		t.Fatalf("unexpected request: %+v", request)
	}
}

func TestNormalizeProgressionRequest_RejectsInvalidRelics(t *testing.T) {
	empty := map[string]int{}
	if _, err := normalizeProgressionRequest(nil, nil, nil, nil, empty); err == nil {
		t.Fatal("expected error for empty relics")
	}

	nonPositive := map[string]int{"CapturePower": 0}
	if _, err := normalizeProgressionRequest(nil, nil, nil, nil, nonPositive); err == nil {
		t.Fatal("expected error for non-positive relic amount")
	}

	request := &GiveProgressionRequest{Relics: map[string]int{"JumpPower": -1}}
	if _, err := normalizeProgressionRequest(request, nil, nil, nil, nil); err == nil {
		t.Fatal("expected error for invalid relics in request")
	}
}

func TestNormalizeProgressionRequest_FromRequest(t *testing.T) {
	request := &GiveProgressionRequest{EXP: 100, TechnologyPoints: 5}

	result, err := normalizeProgressionRequest(request, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != request {
		t.Fatalf("unexpected request: %+v", result)
	}
}

func TestNormalizeProgressionRequest_RejectsEmptyRequest(t *testing.T) {
	_, err := normalizeProgressionRequest(&GiveProgressionRequest{}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty request")
	}
}

func TestNormalizeProgressionRequest_RejectsNegativeRequestValues(t *testing.T) {
	negativeExp := &GiveProgressionRequest{EXP: -5}
	if _, err := normalizeProgressionRequest(negativeExp, nil, nil, nil, nil); err == nil {
		t.Fatal("expected error for negative exp")
	}

	negativePoints := &GiveProgressionRequest{TechnologyPoints: -1}
	if _, err := normalizeProgressionRequest(negativePoints, nil, nil, nil, nil); err == nil {
		t.Fatal("expected error for negative technology points")
	}

	negativeAncientPoints := &GiveProgressionRequest{AncientTechnologyPoints: -1}
	if _, err := normalizeProgressionRequest(negativeAncientPoints, nil, nil, nil, nil); err == nil {
		t.Fatal("expected error for negative ancient technology points")
	}
}

func TestNormalizeProgressionRequest_RejectsEmptyRelicsInRequest(t *testing.T) {
	request := &GiveProgressionRequest{Relics: map[string]int{}}
	if _, err := normalizeProgressionRequest(request, nil, nil, nil, nil); err == nil {
		t.Fatal("expected error for empty relics in request")
	}
}

func TestNormalizeProgressionRequest_RejectsRequestWithKeywordValues(t *testing.T) {
	request := &GiveProgressionRequest{EXP: 1}
	exp := 2

	_, err := normalizeProgressionRequest(request, &exp, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error when passing both request and keyword values")
	}
}

func TestNormalizeProgressionRequest_RejectsNoValues(t *testing.T) {
	_, err := normalizeProgressionRequest(nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error when no progression field is provided")
	}
}

func TestNormalizePalEggInputs_RejectsFractionalLevel(t *testing.T) {
	_, err := normalizePalEggInputs([]PalEggInput{map[string]any{"EggID": "PalEgg_Fire_01", "PalID": "Foxparks", "Level": 12.5}})
	if err == nil {
		t.Fatal("expected error for fractional level")
	}
}

func TestNormalizeTechnologyInputs_RejectsEmptyStrings(t *testing.T) {
	_, err := normalizeTechnologyInputs([]TechnologyInput{""})
	if err == nil {
		t.Fatal("expected error for empty technology")
	}
}

func assertCounts(t *testing.T, got, want map[string]int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected result: got %v want %v", got, want)
	}
	for key, count := range want {
		if got[key] != count {
			t.Fatalf("unexpected result: got %v want %v", got, want)
		}
	}
}

func TestNormalizeItemInputs(t *testing.T) {
	tests := []struct {
		name    string
		inputs  []ItemInput
		want    map[string]int
		wantErr bool
	}{
		{"single string", []ItemInput{"Money"}, map[string]int{"Money": 1}, false},
		{"single sequence", []ItemInput{[]string{"Money", "Wood"}}, map[string]int{"Money": 1, "Wood": 1}, false},
		{"object", []ItemInput{GiveItem{ItemID: "Money", Count: 5}}, map[string]int{"Money": 5}, false},
		{"map", []ItemInput{map[string]any{"ItemID": "Money", "Count": 5}}, map[string]int{"Money": 5}, false},
		{"map float count", []ItemInput{map[string]any{"ItemID": "Money", "Count": 5.0}}, map[string]int{"Money": 5}, false},
		{"map int64 count", []ItemInput{map[string]any{"ItemID": "Money", "Count": int64(5)}}, map[string]int{"Money": 5}, false},
		{"map json.Number count", []ItemInput{map[string]any{"ItemID": "Money", "Count": json.Number("5")}}, map[string]int{"Money": 5}, false},
		{"map json.Number decimal count", []ItemInput{map[string]any{"ItemID": "Money", "Count": json.Number("5.0")}}, map[string]int{"Money": 5}, false},
		{"map json.Number fractional count", []ItemInput{map[string]any{"ItemID": "Money", "Count": json.Number("5.5")}}, nil, true},
		{"map fractional count", []ItemInput{map[string]any{"ItemID": "Money", "Count": 5.5}}, nil, true},
		{"tuple", []ItemInput{[]any{"Money", 5}}, map[string]int{"Money": 5}, false},
		{"tuple float count", []ItemInput{[]any{"Money", 5.0}}, map[string]int{"Money": 5}, false},
		{"mixed and merged", []ItemInput{"Money", []any{"Money", 2}, GiveItem{ItemID: "Wood", Count: 3}}, map[string]int{"Money": 3, "Wood": 3}, false},
		{"single element slice", []ItemInput{[]any{"Money"}}, map[string]int{"Money": 1}, false},
		{"empty", []ItemInput{}, nil, true},
		{"empty string", []ItemInput{""}, nil, true},
		{"object empty item id", []ItemInput{GiveItem{Count: 1}}, nil, true},
		{"object non-positive count", []ItemInput{GiveItem{ItemID: "Money", Count: 0}}, nil, true},
		{"map missing count", []ItemInput{map[string]any{"ItemID": "Money"}}, nil, true},
		{"map missing item id", []ItemInput{map[string]any{"Count": 5}}, nil, true},
		{"map non-positive count", []ItemInput{map[string]any{"ItemID": "Money", "Count": 0}}, nil, true},
		{"tuple wrong length", []ItemInput{[]any{"Money"}, []any{"Wood", 2}}, nil, true},
		{"tuple non-string item id", []ItemInput{[]any{5, 5}, []any{"Wood", 2}}, nil, true},
		{"tuple non-integer count", []ItemInput{[]any{"Money", "x"}, []any{"Wood", 2}}, nil, true},
		{"tuple non-positive count", []ItemInput{[]any{"Money", 0}}, nil, true},
		{"nil input", []ItemInput{nil}, nil, true},
		{"invalid type", []ItemInput{42}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeItemInputs(tt.inputs)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			counts := map[string]int{}
			for _, item := range got {
				counts[item.ItemID] = item.Count
			}
			assertCounts(t, counts, tt.want)
		})
	}
}

func TestNormalizeItemInputs_RejectsCountOverflow(t *testing.T) {
	_, err := normalizeItemInputs([]ItemInput{
		GiveItem{ItemID: "Money", Count: math.MaxInt},
		GiveItem{ItemID: "Money", Count: 1},
	})
	if err == nil {
		t.Fatal("expected error when merged count overflows int")
	}

	_, err = normalizeItemInputs([]ItemInput{
		"Money",
		GiveItem{ItemID: "Money", Count: math.MaxInt},
	})
	if err == nil {
		t.Fatal("expected error when merged count overflows int")
	}
}

func TestNormalizeItemInputs_FirstSeenOrder(t *testing.T) {
	got, err := normalizeItemInputs([]ItemInput{"Wood", []any{"Money", 2}, "Money", GiveItem{ItemID: "Wood", Count: 3}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []GiveItem{{ItemID: "Wood", Count: 4}, {ItemID: "Money", Count: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result: got %v want %v", got, want)
	}
}

func TestNormalizePalInputs(t *testing.T) {
	tests := []struct {
		name    string
		inputs  []PalInput
		want    map[string]int
		wantErr bool
	}{
		{"single string", []PalInput{"Pengullet"}, map[string]int{"Pengullet": 1}, false},
		{"single sequence", []PalInput{[]string{"Pengullet", "Foxparks"}}, map[string]int{"Pengullet": 1, "Foxparks": 1}, false},
		{"object", []PalInput{GivePal{PalID: "Pengullet", Level: 10}}, map[string]int{"Pengullet": 10}, false},
		{"map", []PalInput{map[string]any{"PalID": "Pengullet", "Level": 10}}, map[string]int{"Pengullet": 10}, false},
		{"map float level", []PalInput{map[string]any{"PalID": "Pengullet", "Level": 10.0}}, map[string]int{"Pengullet": 10}, false},
		{"map fractional level", []PalInput{map[string]any{"PalID": "Pengullet", "Level": 10.5}}, nil, true},
		{"tuple", []PalInput{[]any{"Pengullet", 10}}, map[string]int{"Pengullet": 10}, false},
		{"tuple float level", []PalInput{[]any{"Pengullet", 10.0}}, map[string]int{"Pengullet": 10}, false},
		{"single element slice", []PalInput{[]any{"Pengullet"}}, map[string]int{"Pengullet": 1}, false},
		{"empty", []PalInput{}, nil, true},
		{"empty string", []PalInput{""}, nil, true},
		{"object empty pal id", []PalInput{GivePal{Level: 10}}, nil, true},
		{"object non-positive level", []PalInput{GivePal{PalID: "Pengullet", Level: 0}}, nil, true},
		{"map empty pal id", []PalInput{map[string]any{"PalID": "", "Level": 5}}, nil, true},
		{"map non-positive level", []PalInput{map[string]any{"PalID": "Pengullet", "Level": 0}}, nil, true},
		{"tuple wrong length", []PalInput{[]any{"Pengullet"}, []any{"Foxparks", 5}}, nil, true},
		{"tuple non-string pal id", []PalInput{[]any{5, 10}, []any{"Foxparks", 5}}, nil, true},
		{"tuple non-integer level", []PalInput{[]any{"Pengullet", "x"}, []any{"Foxparks", 5}}, nil, true},
		{"tuple non-positive level", []PalInput{[]any{"Pengullet", 0}}, nil, true},
		{"invalid type", []PalInput{42}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePalInputs(tt.inputs)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			counts := map[string]int{}
			for _, pal := range got {
				counts[pal.PalID] = pal.Level
			}
			assertCounts(t, counts, tt.want)
		})
	}
}

func TestNormalizePalEggInputs(t *testing.T) {
	tests := []struct {
		name    string
		inputs  []PalEggInput
		want    []GivePalEgg
		wantErr bool
	}{
		{"object with pal id", []PalEggInput{GivePalEgg{EggID: "PalEgg_Fire_01", PalID: "Foxparks", Level: 12}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalID: "Foxparks", Level: 12}}, false},
		{"object with template", []PalEggInput{GivePalEgg{EggID: "PalEgg_Fire_01", PalTemplate: "Foxparks.json"}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalTemplate: "Foxparks.json"}}, false},
		{"map", []PalEggInput{map[string]any{"EggID": "PalEgg_Fire_01", "PalID": "Foxparks", "Level": 12}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalID: "Foxparks", Level: 12}}, false},
		{"map float level", []PalEggInput{map[string]any{"EggID": "PalEgg_Fire_01", "PalID": "Foxparks", "Level": 12.0}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalID: "Foxparks", Level: 12}}, false},
		{"tuple with level", []PalEggInput{[]any{"PalEgg_Fire_01", "Foxparks", 12}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalID: "Foxparks", Level: 12}}, false},
		{"tuple with float level", []PalEggInput{[]any{"PalEgg_Fire_01", "Foxparks", 12.0}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalID: "Foxparks", Level: 12}}, false},
		{"tuple template", []PalEggInput{[]any{"PalEgg_Fire_01", "Foxparks.json"}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalTemplate: "Foxparks.json"}}, false},
		{"tuple template uppercase suffix", []PalEggInput{[]any{"PalEgg_Fire_01", "FOXPARKS.JSON"}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalTemplate: "FOXPARKS.JSON"}}, false},
		{"tuple without level", []PalEggInput{[]any{"PalEgg_Fire_01", "Foxparks"}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalID: "Foxparks"}}, false},
		{"map zero level", []PalEggInput{map[string]any{"EggID": "PalEgg_Fire_01", "PalID": "Foxparks", "Level": 0}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalID: "Foxparks"}}, false},
		{"tuple zero level", []PalEggInput{[]any{"PalEgg_Fire_01", "Foxparks", 0}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalID: "Foxparks"}}, false},
		{"map negative level", []PalEggInput{map[string]any{"EggID": "PalEgg_Fire_01", "PalID": "Foxparks", "Level": -1}}, nil, true},
		{"tuple negative level", []PalEggInput{[]any{"PalEgg_Fire_01", "Foxparks", -1}}, nil, true},
		{"map wrong type pal id", []PalEggInput{map[string]any{"EggID": "x", "PalID": 123, "PalTemplate": "Foxparks.json"}}, nil, true},
		{"map wrong type template", []PalEggInput{map[string]any{"EggID": "x", "PalID": "Foxparks", "PalTemplate": 5}}, nil, true},
		{"map wrong type egg id", []PalEggInput{map[string]any{"EggID": 5, "PalID": "Foxparks"}}, nil, true},
		{"map both pal id and template", []PalEggInput{map[string]any{"EggID": "x", "PalID": "Foxparks", "PalTemplate": "Foxparks.json"}}, nil, true},
		{"map neither pal id nor template", []PalEggInput{map[string]any{"EggID": "x"}}, nil, true},
		{"map non-integer level", []PalEggInput{map[string]any{"EggID": "x", "PalID": "Foxparks", "Level": "a"}}, nil, true},
		{"tuple wrong length", []PalEggInput{[]any{"PalEgg_Fire_01"}}, nil, true},
		{"tuple non-string egg id", []PalEggInput{[]any{5, "Foxparks"}}, nil, true},
		{"tuple non-string pal value", []PalEggInput{[]any{"PalEgg_Fire_01", 5}}, nil, true},
		{"tuple non-integer level", []PalEggInput{[]any{"PalEgg_Fire_01", "Foxparks", "x"}}, nil, true},
		{"single object list", []PalEggInput{[]any{map[string]any{"EggID": "PalEgg_Fire_01", "PalID": "Foxparks"}}}, []GivePalEgg{{EggID: "PalEgg_Fire_01", PalID: "Foxparks"}}, false},
		{"empty", []PalEggInput{}, nil, true},
		{"empty egg id", []PalEggInput{GivePalEgg{PalID: "Foxparks"}}, nil, true},
		{"both pal id and template", []PalEggInput{GivePalEgg{EggID: "x", PalID: "Foxparks", PalTemplate: "Foxparks.json"}}, nil, true},
		{"neither pal id nor template", []PalEggInput{GivePalEgg{EggID: "x"}}, nil, true},
		{"negative level", []PalEggInput{GivePalEgg{EggID: "x", PalID: "Foxparks", Level: -1}}, nil, true},
		{"invalid type", []PalEggInput{"Foxparks"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePalEggInputs(tt.inputs)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected eggs: got %+v want %+v", got, tt.want)
			}
		})
	}
}

func TestNormalizeTechnologyInputs(t *testing.T) {
	tests := []struct {
		name    string
		inputs  []TechnologyInput
		want    any
		wantErr bool
	}{
		{"single", []TechnologyInput{"Technology_ElecBaton"}, "Technology_ElecBaton", false},
		{"single all", []TechnologyInput{"All"}, "All", false},
		{"single sequence", []TechnologyInput{[]string{"A", "B"}}, []string{"A", "B"}, false},
		{"multiple", []TechnologyInput{"A", "B"}, []string{"A", "B"}, false},
		{"all with others", []TechnologyInput{"All", "B"}, nil, true},
		{"empty", []TechnologyInput{}, nil, true},
		{"empty string", []TechnologyInput{""}, nil, true},
		{"invalid type", []TechnologyInput{42}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTechnologyInputs(tt.inputs)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected result: got %v want %v", got, tt.want)
			}
		})
	}
}

func TestAsInt_RejectsInvalidNumbers(t *testing.T) {
	if _, ok := asInt(json.Number("abc")); ok {
		t.Fatal("expected invalid json.Number to be rejected")
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"empty", "", "", true},
		{"whitespace", "   ", "", true},
		{"no scheme with port", "127.0.0.1:17993", "http://127.0.0.1:17993", false},
		{"no scheme no port", "127.0.0.1", "http://127.0.0.1:17993", false},
		{"explicit default port", "http://127.0.0.1:17993", "http://127.0.0.1:17993", false},
		{"https", "https://pd.example.com", "https://pd.example.com:443", false},
		{"https with port", "https://pd.example.com:17993", "https://pd.example.com:17993", false},
		{"custom port", "http://host.example:8080", "http://host.example:8080", false},
		{"ipv6", "http://[::1]:17993", "http://[::1]:17993", false},
		{"ipv6 no port", "http://[::1]", "http://[::1]:17993", false},
		{"ipv6 zone", "http://[fe80::1%25eth0]:17993", "http://[fe80::1%25eth0]:17993", false},
		{"fqdn trailing dot", "http://host.example.:17993", "http://host.example.:17993", false},
		{"unsupported scheme", "ftp://host", "", true},
		{"userinfo", "http://user:pass@host:17993", "", true},
		{"path", "http://host:17993/api", "", true},
		{"query", "http://host:17993?x=1", "", true},
		{"fragment", "http://host:17993#frag", "", true},
		{"missing host", "http://:17993", "", true},
		{"invalid hostname", "http://exa_mple.com", "", true},
		{"hostname starting dash", "http://-example.com", "", true},
		{"hostname label ending dash", "http://example-.com", "", true},
		{"hostname label too long", "http://" + strings.Repeat("a", 64) + ".com", "", true},
		{"hostname too long", "http://" + strings.Repeat("a", 254), "", true},
		{"invalid port", "http://host:abc", "", true},
		{"port out of range", "http://host:70000", "", true},
		{"zero port", "http://host:0", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeBaseURL(tt.raw, defaultPort)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected URL: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestFlattenSingleSequence(t *testing.T) {
	tests := []struct {
		name string
		in   []any
		want []any
	}{
		{"single sequence flattened", []any{[]string{"A", "B"}}, []any{"A", "B"}},
		{"tuple preserved", []any{[]any{"Money", 5}}, []any{[]any{"Money", 5}}},
		{"multiple elements untouched", []any{"A", []any{"B", 1}}, []any{"A", []any{"B", 1}}},
		{"non-slice untouched", []any{"A"}, []any{"A"}},
		{"byte slice untouched", []any{[]byte("AB")}, []any{[]byte("AB")}},
		{"nested tuples flattened", []any{[]any{[]any{"A", 1}, []any{"B", 2}}}, []any{[]any{"A", 1}, []any{"B", 2}}},
		{"empty sequence flattened", []any{[]any{}}, []any{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flattenSingleSequence(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected result: got %v want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeStringInputs(t *testing.T) {
	got, err := normalizeStringInputs([]string{"Lamball.json", "Foxparks.json"}, "templates")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"Lamball.json", "Foxparks.json"}) {
		t.Fatalf("unexpected result: %v", got)
	}

	if _, err := normalizeStringInputs([]string{""}, "templates"); err == nil {
		t.Fatal("expected error for empty entry")
	}
	if _, err := normalizeStringInputs(nil, "templates"); err == nil {
		t.Fatal("expected error for empty list")
	}
}

func TestRedactUserinfo(t *testing.T) {
	if got := redactUserinfo("http://user:pass@host:17993"); got != "http://xxxxx@host:17993" {
		t.Fatalf("unexpected redacted URL: %q", got)
	}
	if got := redactUserinfo("no-scheme-value"); got != "no-scheme-value" {
		t.Fatalf("unexpected result without scheme: %q", got)
	}
	if got := redactUserinfo("http://host:17993"); got != "http://host:17993" {
		t.Fatalf("unexpected result without userinfo: %q", got)
	}
}

func TestValidatePort(t *testing.T) {
	if _, err := validatePort("17993"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, err := validatePort("017993"); err != nil || got != "17993" {
		t.Fatalf("expected canonical port, got %q, err %v", got, err)
	}
	if _, err := validatePort("-1"); err == nil {
		t.Fatal("expected signed port to be rejected")
	}
	if _, err := validatePort("abc"); err == nil {
		t.Fatal("expected non-numeric port to be rejected")
	}
	if _, err := validatePort("99999999999999999999"); err == nil {
		t.Fatal("expected overflowing port to be rejected")
	}
	if _, err := validatePort("0"); err == nil {
		t.Fatal("expected zero port to be rejected")
	}
	if _, err := validatePort("65536"); err == nil {
		t.Fatal("expected out-of-range port to be rejected")
	}
}
