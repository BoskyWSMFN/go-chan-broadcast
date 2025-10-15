package broadcast

type (
	// sliceStorage provides thread-safe storage for active channels using copy-on-write
	sliceStorage[T any] struct {
		slice []chan T // TODO linked list?
	}
)

// newSliceStorage creates a new slice storage instance
func newSliceStorage[T any]() *sliceStorage[T] {
	return new(sliceStorage[T])
}

// copySliceTo copies the slice contents to a new storage instance
func (s *sliceStorage[T]) copySliceTo(newStorage *sliceStorage[T]) {
	// Ensure slices do not share the same backing array.
	// If capacity of destination is insufficient, reallocate.
	if cap(newStorage.slice) < len(s.slice) {
		newStorage.slice = append(make([]chan T, 0, len(s.slice)), s.slice...)
	} else {
		newStorage.slice = newStorage.slice[:copy(newStorage.slice[:cap(newStorage.slice)], s.slice)]
	}
}

// appendValue adds a channel to the slice if not already present
func (s *sliceStorage[T]) appendValue(v chan T) (appended bool) {
	for _, sliceV := range s.slice {
		if v == sliceV {
			return false
		}
	}

	s.slice = append(s.slice, v)

	return true
}

// removeValue removes a channel from the slice if present
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
