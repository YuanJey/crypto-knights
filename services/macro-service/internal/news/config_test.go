package news

import "testing"

func TestDefaultSourcesAreValidAndTiered(t *testing.T) {
	sources, err := LoadSources("")
	if err != nil {
		t.Fatalf("load default sources: %v", err)
	}
	if len(sources) < 8 {
		t.Fatalf("source count = %d, want at least 8", len(sources))
	}

	var hasOfficial, hasPress, hasDiscovery bool
	for _, source := range sources {
		switch source.Tier {
		case "A":
			hasOfficial = true
		case "B":
			hasPress = true
		case "D":
			hasDiscovery = true
		}
		if source.Kind == SourceKindGDELT && source.PollIntervalSeconds < 300 {
			t.Fatalf("GDELT interval is too short: %d", source.PollIntervalSeconds)
		}
	}
	if !hasOfficial || !hasPress || !hasDiscovery {
		t.Fatalf(
			"missing source tier: official=%v press=%v discovery=%v",
			hasOfficial,
			hasPress,
			hasDiscovery,
		)
	}
}
