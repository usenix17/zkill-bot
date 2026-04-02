package enrichment_test

import (
	"os"
	"testing"

	"zkill-bot/internal/enrichment"
	"zkill-bot/internal/killmail"
)

const dbPath = "../../eve.db"

func skipIfNoDB(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(dbPath); err != nil {
		t.Skip("eve.db not found, skipping enrichment tests")
	}
}

func TestEnrich_VictimShip(t *testing.T) {
	skipIfNoDB(t)

	e, err := enrichment.New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer e.Close()

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
	skipIfNoDB(t)

	e, err := enrichment.New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer e.Close()

	// typeID 671 is a well-known Titan (Avatar), groupID 30
	// Use a known Titan typeID: Avatar = 11567
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
	skipIfNoDB(t)

	e, err := enrichment.New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer e.Close()

	km := &killmail.Killmail{
		KillmailID: 3,
		SequenceID: 3,
		Victim: killmail.Participant{
			ShipTypeID: 999999999, // non-existent
		},
	}

	// Must not panic; enriched data should be empty but present
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
