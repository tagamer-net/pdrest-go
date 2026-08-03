package pdrest

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
)

type (
	// RecipeResolver resolves the materials of a recipe by product ID.
	RecipeResolver func(product string) (map[string]int, error)
	// ItemInput accepts strings, GiveItem objects, dictionaries, or (item_id, count) tuples.
	ItemInput any
	// PalInput accepts strings, GivePal objects, dictionaries, or (pal_id, level) tuples.
	PalInput any
	// PalEggInput accepts GivePalEgg objects, dictionaries, or tuples. In
	// tuple form a trailing ".json" value is treated as a PalTemplate and
	// any other value as a PalID.
	PalEggInput any
	// TechnologyInput accepts technology IDs as strings.
	TechnologyInput any
)

// toAnySlice converts a typed input slice to a slice of any for shared handling.
func toAnySlice[T any](values []T) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

func normalizeItemInputs(values []ItemInput) ([]GiveItem, error) {
	valuesAny := flattenSingleSequence(toAnySlice(values))
	acc := &itemCounts{counts: map[string]int{}}

	for _, value := range valuesAny {
		if err := addItemValue(value, acc); err != nil {
			return nil, err
		}
	}

	if len(acc.counts) == 0 {
		return nil, errors.New("at least one item must be provided")
	}

	normalized := make([]GiveItem, 0, len(acc.order))
	for _, itemID := range acc.order {
		normalized = append(normalized, GiveItem{ItemID: itemID, Count: acc.counts[itemID]})
	}
	return normalized, nil
}

// itemCounts accumulates item quantities while preserving the first-seen
// order of the item IDs.
type itemCounts struct {
	counts map[string]int
	order  []string
}

func (a *itemCounts) add(itemID string, count int) error {
	if count > math.MaxInt-a.counts[itemID] {
		return fmt.Errorf("cumulative count for %s exceeds the maximum integer value", itemID)
	}
	if _, ok := a.counts[itemID]; !ok {
		a.order = append(a.order, itemID)
	}
	a.counts[itemID] += count
	return nil
}

// asInt converts value to int when it holds an integral JSON-compatible
// number; non-integral floats and out-of-range values are rejected. int64
// inputs are converted directly without a range check.
func asInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		if v != math.Trunc(v) || v >= float64(math.MaxInt) || v < float64(math.MinInt) {
			return 0, false
		}
		return int(v), true
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		if f != math.Trunc(f) || f >= float64(math.MaxInt) || f < float64(math.MinInt) {
			return 0, false
		}
		return int(f), true
	}
	return 0, false
}

// addItemValue dispatches a single item input to its shape-specific handler.
func addItemValue(value any, acc *itemCounts) error {
	switch v := value.(type) {
	case string:
		if v == "" {
			return errors.New("item_id must not be empty")
		}
		return acc.add(v, 1)
	case GiveItem:
		return addItemObject(v, acc)
	case map[string]any:
		return addItemMap(v, acc)
	case []any:
		return addItemTuple(v, acc)
	default:
		return errors.New("items must be strings, GiveItem objects, dictionaries, or (item_id, count) tuples")
	}
}

func addItemObject(v GiveItem, acc *itemCounts) error {
	if v.ItemID == "" {
		return errors.New("item_id must not be empty")
	}
	if v.Count <= 0 {
		return errors.New("count must be a positive integer")
	}
	return acc.add(v.ItemID, v.Count)
}

func addItemMap(m map[string]any, acc *itemCounts) error {
	itemID, ok := m["ItemID"].(string)
	if !ok || itemID == "" {
		return errors.New("itemID must be a non-empty string")
	}
	count, ok := asInt(m["Count"])
	if !ok {
		return errors.New("count must be an integer")
	}
	if count <= 0 {
		return errors.New("count must be a positive integer")
	}
	return acc.add(itemID, count)
}

func addItemTuple(t []any, acc *itemCounts) error {
	if len(t) != 2 {
		return errors.New("item tuples must contain exactly (item_id, count)")
	}
	itemID, ok := t[0].(string)
	if !ok || itemID == "" {
		return errors.New("item tuple item_id must be a string")
	}
	count, ok := asInt(t[1])
	if !ok {
		return errors.New("item tuple count must be an integer")
	}
	if count <= 0 {
		return errors.New("item tuple count must be a positive integer")
	}
	return acc.add(itemID, count)
}

func normalizePalInputs(values []PalInput) ([]GivePal, error) {
	valuesAny := flattenSingleSequence(toAnySlice(values))
	normalized := make([]GivePal, 0, len(valuesAny))

	for _, value := range valuesAny {
		pal, err := palFromValue(value)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, pal)
	}

	if len(normalized) == 0 {
		return nil, errors.New("at least one pal must be provided")
	}
	return normalized, nil
}

// palFromValue dispatches a single pal input to its shape-specific handler.
func palFromValue(value any) (GivePal, error) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return GivePal{}, errors.New("pal_id must not be empty")
		}
		return GivePal{PalID: v, Level: 1}, nil
	case GivePal:
		return palFromObject(v)
	case map[string]any:
		return palFromMap(v)
	case []any:
		return palFromTuple(v)
	default:
		return GivePal{}, errors.New("pals must be strings, GivePal objects, dictionaries with PalID and Level, or (pal_id, level) tuples")
	}
}

func palFromObject(v GivePal) (GivePal, error) {
	if v.PalID == "" {
		return GivePal{}, errors.New("pal_id must not be empty")
	}
	if v.Level <= 0 {
		return GivePal{}, errors.New("level must be a positive integer")
	}
	return v, nil
}

func palFromMap(m map[string]any) (GivePal, error) {
	palID, ok := m["PalID"].(string)
	level, levelOK := asInt(m["Level"])
	if ok && levelOK {
		if palID == "" {
			return GivePal{}, errors.New("palID must not be empty")
		}
		if level <= 0 {
			return GivePal{}, errors.New("level must be a positive integer")
		}
		return GivePal{PalID: palID, Level: level}, nil
	}
	return GivePal{}, errors.New("pals must be strings, GivePal objects, dictionaries with PalID and Level, or (pal_id, level) tuples")
}

func palFromTuple(t []any) (GivePal, error) {
	if len(t) != 2 {
		return GivePal{}, errors.New("pal tuples must contain exactly (pal_id, level)")
	}
	palID, ok := t[0].(string)
	if !ok || palID == "" {
		return GivePal{}, errors.New("pal tuple pal_id must be a string")
	}
	level, ok := asInt(t[1])
	if !ok {
		return GivePal{}, errors.New("pal tuple level must be an integer")
	}
	if level <= 0 {
		return GivePal{}, errors.New("pal tuple level must be a positive integer")
	}
	return GivePal{PalID: palID, Level: level}, nil
}

func normalizePalEggInputs(values []PalEggInput) ([]GivePalEgg, error) {
	valuesAny := flattenPalEggInputs(values)
	normalized := make([]GivePalEgg, 0, len(valuesAny))

	for _, value := range valuesAny {
		egg, err := eggFromValue(value)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, egg)
	}

	if len(normalized) == 0 {
		return nil, errors.New("at least one pal egg must be provided")
	}
	return normalized, nil
}

// flattenPalEggInputs flattens a single slice of egg inputs unless it is a
// scalar-only tuple (strings are not valid egg inputs, so a scalar-only
// slice can only be an egg tuple).
func flattenPalEggInputs(values []PalEggInput) []any {
	valuesAny := toAnySlice(values)
	if len(valuesAny) != 1 {
		return valuesAny
	}
	arr, ok := tupleToAnySlice(valuesAny[0])
	if !ok {
		return valuesAny
	}
	for _, elem := range arr {
		switch elem.(type) {
		case string, int, int64, float64, json.Number:
		default:
			return arr
		}
	}
	return valuesAny
}

// eggFromValue dispatches a single pal egg input to its shape-specific handler.
func eggFromValue(value any) (GivePalEgg, error) {
	switch v := value.(type) {
	case GivePalEgg:
		return eggFromObject(v)
	case map[string]any:
		return eggFromMap(v)
	case []any:
		return eggFromTuple(v)
	default:
		return GivePalEgg{}, errors.New("pal eggs must be GivePalEgg objects, dictionaries, or tuples")
	}
}

func eggFromObject(v GivePalEgg) (GivePalEgg, error) {
	if v.EggID == "" {
		return GivePalEgg{}, errors.New("egg_id must not be empty")
	}
	if (v.PalID == "") == (v.PalTemplate == "") {
		return GivePalEgg{}, errors.New("exactly one of pal_id or pal_template must be provided")
	}
	if v.Level < 0 {
		return GivePalEgg{}, errors.New("level must be a non-negative integer when provided")
	}
	return v, nil
}

func eggFromMap(m map[string]any) (GivePalEgg, error) {
	eggID, ok := m["EggID"].(string)
	if !ok || eggID == "" {
		return GivePalEgg{}, errors.New("eggID must be a non-empty string")
	}

	palID, hasPalID := m["PalID"].(string)
	if _, present := m["PalID"]; present && !hasPalID {
		return GivePalEgg{}, errors.New("palID must be a string")
	}
	if hasPalID && palID == "" {
		return GivePalEgg{}, errors.New("palID must be a non-empty string")
	}
	palTemplate, hasPalTemplate := m["PalTemplate"].(string)
	if _, present := m["PalTemplate"]; present && !hasPalTemplate {
		return GivePalEgg{}, errors.New("palTemplate must be a string")
	}
	if hasPalTemplate && palTemplate == "" {
		return GivePalEgg{}, errors.New("palTemplate must be a non-empty string")
	}
	if hasPalID == hasPalTemplate {
		return GivePalEgg{}, errors.New("exactly one of palID or palTemplate must be provided")
	}

	level, err := eggLevelFromMap(m)
	if err != nil {
		return GivePalEgg{}, err
	}

	return GivePalEgg{EggID: eggID, PalID: palID, PalTemplate: palTemplate, Level: level}, nil
}

func eggLevelFromMap(m map[string]any) (int, error) {
	value, ok := m["Level"]
	if !ok {
		return 0, nil
	}
	level, ok := asInt(value)
	if !ok {
		return 0, errors.New("pal egg level must be an integer")
	}
	if level < 0 {
		return 0, errors.New("level must be a non-negative integer when provided")
	}
	return level, nil
}

func eggFromTuple(t []any) (GivePalEgg, error) {
	if len(t) != 2 && len(t) != 3 {
		return GivePalEgg{}, errors.New("pal egg tuples must be (egg_id, pal_id_or_template) or (egg_id, pal_id_or_template, level)")
	}
	eggID, ok := t[0].(string)
	if !ok || eggID == "" {
		return GivePalEgg{}, errors.New("egg tuple egg_id must be a string")
	}
	second, ok := t[1].(string)
	if !ok || second == "" {
		return GivePalEgg{}, errors.New("pal egg tuple second value must be a string")
	}

	level, err := eggLevelFromTuple(t)
	if err != nil {
		return GivePalEgg{}, err
	}

	if len(second) >= 5 && strings.EqualFold(second[len(second)-5:], ".json") {
		return GivePalEgg{EggID: eggID, PalTemplate: second, Level: level}, nil
	}
	return GivePalEgg{EggID: eggID, PalID: second, Level: level}, nil
}

func eggLevelFromTuple(t []any) (int, error) {
	if len(t) != 3 {
		return 0, nil
	}
	lvl, ok := asInt(t[2])
	if !ok {
		return 0, errors.New("pal egg tuple level must be an integer")
	}
	if lvl < 0 {
		return 0, errors.New("pal egg tuple level must be a non-negative integer")
	}
	return lvl, nil
}

func normalizeProgressionRequest(request *GiveProgressionRequest, exp, technologyPoints, ancientTechnologyPoints *int, relics map[string]int) (*GiveProgressionRequest, error) {
	hasKeywordValues := exp != nil || technologyPoints != nil || ancientTechnologyPoints != nil || relics != nil
	if request != nil && hasKeywordValues {
		return nil, errors.New("pass either request or progression keyword arguments, not both")
	}
	if request != nil {
		if err := validateProgressionRequest(request); err != nil {
			return nil, err
		}
		return request, nil
	}

	if !hasKeywordValues {
		return nil, errors.New("at least one progression field must be provided")
	}

	req := &GiveProgressionRequest{}
	if err := assignPositiveInt(exp, "exp", func(value int) {
		req.EXP = value
	}); err != nil {
		return nil, err
	}
	if err := assignPositiveInt(technologyPoints, "technology_points", func(value int) {
		req.TechnologyPoints = value
	}); err != nil {
		return nil, err
	}
	if err := assignPositiveInt(ancientTechnologyPoints, "ancient_technology_points", func(value int) {
		req.AncientTechnologyPoints = value
	}); err != nil {
		return nil, err
	}
	if relics != nil {
		if err := validateRelics(relics); err != nil {
			return nil, err
		}
		req.Relics = relics
	}
	return req, nil
}

func validateProgressionRequest(request *GiveProgressionRequest) error {
	if request.Relics != nil {
		if err := validateRelics(request.Relics); err != nil {
			return err
		}
	}
	if request.EXP == 0 && request.TechnologyPoints == 0 && request.AncientTechnologyPoints == 0 && len(request.Relics) == 0 {
		return errors.New("at least one progression field must be provided")
	}
	if request.EXP < 0 {
		return errors.New("exp must be a positive integer")
	}
	if request.TechnologyPoints < 0 {
		return errors.New("technology_points must be a positive integer")
	}
	if request.AncientTechnologyPoints < 0 {
		return errors.New("ancient_technology_points must be a positive integer")
	}
	return nil
}

func validateRelics(relics map[string]int) error {
	if len(relics) == 0 {
		return errors.New("relics must be a non-empty object")
	}
	for relicType, amount := range relics {
		if amount <= 0 {
			return fmt.Errorf("relic amount for %s must be a positive integer", relicType)
		}
	}
	return nil
}

func assignPositiveInt(value *int, name string, assign func(int)) error {
	if value == nil {
		return nil
	}
	if *value <= 0 {
		return fmt.Errorf("%s must be a positive integer", name)
	}
	assign(*value)
	return nil
}

func normalizeTechnologyInputs(values []TechnologyInput) (any, error) {
	valuesAny := flattenSingleSequence(toAnySlice(values))

	normalized := make([]string, 0, len(valuesAny))

	for _, value := range valuesAny {
		switch v := value.(type) {
		case string:
			if v == "" {
				return nil, errors.New("technology values must be non-empty strings")
			}
			normalized = append(normalized, v)
		default:
			return nil, errors.New("technology values must be strings")
		}
	}

	if len(normalized) == 0 {
		return nil, errors.New("at least one technology must be provided")
	}
	if len(normalized) == 1 {
		return normalized[0], nil
	}
	for _, value := range normalized {
		if value == "All" {
			return nil, errors.New("\"All\" is only valid when passed by itself")
		}
	}
	return normalized, nil
}

func normalizeProductInput(product any) (string, error) {
	switch v := product.(type) {
	case string:
		if v == "" {
			return "", errors.New("product must not be empty")
		}
		return v, nil
	default:
		return "", errors.New("product must be a string")
	}
}

// flattenSingleSequence flattens a list with a single element when it is
// a slice (two-element tuples are preserved).
func flattenSingleSequence(values []any) []any {
	if len(values) != 1 {
		return values
	}
	first := values[0]
	if isSlice(first) && !looksLikeTuple(first) {
		val := reflect.ValueOf(first)
		result := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = val.Index(i).Interface()
		}
		return result
	}
	return values
}

func isSlice(value any) bool {
	if value == nil {
		return false
	}
	switch value.(type) {
	case string, []byte:
		return false
	}
	return reflect.TypeOf(value).Kind() == reflect.Slice
}

func looksLikeTuple(value any) bool {
	arr, ok := tupleToAnySlice(value)
	if !ok || len(arr) != 2 {
		return false
	}
	_, ok1 := arr[0].(string)
	_, ok2 := asInt(arr[1])
	return ok1 && ok2
}

func tupleToAnySlice(value any) ([]any, bool) {
	if !isSlice(value) {
		return nil, false
	}
	val := reflect.ValueOf(value)
	length := val.Len()
	result := make([]any, length)
	for i := 0; i < length; i++ {
		result[i] = val.Index(i).Interface()
	}
	return result, true
}

// normalizeStringInputs validates and normalizes non-empty string inputs.
func normalizeStringInputs(values []string, label string) ([]string, error) {
	normalized := make([]string, 0, len(values))

	for _, value := range values {
		if value == "" {
			return nil, fmt.Errorf("%s entries must be non-empty strings", label)
		}
		normalized = append(normalized, value)
	}

	if len(normalized) == 0 {
		return nil, fmt.Errorf("at least one %s must be provided", strings.TrimSuffix(label, "s"))
	}
	return normalized, nil
}
