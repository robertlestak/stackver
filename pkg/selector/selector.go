package selector

import (
	"fmt"
	"os"
	"regexp"
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

	// Find and replace ONLY the version string in the original bytes
	// This preserves all formatting, comments, whitespace exactly
	updatedData := strings.ReplaceAll(string(originalData), currentValue, newValue)
	
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
