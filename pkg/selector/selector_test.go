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
  # CloudFront controller
  cloudfront:
    version: 1.5.0 # keep this comment
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

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "# CloudFront controller") {
		t.Fatalf("standalone comment was not preserved:\n%s", output)
	}
	if !strings.Contains(output, "version: 1.7.0 # keep this comment") {
		t.Fatalf("inline comment was not preserved:\n%s", output)
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

func TestUpdateValueUpdatesTemplateFieldByCurrentValueWhenPathIndexDoesNotMatch(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "template.yaml")
	input := `spec:
  sources:
  - repoURL: https://github.com/umg/devops-k8s-management
    targetRevision: {{ .Values.global.cluster.gitOpsRef }}
    ref: values
  - repoURL: https://github.com/coredns/helm
    targetRevision: coredns-1.45.1 # chart version
    path: charts/coredns
`
	if err := os.WriteFile(file, []byte(input), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := UpdateValue(file, "$.spec.sources[0].targetRevision", "coredns-1.46.2"); err != nil {
		t.Fatalf("UpdateValue returned error: %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "targetRevision: {{ .Values.global.cluster.gitOpsRef }}") {
		t.Fatalf("templated targetRevision should not be changed:\n%s", output)
	}
	if !strings.Contains(output, "targetRevision: coredns-1.46.2 # chart version") {
		t.Fatalf("chart targetRevision not updated with comment preserved:\n%s", output)
	}
}

func TestUpdateValueUpdatesIndexedYAMLPathWithoutDroppingComments(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Chart.yaml")
	input := `apiVersion: v2
dependencies:
- name: umg-platform # dependency name
  version: 1.2.0 # dependency version
- name: other
  version: 1.2.0
`
	if err := os.WriteFile(file, []byte(input), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := UpdateValue(file, "$.dependencies[0].version", "1.2.5"); err != nil {
		t.Fatalf("UpdateValue returned error: %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "version: 1.2.5 # dependency version") {
		t.Fatalf("selected dependency version not updated with comment preserved:\n%s", output)
	}
	if !strings.Contains(output, "version: 1.2.0") {
		t.Fatalf("unselected dependency version was not preserved:\n%s", output)
	}
	if !strings.Contains(output, "# dependency name") {
		t.Fatalf("dependency name comment was not preserved:\n%s", output)
	}
}
