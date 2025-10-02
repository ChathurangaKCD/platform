package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"gopkg.in/yaml.v3"
)

type Template struct {
	Resources []Resource `yaml:"resources"`
}

type Resource struct {
	ID        string                 `yaml:"id"`
	Condition string                 `yaml:"condition,omitempty"`
	Template  map[string]interface{} `yaml:"template"`
}

// Sentinel value to mark fields for omission
type omitValue struct{}

var omitSentinel = &omitValue{}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: go run main.go <directory_path>")
	}

	dirPath := os.Args[1]

	// Read template.yaml
	templatePath := filepath.Join(dirPath, "template.yaml")
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		log.Fatalf("Error reading template.yaml: %v", err)
	}

	var template Template
	if err := yaml.Unmarshal(templateData, &template); err != nil {
		log.Fatalf("Error parsing template.yaml: %v", err)
	}

	// Read inputs.json
	inputsPath := filepath.Join(dirPath, "inputs.json")
	inputsData, err := os.ReadFile(inputsPath)
	if err != nil {
		log.Fatalf("Error reading inputs.json: %v", err)
	}

	var inputs map[string]interface{}
	if err := json.Unmarshal(inputsData, &inputs); err != nil {
		log.Fatalf("Error parsing inputs.json: %v", err)
	}

	// Process resources with CEL evaluation
	processedResources := make([]map[string]interface{}, 0, len(template.Resources))
	for _, resource := range template.Resources {
		// Check if resource has a condition
		if resource.Condition != "" {
			// Evaluate the condition
			conditionResult, err := evaluateStringCEL(resource.Condition, inputs)
			if err != nil {
				log.Fatalf("Error evaluating condition for resource %s: %v", resource.ID, err)
			}
			// Skip this resource if condition is false
			if boolResult, ok := conditionResult.(bool); ok && !boolResult {
				continue
			}
		}

		processedTemplate, err := evaluateCELExpressions(resource.Template, inputs)
		if err != nil {
			log.Fatalf("Error evaluating CEL expressions for resource %s: %v", resource.ID, err)
		}
		processedResources = append(processedResources, processedTemplate.(map[string]interface{}))
	}

	// Generate output.yaml
	output := map[string]interface{}{
		"resources": processedResources,
	}

	// Remove omitted fields from the output
	cleanedOutput := removeOmittedFields(output)

	outputData, err := yaml.Marshal(cleanedOutput)
	if err != nil {
		log.Fatalf("Error marshaling output: %v", err)
	}

	outputPath := filepath.Join(dirPath, "output.yaml")
	if err := os.WriteFile(outputPath, outputData, 0644); err != nil {
		log.Fatalf("Error writing output.yaml: %v", err)
	}

	fmt.Printf("Successfully generated %s\n", outputPath)
}

func evaluateCELExpressions(data interface{}, inputs map[string]interface{}) (interface{}, error) {
	switch v := data.(type) {
	case string:
		result, err := evaluateStringCEL(v, inputs)
		if err != nil {
			return nil, err
		}
		
		// If the entire value is a CEL expression (after trimming), return the raw result
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "${") && strings.HasSuffix(trimmed, "}") && result != v {
			// Check if this is a pure CEL expression by parsing it
			celExpr := trimmed[2 : len(trimmed)-1]
			// If the expression is the only content, return the actual data structure
			if "${"+celExpr+"}" == trimmed {
				return result, nil
			}
		}
		
		// For mixed content strings, we need to keep them as strings
		return result, nil
		
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			evaluated, err := evaluateCELExpressions(value, inputs)
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
			evaluated, err := evaluateCELExpressions(item, inputs)
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
					// Return a special marker that will be used to omit the field
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
		// Handle both CEL lists and Go slices
		switch list := val.Value().(type) {
		case []ref.Val:
			result := make([]interface{}, len(list))
			for i, item := range list {
				result[i] = convertCELValue(item)
			}
			return result
		case []interface{}:
			// Already a Go slice, just return it
			return list
		default:
			return val.Value()
		}
	case types.MapType:
		// Handle both CEL maps and Go maps
		switch m := val.Value().(type) {
		case map[ref.Val]ref.Val:
			result := make(map[string]interface{})
			for k, v := range m {
				result[k.Value().(string)] = convertCELValue(v)
			}
			return result
		case map[string]interface{}:
			// Already a Go map, just return it
			return m
		default:
			return val.Value()
		}
	default:
		return val.Value()
	}
}

// removeOmittedFields recursively removes fields marked with the omit sentinel
func removeOmittedFields(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			// Check if the value is the omit sentinel (pointer comparison)
			if ptrVal, ok := value.(*omitValue); ok && ptrVal == omitSentinel {
				continue // Skip this field
			}
			// Also check by value comparison
			if value == omitSentinel {
				continue // Skip this field
			}
			// Recursively clean nested structures
			cleaned := removeOmittedFields(value)
			// Only add if the cleaned value is not the sentinel
			if cleaned != omitSentinel {
				result[key] = cleaned
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			// Check if the item is the omit sentinel
			if item == omitSentinel {
				continue // Skip this item
			}
			// Recursively clean nested structures
			cleaned := removeOmittedFields(item)
			// Only add if the cleaned value is not the sentinel
			if cleaned != omitSentinel {
				result = append(result, cleaned)
			}
		}
		return result
	default:
		// For primitive types, return as-is
		return v
	}
}