package killmail_test

import (
	"os"
	"testing"
	"time"

	"zkill-bot/internal/killmail"
)

func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %q: %v", path, err)
	}
	return data
}

func TestNormalizeFromR2Z2_ValidPayload(t *testing.T) {
	raw := loadFixture(t, "../../testdata/killmail_sample.json")

	km, err := killmail.NormalizeFromR2Z2(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if km.KillmailID != 134435757 {
		t.Errorf("KillmailID: got %d, want 134435757", km.KillmailID)
	}
	if km.SequenceID != 96724073 {
		t.Errorf("SequenceID: got %d, want 96724073", km.SequenceID)
	}
	if km.Hash != "1ba1415b43277358eeca2735367d5de36b007c69" {
		t.Errorf("Hash mismatch: %q", km.Hash)
	}
	if km.SolarSystemID != 30000186 {
		t.Errorf("SolarSystemID: got %d, want 30000186", km.SolarSystemID)
	}

	want := time.Date(2026, 4, 2, 6, 40, 33, 0, time.UTC)
	if !km.KillmailTime.Equal(want) {
		t.Errorf("KillmailTime: got %v, want %v", km.KillmailTime, want)
	}

	if km.Victim.CharacterID != 2114529076 {
		t.Errorf("Victim.CharacterID: got %d", km.Victim.CharacterID)
	}
	if km.Victim.ShipTypeID != 670 {
		t.Errorf("Victim.ShipTypeID: got %d, want 670", km.Victim.ShipTypeID)
	}

	if len(km.Attackers) != 1 {
		t.Fatalf("Attackers: got %d, want 1", len(km.Attackers))
	}
	a := km.Attackers[0]
	if a.ShipTypeID != 37456 {
		t.Errorf("Attacker.ShipTypeID: got %d, want 37456", a.ShipTypeID)
	}
	if !a.FinalBlow {
		t.Error("Attacker.FinalBlow: expected true")
	}
	if km.FinalBlow == nil {
		t.Fatal("FinalBlow pointer: expected non-nil")
	}
	if km.FinalBlow.CharacterID != 2115668403 {
		t.Errorf("FinalBlow.CharacterID: got %d", km.FinalBlow.CharacterID)
	}

	if len(km.Items) != 2 {
		t.Errorf("Items: got %d, want 2", len(km.Items))
	}

	if !km.ZKB.NPC == false {
		t.Error("ZKB.NPC: expected false")
	}
	if km.ZKB.TotalValue != 627868229.1 {
		t.Errorf("ZKB.TotalValue: got %f", km.ZKB.TotalValue)
	}
	if len(km.ZKB.Labels) != 5 {
		t.Errorf("ZKB.Labels: got %d, want 5", len(km.ZKB.Labels))
	}
}

func TestNormalizeFromR2Z2_MissingKillmailID(t *testing.T) {
	raw := []byte(`{"killmail_id":0,"sequence_id":1,"esi":{"victim":{"ship_type_id":670,"corporation_id":123}},"zkb":{}}`)
	_, err := killmail.NormalizeFromR2Z2(raw)
	if err == nil {
		t.Error("expected error for missing killmail_id, got nil")
	}
}

func TestNormalizeFromR2Z2_MalformedJSON(t *testing.T) {
	_, err := killmail.NormalizeFromR2Z2([]byte(`{not valid json`))
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestNormalizeFromR2Z2_MissingESI(t *testing.T) {
	raw := []byte(`{"killmail_id":1,"sequence_id":1,"zkb":{}}`)
	_, err := killmail.NormalizeFromR2Z2(raw)
	if err == nil {
		t.Error("expected error for missing esi block, got nil")
	}
}

func TestNormalizeFromR2Z2_MissingSequenceID(t *testing.T) {
	raw := []byte(`{"killmail_id":1,"sequence_id":0,"esi":{"victim":{"ship_type_id":1,"corporation_id":1}},"zkb":{}}`)
	_, err := killmail.NormalizeFromR2Z2(raw)
	if err == nil {
		t.Error("expected error for missing sequence_id, got nil")
	}
}
