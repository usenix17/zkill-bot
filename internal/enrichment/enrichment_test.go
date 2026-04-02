package enrichment_test

import (
	"testing"

	"zkill-bot/internal/enrichment"
	"zkill-bot/internal/killmail"
)

func TestEnrich_VictimShip(t *testing.T) {
	e := enrichment.New()

	km := &killmail.Killmail{
		KillmailID: 1,
		SequenceID: 1,
		Victim: killmail.Participant{
			ShipTypeID: 670, // Capsule
		},
	}

	e.Enrich(km)

	if km.Enriched == nil {
		t.Fatal("Enriched is nil")
	}
	if km.Enriched.VictimShipName != "Capsule" {
		t.Errorf("VictimShipName: got %q, want %q", km.Enriched.VictimShipName, "Capsule")
	}
	if km.Enriched.VictimShipGroup == "" {
		t.Error("VictimShipGroup: expected non-empty")
	}
}

func TestEnrich_CapitalDetection(t *testing.T) {
	e := enrichment.New()

	km := &killmail.Killmail{
		KillmailID: 2,
		SequenceID: 2,
		Victim: killmail.Participant{
			ShipTypeID: 11567, // Avatar Titan
		},
	}

	e.Enrich(km)

	if !km.Enriched.HasCapital {
		t.Error("HasCapital: expected true for Titan ship")
	}
}

func TestEnrich_UnknownTypeID(t *testing.T) {
	e := enrichment.New()

	km := &killmail.Killmail{
		KillmailID: 3,
		SequenceID: 3,
		Victim: killmail.Participant{
			ShipTypeID: 999999999, // non-existent
		},
	}

	e.Enrich(km)

	if km.Enriched == nil {
		t.Fatal("Enriched is nil")
	}
	if km.Enriched.VictimShipName != "" {
		t.Errorf("VictimShipName: expected empty for unknown typeID, got %q", km.Enriched.VictimShipName)
	}
	if km.Enriched.HasCapital {
		t.Error("HasCapital: expected false for unknown typeID")
	}
}

func TestEnrich_SolarSystemName(t *testing.T) {
	e := enrichment.New()

	km := &killmail.Killmail{
		KillmailID:    4,
		SequenceID:    4,
		SolarSystemID: 30000142, // Jita
	}

	e.Enrich(km)

	if km.Enriched.SolarSystemName != "Jita" {
		t.Errorf("SolarSystemName: got %q, want %q", km.Enriched.SolarSystemName, "Jita")
	}
}
