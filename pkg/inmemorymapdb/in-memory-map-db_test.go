package inmemorymapdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInMemoryMapDB(t *testing.T) {
	db := NewInMemoryMapDB[int](10)

	// Test Put and Get
	db.Put("key1", 1)
	db.Put("key1", 2)
	db.Put("key2", 3)

	assert.Equal(t, []int{1, 2}, db.Get("key1"))
	assert.Equal(t, []int{3}, db.Get("key2"))

	// Test Delete
	db.Delete("key1")
	assert.Equal(t, []int(nil), db.Get("key1"))

	// Test Exist
	assert.True(t, db.Exist("key2"))
	assert.False(t, db.Exist("key1"))

	// Test Len
	assert.Equal(t, 1, db.Len())

	// Test Keys
	assert.Equal(t, []string{"key2"}, db.Keys())

	// Test Values
	assert.Equal(t, [][]int{{3}}, db.Values())

	// Test Clear
	db.Clear()
	assert.Equal(t, 0, db.Len())
	assert.Equal(t, []string{}, db.Keys())
	assert.Equal(t, [][]int{}, db.Values())

	assert.Equal(t, 0, len(db.Get("key1")))

	for idx := range db.Get("key1") {
		t.Errorf("db.Get(\"key1\")[%d] = %d, want nothing",
			idx, db.Get("key1")[idx])
	}
	// Test Close
	db.Close()
	assert.True(t, db.IsClosed())

	// Test IsEmpty
	assert.True(t, db.IsEmpty())
}

// test GetNClean

func TestInMemoryMapDB_GetNClean(t *testing.T) {
	db := NewInMemoryMapDB[int](10)

	// Test Put and Get
	db.Put("key1", 1)
	db.Put("key1", 2)
	db.Put("key2", 3)

	assert.Equal(t, []int{1, 2}, db.Get("key1"))
	assert.Equal(t, []int{3}, db.Get("key2"))

	// Test GetNClean
	assert.Equal(t, []int{1, 2}, db.GetNClean("key1"))
	assert.Equal(t, []int{3}, db.GetNClean("key2"))
	key1 := db.GetNClean("key1")
	assert.Equal(t, []int{}, key1)
	assert.Equal(t, 2, cap(key1))
	assert.Equal(t, 0, len(key1))
	assert.Equal(t, []int{}, db.GetNClean("key2"))

}
