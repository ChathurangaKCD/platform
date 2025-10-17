package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
)

// omitValue is a sentinel used to mark values that should be pruned after rendering.
type omitValue struct{}

var (
	omitSentinel = &omitValue{}
	omitErrMsg   = "__OC_RENDERER_OMIT__"
)

// Engine evaluates CEL backed templates that can contain inline expressions, map keys, and nested structures.
type Engine struct{}

// NewEngine creates a new CEL template engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Render walks the provided structure and evaluates CEL expressions against the supplied inputs.
func (e *Engine) Render(data interface{}, inputs map[string]interface{}) (interface{}, error) {
	switch v := data.(type) {
	case string:
		return e.renderString(v, inputs)
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, value := range v {
			evaluatedKey := key
			if renderedKey, err := e.renderString(key, inputs); err == nil {
				if keyStr, ok := renderedKey.(string); ok {
					evaluatedKey = keyStr
				}
			}

			renderedValue, err := e.Render(value, inputs)
			if err != nil {
				return nil, err
			}
			if renderedValue == omitSentinel {
				continue
			}
			if ptr, ok := renderedValue.(*omitValue); ok && ptr == omitSentinel {
				continue
			}
			result[evaluatedKey] = renderedValue
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			rendered, err := e.Render(item, inputs)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		}
		return result, nil
	default:
		return v, nil
	}
}

func (e *Engine) renderString(str string, inputs map[string]interface{}) (interface{}, error) {
	expressions := findCELExpressions(str)
	if len(expressions) == 0 {
		return str, nil
	}

	trimmed := strings.TrimSpace(str)
	if len(expressions) == 1 && expressions[0].fullExpr == trimmed {
		result, err := evaluateCEL(expressions[0].innerExpr, inputs)
		return normalizeCELResult(result, err)
	}

	rendered := str
	for _, match := range expressions {
		value, err := evaluateCEL(match.innerExpr, inputs)
		if err != nil {
			return nil, err
		}

		var replacement string
		switch typed := value.(type) {
		case string:
			replacement = typed
		case int64:
			replacement = fmt.Sprintf("%d", typed)
		case float64:
			replacement = fmt.Sprintf("%g", typed)
		case bool:
			replacement = fmt.Sprintf("%t", typed)
		default:
			bytes, err := json.Marshal(typed)
			if err != nil {
				replacement = fmt.Sprintf("%v", typed)
			} else {
				replacement = string(bytes)
			}
		}

		rendered = strings.Replace(rendered, match.fullExpr, replacement, 1)
	}

	return rendered, nil
}

type celMatch struct {
	fullExpr  string
	innerExpr string
}

func findCELExpressions(str string) []celMatch {
	var matches []celMatch
	i := 0
	for i < len(str) {
		start := strings.Index(str[i:], "${")
		if start == -1 {
			break
		}
		start += i

		brace := 1
		pos := start + 2
		for pos < len(str) && brace > 0 {
			if str[pos] == '{' {
				brace++
			} else if str[pos] == '}' {
				brace--
			}
			pos++
		}

		if brace == 0 {
			matches = append(matches, celMatch{
				fullExpr:  str[start:pos],
				innerExpr: str[start+2 : pos-1],
			})
			i = pos
		} else {
			break
		}
	}
	return matches
}

func normalizeCELResult(result interface{}, err error) (interface{}, error) {
	if err != nil {
		return nil, err
	}
	if result == omitSentinel {
		return omitSentinel, nil
	}
	if val, ok := result.(*omitValue); ok && val == omitSentinel {
		return omitSentinel, nil
	}
	return result, nil
}

func evaluateCEL(expression string, inputs map[string]interface{}) (interface{}, error) {
	env, err := buildEnv(inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to build CEL environment: %w", err)
	}

	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation error: %v", issues.Err())
	}

	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation error: %w", err)
	}

	result, _, err := program.Eval(inputs)
	if err != nil {
		if err.Error() == omitErrMsg {
			return omitSentinel, nil
		}
		return nil, fmt.Errorf("CEL evaluation error: %w", err)
	}

	return convertCELValue(result), nil
}

func buildEnv(inputs map[string]interface{}) (*cel.Env, error) {
	envOptions := []cel.EnvOption{
		cel.OptionalTypes(),
	}

	for key := range inputs {
		envOptions = append(envOptions, cel.Variable(key, cel.DynType))
	}

	envOptions = append(envOptions,
		ext.Strings(),
		ext.Encoders(),
		ext.Math(),
		ext.Lists(),
		ext.Sets(),
		ext.TwoVarComprehensions(),
		cel.Function("omit",
			cel.Overload("omit", []*cel.Type{}, cel.DynType,
				cel.FunctionBinding(func(values ...ref.Val) ref.Val {
					return types.NewErr(omitErrMsg)
				}),
			),
		),
		cel.Function("merge",
			cel.Overload("merge_map_map", []*cel.Type{cel.MapType(cel.StringType, cel.DynType), cel.MapType(cel.StringType, cel.DynType)}, cel.MapType(cel.StringType, cel.DynType),
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					baseVal := lhs.Value()
					overrideVal := rhs.Value()

					baseMap := make(map[string]interface{})
					overrideMap := make(map[string]interface{})

					switch b := baseVal.(type) {
					case map[string]interface{}:
						baseMap = b
					case map[ref.Val]ref.Val:
						for k, v := range b {
							baseMap[string(k.(types.String))] = v.Value()
						}
					}

					switch o := overrideVal.(type) {
					case map[string]interface{}:
						overrideMap = o
					case map[ref.Val]ref.Val:
						for k, v := range o {
							overrideMap[string(k.(types.String))] = v.Value()
						}
					}

					result := make(map[string]interface{})
					for k, v := range baseMap {
						result[k] = v
					}
					for k, v := range overrideMap {
						result[k] = v
					}

					celResult := make(map[ref.Val]ref.Val)
					for k, v := range result {
						celResult[types.String(k)] = types.DefaultTypeAdapter.NativeToValue(v)
					}

					return types.NewDynamicMap(types.DefaultTypeAdapter, celResult)
				}),
			),
		),
	)

	return cel.NewEnv(envOptions...)
}

func convertCELValue(val ref.Val) interface{} {
	if types.IsError(val) {
		if err, ok := val.Value().(error); ok && err.Error() == omitErrMsg {
			return omitSentinel
		}
	}

	switch val.Type() {
	case types.StringType:
		return val.Value().(string)
	case types.IntType:
		return val.Value().(int64)
	case types.UintType:
		return val.Value().(uint64)
	case types.DoubleType:
		return val.Value().(float64)
	case types.BoolType:
		return val.Value().(bool)
	case types.ListType:
		switch list := val.Value().(type) {
		case []ref.Val:
			result := make([]interface{}, len(list))
			for i, item := range list {
				result[i] = convertCELValue(item)
			}
			return result
		case []interface{}:
			result := make([]interface{}, len(list))
			for i, item := range list {
				switch t := item.(type) {
				case ref.Val:
					result[i] = convertCELValue(t)
				case map[ref.Val]ref.Val:
					m := make(map[string]interface{})
					for k, v := range t {
						keyStr := fmt.Sprintf("%v", k.Value())
						m[keyStr] = convertCELValue(v)
					}
					result[i] = m
				default:
					result[i] = item
				}
			}
			return result
		default:
			return val.Value()
		}
	case types.MapType:
		switch m := val.Value().(type) {
		case map[ref.Val]ref.Val:
			result := make(map[string]interface{})
			for k, v := range m {
				result[fmt.Sprintf("%v", k.Value())] = convertCELValue(v)
			}
			return result
		case map[string]interface{}:
			result := make(map[string]interface{})
			for k, v := range m {
				switch nested := v.(type) {
				case ref.Val:
					result[k] = convertCELValue(nested)
				default:
					result[k] = v
				}
			}
			return result
		default:
			return val.Value()
		}
	default:
		switch typed := val.Value().(type) {
		case ref.Val:
			return convertCELValue(typed)
		default:
			return typed
		}
	}
}

// RemoveOmittedFields strips any values tagged with omit() from rendered output.
func RemoveOmittedFields(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, value := range v {
			if value == omitSentinel {
				continue
			}
			if ptr, ok := value.(*omitValue); ok && ptr == omitSentinel {
				continue
			}
			cleaned := RemoveOmittedFields(value)
			if cleaned != omitSentinel {
				result[key] = cleaned
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			if item == omitSentinel {
				continue
			}
			cleaned := RemoveOmittedFields(item)
			if cleaned != omitSentinel {
				result = append(result, cleaned)
			}
		}
		return result
	default:
		return v
	}
}
