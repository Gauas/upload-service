package httpresp

import "encoding/json"

func Refine[S any, T any](entity S) T {
	var out T
	raw, err := json.Marshal(entity)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func RefineList[SL ~[]S, S any, T any](entities SL) []T {
	if len(entities) == 0 {
		return []T{}
	}

	out := make([]T, 0, len(entities))
	for _, entity := range entities {
		out = append(out, Refine[S, T](entity))
	}
	return out
}

