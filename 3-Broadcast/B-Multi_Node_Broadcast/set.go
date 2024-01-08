package main

type void struct{}

type Set map[int]void

func MakeSet() Set {
	return make(map[int]void)
}

func Exists(s Set, key int) bool {
	_, ok := s[key]
	return ok
}

func Add(s Set, key int) {
	s[key] = void{}
}

func GetKeys(s Set) []int {
	arr := make([]int, len(s))
	i := 0
	for k := range s {
		arr[i] = k
		i++
	}
	return arr
}
