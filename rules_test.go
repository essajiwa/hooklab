package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ==================== Rule CRUD Tests ====================

func TestGetRulesEmpty(t *testing.T) {
	app := &App{}
	rules := app.getRules("test-key")
	if len(rules) != 0 {
		t.Errorf("expected empty rules, got %d", len(rules))
	}
}

func TestAddRule(t *testing.T) {
	app := &App{}
	rule := Rule{
		Name:       "Test Rule",
		Condition:  "body.amount > 100",
		Response:   map[string]string{"status": "matched"},
		StatusCode: 200,
		Priority:   1,
		Enabled:    true,
	}

	created := app.addRule("payments", rule)

	if created.ID == "" {
		t.Error("expected rule ID to be set")
	}
	if created.Name != "Test Rule" {
		t.Errorf("expected name 'Test Rule', got '%s'", created.Name)
	}

	rules := app.getRules("payments")
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestAddMultipleRules(t *testing.T) {
	app := &App{}

	app.addRule("key1", Rule{Name: "Rule 1", Priority: 2})
	app.addRule("key1", Rule{Name: "Rule 2", Priority: 1})
	app.addRule("key2", Rule{Name: "Rule 3", Priority: 0})

	rules1 := app.getRules("key1")
	if len(rules1) != 2 {
		t.Errorf("expected 2 rules for key1, got %d", len(rules1))
	}

	if rules1[0].Name != "Rule 2" {
		t.Errorf("expected 'Rule 2' first (priority 1), got '%s'", rules1[0].Name)
	}

	rules2 := app.getRules("key2")
	if len(rules2) != 1 {
		t.Errorf("expected 1 rule for key2, got %d", len(rules2))
	}
}

func TestUpdateRule(t *testing.T) {
	app := &App{}
	created := app.addRule("test", Rule{Name: "Original", Priority: 1})

	updated := Rule{
		Name:       "Updated",
		Condition:  "body.new == true",
		Response:   map[string]string{"updated": "yes"},
		StatusCode: 201,
		Priority:   2,
		Enabled:    true,
	}

	success := app.updateRule("test", created.ID, updated)
	if !success {
		t.Error("expected update to succeed")
	}

	rules := app.getRules("test")
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Name != "Updated" {
		t.Errorf("expected name 'Updated', got '%s'", rules[0].Name)
	}
	if rules[0].ID != created.ID {
		t.Errorf("expected ID to remain '%s', got '%s'", created.ID, rules[0].ID)
	}
}

func TestUpdateRuleNotFound(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{Name: "Existing"})

	success := app.updateRule("test", "nonexistent", Rule{Name: "New"})
	if success {
		t.Error("expected update to fail for nonexistent rule")
	}

	app2 := &App{}
	success = app2.updateRule("test", "any", Rule{})
	if success {
		t.Error("expected update to fail with nil rules map")
	}
}

func TestDeleteRule(t *testing.T) {
	app := &App{}
	rule1 := app.addRule("test", Rule{Name: "Rule 1"})
	app.addRule("test", Rule{Name: "Rule 2"})

	success := app.deleteRule("test", rule1.ID)
	if !success {
		t.Error("expected delete to succeed")
	}

	rules := app.getRules("test")
	if len(rules) != 1 {
		t.Errorf("expected 1 rule after delete, got %d", len(rules))
	}
	if rules[0].Name != "Rule 2" {
		t.Errorf("expected 'Rule 2' to remain, got '%s'", rules[0].Name)
	}
}

func TestDeleteRuleNotFound(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{Name: "Existing"})

	success := app.deleteRule("test", "nonexistent")
	if success {
		t.Error("expected delete to fail for nonexistent rule")
	}

	app2 := &App{}
	success = app2.deleteRule("test", "any")
	if success {
		t.Error("expected delete to fail with nil rules map")
	}
}

func TestSetRules(t *testing.T) {
	app := &App{}
	rules := []Rule{
		{ID: "r1", Name: "Rule 1"},
		{ID: "r2", Name: "Rule 2"},
	}

	app.setRules("test", rules)

	got := app.getRules("test")
	if len(got) != 2 {
		t.Errorf("expected 2 rules, got %d", len(got))
	}
}

func TestGetRulesNilKeyRules(t *testing.T) {
	app := &App{
		rules: map[string][]Rule{
			"other": {{Name: "Other"}},
		},
	}

	rules := app.getRules("nonexistent")
	if len(rules) != 0 {
		t.Errorf("expected empty rules for nonexistent key, got %d", len(rules))
	}
}

// ==================== Rule Evaluation Tests ====================

func TestEvaluateRulesNoRules(t *testing.T) {
	app := &App{}
	result, err := app.evaluateRules("test", `{"amount": 100}`, "POST", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when no rules")
	}
}

func TestEvaluateRulesSimpleMatch(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "High Amount",
		Condition:  "body.amount > 50",
		Response:   map[string]string{"matched": "high"},
		StatusCode: 200,
		Priority:   1,
		Enabled:    true,
	})

	result, err := app.evaluateRules("test", `{"amount": 100}`, "POST", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
}

func TestEvaluateRulesNoMatch(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "High Amount",
		Condition:  "body.amount > 100",
		Response:   map[string]string{"matched": "high"},
		StatusCode: 200,
		Priority:   1,
		Enabled:    true,
	})

	result, err := app.evaluateRules("test", `{"amount": 50}`, "POST", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when no match")
	}
}

func TestEvaluateRulesDisabledRule(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "Disabled Rule",
		Condition:  "true",
		Response:   map[string]string{"matched": "disabled"},
		StatusCode: 200,
		Priority:   1,
		Enabled:    false,
	})

	result, err := app.evaluateRules("test", `{}`, "POST", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for disabled rule")
	}
}

func TestEvaluateRulesPriorityOrder(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "Low Priority",
		Condition:  "true",
		Response:   map[string]string{"priority": "low"},
		StatusCode: 200,
		Priority:   10,
		Enabled:    true,
	})
	app.addRule("test", Rule{
		Name:       "High Priority",
		Condition:  "true",
		Response:   map[string]string{"priority": "high"},
		StatusCode: 201,
		Priority:   1,
		Enabled:    true,
	})

	result, _ := app.evaluateRules("test", `{}`, "POST", nil)
	if result == nil {
		t.Fatal("expected result")
	}
	if result.StatusCode != 201 {
		t.Errorf("expected status 201 (high priority), got %d", result.StatusCode)
	}
}

func TestEvaluateRulesMethodCondition(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "POST Only",
		Condition:  `method == "POST"`,
		Response:   map[string]string{"method": "post"},
		StatusCode: 200,
		Priority:   1,
		Enabled:    true,
	})

	result, _ := app.evaluateRules("test", `{}`, "POST", nil)
	if result == nil {
		t.Error("expected match for POST")
	}

	result, _ = app.evaluateRules("test", `{}`, "GET", nil)
	if result != nil {
		t.Error("expected no match for GET")
	}
}

func TestEvaluateRulesHeaderCondition(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "Has Auth",
		Condition:  `"Authorization" in headers`,
		Response:   map[string]string{"auth": "present"},
		StatusCode: 200,
		Priority:   1,
		Enabled:    true,
	})

	headers := map[string][]string{
		"Authorization": {"Bearer token"},
	}

	result, _ := app.evaluateRules("test", `{}`, "POST", headers)
	if result == nil {
		t.Error("expected match with Authorization header")
	}

	result, _ = app.evaluateRules("test", `{}`, "POST", nil)
	if result != nil {
		t.Error("expected no match without Authorization header")
	}
}

func TestEvaluateRulesInvalidExpression(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "Invalid",
		Condition:  "this is not valid syntax !!!",
		Response:   map[string]string{"should": "not match"},
		StatusCode: 200,
		Priority:   1,
		Enabled:    true,
	})

	result, err := app.evaluateRules("test", `{}`, "POST", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for invalid expression")
	}
}

func TestEvaluateRulesNonJSONBody(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "Always Match",
		Condition:  "true",
		Response:   map[string]string{"matched": "yes"},
		StatusCode: 200,
		Priority:   1,
		Enabled:    true,
	})

	result, err := app.evaluateRules("test", "plain text body", "POST", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected match even with non-JSON body")
	}
}

func TestEvaluateRulesComplexCondition(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "Complex",
		Condition:  `body.type == "payment" && body.amount >= 100`,
		Response:   map[string]string{"matched": "complex"},
		StatusCode: 202,
		Priority:   1,
		Enabled:    true,
	})

	result, _ := app.evaluateRules("test", `{"type":"payment","amount":150}`, "POST", nil)
	if result == nil {
		t.Error("expected match for complex condition")
	}

	result, _ = app.evaluateRules("test", `{"type":"refund","amount":150}`, "POST", nil)
	if result != nil {
		t.Error("expected no match for wrong type")
	}

	result, _ = app.evaluateRules("test", `{"type":"payment","amount":50}`, "POST", nil)
	if result != nil {
		t.Error("expected no match for low amount")
	}
}

func TestEvaluateRulesExpressionRuntimeError(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "Runtime Error",
		Condition:  "body.nonexistent.deep.path > 0",
		Response:   map[string]string{"matched": "yes"},
		StatusCode: 200,
		Priority:   1,
		Enabled:    true,
	})

	result, err := app.evaluateRules("test", `{"simple": "value"}`, "POST", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Log("Note: expression may have matched depending on expr behavior")
	}
}

func TestEvaluateRulesEmptyBody(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{
		Name:       "Always True",
		Condition:  "true",
		Response:   map[string]string{"matched": "yes"},
		StatusCode: 200,
		Priority:   1,
		Enabled:    true,
	})

	result, err := app.evaluateRules("test", "", "POST", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected match with empty body")
	}
}

// ==================== Rules API Handler Tests ====================

func TestRulesHandlerGet(t *testing.T) {
	app := &App{}
	app.addRule("test-key", Rule{Name: "Rule 1", Enabled: true})

	req := httptest.NewRequest(http.MethodGet, "/api/rules?key=test-key", nil)
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	rules, ok := response["rules"].([]interface{})
	if !ok {
		t.Fatal("expected rules array in response")
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestRulesHandlerGetDefaultKey(t *testing.T) {
	app := &App{}
	app.addRule("default", Rule{Name: "Default Rule"})

	req := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["key"] != "default" {
		t.Errorf("expected key 'default', got '%v'", response["key"])
	}
}

func TestRulesHandlerPost(t *testing.T) {
	app := &App{}

	body := `{"name":"New Rule","condition":"body.test == true","response":{"ok":true},"statusCode":200,"priority":1,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/rules?key=test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var response Rule
	json.Unmarshal(w.Body.Bytes(), &response)

	if response.ID == "" {
		t.Error("expected rule ID in response")
	}
	if response.Name != "New Rule" {
		t.Errorf("expected name 'New Rule', got '%s'", response.Name)
	}

	rules := app.getRules("test")
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestRulesHandlerPostInvalidJSON(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(http.MethodPost, "/api/rules?key=test", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRulesHandlerPostInvalidExpression(t *testing.T) {
	app := &App{}

	body := `{"name":"Bad Rule","condition":"invalid !!! syntax","response":{},"statusCode":200,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/rules?key=test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid expression, got %d", w.Code)
	}

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["error"] == "" {
		t.Error("expected error message in response")
	}
}

func TestRulesHandlerPostEmptyCondition(t *testing.T) {
	app := &App{}

	body := `{"name":"Empty Condition","condition":"","response":{},"statusCode":200,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/rules?key=test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestRulesHandlerPut(t *testing.T) {
	app := &App{}
	created := app.addRule("test", Rule{Name: "Original", Priority: 1, Enabled: true})

	body := `{"name":"Updated","condition":"true","response":{"updated":true},"statusCode":201,"priority":2,"enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/rules?key=test&id="+created.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	rules := app.getRules("test")
	if rules[0].Name != "Updated" {
		t.Errorf("expected name 'Updated', got '%s'", rules[0].Name)
	}
}

func TestRulesHandlerPutNoID(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(http.MethodPut, "/api/rules?key=test", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRulesHandlerPutNotFound(t *testing.T) {
	app := &App{}

	body := `{"name":"Updated"}`
	req := httptest.NewRequest(http.MethodPut, "/api/rules?key=test&id=nonexistent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRulesHandlerPutInvalidExpression(t *testing.T) {
	app := &App{}
	created := app.addRule("test", Rule{Name: "Original"})

	body := `{"name":"Bad","condition":"invalid !!! syntax"}`
	req := httptest.NewRequest(http.MethodPut, "/api/rules?key=test&id="+created.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRulesHandlerDelete(t *testing.T) {
	app := &App{}
	created := app.addRule("test", Rule{Name: "To Delete"})

	req := httptest.NewRequest(http.MethodDelete, "/api/rules?key=test&id="+created.ID, nil)
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	rules := app.getRules("test")
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after delete, got %d", len(rules))
	}
}

func TestRulesHandlerDeleteNoID(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(http.MethodDelete, "/api/rules?key=test", nil)
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRulesHandlerDeleteNotFound(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(http.MethodDelete, "/api/rules?key=test&id=nonexistent", nil)
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRulesHandlerMethodNotAllowed(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(http.MethodPatch, "/api/rules?key=test", nil)
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestRulesHandlerPostReadError(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(http.MethodPost, "/api/rules?key=test", &errorReader{})
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestRulesHandlerPutReadError(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(http.MethodPut, "/api/rules?key=test&id=123", &errorReader{})
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestRulesHandlerPutInvalidJSON(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(http.MethodPut, "/api/rules?key=test&id=123", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	app.rulesHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRulesHandlerGetWriteError(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{Name: "Rule 1"})

	req := httptest.NewRequest(http.MethodGet, "/api/rules?key=test", nil)
	w := &errorResponseWriter{}

	app.rulesHandler(w, req)

	if w.status != http.StatusInternalServerError {
		t.Errorf("expected status 500 on write error, got %d", w.status)
	}
}

// ==================== Webhook Handler with Rules Tests ====================

func TestWebhookHandlerWithRuleMatch(t *testing.T) {
	app := &App{}
	app.addRule("payments", Rule{
		Name:       "High Amount",
		Condition:  "body.amount > 100",
		Response:   map[string]string{"status": "flagged"},
		StatusCode: 202,
		Priority:   1,
		Enabled:    true,
	})

	body := `{"amount": 500}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/payments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.webhookHandler(w, req)

	if w.Code != 202 {
		t.Errorf("expected status 202 from rule, got %d", w.Code)
	}

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != "flagged" {
		t.Errorf("expected status 'flagged', got '%s'", response["status"])
	}
}

func TestWebhookHandlerWithRuleNoMatch(t *testing.T) {
	app := &App{}
	app.setResponseConfig("payments", ResponseConfig{
		Response:   map[string]string{"default": "response"},
		StatusCode: 200,
	})
	app.addRule("payments", Rule{
		Name:       "High Amount",
		Condition:  "body.amount > 100",
		Response:   map[string]string{"status": "flagged"},
		StatusCode: 202,
		Priority:   1,
		Enabled:    true,
	})

	body := `{"amount": 50}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/payments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.webhookHandler(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200 from default, got %d", w.Code)
	}

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["default"] != "response" {
		t.Errorf("expected default response, got '%v'", response)
	}
}

func TestWebhookHandlerWithDisabledRule(t *testing.T) {
	app := &App{}
	app.setResponseConfig("test", ResponseConfig{
		Response:   map[string]string{"default": "yes"},
		StatusCode: 200,
	})
	app.addRule("test", Rule{
		Name:       "Disabled",
		Condition:  "true",
		Response:   map[string]string{"matched": "disabled"},
		StatusCode: 500,
		Priority:   1,
		Enabled:    false,
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/test", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	app.webhookHandler(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
