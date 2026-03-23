package compiler

import "testing"

func TestStringTable_Empty(t *testing.T) {
	st := NewStringTable()
	if st.Len() != 1 {
		t.Fatalf("expected len 1 (null byte), got %d", st.Len())
	}
	if st.Intern("") != 0 {
		t.Fatal("empty string should be at offset 0")
	}
}

func TestStringTable_Intern(t *testing.T) {
	st := NewStringTable()
	ofs1 := st.Intern("hello")
	ofs2 := st.Intern("world")
	ofs3 := st.Intern("hello") // duplicate

	if ofs1 == 0 {
		t.Fatal("non-empty string should not be at offset 0")
	}
	if ofs3 != ofs1 {
		t.Fatalf("duplicate string should return same offset: got %d, want %d", ofs3, ofs1)
	}
	if ofs2 == ofs1 {
		t.Fatal("different strings should have different offsets")
	}

	// Verify null termination in raw data
	data := st.Bytes()
	if data[ofs1+5] != 0 {
		t.Fatal("string should be null-terminated")
	}
}

func TestStringTable_Dedup(t *testing.T) {
	st := NewStringTable()
	st.Intern("test")
	st.Intern("test")
	st.Intern("test")

	// Should have: \0 t e s t \0 = 6 bytes
	if st.Len() != 6 {
		t.Fatalf("expected 6 bytes after dedup, got %d", st.Len())
	}
}
