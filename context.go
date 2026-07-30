package feel

import (
	"fmt"
	"maps"
	"sort"
)

func contextGetByKeys(ctx map[string]any, keys []string) (any, bool) {
	for i, key := range keys {
		if i == len(keys)-1 {
			v, ok := ctx[key]
			return v, ok
		} else {
			v, ok := ctx[key]
			if !ok {
				return nil, false
			}
			if subctx, ok := v.(map[string]any); ok {
				ctx = subctx
			} else {
				return nil, false
			}
		}
	}
	return nil, false
}

func contextProbePut(ctx map[string]any, keys []string) bool {
	for i, key := range keys {
		if i == len(keys)-1 {
			return true
		} else {
			v, ok := ctx[key]
			if !ok {
				// empty cell can be put
				return true
			}
			if subctx, ok := v.(map[string]any); ok {
				ctx = subctx
			} else {
				// sub ctx is not map
				return false
			}
		}
	}
	return false
}

func contextCopy(ctx map[string]any) map[string]any {
	newCtx := make(map[string]any)
	maps.Copy(newCtx, ctx)
	return newCtx
}

func contextPutKeys(ctx map[string]any, keys []string, value any) (map[string]any, bool) {
	if !contextProbePut(ctx, keys) {
		// cannot put keys
		return ctx, false
	}

	rootCtx := ctx
	for i, key := range keys {
		if i == len(keys)-1 {
			// the last key
			ctx[key] = value
			return rootCtx, true
		} else {
			if v, ok := ctx[key]; ok {
				if subctx, ok := v.(map[string]any); ok {
					// copy the nested context so we don't mutate the
					// original's shared submap
					subctx = contextCopy(subctx)
					ctx[key] = subctx
					ctx = subctx
				} else {
					return rootCtx, false
				}
			} else {
				subctx := make(map[string]any)
				ctx[key] = subctx
				ctx = subctx
			}
		}
	}
	return rootCtx, false
}

func installContextFunctions(prelude *Prelude) {
	// context/map functions
	// "context" is the DMN 1.3+ parameter name; "m" is an older alias some
	// TCK cases still use.
	prelude.Bind("get value", NewNativeFunc(func(kwargs map[string]any) (any, error) {
		ctx, ok := kwargs["context"].(map[string]any)
		if !ok {
			return Null, nil
		}
		keyArg, ok := kwargs["key"]
		if !ok {
			return Null, nil
		}

		if key, ok := keyArg.(string); ok {
			if v, ok := ctx[key]; ok {
				return v, nil
			}
			return Null, nil
		}
		if keyList, ok := keyArg.([]any); ok {
			keys := make([]string, len(keyList))
			for i, k := range keyList {
				s, ok := k.(string)
				if !ok {
					return Null, nil
				}
				keys[i] = s
			}
			if v, ok := contextGetByKeys(ctx, keys); ok {
				return v, nil
			}
			return Null, nil
		}
		return Null, nil
	}).Required("context", "key").Alias("context", "m"))

	prelude.Bind("get entries", NewNativeFunc(func(kwargs map[string]any) (any, error) {
		ctx, ok := kwargs["context"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("get entries: context must be a context value, got %T", kwargs["context"])
		}
		keys := make([]string, 0, len(ctx))
		for k := range ctx {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		entries := make([]any, 0, len(keys))
		for _, k := range keys {
			entries = append(entries, map[string]any{
				"key":   k,
				"value": ctx[k],
			})
		}
		return entries, nil
	}).Required("context").Alias("context", "m"))

	prelude.Bind("context put", NewNativeFunc(func(kwargs map[string]any) (any, error) {
		ctx, isCtx := kwargs["context"].(map[string]any)
		if !isCtx {
			return Null, nil
		}

		value, hasValue := kwargs["value"]
		if !hasValue {
			return Null, nil
		}

		var keys []string
		switch k := kwargs["key"].(type) {
		case string:
			keys = []string{k}
		case []any:
			keys = make([]string, len(k))
			for i, v := range k {
				s, ok := v.(string)
				if !ok {
					return Null, nil
				}
				keys[i] = s
			}
		default:
			return Null, nil
		}

		if len(keys) == 0 {
			return Null, nil
		}

		newCtx, ok := contextPutKeys(contextCopy(ctx), keys, value)
		if !ok {
			return Null, nil
		}
		return newCtx, nil
	}).Required("context", "key", "value"))

	prelude.Bind("context merge", NewNativeFunc(func(kwargs map[string]any) (any, error) {
		_, hasExtra := kwargs["__extra"]
		if hasExtra {
			return Null, nil
		}

		contextsVal, hasContexts := kwargs["contexts"]
		if !hasContexts {
			return Null, nil
		}

		contextsList, ok := contextsVal.([]any)
		if !ok {
			// A single context behaves as an implicit singleton list.
			if singleCtx, isCtx := contextsVal.(map[string]any); isCtx {
				contextsList = []any{singleCtx}
			} else {
				return Null, nil
			}
		}

		merged := make(map[string]any)
		for _, item := range contextsList {
			ctx, ok := item.(map[string]any)
			if !ok {
				return Null, nil
			}
			maps.Copy(merged, ctx)
		}
		return merged, nil
	}).Optional("contexts").Vararg("__extra"))

	prelude.Bind("context", NewNativeFunc(func(args map[string]any) (any, error) {
		_, hasExtra := args["__extra"]
		if hasExtra {
			return Null, nil
		}
		entriesVal, hasEntries := args["entries"]
		if !hasEntries {
			return Null, nil
		}

		var entriesList []any
		switch ev := entriesVal.(type) {
		case []any:
			entriesList = ev
		case map[string]any:
			entriesList = []any{ev}
		default:
			return Null, nil
		}

		result := make(map[string]any)
		for _, entry := range entriesList {
			entryMap, ok := entry.(map[string]any)
			if !ok {
				return Null, nil
			}
			keyVal, hasKey := entryMap["key"]
			if !hasKey {
				return Null, nil
			}
			if _, isNull := keyVal.(*NullValue); isNull {
				return Null, nil
			}
			keyStr, ok := keyVal.(string)
			if !ok {
				return Null, nil
			}
			value, hasValue := entryMap["value"]
			if !hasValue {
				return Null, nil
			}
			if _, exists := result[keyStr]; exists {
				return Null, nil
			}
			result[keyStr] = value
		}
		return result, nil
	}).Optional("entries").Vararg("__extra"))

}
