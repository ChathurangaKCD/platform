package schema2

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Converter builds OpenAPI schemas from simple schema definitions.
type Converter struct {
	types     map[string]interface{}
	typeCache map[string]*extv1.JSONSchemaProps
	typeStack map[string]bool
}

// NewConverter returns a Converter that knows about the given custom types.
func NewConverter(types map[string]interface{}) *Converter {
	copied := map[string]interface{}{}
	for k, v := range types {
		copied[k] = v
	}

	return &Converter{
		types:     copied,
		typeCache: map[string]*extv1.JSONSchemaProps{},
		typeStack: map[string]bool{},
	}
}

// Convert converts a field map expressed in Kro-style simple schema syntax into an OpenAPI schema.
func (c *Converter) Convert(fields map[string]interface{}) (*extv1.JSONSchemaProps, error) {
	if len(fields) == 0 {
		return &extv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]extv1.JSONSchemaProps{},
		}, nil
	}

	return c.buildObjectSchema(fields)
}

func (c *Converter) buildObjectSchema(fields map[string]interface{}) (*extv1.JSONSchemaProps, error) {
	props := map[string]extv1.JSONSchemaProps{}
	required := []string{}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		field := fields[name]

		schema, requiredValue, requiredExplicit, err := c.buildFieldSchema(field)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", name, err)
		}
		if schema == nil {
			continue
		}
		props[name] = *schema
		switch {
		case requiredExplicit:
			if requiredValue {
				required = append(required, name)
			}
		case schema.Default == nil:
			required = append(required, name)
		}
	}

	result := &extv1.JSONSchemaProps{
		Type:       "object",
		Properties: props,
	}
	if len(required) > 0 {
		result.Required = required
	}
	return result, nil
}

func (c *Converter) buildFieldSchema(raw interface{}) (*extv1.JSONSchemaProps, bool, bool, error) {
	switch typed := raw.(type) {
	case string:
		return c.schemaFromString(typed)
	case map[string]interface{}:
		schema, err := c.buildObjectSchema(typed)
		return schema, false, false, err
	default:
		return nil, false, false, fmt.Errorf("unsupported field definition of type %T", raw)
	}
}

func (c *Converter) schemaFromString(expr string) (*extv1.JSONSchemaProps, bool, bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, false, false, fmt.Errorf("empty schema expression")
	}

	typeExpr := expr
	constraintExpr := ""
	if idx := strings.Index(expr, "|"); idx != -1 {
		typeExpr = strings.TrimSpace(expr[:idx])
		constraintExpr = strings.TrimSpace(expr[idx+1:])
	}

	schema, err := c.schemaFromType(typeExpr)
	if err != nil {
		return nil, false, false, err
	}

	required, explicit, err := applyConstraints(schema, constraintExpr, schema.Type)
	if err != nil {
		return nil, false, false, err
	}
	return schema, required, explicit, nil
}

func (c *Converter) schemaFromType(typeExpr string) (*extv1.JSONSchemaProps, error) {
	switch {
	case typeExpr == "string":
		return &extv1.JSONSchemaProps{Type: "string"}, nil
	case typeExpr == "integer":
		return &extv1.JSONSchemaProps{Type: "integer"}, nil
	case typeExpr == "number":
		return &extv1.JSONSchemaProps{Type: "number"}, nil
	case typeExpr == "boolean":
		return &extv1.JSONSchemaProps{Type: "boolean"}, nil
	case typeExpr == "object":
		return &extv1.JSONSchemaProps{Type: "object"}, nil
	case strings.HasPrefix(typeExpr, "[]"):
		itemTypeExpr := strings.TrimSpace(typeExpr[2:])
		items, err := c.schemaFromType(itemTypeExpr)
		if err != nil {
			return nil, err
		}
		return &extv1.JSONSchemaProps{
			Type: "array",
			Items: &extv1.JSONSchemaPropsOrArray{
				Schema: items,
			},
		}, nil
	case strings.HasPrefix(typeExpr, "array<") && strings.HasSuffix(typeExpr, ">"):
		itemTypeExpr := strings.TrimSpace(typeExpr[len("array<") : len(typeExpr)-1])
		items, err := c.schemaFromType(itemTypeExpr)
		if err != nil {
			return nil, err
		}
		return &extv1.JSONSchemaProps{
			Type: "array",
			Items: &extv1.JSONSchemaPropsOrArray{
				Schema: items,
			},
		}, nil
	case strings.HasPrefix(typeExpr, "map<") && strings.HasSuffix(typeExpr, ">"):
		valueTypeExpr := strings.TrimSpace(typeExpr[len("map<") : len(typeExpr)-1])
		return c.mapSchemaFromType(valueTypeExpr)
	case strings.HasPrefix(typeExpr, "map["):
		closing := strings.Index(typeExpr, "]")
		if closing == -1 {
			return nil, fmt.Errorf("invalid map type expression %q", typeExpr)
		}
		valueTypeExpr := strings.TrimSpace(typeExpr[closing+1:])
		return c.mapSchemaFromType(valueTypeExpr)
	default:
		return c.schemaFromCustomType(typeExpr)
	}
}

func (c *Converter) mapSchemaFromType(valueTypeExpr string) (*extv1.JSONSchemaProps, error) {
	valueSchema, err := c.schemaFromType(valueTypeExpr)
	if err != nil {
		return nil, err
	}

	return &extv1.JSONSchemaProps{
		Type: "object",
		AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
			Allows: true,
			Schema: valueSchema,
		},
	}, nil
}

func (c *Converter) schemaFromCustomType(typeName string) (*extv1.JSONSchemaProps, error) {
	if cached, ok := c.typeCache[typeName]; ok {
		return cached.DeepCopy(), nil
	}

	if c.typeStack[typeName] {
		return nil, fmt.Errorf("detected cyclic type reference involving %q", typeName)
	}

	raw, ok := c.types[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown type %q", typeName)
	}

	c.typeStack[typeName] = true
	defer delete(c.typeStack, typeName)

	var (
		built *extv1.JSONSchemaProps
		err   error
	)

	switch typed := raw.(type) {
	case string:
		var (
			required       bool
			requiredMarker bool
		)
		built, required, requiredMarker, err = c.schemaFromString(typed)
		if required && requiredMarker {
			// required does not make sense on type definitions; ignore.
		}
	case map[string]interface{}:
		built, err = c.buildObjectSchema(typed)
	default:
		err = fmt.Errorf("unsupported custom type definition for %q (type %T)", typeName, raw)
	}
	if err != nil {
		return nil, err
	}

	c.typeCache[typeName] = built
	return built.DeepCopy(), nil
}

func applyConstraints(schema *extv1.JSONSchemaProps, constraintExpr, schemaType string) (bool, bool, error) {
	if strings.TrimSpace(constraintExpr) == "" {
		return false, false, nil
	}

	tokens := tokenizeConstraints(constraintExpr)
	var required bool
	var hasRequired bool

	for _, token := range tokens {
		if !strings.Contains(token, "=") {
			continue
		}
		parts := strings.SplitN(token, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "required":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return false, false, fmt.Errorf("invalid required value %q: %w", value, err)
			}
			required = boolVal
			hasRequired = true
		case "default":
			parsed, err := parseValueForType(value, schemaType)
			if err != nil {
				return false, false, fmt.Errorf("invalid default %q: %w", value, err)
			}
			raw, err := json.Marshal(parsed)
			if err != nil {
				return false, false, fmt.Errorf("failed to marshal default %#v: %w", parsed, err)
			}
			schema.Default = &extv1.JSON{Raw: raw}
		case "enum":
			values := splitAndTrim(value, ",")
			enums := make([]extv1.JSON, 0, len(values))
			for _, v := range values {
				parsed, err := parseValueForType(v, schemaType)
				if err != nil {
					return false, false, fmt.Errorf("invalid enum value %q: %w", v, err)
				}
				raw, err := json.Marshal(parsed)
				if err != nil {
					return false, false, fmt.Errorf("failed to marshal enum value %#v: %w", parsed, err)
				}
				enums = append(enums, extv1.JSON{Raw: raw})
			}
			schema.Enum = enums
		case "pattern":
			schema.Pattern = value
		case "minimum":
			num, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return false, false, fmt.Errorf("invalid minimum %q: %w", value, err)
			}
			schema.Minimum = &num
		case "maximum":
			num, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return false, false, fmt.Errorf("invalid maximum %q: %w", value, err)
			}
			schema.Maximum = &num
		case "exclusiveMinimum":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return false, false, fmt.Errorf("invalid exclusiveMinimum %q: %w", value, err)
			}
			schema.ExclusiveMinimum = boolVal
		case "exclusiveMaximum":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return false, false, fmt.Errorf("invalid exclusiveMaximum %q: %w", value, err)
			}
			schema.ExclusiveMaximum = boolVal
		case "minItems":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return false, false, fmt.Errorf("invalid minItems %q: %w", value, err)
			}
			schema.MinItems = &intVal
		case "maxItems":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return false, false, fmt.Errorf("invalid maxItems %q: %w", value, err)
			}
			schema.MaxItems = &intVal
		case "uniqueItems":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return false, false, fmt.Errorf("invalid uniqueItems %q: %w", value, err)
			}
			schema.UniqueItems = boolVal
		case "minLength":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return false, false, fmt.Errorf("invalid minLength %q: %w", value, err)
			}
			schema.MinLength = &intVal
		case "maxLength":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return false, false, fmt.Errorf("invalid maxLength %q: %w", value, err)
			}
			schema.MaxLength = &intVal
		case "minProperties":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return false, false, fmt.Errorf("invalid minProperties %q: %w", value, err)
			}
			schema.MinProperties = &intVal
		case "maxProperties":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return false, false, fmt.Errorf("invalid maxProperties %q: %w", value, err)
			}
			schema.MaxProperties = &intVal
		case "multipleOf":
			num, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return false, false, fmt.Errorf("invalid multipleOf %q: %w", value, err)
			}
			schema.MultipleOf = &num
		case "title":
			schema.Title = value
		case "description":
			schema.Description = value
		case "format":
			schema.Format = value
		case "example":
			parsed, err := parseArbitraryValue(value)
			if err != nil {
				return false, false, fmt.Errorf("invalid example %q: %w", value, err)
			}
			raw, err := json.Marshal(parsed)
			if err != nil {
				return false, false, fmt.Errorf("failed to marshal example %#v: %w", parsed, err)
			}
			schema.Example = &extv1.JSON{Raw: raw}
		case "nullable":
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return false, false, fmt.Errorf("invalid nullable value %q: %w", value, err)
			}
			schema.Nullable = boolVal
		default:
			// Unknown markers are ignored for now. They can be handled by callers if needed.
		}
	}

	return required, hasRequired, nil
}

func parseValueForType(value, schemaType string) (interface{}, error) {
	switch schemaType {
	case "string":
		return value, nil
	case "integer":
		if value == "" {
			return 0, fmt.Errorf("empty integer value")
		}
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		return intVal, nil
	case "number":
		if value == "" {
			return 0.0, fmt.Errorf("empty number value")
		}
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
		return floatVal, nil
	case "boolean":
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		return boolVal, nil
	case "array", "object":
		if strings.TrimSpace(value) == "" {
			if schemaType == "array" {
				return []interface{}{}, nil
			}
			return map[string]interface{}{}, nil
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	default:
		// For custom object-like types, attempt JSON parsing; fall back to string.
		if strings.TrimSpace(value) == "" {
			return value, nil
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			return parsed, nil
		}
		// Attempt numeric/bool parsing as a best effort.
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal, nil
		}
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal, nil
		}
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal, nil
		}
		return value, nil
	}
}

func parseArbitraryValue(value string) (interface{}, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		var parsed interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	}

	if boolVal, err := strconv.ParseBool(value); err == nil {
		return boolVal, nil
	}
	if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
		return intVal, nil
	}
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal, nil
	}
	return value, nil
}

func tokenizeConstraints(expr string) []string {
	var tokens []string
	var current strings.Builder

	inQuotes := false
	var quoteChar rune
	escaped := false
	bracketDepth := 0

	for _, r := range expr {
		switch {
		case inQuotes:
			current.WriteRune(r)
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quoteChar {
				inQuotes = false
			}
		case r == '"' || r == '\'':
			inQuotes = true
			quoteChar = r
			current.WriteRune(r)
		case r == '{' || r == '[':
			bracketDepth++
			current.WriteRune(r)
		case r == '}' || r == ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
			current.WriteRune(r)
		case unicode.IsSpace(r) && bracketDepth == 0:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func splitAndTrim(value, sep string) []string {
	raw := strings.Split(value, sep)
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
