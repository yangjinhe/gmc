// Copyright 2020 The GMC Author. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// More information at https://github.com/snail007/gmc

package gmap

import (
	"container/list"
	"fmt"
	"sync"
)

type (
	// M alias of type map[string]interface{}
	M = map[string]interface{}
	// Mii alias of type map[interface{}]interface{}
	Mii = map[interface{}]interface{}
	// Mss alias of type map[string]string
	Mss = map[string]string
)

// Map a map can kept the sequence of keys when range the map
// have more useful function, Len(), Shift(), Pop(), Keys(), etc.
type Map struct {
	keys     *list.List
	data     sync.Map
	lock     sync.Mutex
	keyElMap map[interface{}]*list.Element
}

// Clone duplicates the map s.
func (s *Map) Clone() *Map {
	m := NewMap()
	for _, k := range s.Keys() {
		v, _ := s.data.Load(k)
		m.Store(k, v)
	}
	return m
}

// ToMap duplicates the map s.
func (s *Map) ToMap() map[interface{}]interface{} {
	m := map[interface{}]interface{}{}
	s.data.Range(func(key, value interface{}) bool {
		m[key] = value
		return true
	})
	return m
}

// ToStringMap duplicates the map s.
func (s *Map) ToStringMap() map[string]interface{} {
	m := map[string]interface{}{}
	s.data.Range(func(key, value interface{}) bool {
		m[fmt.Sprintf("%v", key)] = value
		return true
	})
	return m
}

// Merge merges a Map to Map s.
func (s *Map) Merge(m *Map) {
	m.data.Range(func(key, value interface{}) bool {
		s.Store(key, value)
		return true
	})
}

// MergeMap merges a map to Map s.
func (s *Map) MergeMap(m Mii) {
	for key, value := range m {
		s.Store(key, value)
	}
}

// MergeStringMap merges a map to Map s.
func (s *Map) MergeStrMap(m M) {
	for key, value := range m {
		s.Store(key, value)
	}
}

// MergeStrStrMap merges a map to Map s.
func (s *Map) MergeStrStrMap(m Mss) {
	for key, value := range m {
		s.Store(key, value)
	}
}

// MergeSyncMap merges a sync.Map to Map s.
func (s *Map) MergeSyncMap(m *sync.Map) {
	m.Range(func(key, value interface{}) bool {
		s.Store(key, value)
		return true
	})
}

// Pop returns the last element of map s or nil if the map is empty.
func (s *Map) Pop() (k, v interface{}, ok bool) {
	return s.removeElement(s.keys.Back())
}

// Shift returns the first element of map s or nil if the map is empty.
func (s *Map) Shift() (k, v interface{}, ok bool) {
	return s.removeElement(s.keys.Front())
}

// used for shift and pop
func (s *Map) removeElement(el *list.Element) (k, v interface{}, ok bool) {
	if el == nil {
		return
	}
	v, ok = s.data.Load(el.Value)
	if ok {
		k = el.Value
		s.Delete(el.Value)
	}
	return
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (s *Map) Load(key interface{}) (value interface{}, ok bool) {
	value, ok = s.data.Load(key)
	return
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (s *Map) LoadOrStore(key, value interface{}) (actual interface{}, loaded bool) {
	actual, loaded = s.data.LoadOrStore(key, value)
	if !loaded {
		s.lock.Lock()
		s.keyElMap[key] = s.keys.PushBack(key)
		s.lock.Unlock()
	}
	return
}

// LoadOrStoreFront returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
// The key will be stored the first in keys queue if key not exists.
func (s *Map) LoadOrStoreFront(key, value interface{}) (actual interface{}, loaded bool) {
	actual, loaded = s.data.LoadOrStore(key, value)
	if !loaded {
		s.lock.Lock()
		s.keyElMap[key] = s.keys.PushFront(key)
		s.lock.Unlock()
	}
	return
}

// StoreFront sets the value for a key.
// The key will be stored the first in keys queue.
func (s *Map) StoreFront(key, value interface{}) {
	s.data.Store(key, value)
	s.lock.Lock()
	if v, ok := s.keyElMap[key]; ok {
		s.keys.Remove(v)
	}
	s.keyElMap[key] = s.keys.PushFront(key)
	s.lock.Unlock()
}

// Store sets the value for a key.
func (s *Map) Store(key, value interface{}) {
	s.data.Store(key, value)
	s.lock.Lock()
	if v, ok := s.keyElMap[key]; ok {
		s.keys.Remove(v)
	}
	s.keyElMap[key] = s.keys.PushBack(key)
	s.lock.Unlock()
}

// Delete deletes the value for a key.
func (s *Map) Delete(key interface{}) {
	s.data.Delete(key)
	s.lock.Lock()
	if el, ok := s.keyElMap[key]; ok {
		s.keys.Remove(el)
		delete(s.keyElMap, key)
	}
	s.lock.Unlock()
}

// Len returns the length of the map s.
func (s *Map) Len() int {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.keys.Len()
}

// Clear deletes all data in the map s.
func (s *Map) Clear() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data = sync.Map{}
	s.keys = list.New()
	s.keyElMap = map[interface{}]*list.Element{}
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Range keep the sequence of store sequence.
func (s *Map) Range(f func(key, value interface{}) bool) {
	snapshot := s.Clone()
	for _, k := range snapshot.Keys() {
		v, _ := snapshot.data.Load(k)
		if !f(k, v) {
			break
		}
	}
}

// RangeFast calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// RangeFast keep the sequence of store sequence.
//
// RangeFast do not create a snapshot for range, so you can not
// modify map s in range loop, indicate do not call Delete(), Store(), LoadOrStore(), Merge(), etc.
func (s *Map) RangeFast(f func(key, value interface{}) bool) {
	for _, k := range s.Keys() {
		v, _ := s.data.Load(k)
		if !f(k, v) {
			break
		}
	}
}

// Keys returns all keys in map s and keep the sequence of store sequence.
func (s *Map) Keys() (keys []interface{}) {
	p := s.keys.Front()
	for {
		if p == nil {
			break
		}
		keys = append(keys, p.Value)
		p = p.Next()
	}
	return
}

// StringKeys returns all keys in map s and keep the sequence of store sequence.
func (s *Map) StringKeys() (keys []string) {
	for _, v := range s.Keys() {
		keys = append(keys, fmt.Sprintf("%v", v))
	}
	return
}

// IsEmpty indicates if the map is empty.
func (s *Map) IsEmpty() bool {
	return s.keys.Len() == 0
}

// IndexOf indicates the index of value in Map s, if not found returns -1.
//
// idx start with 0.
func (s *Map) IndexOf(k interface{}) int {
	p := s.keys.Front()
	p0 := s.keys.Back()
	for i := 0; i <= s.keys.Len()/2; i++ {
		if p != nil {
			if p.Value == k {
				return i
			}
			p = p.Next()
		}
		if p0 != nil {
			if p0.Value == k {
				return s.keys.Len() - 1 - i
			}
			p0 = p0.Prev()
		}
	}
	return -1
}

// String returns string format of the Set.
func (s *Map) String() string {
	return fmt.Sprintf("%v", s.ToMap())
}

// NewMap creates a Map object.
func NewMap() *Map {
	return &Map{
		keys:     list.New(),
		data:     sync.Map{},
		lock:     sync.Mutex{},
		keyElMap: map[interface{}]*list.Element{},
	}
}
