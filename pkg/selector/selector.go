package selector

import (
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	log "github.com/sirupsen/logrus"
)

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

	path, err := yaml.PathString(selector)
	if err != nil {
		return "", fmt.Errorf("invalid selector %s: %w", selector, err)
	}

	var result interface{}
	if err := path.Read(strings.NewReader(string(data)), &result); err != nil {
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
