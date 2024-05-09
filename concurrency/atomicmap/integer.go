/*
Copyright 2024 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package atomicmap

import (
	"sync"

	"golang.org/x/exp/constraints"
)

type Integer[K comparable, T constraints.Integer] interface {
	Get(key K) (*IntegerValue[T], bool)
	GetOrCreate(key K, createT T) *IntegerValue[T]
	Delete(key K)
	ForEach(fn func(key K, value *IntegerValue[T]))
}

func NewInteger[K comparable, T constraints.Integer]() Integer[K, T] {
	return &integer[K, T]{
		items: make(map[K]*IntegerValue[T]),
	}
}

func NewIntegerStringInt64() Integer[int64, int64] {
	return NewInteger[int64, int64]()
}

func NewIntegerStringInt32() Integer[int64, int32] {
	return NewInteger[int64, int32]()
}

func NewIntegerStringUint64() Integer[int64, uint64] {
	return NewInteger[int64, uint64]()
}

func NewIntegerStringUint32() Integer[int64, uint32] {
	return NewInteger[int64, uint32]()
}

type IntegerValue[T constraints.Integer] struct {
	lock  sync.RWMutex
	value T
}

func (i *IntegerValue[T]) Load() T {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.value
}

func (i *IntegerValue[T]) Store(v T) {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.value = v
}

func (i *IntegerValue[T]) Add(v T) T {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.value += v
	return i.value
}

type integer[K comparable, T constraints.Integer] struct {
	lock  sync.RWMutex
	items map[K]*IntegerValue[T]
}

func (i *integer[K, T]) Get(key K) (*IntegerValue[T], bool) {
	i.lock.RLock()
	defer i.lock.RUnlock()

	item, ok := i.items[key]
	if !ok {
		return nil, false
	}
	return item, true
}

func (i *integer[K, T]) GetOrCreate(key K, createT T) *IntegerValue[T] {
	i.lock.RLock()
	item, ok := i.items[key]
	i.lock.RUnlock()
	if !ok {
		i.lock.Lock()
		// Double-check the key exists to avoid race condition
		item, ok = i.items[key]
		if !ok {
			item = &IntegerValue[T]{value: createT}
			i.items[key] = item
		}
		i.lock.Unlock()
	}
	return item
}

func (i *integer[K, T]) Delete(key K) {
	i.lock.Lock()
	delete(i.items, key)
	i.lock.Unlock()
}

func (i *integer[K, T]) ForEach(fn func(key K, value *IntegerValue[T])) {
	i.lock.RLock()
	defer i.lock.RUnlock()
	for k, v := range i.items {
		fn(k, v)
	}
}
