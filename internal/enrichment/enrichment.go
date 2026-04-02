package enrichment

import "zkill-bot/internal/killmail"

// capitalGroupIDs is the set of EVE ship group IDs considered capital-class.
var capitalGroupIDs = map[int64]bool{
	30:   true, // Titan
	485:  true, // Dreadnought
	547:  true, // Carrier
	659:  true, // Supercarrier
	883:  true, // Capital Industrial Ship
	1538: true, // Force Auxiliary
}

// Enricher resolves type IDs and system IDs to human-readable names using
// pre-generated static maps. It requires no external files at runtime.
type Enricher struct{}

// New returns an Enricher. No setup is required.
func New() *Enricher {
	return &Enricher{}
}

// Enrich populates km.Enriched with ship/item names, group/category lookups,
// meta levels, capital flag, and solar system name.
func (e *Enricher) Enrich(km *killmail.Killmail) {
	ed := &killmail.EnrichedData{}

	// Solar system name (static map)
	ed.SolarSystemName = solarSystemNames[km.SolarSystemID]

	// Victim ship
	if t, ok := sdeTypes[km.Victim.ShipTypeID]; ok {
		ed.VictimShipName = t.TypeName
		ed.VictimShipGroup = t.GroupName
		ed.VictimShipCategory = t.CatName
		ed.VictimShipGroupID = t.GroupID
		ed.VictimShipCatID = t.CategoryID
		if capitalGroupIDs[t.GroupID] {
			ed.HasCapital = true
		}
	}

	// Attacker ships
	ed.AttackerShips = make([]killmail.ShipInfo, len(km.Attackers))
	for i, a := range km.Attackers {
		t, ok := sdeTypes[a.ShipTypeID]
		if !ok {
			continue
		}
		ed.AttackerShips[i] = killmail.ShipInfo{
			TypeName:     t.TypeName,
			GroupName:    t.GroupName,
			CategoryName: t.CatName,
			GroupID:      t.GroupID,
			CategoryID:   t.CategoryID,
			MetaLevel:    t.MetaLevel,
			MetaGroupID:  t.MetaGroupID,
			MetaGroup:    t.MetaGroup,
		}
		if capitalGroupIDs[t.GroupID] {
			ed.HasCapital = true
		}
	}

	// Item names
	ed.ItemNames = make([]string, len(km.Items))
	for i, it := range km.Items {
		if t, ok := sdeTypes[it.ItemTypeID]; ok {
			ed.ItemNames[i] = t.TypeName
		}
	}

	km.Enriched = ed
}
