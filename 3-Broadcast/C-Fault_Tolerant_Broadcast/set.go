package main

type void struct{}

type Set[T comparable] struct {
	mp map[T]void
}

func MakeSet[T comparable]() Set[T] {
	return Set[T]{
		mp: make(map[T]void),
	}
}

func (s *Set[T]) Exists(key T) bool {
	_, ok := s.mp[key]
	return ok
}

func (s *Set[T]) Add(key T) {
	s.mp[key] = void{}
}

func (s *Set[T]) Delete(key T) {
	delete(s.mp, key)
}

func (s *Set[T]) GetKeys() []T {
	arr := make([]T, len(s.mp))
	i := 0
	for k := range s.mp {
		arr[i] = k
		i++
	}
	return arr
}
