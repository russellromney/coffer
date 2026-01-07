package resolver

import (
	"testing"
)

func TestResolveNoReferences(t *testing.T) {
	secrets := map[string]string{
		"DATABASE_URL": "postgres://localhost/db",
		"API_KEY":      "secret123",
		"PORT":         "8080",
	}

	resolved, err := Resolve(secrets)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// All values should be unchanged
	for key, value := range secrets {
		if resolved[key] != value {
			t.Errorf("Resolve()[%s] = %v, want %v", key, resolved[key], value)
		}
	}
}

func TestResolveSimpleReference(t *testing.T) {
	secrets := map[string]string{
		"BASE_URL":     "https://api.example.com",
		"API_ENDPOINT": "${BASE_URL}/v1",
	}

	resolved, err := Resolve(secrets)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	expected := "https://api.example.com/v1"
	if resolved["API_ENDPOINT"] != expected {
		t.Errorf("Resolve()[API_ENDPOINT] = %v, want %v", resolved["API_ENDPOINT"], expected)
	}
}

func TestResolveMultipleReferences(t *testing.T) {
	secrets := map[string]string{
		"HOST":     "localhost",
		"PORT":     "5432",
		"USER":     "admin",
		"PASSWORD": "secret",
		"DATABASE": "mydb",
		"DB_URL":   "postgres://${USER}:${PASSWORD}@${HOST}:${PORT}/${DATABASE}",
	}

	resolved, err := Resolve(secrets)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	expected := "postgres://admin:secret@localhost:5432/mydb"
	if resolved["DB_URL"] != expected {
		t.Errorf("Resolve()[DB_URL] = %v, want %v", resolved["DB_URL"], expected)
	}
}

func TestResolveChainedReferences(t *testing.T) {
	secrets := map[string]string{
		"DOMAIN":       "example.com",
		"API_DOMAIN":   "api.${DOMAIN}",
		"API_ENDPOINT": "https://${API_DOMAIN}/v1",
	}

	resolved, err := Resolve(secrets)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if resolved["API_DOMAIN"] != "api.example.com" {
		t.Errorf("Resolve()[API_DOMAIN] = %v, want api.example.com", resolved["API_DOMAIN"])
	}

	if resolved["API_ENDPOINT"] != "https://api.example.com/v1" {
		t.Errorf("Resolve()[API_ENDPOINT] = %v, want https://api.example.com/v1", resolved["API_ENDPOINT"])
	}
}

func TestResolveCircularReference(t *testing.T) {
	secrets := map[string]string{
		"A": "${B}",
		"B": "${A}",
	}

	_, err := Resolve(secrets)
	if err == nil {
		t.Fatal("Resolve() should return error for circular reference")
	}

	if _, ok := err.(*ErrCircularReference); !ok {
		t.Errorf("Resolve() error type = %T, want *ErrCircularReference", err)
	}
}

func TestResolveCircularReferenceSelf(t *testing.T) {
	secrets := map[string]string{
		"A": "${A}",
	}

	_, err := Resolve(secrets)
	if err == nil {
		t.Fatal("Resolve() should return error for self-reference")
	}

	if _, ok := err.(*ErrCircularReference); !ok {
		t.Errorf("Resolve() error type = %T, want *ErrCircularReference", err)
	}
}

func TestResolveCircularReferenceDeep(t *testing.T) {
	secrets := map[string]string{
		"A": "${B}",
		"B": "${C}",
		"C": "${D}",
		"D": "${A}",
	}

	_, err := Resolve(secrets)
	if err == nil {
		t.Fatal("Resolve() should return error for deep circular reference")
	}

	if _, ok := err.(*ErrCircularReference); !ok {
		t.Errorf("Resolve() error type = %T, want *ErrCircularReference", err)
	}
}

func TestResolveUnresolvedReference(t *testing.T) {
	secrets := map[string]string{
		"API_URL": "${BASE_URL}/api",
		// BASE_URL doesn't exist
	}

	_, err := Resolve(secrets)
	if err == nil {
		t.Fatal("Resolve() should return error for unresolved reference")
	}

	if _, ok := err.(*ErrUnresolvedReference); !ok {
		t.Errorf("Resolve() error type = %T, want *ErrUnresolvedReference", err)
	}
}

func TestResolveSameReferenceTwice(t *testing.T) {
	secrets := map[string]string{
		"HOST":   "example.com",
		"CONFIG": "${HOST}:${HOST}",
	}

	resolved, err := Resolve(secrets)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	expected := "example.com:example.com"
	if resolved["CONFIG"] != expected {
		t.Errorf("Resolve()[CONFIG] = %v, want %v", resolved["CONFIG"], expected)
	}
}

func TestResolvePartialString(t *testing.T) {
	secrets := map[string]string{
		"VERSION": "v1",
		"MESSAGE": "API ${VERSION} is ready",
	}

	resolved, err := Resolve(secrets)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	expected := "API v1 is ready"
	if resolved["MESSAGE"] != expected {
		t.Errorf("Resolve()[MESSAGE] = %v, want %v", resolved["MESSAGE"], expected)
	}
}

func TestResolveEmptyMap(t *testing.T) {
	secrets := map[string]string{}

	resolved, err := Resolve(secrets)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(resolved) != 0 {
		t.Errorf("Resolve() returned %d entries, want 0", len(resolved))
	}
}

func TestResolvePreservesUnmatched(t *testing.T) {
	secrets := map[string]string{
		"SHELL_VAR": "$HOME is not replaced",    // Single $ not matched
		"CURLY":     "{NOT_A_REF}",              // No $
		"PARTIAL":   "$INCOMPLETE",              // No braces
		"LOWERCASE": "${lowercase}",             // Lowercase not matched
		"VALID":     "${ACTUAL_VAR}",
		"ACTUAL_VAR": "resolved",
	}

	resolved, err := Resolve(secrets)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if resolved["SHELL_VAR"] != "$HOME is not replaced" {
		t.Errorf("Resolve() should not replace $HOME")
	}
	if resolved["CURLY"] != "{NOT_A_REF}" {
		t.Errorf("Resolve() should not replace {NOT_A_REF}")
	}
	if resolved["PARTIAL"] != "$INCOMPLETE" {
		t.Errorf("Resolve() should not replace $INCOMPLETE")
	}
	if resolved["LOWERCASE"] != "${lowercase}" {
		t.Errorf("Resolve() should not replace ${lowercase}")
	}
	if resolved["VALID"] != "resolved" {
		t.Errorf("Resolve()[VALID] = %v, want 'resolved'", resolved["VALID"])
	}
}

func TestHasReferences(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"${VAR}", true},
		{"${A}${B}", true},
		{"prefix ${VAR} suffix", true},
		{"no references", false},
		{"$VAR", false},
		{"{VAR}", false},
		{"${lowercase}", false},
		{"", false},
	}

	for _, tt := range tests {
		result := HasReferences(tt.value)
		if result != tt.expected {
			t.Errorf("HasReferences(%q) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

func TestGetReferences(t *testing.T) {
	tests := []struct {
		value    string
		expected []string
	}{
		{"${VAR}", []string{"VAR"}},
		{"${A}${B}", []string{"A", "B"}},
		{"prefix ${VAR} suffix", []string{"VAR"}},
		{"no references", []string{}},
		{"${A} middle ${B} end ${C}", []string{"A", "B", "C"}},
		{"${SAME}${SAME}", []string{"SAME", "SAME"}},
	}

	for _, tt := range tests {
		result := GetReferences(tt.value)
		if len(result) != len(tt.expected) {
			t.Errorf("GetReferences(%q) length = %d, want %d", tt.value, len(result), len(tt.expected))
			continue
		}
		for i, ref := range result {
			if ref != tt.expected[i] {
				t.Errorf("GetReferences(%q)[%d] = %v, want %v", tt.value, i, ref, tt.expected[i])
			}
		}
	}
}

func TestErrCircularReferenceMessage(t *testing.T) {
	err := &ErrCircularReference{
		Key:  "A",
		Path: []string{"B", "C", "A"},
	}

	msg := err.Error()
	if msg == "" {
		t.Error("ErrCircularReference.Error() returned empty string")
	}
	if !contains(msg, "A") || !contains(msg, "circular") {
		t.Errorf("ErrCircularReference.Error() = %q, should mention key and 'circular'", msg)
	}
}

func TestErrUnresolvedReferenceMessage(t *testing.T) {
	err := &ErrUnresolvedReference{
		Key:       "API_URL",
		Reference: "BASE_URL",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("ErrUnresolvedReference.Error() returned empty string")
	}
	if !contains(msg, "API_URL") || !contains(msg, "BASE_URL") {
		t.Errorf("ErrUnresolvedReference.Error() = %q, should mention both keys", msg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
