package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicCRUD(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	key, val := "k1", []byte("v1")
	assert.NoError(t, db.Put(key, val))
	got, err := db.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, val, got)

	assert.NoError(t, db.Delete(key))
	_, err = db.Get(key)
	assert.Error(t, err)
}

// func TestTTL(t *testing.T) {
// 	db, cleanup := newTestDB(t)
// 	defer cleanup()

// 	key := "ttlKey"
// 	assert.NoError(t, db.PutWithTTL(key, []byte("data"), time.Second))

// 	rem, err := db.GetTTL(key)
// 	assert.NoError(t, err)
// 	assert.Greater(t, rem, time.Duration(0))

// 	time.Sleep(1100 * time.Millisecond)
// 	_, err = db.Get(key)
// 	assert.Error(t, err)
// }

func TestListKeys(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	for i := 0; i < 5; i++ {
		assert.NoError(t, db.Put(fmt.Sprintf("key%d", i), []byte("v")))
	}
	keys := db.ListKeys()
	assert.Len(t, keys, 5)
}

func TestTransaction(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	tx, err := db.BeginTransaction()
	assert.NoError(t, err)
	_ = tx.Put("tx1", []byte("a"))
	_ = tx.Delete("tx1")
	assert.NoError(t, tx.Commit())

	_, err = db.Get("tx1")
	assert.Error(t, err)
}

func TestCompact(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	for i := 0; i < 100; i++ {
		db.Put(fmt.Sprintf("k%d", i), []byte("v"))
		db.Delete(fmt.Sprintf("k%d", i))
	}
	assert.NoError(t, db.Compact())
	keys := db.ListKeys()
	assert.Len(t, keys, 0)
}
