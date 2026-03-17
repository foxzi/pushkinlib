package storage

import (
	"testing"
)

// TestStringArrayScan_Nil verifies Scan handles nil correctly (#10).
func TestStringArrayScan_Nil(t *testing.T) {
	var sa StringArray
	if err := sa.Scan(nil); err != nil {
		t.Fatalf("unexpected error for nil: %v", err)
	}
	if len(sa) != 0 {
		t.Errorf("expected empty array, got %v", sa)
	}
}

// TestStringArrayScan_Bytes verifies Scan handles []byte input (#10).
func TestStringArrayScan_Bytes(t *testing.T) {
	var sa StringArray
	input := []byte(`["hello","world"]`)
	if err := sa.Scan(input); err != nil {
		t.Fatalf("unexpected error for []byte: %v", err)
	}
	if len(sa) != 2 || sa[0] != "hello" || sa[1] != "world" {
		t.Errorf("unexpected result: %v", sa)
	}
}

// TestStringArrayScan_String verifies Scan handles string input (#10).
func TestStringArrayScan_String(t *testing.T) {
	var sa StringArray
	input := `["foo","bar","baz"]`
	if err := sa.Scan(input); err != nil {
		t.Fatalf("unexpected error for string: %v", err)
	}
	if len(sa) != 3 || sa[0] != "foo" || sa[1] != "bar" || sa[2] != "baz" {
		t.Errorf("unexpected result: %v", sa)
	}
}

// TestStringArrayScan_UnsupportedType verifies Scan returns error for unsupported types (#10).
func TestStringArrayScan_UnsupportedType(t *testing.T) {
	var sa StringArray
	err := sa.Scan(12345)
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
	if sa != nil {
		t.Errorf("expected nil array on error, got %v", sa)
	}
}

// TestStringArrayScan_InvalidJSON verifies Scan returns error for invalid JSON.
func TestStringArrayScan_InvalidJSON(t *testing.T) {
	var sa StringArray
	err := sa.Scan([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestStringArrayValue_Empty verifies Value returns nil for empty array.
func TestStringArrayValue_Empty(t *testing.T) {
	sa := StringArray{}
	val, err := sa.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil for empty array, got %v", val)
	}
}

// TestStringArrayValue_NonEmpty verifies Value returns valid JSON.
func TestStringArrayValue_NonEmpty(t *testing.T) {
	sa := StringArray{"a", "b"}
	val, err := sa.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bytes, ok := val.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", val)
	}
	expected := `["a","b"]`
	if string(bytes) != expected {
		t.Errorf("expected %s, got %s", expected, string(bytes))
	}
}
