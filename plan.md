# ZKill Bot Implementation Plan

## Technology Choices

- **Language:** Go 1.23+
- **Persistent state:** JSON file (`state.json`) for sequence checkpoint + action idempotency history
- **Enrichment:** SQLite read-only (`eve.db`) via `modernc.org/sqlite` (pure Go, no CGO)
- **Rules config:** YAML file for human readability
- **HTTP client:** `net/http` stdlib
- **Logging:** `log/slog` structured logging (stdlib)
- **Discord webhooks:** Plain HTTP POST (no Discord library needed)

## Project Structure

```
zkill-bot/
├── main.go                  # Entry point: load config, wire dependencies, run loop
├── go.mod
├── go.sum
├── .env                     # Environment config (existing)
├── eve.db                   # EVE SDE data (existing, read-only)
├── config/
│   └── rules.yaml           # Rule definitions
├── state.json               # Runtime state (created automatically)
│
├── internal/
│   ├── config/
│   │   └── config.go        # Env loading, validation, fail-fast
│   ├── killmail/
│   │   └── killmail.go      # Killmail types + normalization
│   ├── poller/
│   │   └── poller.go        # R2Z2 HTTP polling loop
│   ├── enrichment/
│   │   └── enrichment.go    # eve.db lookups (type/group/category/meta names)
│   ├── rules/
│   │   ├── rules.go         # Rule loading, priority sorting, evaluation engine
│   │   └── filters.go       # Filter tree: and/or/not + all leaf filter types
│   ├── actions/
│   │   ├── actions.go       # Action dispatcher, retry logic
│   │   ├── console.go       # Console output action
│   │   └── webhook.go       # Discord webhook action
│   ├── state/
│   │   └── state.go         # JSON file read/write for checkpoint + action history
│   └── metrics/
│       └── metrics.go       # In-memory counters, periodic log dump
```

## Implementation Steps

### Step 1: Project Scaffolding

- `go mod init zkill-bot`
- Add dependencies: `modernc.org/sqlite`, `gopkg.in/yaml.v3`, `github.com/joho/godotenv`
- Create directory structure
- Create `main.go` skeleton with signal handling (`SIGINT`, `SIGTERM`) for graceful shutdown

### Step 2: Configuration (`internal/config/`)

Load from environment (with `.env` fallback via godotenv):

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `RULES_FILE_PATH` | yes | `./config/rules.yaml` | Path to rules YAML |
| `EVE_DB_PATH` | yes | `./eve.db` | Path to SQLite enrichment DB |
| `STATE_FILE_PATH` | no | `./state.json` | Path to persistent state JSON |
| `R2Z2_BASE_URL` | no | `https://r2z2.zkillboard.com` | R2Z2 base URL |
| `POLL_INTERVAL_MS` | no | `100` | Delay between successful fetches |
| `POLL_404_BACKOFF_MS` | no | `6000` | Delay after 404 (no new killmails) |
| `OBS_ALERT_WEBHOOK_URL` | no | - | Discord webhook for startup/shutdown alerts |
| `DEBUG` | no | `false` | Enable verbose metrics logging |
| `RETRY_MAX_RETRIES` | no | `3` | Max retries for failed actions |
| `RETRY_BASE_BACKOFF_MS` | no | `250` | Base backoff for action retries |

Validation: fail fast if required paths don't exist, URLs are malformed, or numeric values are invalid.

### Step 3: Killmail Types (`internal/killmail/`)

Define the internal normalized event struct. This is the canonical shape used by rules and actions.

```go
type Killmail struct {
    // Identity
    KillmailID  int64
    Hash        string
    SequenceID  int64

    // Timing
    KillmailTime time.Time
    UploadedAt   time.Time

    // Location
    SolarSystemID int64

    // Victim
    Victim Participant

    // Attackers
    Attackers    []Participant
    AttackerCount int
    FinalBlow    *Participant  // pointer into Attackers slice

    // Victim items
    Items []Item

    // ZKB metadata
    ZKB ZKBMeta

    // Enriched data (populated after enrichment step)
    Enriched *EnrichedData
}

type Participant struct {
    CharacterID   int64
    CorporationID int64
    AllianceID    int64
    ShipTypeID    int64
    WeaponTypeID  int64
    DamageDone    int64   // attackers
    DamageTaken   int64   // victim
    FinalBlow     bool
    SecurityStatus float64
}

type Item struct {
    ItemTypeID         int64
    Flag               int
    QuantityDropped    int64
    QuantityDestroyed  int64
    Singleton          int
}

type ZKBMeta struct {
    LocationID     int64
    FittedValue    float64
    DroppedValue   float64
    DestroyedValue float64
    TotalValue     float64
    Points         int
    NPC            bool
    Solo           bool
    Awox           bool
    Labels         []string
}

type EnrichedData struct {
    VictimShipName     string
    VictimShipGroup    string
    VictimShipCategory string
    // Per-attacker enrichment stored inline on Participant or in a parallel slice
    AttackerShips      []ShipInfo
    // Derived flags
    HasCapital         bool
    SolarSystemName    string   // from labels or leave for future ESI lookup
}

type ShipInfo struct {
    TypeName     string
    GroupName    string
    CategoryName string
    MetaLevel    int
    MetaGroup    string
}
```

**Normalization function:** `NormalizeFromR2Z2(raw json.RawMessage) (*Killmail, error)` — unmarshals R2Z2 JSON into the canonical struct. Returns error for malformed payloads (missing killmail_id, missing victim, etc.) which the pipeline logs and skips.

### Step 4: R2Z2 Poller (`internal/poller/`)

Responsibilities:
- Fetch `sequence.json` for initial sequence (only if no checkpoint exists)
- Poll `{sequence}.json` in a loop
- Return normalized killmails via a channel or callback

```go
type Poller struct {
    baseURL       string
    pollInterval  time.Duration
    backoff404    time.Duration
    httpClient    *http.Client
    lastSequence  int64
}

// Run blocks and pushes raw JSON payloads to the provided channel.
// Stops when ctx is cancelled.
func (p *Poller) Run(ctx context.Context, startSequence int64, out chan<- json.RawMessage) error
```

HTTP status handling:
| Status | Behavior |
|---|---|
| 200 | Parse payload, send to channel, increment sequence, sleep `pollInterval` |
| 404 | No new killmails — sleep `backoff404`, retry same sequence |
| 429 | Rate limited — sleep 10s, retry (log warning) |
| 403 | Blocked — log error, send alert webhook, sleep 60s, retry |
| Other | Transient error — exponential backoff (1s, 2s, 4s... capped at 60s), retry |

Set `User-Agent: zkill-bot/1.0` on all requests.

### Step 5: Enrichment (`internal/enrichment/`)

Opens `eve.db` read-only at startup. Provides lookup methods:

```go
type Enricher struct {
    db *sql.DB
    // In-memory caches (typeID -> info) to avoid repeated queries
    typeCache  map[int64]*TypeInfo
    groupCache map[int64]*GroupInfo
}

type TypeInfo struct {
    TypeID       int64
    TypeName     string
    GroupID      int64
    MetaLevel    int
    MetaGroupID  int64
}

type GroupInfo struct {
    GroupID      int64
    GroupName    string
    CategoryID   int64
    CategoryName string
}

func (e *Enricher) Enrich(km *Killmail) error
```

Enrich populates:
- Victim ship name, group, category
- Each attacker's ship name, group, category
- `HasCapital` flag — true if any attacker or victim ship is in group: Titan (30), Supercarrier (659), Dreadnought (485), Carrier (547), Force Auxiliary (1538), Capital Industrial Ship (883)
- Item type names (for future use / webhook display)

Caches are warmed lazily and live for the process lifetime (eve.db is static).

### Step 6: State Persistence (`internal/state/`)

Single JSON file with atomic write (write to temp file, then rename):

```go
type State struct {
    LastSequence  int64                    `json:"last_sequence"`
    ActionHistory map[string]time.Time     `json:"action_history"` // fingerprint -> timestamp
}
```

**Action fingerprint:** `fmt.Sprintf("%d:%s", killmailID, actionName)` — e.g. `"134435757:webhook-discord-pvp"`

Methods:
- `Load(path string) (*State, error)` — reads file, returns zero state if file doesn't exist
- `Save(path string) error` — atomic write
- `HasExecuted(fingerprint string) bool`
- `RecordExecution(fingerprint string)`
- `PruneHistory(olderThan time.Duration)` — remove entries older than 24h to bound file size

Save is called after each killmail is fully processed (all actions complete or skipped).

### Step 7: Rule System (`internal/rules/`)

#### Rule YAML Format

```yaml
mode: first-match  # or "multi-match"

rules:
  - name: "high-value-capital-kill"
    enabled: true
    priority: 10
    filter:
      and:
        - zkb_value_min: 1000000000       # 1B+ ISK
        - has_capital: true
    actions:
      - type: webhook
        args:
          url: "https://discord.com/api/webhooks/..."
          template: "capital"

  - name: "solo-pvp-lowsec"
    enabled: true
    priority: 20
    filter:
      and:
        - solo: true
        - zkb_label: "loc:lowsec"
        - not:
            npc: true
    actions:
      - type: console
      - type: webhook
        args:
          url: "https://discord.com/api/webhooks/..."

  - name: "corp-loss-alert"
    enabled: true
    priority: 30
    filter:
      victim_corporation_id: [98812444]
    actions:
      - type: webhook
        args:
          url: "https://discord.com/api/webhooks/..."
          template: "loss"
```

#### Filter Types (Leaf Filters)

| Filter Key | Type | Description |
|---|---|---|
| `attacker_count_min` | int | Minimum number of attackers |
| `attacker_count_max` | int | Maximum number of attackers |
| `solo` | bool | ZKB solo flag |
| `npc` | bool | ZKB NPC flag |
| `region_id` | []int | Solar system's region (from labels or mapping) |
| `solar_system_id` | []int | Exact system IDs |
| `zkb_label` | string/[]string | Match ZKB labels (e.g. "loc:lowsec", "pvp") |
| `day_of_week` | []string | Day filter (e.g. ["monday","tuesday"]) |
| `time_window` | {from, to} | UTC time range (e.g. 18:00-23:00) |
| `victim_character_id` | []int | Victim character IDs |
| `victim_corporation_id` | []int | Victim corp IDs |
| `victim_alliance_id` | []int | Victim alliance IDs |
| `attacker_character_id` | []int | Any attacker matches |
| `attacker_corporation_id` | []int | Any attacker matches |
| `attacker_alliance_id` | []int | Any attacker matches |
| `victim_ship_type_id` | []int | Victim ship type |
| `victim_ship_group_id` | []int | Victim ship group (enriched) |
| `attacker_ship_type_id` | []int | Any attacker ship type |
| `item_type_id` | []int | Any item in victim cargo/fit |
| `has_capital` | bool | Capital ship involvement (enriched) |
| `zkb_value_min` | float | Minimum totalValue |
| `zkb_value_max` | float | Maximum totalValue |

#### Composite Filters

| Key | Behavior |
|---|---|
| `and` | All children must match |
| `or` | At least one child must match |
| `not` | Child must NOT match (wraps single filter) |

These nest arbitrarily.

#### Evaluation Engine

```go
func EvaluateRules(km *Killmail, rules []Rule, mode Mode) []RuleMatch

type RuleMatch struct {
    Rule    *Rule
    Actions []ActionConfig
}
```

1. Sort enabled rules by priority (ascending = highest priority first).
2. For each rule, evaluate its filter tree against the killmail.
3. If matched, collect its actions into results.
4. In `first-match` mode: return after first match.
5. In `multi-match` mode: continue through all rules.

### Step 8: Actions (`internal/actions/`)

#### Action Dispatcher

```go
type Dispatcher struct {
    state      *state.State
    maxRetries int
    baseBackoff time.Duration
    handlers   map[string]ActionHandler
    metrics    *metrics.Metrics
}

type ActionHandler interface {
    Execute(ctx context.Context, km *Killmail, args map[string]interface{}) error
}
```

For each `RuleMatch`:
1. Compute fingerprint: `killmailID:ruleName:actionType`
2. Check `state.HasExecuted(fingerprint)` — skip if already done
3. Call handler's `Execute`
4. On success: `state.RecordExecution(fingerprint)`
5. On retryable failure: retry with exponential backoff up to `maxRetries`
6. Record metrics (success/failure/retry/skip)

#### Console Action (`console.go`)

Prints a human-readable one-line summary to stdout:

```
[2026-04-02T06:40:33Z] Kill #134435757 | Victim: Capsule (CharID:2114529076, Corp:1000168) | Ship: Capsule | Attackers: 1 | Value: 627.9M ISK | System: 30000186 | Solo
```

#### Webhook Action (`webhook.go`)

POST to Discord webhook URL with JSON body. Supports a `template` arg to vary embed format:
- Default template: compact summary embed
- `"capital"` template: expanded embed with attacker list
- `"loss"` template: loss-focused embed with fitted value

Discord embed structure:
```go
type DiscordWebhook struct {
    Content string         `json:"content,omitempty"`
    Embeds  []DiscordEmbed `json:"embeds"`
}

type DiscordEmbed struct {
    Title       string          `json:"title"`
    Description string          `json:"description"`
    URL         string          `json:"url"`
    Color       int             `json:"color"`
    Fields      []EmbedField    `json:"fields"`
    Timestamp   string          `json:"timestamp"`
    Footer      *EmbedFooter    `json:"footer,omitempty"`
    Thumbnail   *EmbedThumbnail `json:"thumbnail,omitempty"`
}
```

URL links to `https://zkillboard.com/kill/{killmail_id}/`.  
Thumbnail uses `https://images.evetech.net/types/{ship_type_id}/icon?size=64`.

### Step 9: Metrics (`internal/metrics/`)

In-memory atomic counters, dumped to structured log on a configurable interval (default 60s) when `DEBUG=true`:

```go
type Metrics struct {
    FetchOK          atomic.Int64
    Fetch404         atomic.Int64
    Fetch429         atomic.Int64
    FetchError       atomic.Int64
    KillmailsProcessed atomic.Int64
    KillmailsRejected  atomic.Int64
    RuleMatches      atomic.Int64
    ActionSuccess    atomic.Int64
    ActionFailure    atomic.Int64
    ActionRetry      atomic.Int64
    ActionSkipDupe   atomic.Int64
    LastProcessedAt  atomic.Int64   // unix timestamp
    LastSequenceID   atomic.Int64
}
```

Lag metric: `time.Now().Unix() - km.UploadedAt.Unix()` logged per killmail when debug is on.

### Step 10: Main Loop (`main.go`)

```
main()
├── Load config (fail fast)
├── Load rules from YAML (fail fast)
├── Open eve.db enricher
├── Load state.json
├── Send startup notification to Discord webhook (if configured)
├── Determine start sequence:
│   ├── If state has LastSequence > 0: use LastSequence + 1
│   └── Else: fetch sequence.json
├── Start metrics reporter goroutine (if DEBUG)
├── Start poller loop:
│   for each raw killmail from poller:
│   ├── Normalize
│   │   └── On error: log, increment rejected counter, continue
│   ├── Enrich
│   ├── Evaluate rules
│   ├── For each matched action:
│   │   ├── Check idempotency
│   │   ├── Execute (with retries)
│   │   └── Record result
│   ├── Save state (checkpoint + action history)
│   └── Prune old action history (every 1000 killmails)
├── On SIGINT/SIGTERM:
│   ├── Stop poller
│   ├── Save state
│   ├── Send shutdown notification to Discord webhook
│   └── Exit
```

### Step 11: Testing Strategy

- **Unit tests** for each package:
  - `killmail`: normalization of valid/malformed JSON
  - `rules/filters`: each leaf filter + composite logic (and/or/not)
  - `rules`: first-match vs multi-match evaluation
  - `state`: load/save/idempotency/prune
  - `enrichment`: mock DB lookups
- **Integration test**: feed a known killmail JSON through the full pipeline (normalize -> enrich -> evaluate -> action) and verify output
- **Test fixtures**: sample R2Z2 JSON payloads saved as `testdata/*.json`

### Build Order

| Phase | Packages | Milestone |
|---|---|---|
| 1 | config, killmail, state | Can load config, parse a killmail, persist state |
| 2 | poller | Can poll R2Z2 and print raw JSON |
| 3 | enrichment | Can enrich killmails with ship/group names |
| 4 | rules, filters | Can evaluate rules against enriched killmails |
| 5 | actions (console) | End-to-end: poll -> normalize -> enrich -> evaluate -> print |
| 6 | actions (webhook) | Discord notifications working |
| 7 | metrics | Observability counters and periodic logging |
| 8 | startup/shutdown alerts | Discord lifecycle notifications |
| 9 | Tests + hardening | Comprehensive test suite, edge case handling |
