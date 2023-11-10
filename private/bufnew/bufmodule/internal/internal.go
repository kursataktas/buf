// Copyright 2020-2023 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import "sync"

// OnceValues3 returns a function that invokes f only once and returns the values
// returned by f. The returned function may be called concurrently.
//
// If f panics, the returned function will panic with the same value on every call.
//
// This is copied from sync.OnceValues and extended to for three values.
func OnceValues3[T1, T2, T3 any](f func() (T1, T2, T3)) func() (T1, T2, T3) {
	var (
		once  sync.Once
		valid bool
		p     any
		r1    T1
		r2    T2
		r3    T3
	)
	g := func() {
		defer func() {
			p = recover()
			if !valid {
				panic(p)
			}
		}()
		r1, r2, r3 = f()
		valid = true
	}
	return func() (T1, T2, T3) {
		once.Do(g)
		if !valid {
			panic(p)
		}
		return r1, r2, r3
	}
}

// FilterSlice filters the slice to only the values where f returns true.
func FilterSlice[T any](s []T, f func(T) bool) []T {
	sf := make([]T, 0, len(s))
	for _, e := range s {
		if f(e) {
			sf = append(sf, e)
		}
	}
	return sf
}

// FilterSliceError filters the slice to only the values where f returns true.
//
// Returns error the first time f returns error.
func FilterSliceError[T any](s []T, f func(T) (bool, error)) ([]T, error) {
	sf := make([]T, 0, len(s))
	for _, e := range s {
		ok, err := f(e)
		if err != nil {
			return nil, err
		}
		if ok {
			sf = append(sf, e)
		}
	}
	return sf, nil
}

// MapSlice maps the slice.
func MapSlice[T1, T2 any](s []T1, f func(T1) T2) []T2 {
	sm := make([]T2, len(s))
	for i, e := range s {
		sm[i] = f(e)
	}
	return sm
}

// MapSliceError maps the slice.
//
// Returns error the first time f returns error.
func MapSliceError[T1, T2 any](s []T1, f func(T1) (T2, error)) ([]T2, error) {
	sm := make([]T2, len(s))
	for i, e := range s {
		em, err := f(e)
		if err != nil {
			return nil, err
		}
		sm[i] = em
	}
	return sm, nil
}

// GetOrAddToCacheDoubleLock does a double-lock around the cache to get the value for the key,
// or adds it after calling getFunc.
func GetOrAddToCacheDoubleLock[T any](
	lock *sync.RWMutex,
	cache map[string]*Tuple[T, error],
	key string,
	getFunc func() (T, error),
) (T, error) {
	lock.RLock()
	tuple, ok := cache[key]
	lock.RUnlock()
	if ok {
		return tuple.V1, tuple.V2
	}
	lock.Lock()
	value, err := GetOrAddToCache(cache, key, getFunc)
	lock.Unlock()
	return value, err
}

// GetOrAddToCache gets the value from the cache for the key, or adds it after calling getFunc.
func GetOrAddToCache[T any](
	cache map[string]*Tuple[T, error],
	key string,
	get func() (T, error),
) (T, error) {
	tuple, ok := cache[key]
	if ok {
		return tuple.V1, tuple.V2
	}
	value, err := get()
	cache[key] = newTuple(value, err)
	return value, err
}

// Tuple is a tuple.
type Tuple[T1, T2 any] struct {
	V1 T1
	V2 T2
}

func newTuple[T1, T2 any](v1 T1, v2 T2) *Tuple[T1, T2] {
	return &Tuple[T1, T2]{
		V1: v1,
		V2: v2,
	}
}
