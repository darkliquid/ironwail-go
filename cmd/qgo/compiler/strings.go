package compiler

// StringTable manages a deduplicated, null-terminated string table for progs.dat.
// Offset 0 is always the empty string "".
type StringTable struct {
	data   []byte         // null-terminated strings concatenated
	lookup map[string]int32 // string -> offset in data
}

// NewStringTable creates a new string table with the empty string at offset 0.
func NewStringTable() *StringTable {
	st := &StringTable{
		data:   []byte{0}, // offset 0 = empty string (single null byte)
		lookup: map[string]int32{"": 0},
	}
	return st
}

// Intern adds a string to the table if not already present and returns its offset.
func (st *StringTable) Intern(s string) int32 {
	if ofs, ok := st.lookup[s]; ok {
		return ofs
	}
	ofs := int32(len(st.data))
	st.data = append(st.data, s...)
	st.data = append(st.data, 0) // null terminator
	st.lookup[s] = ofs
	return ofs
}

// Bytes returns the raw string table data.
func (st *StringTable) Bytes() []byte {
	return st.data
}

// Len returns the total size of the string table in bytes.
func (st *StringTable) Len() int {
	return len(st.data)
}
