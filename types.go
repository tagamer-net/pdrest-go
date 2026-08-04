package pdrest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ErrorEnvelope is the error body documented by the PalDefender REST API.
type ErrorEnvelope struct {
	// Error holds the error details.
	Error ErrorDetail `json:"Error"`
}

// ErrorDetail are the details of an API error.
type ErrorDetail struct {
	// Code is the machine-readable error code.
	Code string `json:"Code"`
	// Message is the human-readable error message.
	Message string `json:"Message"`
	// Details is the server-provided details payload, when present.
	Details any `json:"Details"`
}

// APIError represents HTTP errors returned by the PalDefender REST API.
type APIError struct {
	// StatusCode is the HTTP status code of the response.
	StatusCode int `json:"status_code"`
	// Method is the HTTP method of the request.
	Method string `json:"method"`
	// Path is the path of the request.
	Path string `json:"path"`
	// Envelope is the decoded documented error envelope, or nil when the
	// response body does not follow the envelope shape.
	Envelope *ErrorEnvelope `json:"envelope,omitempty"`
	// ResponseBody is the decoded body of the error response.
	ResponseBody any `json:"response_body"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("pdrest api error: status=%d method=%s path=%s body=%v", e.StatusCode, e.Method, e.Path, e.ResponseBody)
}

// VersionInfo describes the version of the PalDefender REST API.
type VersionInfo struct {
	// Major is the major version.
	Major int `json:"Major"`
	// Minor is the minor version.
	Minor int `json:"Minor"`
	// Patch is the patch version.
	Patch int `json:"Patch"`
	// Build is the build number.
	Build int `json:"Build"`
	// Version is the short version string.
	Version string `json:"Version"`
	// VersionLong is the long version string.
	VersionLong string `json:"VersionLong"`
	// Beta indicates whether it is a beta version.
	Beta bool `json:"Beta"`
}

// VersionResponse wraps the version details returned by the API.
type VersionResponse struct {
	// Version is the PalDefender version details.
	Version VersionInfo `json:"Version"`
}

// Coordinates are a position in the world or on the map.
type Coordinates struct {
	// X is the X coordinate.
	X float64 `json:"x"`
	// Y is the Y coordinate.
	Y float64 `json:"y"`
	// Z is the Z coordinate.
	Z float64 `json:"z"`
}

// GuildCampSummary summarizes a guild base camp.
type GuildCampSummary struct {
	// ID is the base camp identifier.
	ID string `json:"id"`
	// WorldPos is the position in the world.
	WorldPos Coordinates `json:"world_pos"`
	// MapPos is the position on the map.
	MapPos Coordinates `json:"map_pos"`
}

// GuildAdmin identifies the administrator of a guild.
type GuildAdmin struct {
	// ID is the admin PlayerUID.
	ID string `json:"id"`
	// Name is the admin player name.
	Name string `json:"name"`
}

// GuildSummary summarizes a guild with counts and members.
type GuildSummary struct {
	// Name is the guild name.
	Name string `json:"name"`
	// Level is the guild level.
	Level int `json:"Level"`
	// Admin is the guild administrator.
	Admin GuildAdmin `json:"admin"`
	// CampCount is the number of guild base camps.
	CampCount int `json:"camp_count"`
	// Camps are the summaries of the guild base camps.
	Camps []GuildCampSummary `json:"camps"`
	// MemberCount is the number of guild members.
	MemberCount int `json:"member_count"`
	// Members are the PlayerUIDs of the guild members.
	Members []string `json:"members"`
}

// GuildsMeta describes the guild list returned by the API.
type GuildsMeta struct {
	// GuildCount is the number of guilds returned.
	GuildCount int `json:"GuildCount"`
}

// GuildsResponse maps the guild UUID to GuildSummary.
type GuildsResponse struct {
	// Meta is the guild list metadata.
	Meta GuildsMeta `json:"Meta"`
	// Guilds are the known guild summaries keyed by guild UUID.
	Guilds map[string]GuildSummary `json:"Guilds"`
}

// GuildMember describes a guild member.
type GuildMember struct {
	// PlayerUID is the player identifier.
	PlayerUID string `json:"player_uid"`
	// PlayerName is the player name.
	PlayerName string `json:"player_name"`
	// Status is the player status.
	Status string `json:"status"`
}

// GuildCampPal is a worker pal of a base camp.
type GuildCampPal struct {
	// Nickname is the pal nickname.
	Nickname string `json:"nickname"`
	// PalID is the pal species identifier.
	PalID string `json:"pal_id"`
	// NpcID is the unique NPC ID.
	NpcID string `json:"npc_id"`
	// SkinID is the skin ID.
	SkinID string `json:"skin_id"`
	// Gender is the pal gender.
	Gender string `json:"gender"`
	// Level is the pal level.
	Level int `json:"level"`
	// Shiny indicates whether this is a rare pal.
	Shiny bool `json:"shiny"`
	// PhysicalHealth is the physical health state. The JSON key matches the
	// official documentation spelling "phisical_health".
	PhysicalHealth string `json:"phisical_health"`
	// WorkerSick is the worker sickness state.
	WorkerSick string `json:"worker_sick"`
	// San is the sanity value.
	San float64 `json:"san"`
	// Imported indicates whether this is marked as imported.
	Imported bool `json:"imported"`
	// Friendship is the friendship point value.
	Friendship int `json:"friendship"`
	// ActiveSkills are the equipped active skills.
	ActiveSkills []string `json:"active_skills"`
	// LearntSkills are the learned skills.
	LearntSkills []string `json:"learnt_skills"`
	// Passives are the passive skill IDs.
	Passives []string `json:"passives"`
}

// GuildCampDetail details a guild base camp.
type GuildCampDetail struct {
	// ID is the base camp identifier.
	ID string `json:"id"`
	// WorldPos is the position in the world.
	WorldPos Coordinates `json:"world_pos"`
	// MapPos is the position on the map.
	MapPos Coordinates `json:"map_pos"`
	// Level is the base camp level.
	Level int `json:"level"`
	// State is the base camp state.
	State string `json:"state"`
	// Pals are the worker Pals of the base camp keyed by Pal instance ID.
	Pals map[string]GuildCampPal `json:"pals"`
	// Buildings is the building payload placeholder.
	Buildings string `json:"buildings"`
}

// GuildSlot is an item stored in a guild storage container slot.
type GuildSlot struct {
	// ItemID is the item identifier.
	ItemID string `json:"item_id"`
	// Count is the stack count in the slot.
	Count int `json:"count"`
}

// GuildStorage represents the item container of a guild.
type GuildStorage struct {
	// ContainerID is the container identifier.
	ContainerID string `json:"container_id"`
	// Current is the number of occupied slots.
	Current int `json:"current"`
	// Max is the total slot count.
	Max int `json:"max"`
	// Slots are the items keyed by slot index.
	Slots map[string]GuildSlot
}

// UnmarshalJSON captures the dynamic slot keys of the container.
func (g *GuildStorage) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*g = GuildStorage{}
	for key, value := range raw {
		if err := g.unmarshalKey(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (g *GuildStorage) unmarshalKey(key string, value json.RawMessage) error {
	switch key {
	case "container_id":
		return json.Unmarshal(value, &g.ContainerID)
	case "current":
		return json.Unmarshal(value, &g.Current)
	case "max":
		return json.Unmarshal(value, &g.Max)
	}
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 || trimmed[0] != '{' || !isSlotIndex(key) {
		return nil
	}
	if g.Slots == nil {
		g.Slots = map[string]GuildSlot{}
	}
	var slot GuildSlot
	if err := json.Unmarshal(value, &slot); err != nil {
		return err
	}
	g.Slots[key] = slot
	return nil
}

// isSlotIndex reports whether key is a numeric slot index.
func isSlotIndex(key string) bool {
	_, err := strconv.Atoi(key)
	return err == nil
}

// GuildExpeditions describes the guild expedition state.
type GuildExpeditions struct {
	// Finished is the number of finished expeditions.
	Finished int `json:"finished"`
	// Missions are the released mission flags keyed by mission ID.
	Missions map[string]bool `json:"missions"`
}

// ResearchProgress is the progress of a guild laboratory research.
type ResearchProgress struct {
	// WorkAmount is the current work amount.
	WorkAmount float64 `json:"work_amount"`
	// RequiredWorkAmount is the required work amount.
	RequiredWorkAmount float64 `json:"required_work_amount"`
	// Percentage is the current work divided by the required work.
	Percentage float64 `json:"percentage"`
}

// GuildLaboratory describes the guild laboratory research state.
type GuildLaboratory struct {
	// CurrentResearch is the current research ID.
	CurrentResearch string `json:"current_research"`
	// Researches are the active researches keyed by research ID.
	Researches map[string]ResearchProgress `json:"researches"`
}

// GuildDetail details a guild with members, base camps, and items.
type GuildDetail struct {
	// Name is the guild name.
	Name string `json:"name"`
	// Level is the guild level.
	Level int `json:"Level"`
	// Admin is the guild administrator.
	Admin GuildAdmin `json:"admin"`
	// MemberCount is the number of guild members.
	MemberCount int `json:"member_count"`
	// Members are the guild members.
	Members []GuildMember `json:"members"`
	// CampCount is the number of guild base camps.
	CampCount int `json:"camp_count"`
	// Camps are the guild base camps.
	Camps []GuildCampDetail `json:"camps"`
	// Items are the guild item storage data.
	Items GuildStorage `json:"items"`
	// Expeditions are the guild expeditions.
	Expeditions GuildExpeditions `json:"expeditions"`
	// Laboratory is the guild laboratory.
	Laboratory GuildLaboratory `json:"laboratory"`
}

// GuildResponse wraps the guild details returned by the API.
type GuildResponse struct {
	// Guild is the guild details.
	Guild GuildDetail `json:"Guild"`
}

// PlayerInfo describes a server player.
type PlayerInfo struct {
	// Name is the player name.
	Name string `json:"Name"`
	// IP is the player IP address.
	IP string `json:"IP"`
	// PlayerUID is the player identifier.
	PlayerUID string `json:"PlayerUID"`
	// UserId is the user ID on the platform.
	UserId string `json:"UserId"`
	// GuildName is the name of the player's guild.
	GuildName string `json:"GuildName"`
	// GuildUUID is the UUID of the player's guild.
	GuildUUID string `json:"GuildUUID"`
	// Status is the saved player account state.
	Status string `json:"Status"`
	// WorldLocation is the location in the world.
	WorldLocation Coordinates `json:"WorldLocation"`
	// MapLocation is the location on the map.
	MapLocation Coordinates `json:"MapLocation"`
}

// PlayerResponse wraps the player details returned by the API.
type PlayerResponse struct {
	// Player is the player details.
	Player PlayerInfo `json:"Player"`
}

// PlayersMeta describes the player list returned by the API.
type PlayersMeta struct {
	// PlayerCount is the number of player accounts returned.
	PlayerCount int `json:"PlayerCount"`
	// OnlineCount is the number of returned players currently online.
	OnlineCount int `json:"OnlineCount"`
}

// PlayersResponse lists the known players.
type PlayersResponse struct {
	// Meta is the player list metadata.
	Meta PlayersMeta `json:"Meta"`
	// Players are the known players.
	Players []PlayerInfo `json:"Players"`
}

// PalSouls are the pal soul ranks of a pal.
type PalSouls struct {
	// Health is the health soul rank.
	Health int `json:"Health"`
	// Attack is the attack soul rank.
	Attack int `json:"Attack"`
	// Defense is the defense soul rank.
	Defense int `json:"Defense"`
	// CraftSpeed is the craft speed soul rank.
	CraftSpeed int `json:"CraftSpeed"`
}

// PalIVs are the IV values of a pal.
type PalIVs struct {
	// Health is the health IV.
	Health float64 `json:"Health"`
	// AttackMelee is the melee attack IV.
	AttackMelee float64 `json:"AttackMelee"`
	// AttackShot is the ranged attack IV.
	AttackShot float64 `json:"AttackShot"`
	// Defense is the defense IV.
	Defense float64 `json:"Defense"`
}

// PalInstance represents a pal owned by a player.
type PalInstance struct {
	// PalID is the pal species identifier.
	PalID string `json:"PalID"`
	// UniqueNPCID is the unique NPC ID, when present.
	UniqueNPCID string `json:"UniqueNPCID"`
	// Nickname is the custom pal nickname, or an empty string.
	Nickname string `json:"Nickname"`
	// SkinId is the skin ID, or an empty string.
	SkinId string `json:"SkinId"`
	// Gender is the pal gender.
	Gender string `json:"Gender"`
	// Level is the pal level.
	Level int `json:"Level"`
	// Exp is the pal experience.
	Exp int `json:"Exp"`
	// Shiny indicates whether this is a rare pal.
	Shiny bool `json:"Shiny"`
	// PartnerSkillLevel is the partner skill rank.
	PartnerSkillLevel int `json:"PartnerSkillLevel"`
	// CondensedPals is the condense rank progress.
	CondensedPals int `json:"CondensedPals"`
	// UnusedStatusPoints are the unused pal status points.
	UnusedStatusPoints int `json:"UnusedStatusPoints"`
	// FriendshipPoints is the friendship point value.
	FriendshipPoints int `json:"FriendshipPoints"`
	// PhysicalHealth is the physical health state.
	PhysicalHealth string `json:"PhysicalHealth"`
	// WorkerSick is the worker sickness state.
	WorkerSick string `json:"WorkerSick"`
	// ImportedCharacter indicates whether this is marked as imported.
	ImportedCharacter bool `json:"ImportedCharacter"`
	// HP is the current health.
	HP float64 `json:"HP"`
	// MP is the current MP, when present.
	MP float64 `json:"MP"`
	// SP is the current stamina, when present.
	SP float64 `json:"SP"`
	// Shield is the current shield value, when present.
	Shield float64 `json:"Shield"`
	// Hunger is the current hunger value.
	Hunger float64 `json:"Hunger"`
	// MaxHunger is the maximum hunger value.
	MaxHunger float64 `json:"MaxHunger"`
	// SAN is the sanity value.
	SAN float64 `json:"SAN"`
	// Support is the support value.
	Support int `json:"Support"`
	// CraftSpeed is the craft speed value.
	CraftSpeed int `json:"CraftSpeed"`
	// PalSouls are the pal soul ranks.
	PalSouls PalSouls `json:"PalSouls"`
	// IVs are the pal IV values.
	IVs PalIVs `json:"IVs"`
	// ActiveSkills are the equipped active skills.
	ActiveSkills []string `json:"ActiveSkills"`
	// LearntSkills are the learned skills.
	LearntSkills []string `json:"LearntSkills"`
	// Passives are the passive skill IDs.
	Passives []string `json:"Passives"`
	// ExtraWorkSuitabilities are the additional work suitability ranks keyed by suitability ID.
	ExtraWorkSuitabilities map[string]int `json:"ExtraWorkSuitabilities"`
	// DisableWorkPreferences are the disabled work preference IDs.
	DisableWorkPreferences []string `json:"DisableWorkPreferences"`
	// TeamSlotIndex is the team slot index, only on team pals.
	TeamSlotIndex int `json:"team_slot_index"`
	// Page is the palbox page index, only on palbox pals.
	Page int `json:"page"`
	// Slot is the palbox slot index, only on palbox pals.
	Slot int `json:"slot"`
	// BaseCampSlotIndex is the base camp worker slot index, only on base camp pals.
	BaseCampSlotIndex int `json:"base_camp_slot_index"`
}

// PlayerBaseCamp details a player's base camp.
type PlayerBaseCamp struct {
	// ID is the base camp identifier.
	ID string `json:"id"`
	// Level is the base camp level.
	Level int `json:"level"`
	// WorldPos is the position in the world.
	WorldPos Coordinates `json:"world_pos"`
	// MapPos is the position on the map.
	MapPos Coordinates `json:"map_pos"`
	// State is the base camp state.
	State string `json:"state"`
	// Pals are the worker Pals of the base camp keyed by Pal instance ID.
	Pals map[string]PalInstance `json:"pals"`
}

// PalsData groups the team, palbox, and base camps of a player.
type PalsData struct {
	// Team are the team pals keyed by Pal instance ID.
	Team map[string]PalInstance `json:"Team"`
	// Palbox are the palbox pals keyed by Pal instance ID.
	Palbox map[string]PalInstance `json:"Palbox"`
	// BaseCamps are the base camps of the player.
	BaseCamps []PlayerBaseCamp `json:"BaseCamps"`
}

// PlayerPalsMeta describes the pal data returned by the API.
type PlayerPalsMeta struct {
	// PlayerUID is the player identifier.
	PlayerUID string `json:"PlayerUID"`
	// Player is the player identifier supplied in the request path.
	Player string `json:"Player"`
	// TeamCount is the number of pals in the player team.
	TeamCount int `json:"TeamCount"`
	// PalboxCount is the number of pals in the player palbox.
	PalboxCount int `json:"PalboxCount"`
	// BaseCampCount is the number of base camps included.
	BaseCampCount int `json:"BaseCampCount"`
}

// PlayerPalsResponse groups the team, palbox, and base camps of a player.
type PlayerPalsResponse struct {
	// Meta is the pal data metadata.
	Meta PlayerPalsMeta `json:"Meta"`
	// Pals are the team, palbox, and base camp pals.
	Pals PalsData `json:"Pals"`
}

// InventorySlot represents an item in an inventory slot.
type InventorySlot struct {
	// ItemID is the item identifier.
	ItemID string `json:"ItemID"`
	// Count is the quantity of the item.
	Count int `json:"Count"`
}

// InventorySection represents a section of a player's inventory.
type InventorySection struct {
	// Available indicates whether the container was available.
	Available bool `json:"Available"`
	// ContainerID is the container identifier.
	ContainerID string `json:"ContainerID"`
	// UsedSlots is the number of used slots.
	UsedSlots int `json:"UsedSlots"`
	// MaxSlots is the maximum number of slots.
	MaxSlots int `json:"MaxSlots"`
	// FreeSlots is the number of free slots.
	FreeSlots int `json:"FreeSlots"`
	// Slots are the item slots of the section.
	Slots map[string]InventorySlot `json:"Slots"`
}

// PlayerInventory groups the inventory sections of a player.
type PlayerInventory struct {
	// Items is the items section.
	Items InventorySection `json:"Items"`
	// KeyItems is the key items section.
	KeyItems InventorySection `json:"KeyItems"`
	// Weapons is the weapons section.
	Weapons InventorySection `json:"Weapons"`
	// Armor is the armor section.
	Armor InventorySection `json:"Armor"`
	// Food is the food section.
	Food InventorySection `json:"Food"`
	// DropSlot is the dropped items section.
	DropSlot InventorySection `json:"DropSlot"`
}

// PlayerItemsMeta describes the inventory returned by the API.
type PlayerItemsMeta struct {
	// PlayerUID is the player identifier.
	PlayerUID string `json:"PlayerUID"`
	// Player is the player identifier supplied in the request path.
	Player string `json:"Player"`
}

// PlayerItemsResponse groups the inventory containers of a player.
type PlayerItemsResponse struct {
	// Meta is the inventory metadata.
	Meta PlayerItemsMeta `json:"Meta"`
	// Inventory are the inventory containers of the player.
	Inventory PlayerInventory `json:"Inventory"`
}

// PlayerTechsMeta describes the technology data returned by the API.
type PlayerTechsMeta struct {
	// PlayerUID is the player identifier.
	PlayerUID string `json:"PlayerUID"`
	// Player is the player identifier supplied in the request path.
	Player string `json:"Player"`
	// UnlockedCount is the number of unlocked technologies.
	UnlockedCount int `json:"UnlockedCount"`
	// LockedCount is the number of locked technologies.
	LockedCount int `json:"LockedCount"`
	// TotalCount is the total number of technologies.
	TotalCount int `json:"TotalCount"`
}

// PlayerTechsData describes the technologies unlocked by a player.
type PlayerTechsData struct {
	// Unlocked are the IDs of the unlocked technologies.
	Unlocked []string `json:"Unlocked"`
}

// PlayerTechsResponse describes the technologies of a player.
type PlayerTechsResponse struct {
	// Meta is the technology count metadata.
	Meta PlayerTechsMeta `json:"Meta"`
	// Techs are the technology data of the player.
	Techs PlayerTechsData `json:"Techs"`
}

// ProgressionPlayer describes a player's level and experience.
type ProgressionPlayer struct {
	// Level is the player level.
	Level int `json:"level"`
	// Exp is the player's experience.
	Exp int `json:"exp"`
	// UnusedStatusPoints are the unused status points.
	UnusedStatusPoints int `json:"unusedStatusPoints"`
}

// ProgressionCurrencies describes a player's currencies.
type ProgressionCurrencies struct {
	// Relics are the relic point totals keyed by relic type.
	Relics map[string]int `json:"relics"`
	// TechnologyPoints is the number of technology points.
	TechnologyPoints int `json:"technologyPoints"`
	// AncientTechnologyPoints is the number of ancient technology points.
	AncientTechnologyPoints int `json:"ancientTechnologyPoints"`
}

// ProgressionBosses describes the player's progress against bosses.
type ProgressionBosses struct {
	// TowerBossDefeatCounts are the tower boss defeat counts keyed by boss ID.
	TowerBossDefeatCounts map[string]int `json:"towerBossDefeatCounts"`
	// NormalBossDefeatFlags are the normal boss defeat flags keyed by boss ID.
	NormalBossDefeatFlags map[string]bool `json:"normalBossDefeatFlags"`
	// RaidBossDefeatCounts are the raid boss defeat counts keyed by boss ID.
	RaidBossDefeatCounts map[string]int `json:"raidBossDefeatCounts"`
	// TotalBossDefeatCount is the total number of defeated bosses.
	TotalBossDefeatCount int `json:"totalBossDefeatCount"`
	// PredatorDefeatCount is the number of defeated predators.
	PredatorDefeatCount int `json:"predatorDefeatCount"`
}

// ProgressionCaptures describes the player's pal captures.
type ProgressionCaptures struct {
	// TribeCaptureCount is the number of captures per tribe.
	TribeCaptureCount int `json:"tribeCaptureCount"`
	// PalCaptureCounts are the capture counts keyed by Pal ID.
	PalCaptureCounts map[string]int `json:"palCaptureCounts"`
	// PalCaptureBonusCounts are the capture bonus counts keyed by Pal ID.
	PalCaptureBonusCounts map[string]int `json:"palCaptureBonusCounts"`
	// PalButcherCounts are the butcher counts keyed by Pal ID.
	PalButcherCounts map[string]int `json:"palButcherCounts"`
}

// ProgressionActivities describes the activities performed by the player.
type ProgressionActivities struct {
	// CraftItemCounts are the crafted item counts keyed by item ID.
	CraftItemCounts map[string]int `json:"craftItemCounts"`
	// NormalDungeonClearCount is the number of completed normal dungeons.
	NormalDungeonClearCount int `json:"normalDungeonClearCount"`
	// FixedDungeonClearCount is the number of completed fixed dungeons.
	FixedDungeonClearCount int `json:"fixedDungeonClearCount"`
	// OilrigClearCount is the number of completed oil rigs.
	OilrigClearCount int `json:"oilrigClearCount"`
	// PalRankUpCounts are the pal rank-up counts keyed by Pal ID.
	PalRankUpCounts map[string]int `json:"palRankUpCounts"`
	// ArenaSoloClearCounts are the arena solo clear counts keyed by arena ID.
	ArenaSoloClearCounts map[string]int `json:"arenaSoloClearCounts"`
	// NPCTalkCounts are the NPC talk counts keyed by NPC ID.
	NPCTalkCounts map[string]int `json:"npcTalkCounts"`
	// FishingCounts are the fishing counts keyed by fish ID.
	FishingCounts map[string]int `json:"fishingCounts"`
	// FoundTreasureCount is the number of found treasures.
	FoundTreasureCount int `json:"foundTreasureCount"`
	// CampConqueredCount is the number of conquered base camps.
	CampConqueredCount int `json:"campConqueredCount"`
	// FirstFishingComplete indicates whether the first fishing session was completed.
	FirstFishingComplete bool `json:"firstFishingComplete"`
}

// PlayerProgressionData aggregates the complete progression of a player.
type PlayerProgressionData struct {
	// Player is the level and experience progress.
	Player ProgressionPlayer `json:"Player"`
	// Currencies are the player's currencies.
	Currencies ProgressionCurrencies `json:"Currencies"`
	// Bosses is the progress against bosses.
	Bosses ProgressionBosses `json:"Bosses"`
	// Captures is the capture progress.
	Captures ProgressionCaptures `json:"Captures"`
	// Activities is the activities progress.
	Activities ProgressionActivities `json:"Activities"`
}

// PlayerProgressionMeta describes the progression returned by the API.
type PlayerProgressionMeta struct {
	// PlayerUID is the player identifier.
	PlayerUID string `json:"PlayerUID"`
	// Player is the player identifier supplied in the request path.
	Player string `json:"Player"`
}

// PlayerProgressionResponse aggregates the complete progression of a player.
type PlayerProgressionResponse struct {
	// Meta is the progression metadata.
	Meta PlayerProgressionMeta `json:"Meta"`
	// Progression is the progression data of the player.
	Progression PlayerProgressionData `json:"Progression"`
}

// GiveItem is the input for giving an item to a player.
type GiveItem struct {
	// ItemID is the item identifier.
	ItemID string `json:"ItemID"`
	// Count is the quantity of the item.
	Count int `json:"Count"`
}

// GivePal is the input for giving a pal to a player.
type GivePal struct {
	// PalID is the pal identifier.
	PalID string `json:"PalID"`
	// Level is the pal level.
	Level int `json:"Level"`
}

// GivePalEgg is the input for giving a pal egg to a player.
type GivePalEgg struct {
	// EggID is the egg identifier.
	EggID string `json:"EggID"`
	// PalID is the identifier of the pal contained in the egg.
	PalID string `json:"PalID,omitempty"`
	// PalTemplate is the template of the pal contained in the egg.
	PalTemplate string `json:"PalTemplate,omitempty"`
	// Level is the level of the pal at birth.
	Level int `json:"Level,omitempty"`
}

// GiveProgressionRequest defines the progression fields to be granted.
type GiveProgressionRequest struct {
	// EXP is the experience to grant.
	EXP int `json:"EXP,omitempty"`
	// TechnologyPoints is the number of technology points to grant.
	TechnologyPoints int `json:"TechnologyPoints,omitempty"`
	// AncientTechnologyPoints is the number of ancient technology points to grant.
	AncientTechnologyPoints int `json:"AncientTechnologyPoints,omitempty"`
	// Relics are the relic point amounts to grant keyed by relic type.
	Relics map[string]int `json:"Relics,omitempty"`
}

// GrantValues are the values granted by a grant endpoint. Only the fields
// documented for the called endpoint are populated.
type GrantValues struct {
	// EXP is the experience granted, on progression grants.
	EXP int `json:"EXP,omitempty"`
	// TechnologyPoints are the technology points granted, on progression grants.
	TechnologyPoints int `json:"TechnologyPoints,omitempty"`
	// AncientTechnologyPoints are the ancient technology points granted, on progression grants.
	AncientTechnologyPoints int `json:"AncientTechnologyPoints,omitempty"`
	// Relics are the relic point amounts granted by relic type, on progression grants.
	Relics map[string]int `json:"Relics,omitempty"`
	// Items is the number of item units granted, on item grants.
	Items int `json:"Items,omitempty"`
	// Pals is the number of Pals granted, on pal grants.
	Pals int `json:"Pals,omitempty"`
	// PalTemplates is the number of Pal templates granted, on pal template grants.
	PalTemplates int `json:"PalTemplates,omitempty"`
	// PalEggs is the number of Pal eggs granted, on pal egg grants.
	PalEggs int `json:"PalEggs,omitempty"`
}

// GrantTotals are the updated currency totals returned by a grant endpoint.
type GrantTotals struct {
	// TechnologyPoints is the updated technology point total.
	TechnologyPoints int `json:"TechnologyPoints,omitempty"`
	// AncientTechnologyPoints is the updated ancient technology point total.
	AncientTechnologyPoints int `json:"AncientTechnologyPoints,omitempty"`
	// Relics are the updated relic point totals keyed by relic type.
	Relics map[string]int `json:"Relics,omitempty"`
}

// GrantResult describes the result of a grant operation.
type GrantResult struct {
	// Granted are the values granted by the request.
	Granted GrantValues `json:"Granted"`
	// Totals are the updated totals for granted currencies, when applicable.
	Totals GrantTotals `json:"Totals,omitempty"`
}

// LearnTechResponse describes the result of learning technologies.
type LearnTechResponse struct {
	// UnlockedCount is the number of technologies unlocked by the request.
	UnlockedCount int `json:"UnlockedCount"`
	// Unlocked are the technology IDs that were unlocked.
	Unlocked []string `json:"Unlocked"`
	// Skipped are the technology IDs skipped because they were already unlocked.
	Skipped []string `json:"Skipped"`
}

// ForgottenTechs are the technology IDs removed by a forget request. All
// reports whether every unlocked technology was removed ("All").
type ForgottenTechs struct {
	// IDs are the removed technology IDs.
	IDs []string
	// All indicates whether every unlocked technology was removed.
	All bool
}

// UnmarshalJSON accepts a single technology ID, the string "All", or an array
// of technology IDs.
func (f *ForgottenTechs) UnmarshalJSON(data []byte) error {
	*f = ForgottenTechs{}
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		if strings.EqualFold(value, "All") {
			f.All = true
			return nil
		}
		f.IDs = []string{value}
		return nil
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return fmt.Errorf("forgotten must be a technology ID, %q, or an array: %w", "All", err)
	}
	f.IDs = ids
	return nil
}

// ForgetTechResponse describes the result of forgetting technologies.
type ForgetTechResponse struct {
	// ForgottenCount is the number of technologies removed from the player.
	ForgottenCount int `json:"ForgottenCount"`
	// Forgotten are the removed technology IDs, or All when every unlocked
	// technology was removed.
	Forgotten ForgottenTechs `json:"Forgotten"`
	// Skipped are the technology IDs skipped because they were not unlocked.
	Skipped []string `json:"Skipped"`
}

// DeleteBaseCamp is the summary of a deleted base camp.
type DeleteBaseCamp struct {
	// ID is the base camp GUID.
	ID string `json:"Id"`
	// Summary is the human-readable base camp summary.
	Summary string `json:"Summary"`
}

// DeleteBaseDeleted are the cleanup counts of a deleted base camp.
type DeleteBaseDeleted struct {
	// BaseCampPals is the number of base camp pals deleted.
	BaseCampPals int `json:"BaseCampPals"`
	// StorageContainers is the number of storage containers cleared.
	StorageContainers int `json:"StorageContainers"`
	// ItemStacks is the number of item stacks deleted.
	ItemStacks int `json:"ItemStacks"`
	// ItemCount is the total item count deleted.
	ItemCount int `json:"ItemCount"`
	// Buildings is the number of buildings deleted.
	Buildings int `json:"Buildings"`
	// DropItems is the number of dropped item actors deleted.
	DropItems int `json:"DropItems"`
	// DefenseModels is the number of defense models deleted.
	DefenseModels int `json:"DefenseModels"`
	// OtherMapObjects is the number of other map objects deleted.
	OtherMapObjects int `json:"OtherMapObjects"`
	// PalBox indicates whether the base Palbox was deleted.
	PalBox bool `json:"PalBox"`
}

// DeleteBaseResponse describes the result of deleting a base camp.
type DeleteBaseResponse struct {
	// BaseCamp is the deleted base camp summary.
	BaseCamp DeleteBaseCamp `json:"BaseCamp"`
	// Deleted are the cleanup counts of the deleted base camp.
	Deleted DeleteBaseDeleted `json:"Deleted"`
	// Archive is the path to the generated audit archive.
	Archive string `json:"Archive"`
}

// BanRequest is the body of a player ban request.
type BanRequest struct {
	// Reason is the ban reason.
	Reason string `json:"Reason,omitempty"`
	// IP indicates whether the ban should include the IP.
	IP bool `json:"IP,omitempty"`
}

// BanResponse describes the result of banning a player.
type BanResponse struct {
	// Success indicates whether the ban was successful.
	Success bool `json:"Success"`
	// UserId is the ID of the banned user.
	UserId string `json:"UserId"`
	// IP indicates whether the IP was banned.
	IP bool `json:"IP"`
	// BannedIP is the banned IP.
	BannedIP string `json:"BannedIP"`
	// Kicked is the number of kicked players.
	Kicked int `json:"Kicked"`
}

// UnbanRequest is the body of a player unban request.
type UnbanRequest struct {
	// Reason is the unban reason.
	Reason string `json:"Reason,omitempty"`
}

// UnbanResponse describes the result of unbanning a player.
type UnbanResponse struct {
	// Success indicates whether the unban was successful.
	Success bool `json:"Success"`
	// UserId is the ID of the unbanned user.
	UserId string `json:"UserId"`
}

// BanIPRequest is the body of an IP ban request.
type BanIPRequest struct {
	// Reason is the ban reason.
	Reason string `json:"Reason,omitempty"`
	// UserId is the ID of the user associated with the IP.
	UserId string `json:"UserId,omitempty"`
}

// BanIPResponse describes the result of banning an IP.
type BanIPResponse struct {
	// Success indicates whether the ban was successful.
	Success bool `json:"Success"`
	// IP is the banned IP.
	IP string `json:"IP"`
	// UserId is the ID of the user associated with the IP.
	UserId string `json:"UserId"`
	// Kicked is the number of kicked players.
	Kicked int `json:"Kicked"`
}

// UnbanIPRequest is the body of an IP unban request.
type UnbanIPRequest struct {
	// Reason is the unban reason.
	Reason string `json:"Reason,omitempty"`
}

// UnbanIPResponse describes the result of unbanning an IP.
type UnbanIPResponse struct {
	// Success indicates whether the unban was successful.
	Success bool `json:"Success"`
	// IP is the unbanned IP.
	IP string `json:"IP"`
}

// KickRequest is the body of a player kick request.
type KickRequest struct {
	// Reason is the kick reason.
	Reason string `json:"Reason,omitempty"`
}

// KickResponse describes the result of kicking a player.
type KickResponse struct {
	// Success indicates whether the kick was successful.
	Success bool `json:"Success"`
	// UserId is the ID of the kicked user.
	UserId string `json:"UserId"`
}

// BroadcastRequest is the body of a broadcast request.
type BroadcastRequest struct {
	// Message is the broadcast message.
	Message string `json:"Message"`
	// Sender is the name of the broadcast sender.
	Sender string `json:"Sender,omitempty"`
}

// BroadcastResponse describes the result of a broadcast.
type BroadcastResponse struct {
	// Success indicates whether the broadcast was successful.
	Success bool `json:"Success"`
}

// AlertRequest is the body of an alert request.
type AlertRequest struct {
	// Message is the alert message.
	Message string `json:"Message"`
}

// AlertResponse describes the result of an alert.
type AlertResponse struct {
	// Success indicates whether the alert was sent.
	Success bool `json:"Success"`
}

// ReloadConfigResponse describes the result of reloading the configuration.
type ReloadConfigResponse struct {
	// Success indicates whether the configuration was reloaded.
	Success bool `json:"Success"`
}

// SendPlayerMessageRequest is the body of a request to send a message to players.
type SendPlayerMessageRequest struct {
	// SendType is the message send type.
	SendType string `json:"SendType"`
	// Message is the message to send.
	Message string `json:"Message"`
	// UserID is the ID of the destination user.
	UserID string `json:"UserID,omitempty"`
	// UserIDs are the IDs of the destination users.
	UserIDs []string `json:"UserIDs,omitempty"`
	// Sender is the name of the message sender.
	Sender string `json:"Sender,omitempty"`
}

// SendPlayerMessageResponse describes the result of sending a message.
type SendPlayerMessageResponse struct {
	// Success indicates whether the message was sent.
	Success bool `json:"Success"`
	// SentCount is the number of sent messages.
	SentCount int `json:"SentCount"`
}

// BanlistTimestamp describes UTC timestamp components of an action.
type BanlistTimestamp struct {
	// UTC is the Unix timestamp in seconds.
	UTC int64 `json:"UTC"`
	// Year is the UTC year.
	Year int `json:"Year"`
	// Month is the UTC month.
	Month int `json:"Month"`
	// Day is the UTC day of month.
	Day int `json:"Day"`
	// Hour is the UTC hour.
	Hour int `json:"Hour"`
	// Min is the UTC minute.
	Min int `json:"Min"`
	// Sec is the UTC second.
	Sec int `json:"Sec"`
	// Msec is the millisecond component.
	Msec int `json:"Msec"`
}

// BanlistIssuer describes the issuer of a ban or unban action.
type BanlistIssuer struct {
	// Type is the issuer type, such as rest, player, or system.
	Type string `json:"Type"`
	// NameValue is the issuer name, user ID, token, or type fallback.
	NameValue string `json:"NameValue"`
	// IP is the issuer IP address metadata.
	IP string `json:"IP"`
	// Reason is the reason recorded for the action.
	Reason string `json:"Reason"`
	// Timestamp is the UTC timestamp of the action.
	Timestamp BanlistTimestamp `json:"Timestamp"`
}

// BanlistUserEntry is a user ban entry of the ban list.
type BanlistUserEntry struct {
	// UserId is the banned user ID.
	UserId string `json:"UserId"`
	// Active indicates whether the ban is currently active.
	Active bool `json:"Active"`
	// BannedBy is the issuer data for the ban action.
	BannedBy BanlistIssuer `json:"BannedBy"`
	// UnbannedBy is the issuer data for the unban action, when present.
	UnbannedBy *BanlistIssuer `json:"UnbannedBy,omitempty"`
}

// BanlistIPEntry is an IP ban entry of the ban list.
type BanlistIPEntry struct {
	// IP is the banned IP address.
	IP string `json:"IP"`
	// Active indicates whether the ban is currently active.
	Active bool `json:"Active"`
	// BannedBy is the issuer data for the ban action.
	BannedBy BanlistIssuer `json:"BannedBy"`
	// UnbannedBy is the issuer data for the unban action, when present.
	UnbannedBy *BanlistIssuer `json:"UnbannedBy,omitempty"`
}

// BanlistData is the ban list content returned by the API.
type BanlistData struct {
	// Version is the ban list file format version.
	Version int `json:"Version"`
	// BannedMessage is the message shown to banned players.
	BannedMessage string `json:"BannedMessage"`
	// UserEntries are the user ban entries matching the applied filters.
	UserEntries []BanlistUserEntry `json:"UserEntries"`
	// IPEntries are the IP ban entries matching the applied filters.
	IPEntries []BanlistIPEntry `json:"IPEntries"`
}

// BanlistResponse wraps the ban list content returned by the API.
type BanlistResponse struct {
	// Banlist is the ban list data after applying query filters.
	Banlist BanlistData `json:"Banlist"`
}
