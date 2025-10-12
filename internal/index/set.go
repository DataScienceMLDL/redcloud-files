package index

import "github.com/sekai02/redcloud-files/internal/ids"

type FileSet map[ids.FileID]struct{}

func NewFileSet() FileSet {
	return make(FileSet)
}

func (s FileSet) Add(fid ids.FileID) {
	s[fid] = struct{}{}
}

func (s FileSet) Remove(fid ids.FileID) {
	delete(s, fid)
}

func (s FileSet) Contains(fid ids.FileID) bool {
	_, exists := s[fid]
	return exists
}

func (s FileSet) Size() int {
	return len(s)
}

func (s FileSet) ToSlice() []ids.FileID {
	result := make([]ids.FileID, 0, len(s))
	for fid := range s {
		result = append(result, fid)
	}
	return result
}

func Intersect(sets []FileSet) FileSet {
	if len(sets) == 0 {
		return NewFileSet()
	}

	smallest := 0
	for i := 1; i < len(sets); i++ {
		if sets[i].Size() < sets[smallest].Size() {
			smallest = i
		}
	}

	result := NewFileSet()
	for fid := range sets[smallest] {
		inAll := true
		for i, s := range sets {
			if i == smallest {
				continue
			}
			if !s.Contains(fid) {
				inAll = false
				break
			}
		}
		if inAll {
			result.Add(fid)
		}
	}

	return result
}

func Union(sets []FileSet) FileSet {
	result := NewFileSet()
	for _, s := range sets {
		for fid := range s {
			result.Add(fid)
		}
	}
	return result
}
