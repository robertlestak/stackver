package selector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateValueUpdatesOnlySelectedYAMLPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "values.yaml")
	input := `controllers:
  cloudfront:
    version: 1.5.0
  cloudwatch:
    version: 1.5.0
  sns:
    version: 1.5.0
`
	if err := os.WriteFile(file, []byte(input), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := UpdateValue(file, "$.controllers.cloudfront.version", "1.7.0"); err != nil {
		t.Fatalf("UpdateValue returned error: %v", err)
	}

	cloudfront, err := ReadValue(file, "$.controllers.cloudfront.version")
	if err != nil {
		t.Fatalf("read cloudfront: %v", err)
	}
	cloudwatch, err := ReadValue(file, "$.controllers.cloudwatch.version")
	if err != nil {
		t.Fatalf("read cloudwatch: %v", err)
	}
	sns, err := ReadValue(file, "$.controllers.sns.version")
	if err != nil {
		t.Fatalf("read sns: %v", err)
	}

	if cloudfront != "1.7.0" {
		t.Fatalf("cloudfront = %q, want 1.7.0", cloudfront)
	}
	if cloudwatch != "1.5.0" {
		t.Fatalf("cloudwatch = %q, want 1.5.0", cloudwatch)
	}
	if sns != "1.5.0" {
		t.Fatalf("sns = %q, want 1.5.0", sns)
	}
}

func TestUpdateValueUpdatesTemplateFieldLine(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "template.yaml")
	input := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: {{ .Values.global.cluster.name }}-vpa
spec:
  sources:
  - repoURL: https://github.com/kubernetes/autoscaler
    targetRevision: vertical-pod-autoscaler-chart-0.7.0
`
	if err := os.WriteFile(file, []byte(input), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := UpdateValue(file, "$.spec.sources[0].targetRevision", "vertical-pod-autoscaler-chart-0.10.0"); err != nil {
		t.Fatalf("UpdateValue returned error: %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "targetRevision: vertical-pod-autoscaler-chart-0.10.0") {
		t.Fatalf("updated targetRevision not found in output:\n%s", output)
	}
	if strings.Contains(output, "targetRevision: vertical-pod-autoscaler-chart-0.7.0") {
		t.Fatalf("old targetRevision still present in output:\n%s", output)
	}
}
