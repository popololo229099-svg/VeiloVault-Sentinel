package tree

import (
	"fmt"
	"sync"
)

type BSTNode struct {
	Key   int
	Value interface{}
	Left  *BSTNode
	Right *BSTNode
}

type BST struct {
	Root *BSTNode
	mu   sync.RWMutex
}

func NewBST() *BST {
	return &BST{}
}

func (t *BST) Insert(key int, value interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Root = t.insert(t.Root, key, value)
}

func (t *BST) insert(node *BSTNode, key int, value interface{}) *BSTNode {
	if node == nil {
		return &BSTNode{Key: key, Value: value}
	}
	if key < node.Key {
		node.Left = t.insert(node.Left, key, value)
	} else if key > node.Key {
		node.Right = t.insert(node.Right, key, value)
	} else {
		node.Value = value
	}
	return node
}

func (t *BST) Search(key int) (interface{}, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	node := t.search(t.Root, key)
	if node == nil {
		return nil, false
	}
	return node.Value, true
}

func (t *BST) search(node *BSTNode, key int) *BSTNode {
	if node == nil || node.Key == key {
		return node
	}
	if key < node.Key {
		return t.search(node.Left, key)
	}
	return t.search(node.Right, key)
}

func (t *BST) Delete(key int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Root = t.delete(t.Root, key)
}

func (t *BST) delete(node *BSTNode, key int) *BSTNode {
	if node == nil {
		return nil
	}
	if key < node.Key {
		node.Left = t.delete(node.Left, key)
	} else if key > node.Key {
		node.Right = t.delete(node.Right, key)
	} else {
		if node.Left == nil {
			return node.Right
		}
		if node.Right == nil {
			return node.Left
		}
		minNode := t.findMin(node.Right)
		node.Key = minNode.Key
		node.Value = minNode.Value
		node.Right = t.delete(node.Right, minNode.Key)
	}
	return node
}

func (t *BST) findMin(node *BSTNode) *BSTNode {
	for node.Left != nil {
		node = node.Left
	}
	return node
}

func (t *BST) InOrder() []int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]int, 0)
	t.inOrder(t.Root, &result)
	return result
}

func (t *BST) inOrder(node *BSTNode, result *[]int) {
	if node == nil {
		return
	}
	t.inOrder(node.Left, result)
	*result = append(*result, node.Key)
	t.inOrder(node.Right, result)
}

func (t *BST) Min() (int, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.Root == nil {
		return 0, false
	}
	return t.findMin(t.Root).Key, true
}

func (t *BST) Max() (int, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.Root == nil {
		return 0, false
	}
	node := t.Root
	for node.Right != nil {
		node = node.Right
	}
	return node.Key, true
}

func (t *BST) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.count(t.Root)
}

func (t *BST) count(node *BSTNode) int {
	if node == nil {
		return 0
	}
	return 1 + t.count(node.Left) + t.count(node.Right)
}

func (t *BST) Height() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.height(t.Root)
}

func (t *BST) height(node *BSTNode) int {
	if node == nil {
		return 0
	}
	leftH := t.height(node.Left)
	rightH := t.height(node.Right)
	if leftH > rightH {
		return leftH + 1
	}
	return rightH + 1
}

type RBNode struct {
	Key    int
	Value  interface{}
	Color  bool
	Left   *RBNode
	Right  *RBNode
	Parent *RBNode
}

type RedBlackTree struct {
	nil    *RBNode
	Root   *RBNode
	mu     sync.RWMutex
}

func NewRedBlackTree() *RedBlackTree {
	nilNode := &RBNode{Color: false}
	return &RedBlackTree{
		nil:  nilNode,
		Root: nilNode,
	}
}

func (t *RedBlackTree) Insert(key int, value interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	node := &RBNode{
		Key:   key,
		Value: value,
		Color: true,
		Left:  t.nil,
		Right: t.nil,
	}

	y := t.nil
	x := t.Root

	for x != t.nil {
		y = x
		if node.Key < x.Key {
			x = x.Left
		} else {
			x = x.Right
		}
	}

	node.Parent = y
	if y == t.nil {
		t.Root = node
	} else if node.Key < y.Key {
		y.Left = node
	} else {
		y.Right = node
	}

	if node.Parent == t.nil {
		node.Color = false
		return
	}

	if node.Parent.Parent == t.nil {
		return
	}

	t.fixInsert(node)
}

func (t *RedBlackTree) fixInsert(node *RBNode) {
	for node.Parent != nil && node.Parent.Color {
		if node.Parent == node.Parent.Parent.Right {
			uncle := node.Parent.Parent.Left
			if uncle != nil && uncle.Color {
				uncle.Color = false
				node.Parent.Color = false
				node.Parent.Parent.Color = true
				node = node.Parent.Parent
			} else {
				if node == node.Parent.Left {
					node = node.Parent
					t.rightRotate(node)
				}
				node.Parent.Color = false
				node.Parent.Parent.Color = true
				t.leftRotate(node.Parent.Parent)
			}
		} else {
			uncle := node.Parent.Parent.Right
			if uncle != nil && uncle.Color {
				uncle.Color = false
				node.Parent.Color = false
				node.Parent.Parent.Color = true
				node = node.Parent.Parent
			} else {
				if node == node.Parent.Right {
					node = node.Parent
					t.leftRotate(node)
				}
				node.Parent.Color = false
				node.Parent.Parent.Color = true
				t.rightRotate(node.Parent.Parent)
			}
		}
		if node == t.Root {
			break
		}
	}
	t.Root.Color = false
}

func (t *RedBlackTree) leftRotate(x *RBNode) {
	y := x.Right
	x.Right = y.Left
	if y.Left != t.nil {
		y.Left.Parent = x
	}
	y.Parent = x.Parent
	if x.Parent == nil {
		t.Root = y
	} else if x == x.Parent.Left {
		x.Parent.Left = y
	} else {
		x.Parent.Right = y
	}
	y.Left = x
	x.Parent = y
}

func (t *RedBlackTree) rightRotate(x *RBNode) {
	y := x.Left
	x.Left = y.Right
	if y.Right != t.nil {
		y.Right.Parent = x
	}
	y.Parent = x.Parent
	if x.Parent == nil {
		t.Root = y
	} else if x == x.Parent.Right {
		x.Parent.Right = y
	} else {
		x.Parent.Left = y
	}
	y.Right = x
	x.Parent = y
}

func (t *RedBlackTree) Search(key int) (interface{}, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.Root
	for node != t.nil {
		if key == node.Key {
			return node.Value, true
		} else if key < node.Key {
			node = node.Left
		} else {
			node = node.Right
		}
	}
	return nil, false
}

type AVLNode struct {
	Key    int
	Value  interface{}
	Height int
	Left   *AVLNode
	Right  *AVLNode
}

type AVLTree struct {
	Root *AVLNode
	mu   sync.RWMutex
}

func NewAVLTree() *AVLTree {
	return &AVLTree{}
}

func (t *AVLTree) Insert(key int, value interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Root = t.insert(t.Root, key, value)
}

func (t *AVLTree) insert(node *AVLNode, key int, value interface{}) *AVLNode {
	if node == nil {
		return &AVLNode{Key: key, Value: value, Height: 1}
	}
	if key < node.Key {
		node.Left = t.insert(node.Left, key, value)
	} else if key > node.Key {
		node.Right = t.insert(node.Right, key, value)
	} else {
		node.Value = value
		return node
	}

	node.Height = 1 + t.max(t.getHeight(node.Left), t.getHeight(node.Right))

	balance := t.getBalance(node)

	if balance > 1 && key < node.Left.Key {
		return t.rightRotate(node)
	}
	if balance < -1 && key > node.Right.Key {
		return t.leftRotate(node)
	}
	if balance > 1 && key > node.Left.Key {
		node.Left = t.leftRotate(node.Left)
		return t.rightRotate(node)
	}
	if balance < -1 && key < node.Right.Key {
		node.Right = t.rightRotate(node.Right)
		return t.leftRotate(node)
	}

	return node
}

func (t *AVLTree) leftRotate(x *AVLNode) *AVLNode {
	y := x.Right
	x.Right = y.Left
	y.Left = x
	x.Height = 1 + t.max(t.getHeight(x.Left), t.getHeight(x.Right))
	y.Height = 1 + t.max(t.getHeight(y.Left), t.getHeight(y.Right))
	return y
}

func (t *AVLTree) rightRotate(y *AVLNode) *AVLNode {
	x := y.Left
	y.Left = x.Right
	x.Right = y
	y.Height = 1 + t.max(t.getHeight(y.Left), t.getHeight(y.Right))
	x.Height = 1 + t.max(t.getHeight(x.Left), t.getHeight(x.Right))
	return x
}

func (t *AVLTree) getHeight(node *AVLNode) int {
	if node == nil {
		return 0
	}
	return node.Height
}

func (t *AVLTree) getBalance(node *AVLNode) int {
	if node == nil {
		return 0
	}
	return t.getHeight(node.Left) - t.getHeight(node.Right)
}

func (t *AVLTree) max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (t *AVLTree) Search(key int) (interface{}, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.Root
	for node != nil {
		if key == node.Key {
			return node.Value, true
		} else if key < node.Key {
			node = node.Left
		} else {
			node = node.Right
		}
	}
	return nil, false
}

type TrieNode struct {
	Children map[rune]*TrieNode
	IsEnd    bool
	Value    interface{}
}

type Trie struct {
	Root *TrieNode
	size int
	mu   sync.RWMutex
}

func NewTrie() *Trie {
	return &Trie{
		Root: &TrieNode{Children: make(map[rune]*TrieNode)},
	}
}

func (t *Trie) Insert(word string, value interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	node := t.Root
	for _, ch := range word {
		if _, exists := node.Children[ch]; !exists {
			node.Children[ch] = &TrieNode{Children: make(map[rune]*TrieNode)}
		}
		node = node.Children[ch]
	}
	if !node.IsEnd {
		t.size++
	}
	node.IsEnd = true
	node.Value = value
}

func (t *Trie) Search(word string) (interface{}, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.Root
	for _, ch := range word {
		child, exists := node.Children[ch]
		if !exists {
			return nil, false
		}
		node = child
	}
	if node.IsEnd {
		return node.Value, true
	}
	return nil, false
}

func (t *Trie) StartsWith(prefix string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.Root
	for _, ch := range prefix {
		child, exists := node.Children[ch]
		if !exists {
			return false
		}
		node = child
	}
	return true
}

func (t *Trie) Delete(word string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.delete(t.Root, word, 0)
}

func (t *Trie) delete(node *TrieNode, word string, depth int) bool {
	if node == nil {
		return false
	}

	if depth == len(word) {
		if !node.IsEnd {
			return false
		}
		node.IsEnd = false
		t.size--
		return len(node.Children) == 0
	}

	ch := rune(word[depth])
	child := node.Children[ch]
	shouldDelete := t.delete(child, word, depth+1)

	if shouldDelete {
		delete(node.Children, ch)
		return !node.IsEnd && len(node.Children) == 0
	}

	return false
}

func (t *Trie) Words() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]string, 0)
	t.collect(t.Root, "", &result)
	return result
}

func (t *Trie) collect(node *TrieNode, prefix string, result *[]string) {
	if node.IsEnd {
		*result = append(*result, prefix)
	}
	for ch, child := range node.Children {
		t.collect(child, prefix+string(ch), result)
	}
}

func (t *Trie) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.size
}

type IntervalTree struct {
	nodes []*IntervalNode
	mu    sync.RWMutex
}

type Interval struct {
	Start int
	End   int
}

type IntervalNode struct {
	Interval
	Value    interface{}
	Max      int
	Left     *IntervalNode
	Right    *IntervalNode
}

func NewIntervalTree() *IntervalTree {
	return &IntervalTree{
		nodes: make([]*IntervalNode, 0),
	}
}

func (it *IntervalTree) Insert(start, end int, value interface{}) {
	it.mu.Lock()
	defer it.mu.Unlock()
	it.nodes = append(it.nodes, &IntervalNode{
		Interval: Interval{Start: start, End: end},
		Value:    value,
		Max:      end,
	})
}

func (it *IntervalTree) Search(start, end int) []interface{} {
	it.mu.RLock()
	defer it.mu.RUnlock()

	result := make([]interface{}, 0)
	for _, node := range it.nodes {
		if node.Start <= end && node.End >= start {
			result = append(result, node.Value)
		}
	}
	return result
}

func (it *IntervalTree) Count() int {
	it.mu.RLock()
	defer it.mu.RUnlock()
	return len(it.nodes)
}

type KDNode struct {
	Points [2]float64
	Value  interface{}
	Left   *KDNode
	Right  *KDNode
	Axis   int
}

type KDTree struct {
	Root *KDNode
	mu   sync.RWMutex
}

func NewKDTree() *KDTree {
	return &KDTree{}
}

func (kt *KDTree) Insert(x, y float64, value interface{}) {
	kt.mu.Lock()
	defer kt.mu.Unlock()
	kt.Root = kt.insert(kt.Root, [2]float64{x, y}, value, 0)
}

func (kt *KDTree) insert(node *KDNode, point [2]float64, value interface{}, depth int) *KDNode {
	if node == nil {
		return &KDNode{Points: point, Value: value, Axis: depth % 2}
	}

	axis := depth % 2
	if point[axis] < node.Points[axis] {
		node.Left = kt.insert(node.Left, point, value, depth+1)
	} else {
		node.Right = kt.insert(node.Right, point, value, depth+1)
	}
	return node
}

func (kt *KDTree) Search(x, y float64) (interface{}, bool) {
	kt.mu.RLock()
	defer kt.mu.RUnlock()
	return kt.search(kt.Root, [2]float64{x, y})
}

func (kt *KDTree) search(node *KDNode, point [2]float64) (interface{}, bool) {
	if node == nil {
		return nil, false
	}
	if node.Points[0] == point[0] && node.Points[1] == point[1] {
		return node.Value, true
	}

	axis := node.Axis
	if point[axis] < node.Points[axis] {
		return kt.search(node.Left, point)
	}
	return kt.search(node.Right, point)
}

func (kt *KDTree) Nearest(x, y float64) (interface{}, float64, bool) {
	kt.mu.RLock()
	defer kt.mu.RUnlock()

	if kt.Root == nil {
		return nil, 0, false
	}

	best := kt.Root
	bestDist := kt.distance(kt.Root.Points, [2]float64{x, y})
	kt.nearest(kt.Root, [2]float64{x, y}, &best, &bestDist)

	return best.Value, bestDist, true
}

func (kt *KDTree) nearest(node *KDNode, point [2]float64, best **KDNode, bestDist *float64) {
	if node == nil {
		return
	}

	dist := kt.distance(node.Points, point)
	if dist < *bestDist {
		*best = node
		*bestDist = dist
	}

	axis := node.Axis
	var first, second *KDNode
	if point[axis] < node.Points[axis] {
		first, second = node.Left, node.Right
	} else {
		first, second = node.Right, node.Left
	}

	kt.nearest(first, point, best, bestDist)

	diff := point[axis] - node.Points[axis]
	if diff*diff < *bestDist {
		kt.nearest(second, point, best, bestDist)
	}
}

func (kt *KDTree) distance(a, b [2]float64) float64 {
	dx := a[0] - b[0]
	dy := a[1] - b[1]
	return dx*dx + dy*dy
}

type BTree struct {
	order int
	root  *BTreeNode
	mu    sync.RWMutex
}

type BTreeNode struct {
	keys     []int
	values   []interface{}
	children []*BTreeNode
	leaf     bool
}

func NewBTree(order int) *BTree {
	if order < 3 {
		order = 3
	}
	return &BTree{order: order}
}

func (bt *BTree) Insert(key int, value interface{}) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if bt.root == nil {
		bt.root = &BTreeNode{
			keys:     []int{key},
			values:   []interface{}{value},
			children: make([]*BTreeNode, 0),
			leaf:     true,
		}
		return
	}

	if len(bt.root.keys) == bt.order-1 {
		newRoot := &BTreeNode{
			children: []*BTreeNode{bt.root},
			leaf:     false,
		}
		bt.splitChild(newRoot, 0)
		bt.root = newRoot
	}
	bt.insertNonFull(bt.root, key, value)
}

func (bt *BTree) splitChild(parent *BTreeNode, i int) {
	order := bt.order
	child := parent.children[i]
	mid := order / 2

	newNode := &BTreeNode{
		keys:     child.keys[mid+1:],
		values:   child.values[mid+1:],
		children: make([]*BTreeNode, 0),
		leaf:     child.leaf,
	}

	child.keys = child.keys[:mid]
	child.values = child.values[:mid]

	if !child.leaf {
		newNode.children = child.children[mid+1:]
		child.children = child.children[:mid+1]
	}

	parent.keys = append(parent.keys[:i], append([]int{child.keys[mid]}, parent.keys[i:]...)...)
	parent.values = append(parent.values[:i], append([]interface{}{child.values[mid]}, parent.values[i:]...)...)
	parent.children = append(parent.children[:i+1], append([]*BTreeNode{newNode}, parent.children[i+1:]...)...)
}

func (bt *BTree) insertNonFull(node *BTreeNode, key int, value interface{}) {
	i := len(node.keys) - 1

	if node.leaf {
		node.keys = append(node.keys, 0)
		node.values = append(node.values, nil)
		for i >= 0 && key < node.keys[i] {
			node.keys[i+1] = node.keys[i]
			node.values[i+1] = node.values[i]
			i--
		}
		node.keys[i+1] = key
		node.values[i+1] = value
	} else {
		for i >= 0 && key < node.keys[i] {
			i--
		}
		i++
		if len(node.children[i].keys) == bt.order-1 {
			bt.splitChild(node, i)
			if key > node.keys[i] {
				i++
			}
		}
		bt.insertNonFull(node.children[i], key, value)
	}
}

func (bt *BTree) Search(key int) (interface{}, bool) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	if bt.root == nil {
		return nil, false
	}
	return bt.search(bt.root, key)
}

func (bt *BTree) search(node *BTreeNode, key int) (interface{}, bool) {
	i := 0
	for i < len(node.keys) && key > node.keys[i] {
		i++
	}
	if i < len(node.keys) && key == node.keys[i] {
		return node.values[i], true
	}
	if node.leaf {
		return nil, false
	}
	return bt.search(node.children[i], key)
}

func (bt *BTree) Count() int {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.count(bt.root)
}

func (bt *BTree) count(node *BTreeNode) int {
	if node == nil {
		return 0
	}
	n := len(node.keys)
	for _, child := range node.children {
		n += bt.count(child)
	}
	return n
}

type SuffixNode struct {
	Children map[rune]*SuffixNode
	Indices  []int
}

type SuffixTree struct {
	Root *SuffixNode
	text string
	mu   sync.RWMutex
}

func NewSuffixTree(text string) *SuffixTree {
	st := &SuffixTree{
		Root: &SuffixNode{Children: make(map[rune]*SuffixNode)},
		text: text,
	}
	for i := 0; i < len(text); i++ {
		st.insertSuffix(text[i:], i)
	}
	return st
}

func (st *SuffixTree) insertSuffix(suffix string, index int) {
	node := st.Root
	for _, ch := range suffix {
		if _, exists := node.Children[ch]; !exists {
			node.Children[ch] = &SuffixNode{Children: make(map[rune]*SuffixNode)}
		}
		node = node.Children[ch]
	}
	node.Indices = append(node.Indices, index)
}

func (st *SuffixTree) Search(pattern string) []int {
	st.mu.RLock()
	defer st.mu.RUnlock()

	node := st.Root
	for _, ch := range pattern {
		child, exists := node.Children[ch]
		if !exists {
			return nil
		}
		node = child
	}
	return node.Indices
}

func (st *SuffixTree) Contains(pattern string) bool {
	return len(st.Search(pattern)) > 0
}

func PrintBST(root *BSTNode, level int) {
	if root == nil {
		return
	}
	PrintBST(root.Right, level+1)
	for i := 0; i < level; i++ {
		fmt.Print("  ")
	}
	fmt.Printf("%d\n", root.Key)
	PrintBST(root.Left, level+1)
}
