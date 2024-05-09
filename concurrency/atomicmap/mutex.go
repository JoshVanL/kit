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
)

type Mutex[T comparable] interface {
	Lock(key T)
	Unlock(key T)
	RLock(key T)
	RUnlock(key T)
	Delete(key T)
}

func NewMutex[T comparable]() Mutex[T] {
	return &mutex[T]{
		items: make(map[T]*sync.RWMutex),
	}
}

func NewMutexString() Mutex[string] {
	return NewMutex[string]()
}

func NewMutexUint32() Mutex[uint32] {
	return NewMutex[uint32]()
}

type mutex[T comparable] struct {
	lock  sync.Mutex
	items map[T]*sync.RWMutex
}

func (m *mutex[T]) Lock(key T) {
	m.lock.Lock()
	mutex, ok := m.items[key]
	if !ok {
		mutex, ok = m.items[key]
		if !ok {
			mutex = &sync.RWMutex{}
			m.items[key] = mutex
		}
	}
	m.lock.Unlock()
	mutex.Lock()
}

func (m *mutex[T]) Unlock(key T) {
	m.lock.Lock()
	mutex, ok := m.items[key]
	m.lock.Unlock()
	if ok {
		mutex.Unlock()
	}
}

func (m *mutex[T]) RLock(key T) {
	m.lock.Lock()
	mutex, ok := m.items[key]
	if !ok {
		mutex, ok = m.items[key]
		if !ok {
			mutex = &sync.RWMutex{}
			m.items[key] = mutex
		}
	}
	m.lock.Unlock()
	mutex.Lock()
}

func (m *mutex[T]) RUnlock(key T) {
	m.lock.Lock()
	mutex, ok := m.items[key]
	m.lock.Unlock()
	if ok {
		mutex.Unlock()
	}
}

func (m *mutex[T]) Delete(key T) {
	m.lock.Lock()
	delete(m.items, key)
	m.lock.Unlock()
}
