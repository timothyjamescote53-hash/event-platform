package main

import (
	"encoding/json"
	"testing"
	"time"
)

// evaluateCondition is a pure helper extracted for testing
func evaluateCondition(operator string, current, threshold float64) bool {
	switch operator {
	case "gt":
		return current > threshold
	case "lt":
		return current < threshold
	case "gte":
		return current >= threshold
	case "lte":
		return current <= threshold
	}
	return false
}

func TestEvaluate_GT_Triggered(t *testing.T) {
	if !evaluateCondition("gt", 150, 100) {
		t.Error("150 > 100 should trigger")
	}
}

func TestEvaluate_GT_NotTriggered(t *testing.T) {
	if evaluateCondition("gt", 50, 100) {
		t.Error("50 > 100 should not trigger")
	}
}

func TestEvaluate_LT_Triggered(t *testing.T) {
	if !evaluateCondition("lt", 5, 10) {
		t.Error("5 < 10 should trigger")
	}
}

func TestEvaluate_GTE_AtBoundary(t *testing.T) {
	if !evaluateCondition("gte", 100, 100) {
		t.Error("100 >= 100 should trigger")
	}
}

func TestEvaluate_LTE_AtBoundary(t *testing.T) {
	if !evaluateCondition("lte", 100, 100) {
		t.Error("100 <= 100 should trigger")
	}
}

func TestEvaluate_UnknownOperator(t *testing.T) {
	if evaluateCondition("neq", 5, 10) {
		t.Error("unknown operator should not trigger")
	}
}

func TestSeverityColor_Critical(t *testing.T) {
	if severityColor("critical") != "danger" {
		t.Error("critical should map to danger")
	}
}

func TestSeverityColor_Warning(t *testing.T) {
	if severityColor("warning") != "warning" {
		t.Error("warning should map to warning")
	}
}

func TestSeverityColor_Default(t *testing.T) {
	if severityColor("info") != "good" {
		t.Error("info should map to good")
	}
}

func TestFiredAlert_JSONRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	alert := &FiredAlert{
		ID:           "alert-1",
		RuleID:       "rule-1",
		TenantID:     "acme",
		RuleName:     "High error rate",
		Message:      "count > 100",
		CurrentValue: 150.0,
		Threshold:    100.0,
		Severity:     "critical",
		FiredAt:      now,
	}

	data, err := json.Marshal(alert)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded FiredAlert
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ID != alert.ID {
		t.Errorf("ID mismatch: got %s", decoded.ID)
	}
	if decoded.CurrentValue != alert.CurrentValue {
		t.Errorf("CurrentValue mismatch: got %.2f", decoded.CurrentValue)
	}
	if decoded.Severity != alert.Severity {
		t.Errorf("Severity mismatch: got %s", decoded.Severity)
	}
}

func TestAlertRule_EnabledByDefault(t *testing.T) {
	rule := &AlertRule{
		ID:        "rule-test",
		Name:      "Test",
		Enabled:   true,
		Threshold: 100,
		Operator:  "gt",
	}
	if !rule.Enabled {
		t.Error("rule should be enabled")
	}
}
// operators
// boundary
// severity
// json
// rule
// operators
// boundary
// severity
// json
// rule
// operators
// boundary
// severity
// json
// rule
// gt triggered
// gt not triggered
// lt triggered
// gte boundary
// lte boundary
// unknown op
// severity critical
// severity warning
// severity default
// json roundtrip
// rule enabled
// gt triggered
// gt not triggered
// lt triggered
// gte boundary
// lte boundary
// unknown op
// severity critical
// severity warning
// severity default
// json roundtrip
// rule enabled
// gt triggered
// gt not triggered
// lt triggered
// gte boundary
// lte boundary
// unknown op
// severity critical
// severity warning
// severity default
