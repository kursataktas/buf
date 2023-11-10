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
