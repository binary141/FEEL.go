package feel

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
	for k, v := range ctx {
		newCtx[k] = v
	}
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
	prelude.Bind("get value", NewNativeFunc(func(kwargs map[string]any) (any, error) {
		type getvalueByKey struct {
			Context map[string]any `json:"context"`
			Key     string         `json:"key"`
		}

		type getvalueByKeys struct {
			Context map[string]any `json:"context"`
			Keys    []string       `json:"key"`
		}

		argsByKey := getvalueByKey{}

		if err := decodeKWArgs(kwargs, &argsByKey); err != nil {
			argsByKeys := getvalueByKeys{}
			if err := decodeKWArgs(kwargs, &argsByKeys); err != nil {
				return nil, err
			}

			if v, ok := contextGetByKeys(argsByKeys.Context, argsByKeys.Keys); ok {
				return v, nil
			} else {
				return Null, nil
			}
		} else {
			if v, ok := argsByKey.Context[argsByKey.Key]; ok {
				return v, nil
			} else {
				return Null, nil
			}
		}
	}).Required("context", "key"))

	prelude.Bind("get entries", wrapTyped(func(ctx map[string]any) ([](map[string]any), error) {
		entries := make([](map[string]any), 0)
		for k, v := range ctx {
			ent := map[string]any{
				"key":   k,
				"value": v,
			}
			entries = append(entries, ent)
		}
		return entries, nil
	}).Required("context"))

	prelude.Bind("context put", NewNativeFunc(func(kwargs map[string]any) (any, error) {
		type putByKey struct {
			Context map[string]any `json:"context"`
			Key     string         `json:"key"`
			Value   any            `json:"value"`
		}

		type putByKeys struct {
			Context map[string]any `json:"context"`
			Keys    []string       `json:"key"`
			Value   any            `json:"value"`
		}

		argsByKey := putByKey{}

		if err := decodeKWArgs(kwargs, &argsByKey); err != nil {
			argsByKeys := putByKeys{}
			if err := decodeKWArgs(kwargs, &argsByKeys); err != nil {
				return nil, err
			}
			ctx, _ := contextPutKeys(argsByKeys.Context, argsByKeys.Keys, argsByKeys.Value)
			return ctx, nil
		} else {
			ctx, _ := contextPutKeys(argsByKey.Context, []string{argsByKey.Key}, argsByKey.Value)
			return ctx, nil
		}
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
			return Null, nil
		}

		merged := make(map[string]any)
		for _, item := range contextsList {
			ctx, ok := item.(map[string]any)
			if !ok {
				return Null, nil
			}
			for k, v := range ctx {
				merged[k] = v
			}
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
			if keyStr == "" {
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
