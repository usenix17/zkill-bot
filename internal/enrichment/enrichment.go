package enrichment

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"

	"zkill-bot/internal/killmail"
)

// capitalGroupIDs is the set of EVE ship group IDs considered capital-class.
var capitalGroupIDs = map[int64]bool{
	30:   true, // Titan
	485:  true, // Dreadnought
	547:  true, // Carrier
	659:  true, // Supercarrier
	883:  true, // Capital Industrial Ship
	1538: true, // Force Auxiliary
}

// typeInfo holds cached enrichment data for a single type ID.
type typeInfo struct {
	typeID      int64
	typeName    string
	groupID     int64
	groupName   string
	categoryID  int64
	catName     string
	metaLevel   int
	metaGroupID int64
	metaGroup   string
}

// Enricher resolves type IDs to human-readable names using the EVE SDE SQLite database.
type Enricher struct {
	db    *sql.DB
	cache map[int64]*typeInfo // keyed by typeID; nil means "not found"
}

// New opens the SQLite database at dbPath for read-only enrichment lookups.
func New(dbPath string) (*Enricher, error) {
	// file URI with mode=ro prevents any writes
	dsn := fmt.Sprintf("file:%s?mode=ro&_foreign_keys=off", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("enrichment: open db %q: %w", dbPath, err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("enrichment: ping db: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer; read-only is safe with 1 conn
	return &Enricher{db: db, cache: make(map[int64]*typeInfo)}, nil
}

// Close releases the database connection.
func (e *Enricher) Close() error {
	return e.db.Close()
}

// Enrich populates km.Enriched with ship/item names, group/category lookups,
// meta levels, and a HasCapital flag. Lookups are cached per process.
// Missing type IDs are silently skipped (enrichment is best-effort).
func (e *Enricher) Enrich(km *killmail.Killmail) {
	ed := &killmail.EnrichedData{}

	// Solar system name (static map, no DB hit)
	ed.SolarSystemName = solarSystemNames[km.SolarSystemID]

	// Victim ship
	if ti := e.lookup(km.Victim.ShipTypeID); ti != nil {
		ed.VictimShipName = ti.typeName
		ed.VictimShipGroup = ti.groupName
		ed.VictimShipCategory = ti.catName
		ed.VictimShipGroupID = ti.groupID
		ed.VictimShipCatID = ti.categoryID
		if capitalGroupIDs[ti.groupID] {
			ed.HasCapital = true
		}
	}

	// Attacker ships
	ed.AttackerShips = make([]killmail.ShipInfo, len(km.Attackers))
	for i, a := range km.Attackers {
		if a.ShipTypeID == 0 {
			continue
		}
		ti := e.lookup(a.ShipTypeID)
		if ti == nil {
			continue
		}
		ed.AttackerShips[i] = killmail.ShipInfo{
			TypeName:     ti.typeName,
			GroupName:    ti.groupName,
			CategoryName: ti.catName,
			GroupID:      ti.groupID,
			CategoryID:   ti.categoryID,
			MetaLevel:    ti.metaLevel,
			MetaGroupID:  ti.metaGroupID,
			MetaGroup:    ti.metaGroup,
		}
		if capitalGroupIDs[ti.groupID] {
			ed.HasCapital = true
		}
	}

	// Item names
	ed.ItemNames = make([]string, len(km.Items))
	for i, it := range km.Items {
		if ti := e.lookup(it.ItemTypeID); ti != nil {
			ed.ItemNames[i] = ti.typeName
		}
	}

	km.Enriched = ed
}

// lookup returns cached type info for typeID, querying the DB on first access.
// Returns nil if the typeID is 0 or not found in the database.
func (e *Enricher) lookup(typeID int64) *typeInfo {
	if typeID == 0 {
		return nil
	}
	if ti, ok := e.cache[typeID]; ok {
		return ti // may be nil (negative cache)
	}

	ti, err := e.queryType(typeID)
	if err != nil {
		slog.Debug("enrichment: lookup failed", "type_id", typeID, "error", err)
		e.cache[typeID] = nil
		return nil
	}
	e.cache[typeID] = ti
	return ti
}

const typeQuery = `
SELECT
    t.typeID,
    COALESCE(t.typeName, ''),
    COALESCE(t.groupID, 0),
    COALESCE(g.name, ''),
    COALESCE(g.categoryID, 0),
    COALESCE(c.name, ''),
    COALESCE(t.metaLevel, 0),
    COALESCE(t.metaGroupID, 0),
    COALESCE(mg.metaGroupName, '')
FROM invtypes t
LEFT JOIN invgroups g ON g.groupID = t.groupID
LEFT JOIN invcategories c ON c.categoryID = g.categoryID
LEFT JOIN invmetagroups mg ON mg.metaGroupID = t.metaGroupID
WHERE t.typeID = ?
LIMIT 1
`

func (e *Enricher) queryType(typeID int64) (*typeInfo, error) {
	row := e.db.QueryRow(typeQuery, typeID)
	ti := &typeInfo{}
	err := row.Scan(
		&ti.typeID,
		&ti.typeName,
		&ti.groupID,
		&ti.groupName,
		&ti.categoryID,
		&ti.catName,
		&ti.metaLevel,
		&ti.metaGroupID,
		&ti.metaGroup,
	)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // intentional negative cache sentinel
	}
	if err != nil {
		return nil, fmt.Errorf("enrichment: scan type %d: %w", typeID, err)
	}
	return ti, nil
}
