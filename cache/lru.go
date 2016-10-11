package cache

import (
	"fmt"
	"sync/atomic"

	"github.com/skipor/memcached/internal/tag"
)

const (
	inactive = 0
	active   = 1
)

// Pre and post conditions (Invariants) for pushBack and shrink methods:
// * lru owns nodes between fakeHead and fakeTail.
// * {fakeHead, all owned nodes, fakeTail} are correct doubly linked list.
// * all nodes owned by lru have field node.owner equal to &lru
// * lru.size equal sum of owned nodes size()
// * there are no recycled data in nodes.
type lru struct {
	size int64
	// callbacks called in shrink.
	// Callback should save lru invariants: attach to same owner, or disown node.
	callbacks

	// Fake nodes. Real nodes are between them.
	// nil <- fakeHead <-> node_0 <-> ... <-> node_(n-1) <-> fakeTail -> nil
	// Such structure prevent nil checks in code.

	// fakeHead is bottom of lru. fakeHead.next is most lately added item.
	fakeHead *node

	// fakeTail is top of lru. All new added before fakeTail.
	fakeTail *node
}

type callbacks struct {
	onExpire   func(*node)
	onActive   func(*node)
	onInactive func(*node)
}

func (l *lru) pushBack(n *node) {
	n.owner = l
	l.size += n.size()
	attachAsInactive(n)
}

// shrink detach nodes from head to tail, and call callback chosen on node state
// (expired, active, inactive). Nodes detached in shrink have invalid node.prev pointer.
// node.next is valid during callback call.
func (l *lru) shrink(toSize int64, now int64) {
	if toSize < 0 {
		panic(fmt.Sprintf("try shrink to negative size %v", toSize))
	}
	cur, next := l.head(), l.head().next
	for ; toSize < l.size; cur, next = next, next.next {
		l.assertNotTail(cur)
		if tag.Debug {
			cur.prev = nil
		}
		if cur.expired(now) {
			l.onExpire(cur)
			continue
		}
		if cur.isActive() {
			l.onActive(cur)
			continue
		}
		l.onInactive(cur)
	}
	link(l.fakeHead, cur)
}

func (l *lru) init() {
	l.fakeHead, l.fakeTail = &node{}, &node{}
	link(l.fakeHead, l.fakeTail)
}

func (l *lru) head() *node      { return l.fakeHead.next }
func (l *lru) tail() *node      { return l.fakeTail.prev }
func (l *lru) end(n *node) bool { return n == l.fakeTail }

type node struct {
	Item
	// active can have concurrent and atomic access with read lock acquired,
	// or exclusive access with write lock acquired.
	active int32
	owner  *lru
	prev   *node
	next   *node
}

func newNode(i Item) *node { return &node{Item: i} }

func (n *node) disown() {
	n.owner.size -= n.size()
	if tag.Debug {
		n.owner = nil
	}
}

func (n *node) detach() {
	link(n.prev, n.next)
	if tag.Debug {
		n.prev = nil
		n.next = nil
	}
}

// require read lock be acquired
func (n *node) setActive() { atomic.StoreInt32(&n.active, active) }

// require write lock be acquired
func (n *node) isActive() bool { return n.active == active }

// extraMemoryForItem is approximation how much memory needed to save empty item.
// Without such compensation it is possible to blow up cache with small values.
const extraSizePerNode = 256 // Item, recycle.Data, node, two hash table cells.

// MemSize return approximation how much memory needed to save empty item.
func (n *node) size() int64 {
	return int64(extraSizePerNode + len(n.Key) + n.Bytes)
}

func (l *lru) assertNotTail(n *node) {
	if n == l.fakeTail {
		panic("node pointer out of range")
	}
}

func link(a, b *node) { a.next, b.prev = b, a }

func attachAsInactive(n *node) {
	n.active = inactive
	link(n.owner.tail(), n)
	link(n, n.owner.fakeTail)
}

func moveTo(other *lru) func(*node) {
	return func(n *node) {
		n.disown()
		other.pushBack(n)
	}
}
