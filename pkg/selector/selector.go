package selector

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	log "github.com/sirupsen/logrus"
)

var (
	// Regex patterns for common template syntax
	helmTemplatePattern = regexp.MustCompile(`\{\{[^}]*\}\}`)
	goTemplatePattern   = regexp.MustCompile(`\$\{[^}]*\}`)
)

// preprocessTemplate removes or replaces template syntax to make YAML parseable
func preprocessTemplate(data []byte) []byte {
	content := string(data)

	// Remove conditional blocks that would break YAML structure
	lines := strings.Split(content, "\n")
	var processedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip template control structures
		if strings.HasPrefix(trimmed, "{{-") && (strings.Contains(trimmed, "if") || strings.Contains(trimmed, "range") || strings.Contains(trimmed, "with")) {
			continue
		}
		if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "-}}") && (strings.Contains(trimmed, "end") || strings.Contains(trimmed, "else")) {
			continue
		}

		// Process template variables in the line
		processedLine := helmTemplatePattern.ReplaceAllStringFunc(line, func(match string) string {
			// For common patterns, provide reasonable defaults
			switch {
			case strings.Contains(match, ".Values.global.cluster.name"):
				return "cluster-name"
			case strings.Contains(match, ".Values.global.cluster.apiUrl"):
				return "https://cluster-api-url"
			case strings.Contains(match, ".Values.global.cluster.gitOpsRef"):
				return "main"
			case strings.Contains(match, ".Values."):
				return "placeholder-value"
			default:
				return "placeholder"
			}
		})

		processedLines = append(processedLines, processedLine)
	}

	content = strings.Join(processedLines, "\n")

	// Replace other template patterns
	content = goTemplatePattern.ReplaceAllString(content, "placeholder")

	return []byte(content)
}

// isTemplateFile checks if a file contains template syntax
func isTemplateFile(data []byte) bool {
	content := string(data)
	return helmTemplatePattern.MatchString(content) || goTemplatePattern.MatchString(content)
}

// extractVersionFromTemplate uses improved regex patterns for template files
func extractVersionFromTemplate(data []byte, selector string) (string, error) {
	content := string(data)

	// Extract the field name from the JSONPath selector
	// e.g., "$.spec.sources[0].targetRevision" -> "targetRevision"
	parts := strings.Split(selector, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid selector format: %s", selector)
	}

	fieldName := parts[len(parts)-1]

	// Create dynamic regex pattern for the field
	pattern := fmt.Sprintf(`%s:\s*([^{\s\n]+)`, regexp.QuoteMeta(fieldName))
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		value := strings.Trim(matches[1], `"'`)
		return value, nil
	}

	return "", fmt.Errorf("could not extract %s using pattern %s", fieldName, pattern)
}

// ReadValue reads a value from a YAML/JSON file using a JSONPath selector
func ReadValue(filePath, selector string) (string, error) {
	l := log.WithFields(log.Fields{
		"file":     filePath,
		"selector": selector,
	})
	l.Debug("reading value from file")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// If it's a template file, try regex extraction first
	if isTemplateFile(data) {
		l.Debug("detected template file, trying regex extraction")
		if value, err := extractVersionFromTemplate(data, selector); err == nil {
			return value, nil
		}
		l.Debug("regex extraction failed, trying template preprocessing")
	}

	// Preprocess templates if needed
	processedData := data
	if isTemplateFile(data) {
		processedData = preprocessTemplate(data)
	}

	path, err := yaml.PathString(selector)
	if err != nil {
		return "", fmt.Errorf("invalid selector %s: %w", selector, err)
	}

	var result interface{}
	if err := path.Read(strings.NewReader(string(processedData)), &result); err != nil {
		return "", fmt.Errorf("failed to read path %s from %s: %w", selector, filePath, err)
	}

	// Convert result to string
	switch v := result.(type) {
	case string:
		return v, nil
	case int, int64, float64:
		return fmt.Sprintf("%v", v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// UpdateValue surgically updates ONLY the version value, preserving all formatting
func UpdateValue(filePath, selector, newValue string) error {
	l := log.WithFields(log.Fields{
		"file":     filePath,
		"selector": selector,
		"value":    newValue,
	})
	l.Debug("surgically updating version in file")

	// Read original file as bytes to preserve everything
	originalData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Get current value to find its exact location
	currentValue, err := ReadValue(filePath, selector)
	if err != nil {
		return fmt.Errorf("failed to read current value: %w", err)
	}

	updatedData, err := updateValueBySelectorLine(string(originalData), selector, currentValue, newValue)
	if err != nil {
		return err
	}

	// Verify we only made one replacement by checking it's different
	if updatedData == string(originalData) {
		return fmt.Errorf("no replacement made - current value '%s' not found in file", currentValue)
	}

	// Write back with exact same permissions
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(updatedData), fileInfo.Mode()); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	l.Debug("successfully updated version in file")
	return nil
}

type selectorToken struct {
	key   string
	index *int
}

type pathEntry struct {
	key    string
	index  *int
	indent int
}

func updateValueBySelectorLine(content, selector, currentValue, newValue string) (string, error) {
	tokens, err := parseSelector(selector)
	if err != nil {
		return "", err
	}
	if len(tokens) == 0 {
		return "", fmt.Errorf("invalid selector format: %s", selector)
	}

	lines := strings.SplitAfter(content, "\n")
	stack := []pathEntry{}
	leaf := tokens[len(tokens)-1]

	for i, line := range lines {
		lineWithoutNewline := strings.TrimSuffix(line, "\n")
		newline := line[len(lineWithoutNewline):]
		trimmed := strings.TrimSpace(lineWithoutNewline)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := leadingSpaces(lineWithoutNewline)
		isSequenceItem := strings.HasPrefix(trimmed, "- ")
		if isSequenceItem {
			stack = popStack(stack, indent, false)
			stack = enterSequenceItem(stack, indent)
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		} else {
			stack = popStack(stack, indent, true)
		}

		key, hasValue, ok := parseMappingLine(trimmed)
		if !ok {
			continue
		}

		if key == leaf.key && pathMatches(stack, tokens[:len(tokens)-1]) && hasValue {
			updatedLine, replaced := replaceValueInLine(lineWithoutNewline, currentValue, newValue)
			if !replaced {
				continue
			}
			lines[i] = updatedLine + newline
			return strings.Join(lines, ""), nil
		}

		if !hasValue && nextTokenMatches(stack, tokens, key) {
			stack = append(stack, pathEntry{key: key, indent: indent})
		}
	}

	if updated, ok := updateFieldLineByCurrentValue(content, leaf.key, currentValue, newValue); ok {
		return updated, nil
	}

	return "", fmt.Errorf("no replacement made - current value '%s' not found in file", currentValue)
}

func parseSelector(selector string) ([]selectorToken, error) {
	selector = strings.TrimPrefix(selector, "$.")
	if selector == "" || selector == "$" {
		return nil, fmt.Errorf("invalid selector format: %s", selector)
	}

	var tokens []selectorToken
	for _, part := range strings.Split(selector, ".") {
		token := selectorToken{key: part}
		if open := strings.Index(part, "["); open >= 0 {
			close := strings.Index(part[open:], "]")
			if close < 0 {
				return nil, fmt.Errorf("invalid selector format: %s", selector)
			}
			close += open
			index, err := strconv.Atoi(part[open+1 : close])
			if err != nil {
				return nil, fmt.Errorf("invalid selector index in %s: %w", selector, err)
			}
			token.key = part[:open]
			token.index = &index
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func popStack(stack []pathEntry, indent int, popSameIndent bool) []pathEntry {
	for len(stack) > 0 {
		last := stack[len(stack)-1]
		if last.indent > indent || (popSameIndent && last.indent >= indent) {
			stack = stack[:len(stack)-1]
			continue
		}
		break
	}
	return stack
}

func enterSequenceItem(stack []pathEntry, indent int) []pathEntry {
	if len(stack) == 0 {
		return stack
	}
	parent := &stack[len(stack)-1]
	if parent.indent != indent {
		return stack
	}
	if parent.index == nil {
		index := 0
		parent.index = &index
		return stack
	}
	next := *parent.index + 1
	parent.index = &next
	return stack
}

func parseMappingLine(trimmed string) (string, bool, bool) {
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return "", false, false
	}
	key := strings.Trim(strings.TrimSpace(parts[0]), `"'`)
	if key == "" {
		return "", false, false
	}
	return key, strings.TrimSpace(parts[1]) != "", true
}

func nextTokenMatches(stack []pathEntry, tokens []selectorToken, key string) bool {
	if len(stack) >= len(tokens) {
		return false
	}
	if !pathMatches(stack, tokens[:len(stack)]) {
		return false
	}
	return tokens[len(stack)].key == key
}

func pathMatches(stack []pathEntry, tokens []selectorToken) bool {
	if len(stack) != len(tokens) {
		return false
	}
	for i := range tokens {
		if stack[i].key != tokens[i].key {
			return false
		}
		if tokens[i].index == nil {
			continue
		}
		if stack[i].index == nil || *stack[i].index != *tokens[i].index {
			return false
		}
	}
	return true
}

func replaceValueInLine(line, currentValue, newValue string) (string, bool) {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return line, false
	}
	valueStart := colon + 1
	valuePart := line[valueStart:]
	idx := strings.Index(valuePart, currentValue)
	if idx < 0 {
		return line, false
	}
	start := valueStart + idx
	end := start + len(currentValue)
	return line[:start] + newValue + line[end:], true
}

func updateFieldLineByCurrentValue(content, fieldName, currentValue, newValue string) (string, bool) {
	lines := strings.SplitAfter(content, "\n")
	for i, line := range lines {
		lineWithoutNewline := strings.TrimSuffix(line, "\n")
		newline := line[len(lineWithoutNewline):]
		trimmed := strings.TrimSpace(lineWithoutNewline)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "- ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		}
		key, hasValue, ok := parseMappingLine(trimmed)
		if !ok || !hasValue || key != fieldName {
			continue
		}

		updatedLine, replaced := replaceValueInLine(lineWithoutNewline, currentValue, newValue)
		if !replaced {
			continue
		}
		lines[i] = updatedLine + newline
		return strings.Join(lines, ""), true
	}
	return "", false
}
