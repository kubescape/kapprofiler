package inmemorymapdb

import "sync"

type InMemoryMapDB[T any] struct {
	m    map[string][]T
	lock sync.RWMutex
	// will be used when creating a new entry to be inserted
	defaultInitalCap int
}

func NewInMemoryMapDB[T any](defaultInitalCap int) *InMemoryMapDB[T] {
	return &InMemoryMapDB[T]{
		m:    make(map[string][]T),
		lock: sync.RWMutex{},
	}
}

func (db *InMemoryMapDB[T]) Put(key string, value T) {
	db.lock.Lock()
	defer db.lock.Unlock()
	// TODO: use lock per key instead of global lock
	if _, ok := db.m[key]; !ok {
		if _, ok := db.m[key]; !ok {
			db.m[key] = make([]T, 0, db.defaultInitalCap)
		}
	}

	db.m[key] = append(db.m[key], value)
}

func (db *InMemoryMapDB[T]) Get(key string) []T {
	db.lock.RLock()
	defer db.lock.RUnlock()

	return db.m[key]
}

func (db *InMemoryMapDB[T]) Delete(key string) {
	db.lock.Lock()
	defer db.lock.Unlock()

	delete(db.m, key)
}

func (db *InMemoryMapDB[T]) Exist(key string) bool {
	db.lock.RLock()
	defer db.lock.RUnlock()

	_, ok := db.m[key]
	return ok
}

func (db *InMemoryMapDB[T]) Len() int {
	db.lock.RLock()
	defer db.lock.RUnlock()

	return len(db.m)
}

func (db *InMemoryMapDB[T]) Keys() []string {
	db.lock.RLock()
	defer db.lock.RUnlock()

	keys := make([]string, 0, len(db.m))
	for k := range db.m {
		keys = append(keys, k)
	}
	return keys
}

func (db *InMemoryMapDB[T]) Values() [][]T {
	db.lock.RLock()
	defer db.lock.RUnlock()

	values := make([][]T, 0, len(db.m))
	for _, v := range db.m {
		values = append(values, v)
	}
	return values
}

func (db *InMemoryMapDB[T]) Clear() {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.m = make(map[string][]T)
}

func (db *InMemoryMapDB[T]) Close() {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.m = nil
}

func (db *InMemoryMapDB[T]) IsClosed() bool {
	db.lock.RLock()
	defer db.lock.RUnlock()

	return db.m == nil
}

func (db *InMemoryMapDB[T]) IsEmpty() bool {
	db.lock.RLock()
	defer db.lock.RUnlock()

	return len(db.m) == 0
}

func (db *InMemoryMapDB[T]) GetNClean(key string) []T {
	// we don't want to allow writes while we're returning and cleaning up
	db.lock.Lock()
	defer db.lock.Unlock()
	currentLen := len(db.m[key])
	if _, ok := db.m[key]; !ok {
		return nil
	}
	defer func() {
		// clean up the slice, but keep the capacity to avoid unnecessary allocations
		db.m[key] = make([]T, 0, currentLen)
	}()
	return db.m[key]
}
