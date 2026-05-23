// Package godville provides an HTTP client for the Godville public/private API.
package godville

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/cockroachdb/errors"
)

// Pet represents the hero's pet. Has a custom UnmarshalJSON because the
// Godville API returns `"pet_level": ""` (empty string) when the pet has
// just lost a level — a common gameplay event. A plain `int` field would
// fail to decode for any pet in that state.
type Pet struct {
	Name    string `json:"pet_name,omitempty"`
	Class   string `json:"pet_class,omitempty"`
	Level   int    `json:"pet_level,omitempty"`
	Wounded bool   `json:"wounded,omitempty"`
}

// petRaw mirrors Pet but uses a tolerant level type so we can normalise the
// empty-string-means-zero quirk in one place.
type petRaw struct {
	Name    string          `json:"pet_name,omitempty"`
	Class   string          `json:"pet_class,omitempty"`
	Level   json.RawMessage `json:"pet_level,omitempty"`
	Wounded bool            `json:"wounded,omitempty"`
}

// UnmarshalJSON tolerates the "pet lost a level" form: `pet_level: ""`.
func (pet *Pet) UnmarshalJSON(data []byte) error {
	var raw petRaw

	err := json.Unmarshal(data, &raw)
	if err != nil {
		return errors.Wrap(err, "decode pet")
	}

	pet.Name = raw.Name
	pet.Class = raw.Class
	pet.Wounded = raw.Wounded

	level, err := decodeIntOrEmptyString(raw.Level)
	if err != nil {
		return errors.Wrap(err, "decode pet_level")
	}

	pet.Level = level

	return nil
}

// HasContent reports whether the Pet carries any actual data. An empty JSON
// object {"pet": {}} unmarshalls into a non-nil zero Pet; downstream tools
// should treat that as "no pet" rather than render leading-space artefacts.
func (pet *Pet) HasContent() bool {
	if pet == nil {
		return false
	}

	return pet.Name != "" || pet.Class != "" || pet.Level > 0 || pet.Wounded
}

// Hero is the full hero information returned by the Godville API. Private
// fields are only populated when the request is authenticated with a userkey.
type Hero struct {
	// Identity and persona (public).
	Name      string `json:"name,omitempty"`
	Godname   string `json:"godname,omitempty"`
	Gender    string `json:"gender,omitempty"`
	Motto     string `json:"motto,omitempty"`
	Alignment string `json:"alignment,omitempty"`
	Clan      string `json:"clan,omitempty"`
	ClanPos   string `json:"clan_position,omitempty"`

	// Levels and progression (public).
	Level        int `json:"level,omitempty"`
	MaxHealth    int `json:"max_health,omitempty"`
	InventoryMax int `json:"inventory_max_num,omitempty"`
	BricksCnt    int `json:"bricks_cnt,omitempty"`
	WoodCnt      int `json:"wood_cnt,omitempty"`
	ArenaWon     int `json:"arena_won,omitempty"`
	ArenaLost    int `json:"arena_lost,omitempty"`
	TLevel       int `json:"t_level,omitempty"`
	ArkMale      int `json:"ark_m,omitempty"`
	ArkFemale    int `json:"ark_f,omitempty"`

	// Percent-style fields: the API documents these as strings (e.g. "25")
	// not ints. A plain int declaration fails to decode on any populated
	// hero.
	SoulsPercent  string `json:"souls_percent,omitempty"`
	RelicsPercent string `json:"relics_percent,omitempty"`

	// Counters with arbitrary scalars. Words/Savings are public; GoldApprox
	// is documented as private (requires userkey) per the Russian wiki.
	Words      int    `json:"words,omitempty"`
	Savings    string `json:"savings,omitempty"`
	GoldApprox string `json:"gold_approx,omitempty"`

	// Long-form goal completions (public).
	TempleCompletedAt  string `json:"temple_completed_at,omitempty"`
	ArkCompletedAt     string `json:"ark_completed_at,omitempty"`
	SavingsCompletedAt string `json:"savings_completed_at,omitempty"`
	BookAt             string `json:"book_at,omitempty"`
	SoulsAt            string `json:"souls_at,omitempty"`
	PairsAt            string `json:"pairs_at,omitempty"`
	ShopName           string `json:"shop_name,omitempty"`
	ArkName            string `json:"ark_name,omitempty"`

	// Combat snapshot (public). `boss_power` is numeric per the API.
	BossName  string `json:"boss_name,omitempty"`
	BossPower int    `json:"boss_power,omitempty"`

	// Pet (public — class/name/level visible without userkey).
	Pet *Pet `json:"pet,omitempty"`

	// Private fields (require userkey).
	Health          int    `json:"health,omitempty"`
	Godpower        int    `json:"godpower,omitempty"`
	Experience      int    `json:"exp_progress,omitempty"`
	InventoryNum    int    `json:"inventory_num,omitempty"`
	Quest           string `json:"quest,omitempty"`
	QuestProgress   int    `json:"quest_progress,omitempty"`
	SideJob         string `json:"side_job,omitempty"`
	SideJobProgress int    `json:"side_job_progress,omitempty"`
	Distance        int    `json:"distance,omitempty"`
	TownName        string `json:"town_name,omitempty"`
	DiaryLast       string `json:"diary_last,omitempty"`
	EyeLast         string `json:"eye_last,omitempty"`
	Aura            string `json:"aura,omitempty"`
	FightType       string `json:"fight_type,omitempty"`
	// ArenaFight is a bool per the API ("are we currently in an arena/boss/
	// dungeon/swim/polygon fight"). Declared as bool, NOT string.
	ArenaFight   bool     `json:"arena_fight,omitempty"`
	Activatables []string `json:"activatables,omitempty"`
	// Inventory was removed upstream and replaced by activatables, but the
	// API still returns an empty object for backward compat. Kept for that
	// edge case; new accounts will not populate it.
	Inventory map[string]int `json:"inventory,omitempty"`
	Expired   bool           `json:"expired,omitempty"`

	// Raw holds the original JSON payload as a generic map so the hero_raw
	// tool can expose fields we have not modelled explicitly. NOTE: numbers
	// in this map are float64 — large integers (IDs, timestamps, counters
	// past 2^53) lose precision. Use RawBytes for byte-exact inspection.
	Raw map[string]any `json:"-"`

	// RawBytes is the original API response body, untouched. Use this to
	// inspect numeric fields without the float64 precision loss inherent
	// in decoding into map[string]any.
	RawBytes []byte `json:"-"`
}

// ErrorPayload is what the Godville API returns when access is denied or
// rate-limited.
type ErrorPayload struct {
	Error string `json:"error,omitempty"`
}

// decodeIntOrEmptyString accepts an int (1, 7) or an empty string ("") and
// returns the int (or 0 for the empty case). Used for pet_level which the
// API documents as numeric but returns as "" when the pet has lost a level.
func decodeIntOrEmptyString(data json.RawMessage) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte(`""`)) {
		return 0, nil
	}

	if trimmed[0] == '"' {
		var str string

		err := json.Unmarshal(trimmed, &str)
		if err != nil {
			return 0, errors.Wrap(err, "decode string scalar")
		}

		if str == "" {
			return 0, nil
		}

		val, err := strconv.Atoi(str)
		if err != nil {
			return 0, errors.Wrapf(err, "parse int from quoted scalar %q", str)
		}

		return val, nil
	}

	var val int

	err := json.Unmarshal(trimmed, &val)
	if err != nil {
		return 0, errors.Wrap(err, "decode int scalar")
	}

	return val, nil
}
