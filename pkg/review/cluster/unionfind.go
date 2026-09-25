package cluster

// UnionFind is a path-compressed, union-by-rank structure over string keys.
type UnionFind struct {
	parent map[string]string
	rank   map[string]int
	order  []string
}

// NewUnionFind initialises one component per item, preserving item order for Components.
func NewUnionFind(items []string) *UnionFind {
	uf := &UnionFind{
		parent: make(map[string]string, len(items)),
		rank:   make(map[string]int, len(items)),
		order:  append([]string(nil), items...),
	}
	for _, item := range items {
		uf.parent[item] = item
	}
	return uf
}

// Find returns the root representative with path compression.
func (uf *UnionFind) Find(item string) string {
	if uf.parent[item] != item {
		uf.parent[item] = uf.Find(uf.parent[item])
	}
	return uf.parent[item]
}

// Union merges the components containing a and b.
func (uf *UnionFind) Union(a, b string) {
	rootA, rootB := uf.Find(a), uf.Find(b)
	if rootA == rootB {
		return
	}
	if uf.rank[rootA] < uf.rank[rootB] {
		rootA, rootB = rootB, rootA
	}
	uf.parent[rootB] = rootA
	if uf.rank[rootA] == uf.rank[rootB] {
		uf.rank[rootA]++
	}
}

// Components returns all connected components in insertion order.
func (uf *UnionFind) Components() [][]string {
	groups := make(map[string][]string)
	for _, item := range uf.order {
		root := uf.Find(item)
		groups[root] = append(groups[root], item)
	}
	result := make([][]string, 0, len(groups))
	for _, item := range uf.order {
		root := uf.Find(item)
		if len(groups[root]) == 0 {
			continue
		}
		result = append(result, groups[root])
		delete(groups, root)
	}
	return result
}
