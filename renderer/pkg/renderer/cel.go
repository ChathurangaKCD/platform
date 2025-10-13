package renderer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// Sentinel value to mark fields for omission
type omitValue struct{}

var omitSentinel = &omitValue{}

// EvaluateCELExpressions recursively evaluates CEL expressions in the data structure
func EvaluateCELExpressions(data interface{}, inputs map[string]interface{}) (interface{}, error) {
	switch v := data.(type) {
	case string:
		result, err := evaluateStringCEL(v, inputs)
		if err != nil {
			return nil, err
		}

		// If the entire value is a CEL expression, return the raw result
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "${") && strings.HasSuffix(trimmed, "}") && result != v {
			celExpr := trimmed[2 : len(trimmed)-1]
			if "${"+celExpr+"}" == trimmed {
				return result, nil
			}
		}

		return result, nil

	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			evaluated, err := EvaluateCELExpressions(value, inputs)
			if err != nil {
				return nil, err
			}
			// Skip fields marked for omission
			if evaluated == omitSentinel {
				continue
			}
			if ptrVal, ok := evaluated.(*omitValue); ok && ptrVal == omitSentinel {
				continue
			}
			result[key] = evaluated
		}
		return result, nil

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			evaluated, err := EvaluateCELExpressions(item, inputs)
			if err != nil {
				return nil, err
			}
			result[i] = evaluated
		}
		return result, nil

	default:
		return v, nil
	}
}

func evaluateStringCEL(str string, inputs map[string]interface{}) (interface{}, error) {
	// Find CEL expressions in ${...} format - handle nested braces properly
	var matches [][]string
	i := 0
	for i < len(str) {
		start := strings.Index(str[i:], "${")
		if start == -1 {
			break
		}
		start += i

		// Find matching closing brace
		braceCount := 1
		pos := start + 2
		for pos < len(str) && braceCount > 0 {
			if str[pos] == '{' {
				braceCount++
			} else if str[pos] == '}' {
				braceCount--
			}
			pos++
		}

		if braceCount == 0 {
			fullMatch := str[start:pos]
			expression := str[start+2 : pos-1]
			matches = append(matches, []string{fullMatch, expression})
			i = pos
		} else {
			break
		}
	}

	if len(matches) == 0 {
		return str, nil
	}

	// If the entire string is a single CEL expression, evaluate and return the result directly
	trimmed := strings.TrimSpace(str)
	if len(matches) == 1 && matches[0][0] == trimmed {
		celResult, err := evaluateCELExpression(matches[0][1], inputs)
		return celResult, err
	}

	// Replace multiple CEL expressions in the string
	result := str
	for _, match := range matches {
		fullMatch := match[0]
		expression := match[1]

		evaluated, err := evaluateCELExpression(expression, inputs)
		if err != nil {
			return nil, err
		}

		// Convert result to string for replacement
		var evaluatedStr string
		switch v := evaluated.(type) {
		case string:
			evaluatedStr = v
		case int64:
			evaluatedStr = fmt.Sprintf("%d", v)
		case float64:
			evaluatedStr = fmt.Sprintf("%g", v)
		case bool:
			evaluatedStr = fmt.Sprintf("%t", v)
		default:
			// For complex types like arrays/maps, convert to JSON
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				evaluatedStr = fmt.Sprintf("%v", v)
			} else {
				evaluatedStr = string(jsonBytes)
			}
		}
		result = strings.Replace(result, fullMatch, evaluatedStr, 1)
	}

	return result, nil
}

func evaluateCELExpression(expression string, inputs map[string]interface{}) (interface{}, error) {
	// Create CEL environment with custom functions
	env, err := cel.NewEnv(
		cel.Variable("metadata", cel.DynType),
		cel.Variable("spec", cel.DynType),
		cel.Variable("build", cel.DynType),
		cel.Variable("item", cel.DynType),
		cel.Variable("instanceId", cel.DynType),
		cel.OptionalTypes(),
		cel.Function("join",
			cel.MemberOverload("list_join_string", []*cel.Type{cel.ListType(cel.StringType), cel.StringType}, cel.StringType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					list := lhs.Value().([]ref.Val)
					separator := rhs.Value().(string)
					var result []string
					for _, item := range list {
						result = append(result, item.Value().(string))
					}
					return types.String(strings.Join(result, separator))
				}),
			),
		),
		cel.Function("omit",
			cel.Overload("omit", []*cel.Type{}, cel.DynType,
				cel.FunctionBinding(func(values ...ref.Val) ref.Val {
					return types.NewErr("__OMIT_FIELD__")
				}),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %v", err)
	}

	// Parse the expression
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation error: %v", issues.Err())
	}

	// Create program
	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation error: %v", err)
	}

	// Evaluate
	result, _, err := prg.Eval(inputs)
	if err != nil {
		// Check if this is our special omit error
		if err.Error() == "__OMIT_FIELD__" {
			return omitSentinel, nil
		}
		return nil, fmt.Errorf("CEL evaluation error: %v", err)
	}

	return convertCELValue(result), nil
}

func convertCELValue(val ref.Val) interface{} {
	// Check if this is an error type (used for omit sentinel)
	if types.IsError(val) {
		errMsg := val.Value().(error).Error()
		if errMsg == "__OMIT_FIELD__" {
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
			return list
		default:
			return val.Value()
		}
	case types.MapType:
		switch m := val.Value().(type) {
		case map[ref.Val]ref.Val:
			result := make(map[string]interface{})
			for k, v := range m {
				result[k.Value().(string)] = convertCELValue(v)
			}
			return result
		case map[string]interface{}:
			return m
		default:
			return val.Value()
		}
	default:
		return val.Value()
	}
}

// RemoveOmittedFields recursively removes fields marked with the omit sentinel
func RemoveOmittedFields(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			if ptrVal, ok := value.(*omitValue); ok && ptrVal == omitSentinel {
				continue
			}
			if value == omitSentinel {
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
