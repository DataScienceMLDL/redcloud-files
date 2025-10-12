package tagtree

import (
	"sort"
	"sync"

	"github.com/sekai02/redcloud-files/internal/ids"
	"github.com/sekai02/redcloud-files/internal/sys"
)

type entry struct {
	key string
	tid ids.TagID
}

type node struct {
	isLeaf   bool
	entries  []entry
	children []*node
	next     *node
}

type Tree struct {
	mu    sync.RWMutex
	root  *node
	order int
}

func NewTree() *Tree {
	return &Tree{
		root: &node{
			isLeaf:   true,
			entries:  []entry{},
			children: nil,
			next:     nil,
		},
		order: sys.BPTreeOrder,
	}
}

func (t *Tree) Lookup(name string) (ids.TagID, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	leaf := t.findLeaf(name)
	for _, e := range leaf.entries {
		if e.key == name {
			return e.tid, true
		}
	}
	return 0, false
}

func (t *Tree) Insert(name string, tid ids.TagID) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.root.entries) == 0 {
		t.root.entries = append(t.root.entries, entry{key: name, tid: tid})
		return
	}

	leaf := t.findLeaf(name)

	for i, e := range leaf.entries {
		if e.key == name {
			leaf.entries[i].tid = tid
			return
		}
	}

	leaf.entries = append(leaf.entries, entry{key: name, tid: tid})
	sort.Slice(leaf.entries, func(i, j int) bool {
		return leaf.entries[i].key < leaf.entries[j].key
	})

	if len(leaf.entries) >= t.order {
		t.splitLeaf(leaf)
	}
}

func (t *Tree) Delete(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	leaf := t.findLeaf(name)
	for i, e := range leaf.entries {
		if e.key == name {
			leaf.entries = append(leaf.entries[:i], leaf.entries[i+1:]...)
			return
		}
	}
}

func (t *Tree) findLeaf(key string) *node {
	n := t.root
	for !n.isLeaf {
		i := 0
		for i < len(n.entries) && key >= n.entries[i].key {
			i++
		}
		n = n.children[i]
	}
	return n
}

func (t *Tree) splitLeaf(leaf *node) {
	mid := len(leaf.entries) / 2
	newLeaf := &node{
		isLeaf:  true,
		entries: append([]entry{}, leaf.entries[mid:]...),
		next:    leaf.next,
	}
	leaf.entries = leaf.entries[:mid]
	leaf.next = newLeaf

	promotedKey := newLeaf.entries[0].key

	if leaf == t.root {
		newRoot := &node{
			isLeaf:   false,
			entries:  []entry{{key: promotedKey, tid: 0}},
			children: []*node{leaf, newLeaf},
		}
		t.root = newRoot
	} else {
		t.insertIntoParent(leaf, promotedKey, newLeaf)
	}
}

func (t *Tree) insertIntoParent(left *node, key string, right *node) {
	parent := t.findParent(t.root, left)
	if parent == nil {
		return
	}

	i := 0
	for i < len(parent.entries) && key >= parent.entries[i].key {
		i++
	}

	parent.entries = append(parent.entries[:i], append([]entry{{key: key, tid: 0}}, parent.entries[i:]...)...)
	parent.children = append(parent.children[:i+1], append([]*node{right}, parent.children[i+1:]...)...)

	if len(parent.entries) >= t.order {
		t.splitInternal(parent)
	}
}

func (t *Tree) splitInternal(n *node) {
	mid := len(n.entries) / 2
	promotedKey := n.entries[mid].key

	newNode := &node{
		isLeaf:   false,
		entries:  append([]entry{}, n.entries[mid+1:]...),
		children: append([]*node{}, n.children[mid+1:]...),
	}
	n.entries = n.entries[:mid]
	n.children = n.children[:mid+1]

	if n == t.root {
		newRoot := &node{
			isLeaf:   false,
			entries:  []entry{{key: promotedKey, tid: 0}},
			children: []*node{n, newNode},
		}
		t.root = newRoot
	} else {
		t.insertIntoParent(n, promotedKey, newNode)
	}
}

func (t *Tree) findParent(current, target *node) *node {
	if current.isLeaf || current == target {
		return nil
	}

	for _, child := range current.children {
		if child == target {
			return current
		}
	}

	for _, child := range current.children {
		if !child.isLeaf {
			parent := t.findParent(child, target)
			if parent != nil {
				return parent
			}
		}
	}

	return nil
}

type TagTreeSnapshot struct {
	Entries map[string]ids.TagID
}

func (t *Tree) Snapshot() TagTreeSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entries := make(map[string]ids.TagID)
	t.collectEntries(t.root, entries)

	return TagTreeSnapshot{
		Entries: entries,
	}
}

func (t *Tree) collectEntries(n *node, entries map[string]ids.TagID) {
	if n == nil {
		return
	}

	if n.isLeaf {
		for _, e := range n.entries {
			entries[e.key] = e.tid
		}
	} else {
		for _, child := range n.children {
			t.collectEntries(child, entries)
		}
	}
}

func (t *Tree) Restore(snap TagTreeSnapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.root = &node{
		isLeaf:   true,
		entries:  []entry{},
		children: nil,
		next:     nil,
	}

	for key, tid := range snap.Entries {
		t.insertWithoutLock(key, tid)
	}
}

func (t *Tree) insertWithoutLock(name string, tid ids.TagID) {
	if len(t.root.entries) == 0 {
		t.root.entries = append(t.root.entries, entry{key: name, tid: tid})
		return
	}

	leaf := t.findLeaf(name)

	for i, e := range leaf.entries {
		if e.key == name {
			leaf.entries[i].tid = tid
			return
		}
	}

	leaf.entries = append(leaf.entries, entry{key: name, tid: tid})
	sort.Slice(leaf.entries, func(i, j int) bool {
		return leaf.entries[i].key < leaf.entries[j].key
	})

	if len(leaf.entries) >= t.order {
		t.splitLeaf(leaf)
	}
}
