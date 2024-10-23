package binarytree_test

import (
	"testing"

	"github.com/I-Am-Dench/goverbuild/archive/internal/binarytree"
)

type Item struct {
	binarytree.Indices
}

func (item *Item) TreeIndices() *binarytree.Indices {
	return &item.Indices
}

func createItems(size int) []*Item {
	s := make([]*Item, size)
	for i := range s {
		s[i] = &Item{}
	}
	return s
}

func isExpected(expected []binarytree.Indices, actual []*Item) bool {
	if len(expected) != len(actual) {
		return false
	}

	for i, v := range expected {
		indices := actual[i].TreeIndices()
		if indices.LowerIndex != v.LowerIndex || indices.UpperIndex != v.UpperIndex {
			return false
		}
	}
	return true
}

func collectIndices(s []*Item) []binarytree.Indices {
	indices := []binarytree.Indices{}
	for _, v := range s {
		indices = append(indices, v.Indices)
	}
	return indices
}

func TestUpdateIndices(t *testing.T) {
	for n, expected := range map[int][]binarytree.Indices{
		1:  {{-1, -1}},
		2:  {{-1, -1}, {0, -1}},
		6:  {{-1, -1}, {0, 2}, {-1, -1}, {1, 5}, {-1, -1}, {4, -1}},
		12: {{-1, -1}, {0, 2}, {-1, -1}, {1, 5}, {-1, -1}, {4, -1}, {3, 9}, {-1, -1}, {7, -1}, {8, 11}, {-1, -1}, {10, -1}},
		20: {{-1, -1}, {0, -1}, {1, 4}, {-1, -1}, {3, -1}, {2, 8}, {-1, -1}, {6, -1}, {7, 9}, {-1, -1}, {5, 15}, {-1, -1}, {11, -1}, {12, 14}, {-1, -1}, {13, 18}, {-1, -1}, {16, -1}, {17, 19}, {-1, -1}},
	} {
		actual := createItems(n)
		binarytree.UpdateIndices(actual)

		if !isExpected(expected, actual) {
			t.Errorf("update indices:\nexpected = %v\nactual   = %v", expected, collectIndices(actual))
		}
	}
}
