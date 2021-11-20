package tree

import (
	"bytes"
	"encoding/json"
	//"log"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/pyroscope-io/pyroscope/pkg/structs/merge"
)

type jsonableSlice []byte

type treeNode struct {
	Name          jsonableSlice `json:"name,string"`
	Total         uint64        `json:"total"`
	Self          uint64        `json:"self"`
	ChildrenNodes []*treeNode   `json:"children"`
}

func (a jsonableSlice) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(a))
}

func (n *treeNode) clone(m, d uint64) *treeNode {
	newNode := &treeNode{
		Name:  n.Name,
		Total: n.Total * m / d,
		Self:  n.Self * m / d,
	}
	newNode.ChildrenNodes = make([]*treeNode, len(n.ChildrenNodes))
	for i, cn := range n.ChildrenNodes {
		newNode.ChildrenNodes[i] = cn.clone(m, d)
	}
	return newNode
}

func newNode(label []byte) *treeNode {
	return &treeNode{
		Name:          label,
		ChildrenNodes: []*treeNode{},
	}
}

var (
	placeholderTreeNode = &treeNode{}
)

const semicolon = byte(';')

type Tree struct {
	sync.RWMutex
	root *treeNode
}

func New() *Tree {
	return &Tree{
		root: newNode([]byte{}),
	}
}

func (t *Tree) Merge(srcTrieI merge.Merger) {
	srcTrie := srcTrieI.(*Tree)

	srcNodes := make([]*treeNode, 0, 128)
	srcNodes = append(srcNodes, srcTrie.root)

	dstNodes := make([]*treeNode, 0, 128)
	dstNodes = append(dstNodes, t.root)

	for len(srcNodes) > 0 {
		st := srcNodes[0]
		srcNodes = srcNodes[1:]

		dt := dstNodes[0]
		dstNodes = dstNodes[1:]

		dt.Self += st.Self
		dt.Total += st.Total

		for _, srcChildNode := range st.ChildrenNodes {
			dstChildNode := dt.insert(srcChildNode.Name)
			srcNodes = prependTreeNode(srcNodes, srcChildNode)
			dstNodes = prependTreeNode(dstNodes, dstChildNode)
		}
	}
}

func prependTreeNode(s []*treeNode, x *treeNode) []*treeNode {
	s = append(s, nil)
	copy(s[1:], s)
	s[0] = x
	return s
}

func prependBytes(s [][]byte, x []byte) [][]byte {
	s = append(s, nil)
	copy(s[1:], s)
	s[0] = x
	return s
}

func prependInt(s []int, x int) []int {
	s = append(s, 0)
	copy(s[1:], s)
	s[0] = x
	return s
}

func (t *Tree) String() string {
	t.RLock()
	defer t.RUnlock()

	res := ""
	t.Iterate(func(k []byte, v uint64) {
		if v > 0 {
			res += fmt.Sprintf("%q %d\n", k[2:], v)
		}
	})

	return res
}

func (n *treeNode) insert(targetLabel []byte) *treeNode {
	i := sort.Search(len(n.ChildrenNodes), func(i int) bool {
		return bytes.Compare(n.ChildrenNodes[i].Name, targetLabel) >= 0
	})

	searchFailed := i == len(n.ChildrenNodes)

	if searchFailed || !bytes.Equal(n.ChildrenNodes[i].Name, targetLabel) {

		// Inserts new node at such an index as to maintain ordering
		// (ordering is required by sort.Search).

		l := make([]byte, len(targetLabel))
		copy(l, targetLabel)
		child := newNode(l)
		n.ChildrenNodes = append(n.ChildrenNodes, child)
		copy(n.ChildrenNodes[i+1:], n.ChildrenNodes[i:])
		n.ChildrenNodes[i] = child
	}
	return n.ChildrenNodes[i]
}

func (t *Tree) InsertInt(key []byte, value int) { t.Insert(key, uint64(value)) }

func (t *Tree) Insert(key []byte, value uint64) {
	// TODO: can optimize this, split is not necessary?
	labels := bytes.Split(key, []byte(";"))
	node := t.root
	for _, l := range labels {
		buf := make([]byte, len(l))
		copy(buf, l)
		l = buf

		n := node.insert(l)
		node.Total += value
		node = n
	}
	node.Self += value
	node.Total += value
}

func (t *Tree) Iterate(cb func(key []byte, val uint64)) {
	nodes := []*treeNode{t.root}
	prefixes := make([][]byte, 1)
	prefixes[0] = make([]byte, 0)
	for len(nodes) > 0 { // bfs
		node := nodes[0]
		nodes = nodes[1:]

		prefix := prefixes[0]
		prefixes = prefixes[1:]

		label := append(prefix, semicolon) // byte(';'),
		l := node.Name
		label = append(label, l...) // byte(';'),

		cb(label, node.Self)

		nodes = append(node.ChildrenNodes, nodes...)
		for i := 0; i < len(node.ChildrenNodes); i++ {
			prefixes = prependBytes(prefixes, label)
		}
	}
}

func copiedByteSlice(oldSlice []byte) []byte {
	newSlice := make([]byte, len(oldSlice))
	copy(newSlice, oldSlice)
	return newSlice
}

func keyWithPrefix(node *treeNode, prefix []byte) []byte {
	key := append(prefix, semicolon)
	key = append(key, node.Name...)
	// TODO why is this slice copy necessary?
	// when omitted, some node names become corrupted
	return copiedByteSlice(key)
}

func keysWithPrefix(nodes []*treeNode, prefix []byte) [][]byte {
	keys := [][]byte{}
	for _, node := range nodes {
		key := keyWithPrefix(node, prefix)
		keys = append(keys, key)
	}
	return keys
}

func firstPrefix() []byte {
	return make([]byte, 0)
}

// TODO look for ways to deduplicate logic in these Iterate methods
func (t *Tree) IterateWithChildKeys(cb func(key []byte, val uint64, childKeys [][]byte)) {
	nodes := []*treeNode{t.root}
	prefixes := make([][]byte, 1)
	prefixes[0] = firstPrefix()
	for len(nodes) > 0 { // bfs
		node := nodes[0]
		nodes = nodes[1:]

		prefix := prefixes[0]
		prefixes = prefixes[1:]

		key := keyWithPrefix(node, prefix)

		children := node.ChildrenNodes

		prefixForChildren := key
		childKeys := keysWithPrefix(children, prefixForChildren)

		cb(key, node.Self, childKeys)

		nodes = append(children, nodes...)

		for i := 0; i < len(children); i++ {
			prefixes = prependBytes(prefixes, prefixForChildren)
		}
	}
}

func (t *Tree) iterateWithTotal(cb func(total uint64) bool) {
	nodes := []*treeNode{t.root}
	i := 0
	for len(nodes) > 0 {
		node := nodes[0]
		nodes = nodes[1:]
		i++
		if cb(node.Total) {
			nodes = append(node.ChildrenNodes, nodes...)
		}
	}
}

func (t *Tree) Samples() uint64 {
	return t.root.Total
}

func (t *Tree) RootKey() []byte {
	return keyWithPrefix(t.root, firstPrefix())
}

func (t *Tree) Clone(r *big.Rat) *Tree {
	t.RLock()
	defer t.RUnlock()

	m := uint64(r.Num().Int64())
	d := uint64(r.Denom().Int64())
	newTrie := &Tree{
		root: t.root.clone(m, d),
	}

	return newTrie
}

func (t *Tree) MarshalJSON() ([]byte, error) {
	t.RLock()
	defer t.RUnlock()
	return json.Marshal(t.root)
}
