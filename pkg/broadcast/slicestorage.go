package broadcast

type (
	// slice storage - storage for slice of active objects
	sliceStorage[T any] struct {
		slice []chan T
	}
)

func newSliceStorage[T any]() *sliceStorage[T] {
	return new(sliceStorage[T])
}

func (s *sliceStorage[T]) copySliceTo(newStorage *sliceStorage[T]) {
	// In Go struct is basically a struct with uintptr as its last element. append() just copies this uintptr if cap
	// is unchanged. This behavior might be tricky so make sure slices won't share the same uintptr.
	if cap(newStorage.slice) < len(s.slice) { // new slice won't fit old one's element, realloc and copy
		newStorage.slice = append(make([]chan T, 0, len(s.slice)), s.slice...)
	} else { // new slice fits, just copying
		newStorage.slice = newStorage.slice[:copy(newStorage.slice[:cap(newStorage.slice)], s.slice)]
	}
}

func (s *sliceStorage[T]) appendValue(v chan T) (appended bool) {
	var valFound bool

	for _, sliceV := range s.slice {
		if v == sliceV {
			valFound = true

			break
		}
	}

	if valFound {
		return false
	}

	s.slice = append(s.slice, v)

	return true
}

func (s *sliceStorage[T]) removeValue(v chan T) (removed bool) {
	var (
		valueFound bool
		valueIndex int
	)

	for i, sliceV := range s.slice {
		if v == sliceV {
			valueFound = true
			valueIndex = i

			break
		}
	}

	if !valueFound {
		return false
	}

	lastIndex := len(s.slice)
	lastIndex--
	s.slice[valueIndex] = s.slice[lastIndex]
	s.slice = s.slice[:lastIndex]

	return true
}
