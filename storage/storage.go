package storage

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

type Storage[T any] interface {
	Store(key string, value T) error
	Retrieve(key string) (T, bool)
	Erase(key string) error
	Import() error
	Export() error
	GetKeys() []string
}

type storage[T any] struct {
	mutex    sync.RWMutex
	data     map[string]T
	filename string
}

func NewStorage[T any](filename string) Storage[T] {
	s := storage[T]{
		data:     make(map[string]T),
		filename: filename,
	}
	if err := s.Import(); err != nil {
		log.Panicf("error while restoring %s; %s", filename, err.Error())
	}
	return &s
}

func (s *storage[T]) Store(key string, value T) error {
	s.mutex.Lock()
	s.data[key] = value
	s.mutex.Unlock()

	return s.Export()
}

func (s *storage[T]) Erase(key string) error {
	s.mutex.Lock()
	delete(s.data, key)
	s.mutex.Unlock()
	return s.Export()
}

func (s *storage[T]) Retrieve(key string) (T, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	val, hasValue := s.data[key]
	return val, hasValue
}

func (s *storage[T]) Import() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	blob, _ := os.ReadFile(s.filename)
	if blob == nil {
		return nil
	}

	return json.Unmarshal(blob, &s.data)
}

func (s *storage[T]) Export() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	blob, err := json.Marshal(s.data)
	if err != nil {
		log.Printf("Warning: error while saving %s; %s\n", s.filename, err.Error())
	}

	return os.WriteFile(s.filename, blob, 0777)
}

func (s *storage[T]) GetKeys() []string {
	keys := make([]string, 0, len(s.data))
	for key, _ := range s.data {
		keys = append(keys, key)
	}
	return keys
}
