package binarytree

type Indices struct {
	LowerIndex int32
	UpperIndex int32
}

type TreeNode interface {
	TreeIndices() *Indices
}

func updateNodeIndices[S ~[]E, E TreeNode](nodes S, bottom int) int32 {
	if len(nodes) == 0 {
		return -1
	}
	center := int32(len(nodes) / 2)

	indices := nodes[center].TreeIndices()
	indices.LowerIndex = updateNodeIndices(nodes[:center], bottom)
	indices.UpperIndex = updateNodeIndices(nodes[center+1:], int(center)+1+bottom)

	return center + int32(bottom)
}

func UpdateIndices[S ~[]E, E TreeNode](nodes S) {
	updateNodeIndices(nodes, 0)
}
