package search

import (
	"sync"
)

type BM25Searcher struct {
	k1      float64
	b       float64
	docLens []int
	avgDl   float64
	nDocs   int
	docs    []string
	terms   [][]string
	mu      sync.RWMutex
}

func NewBM25Searcher(k1, b float64) *BM25Searcher {
	if k1 <= 0 {
		k1 = 1.5
	}
	if b <= 0 || b > 1 {
		b = 0.75
	}
	return &BM25Searcher{k1: k1, b: b}
}

func (bs *BM25Searcher) Index(docs []string) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	bs.docs = docs
	bs.nDocs = len(docs)
	bs.docLens = make([]int, bs.nDocs)
	bs.terms = make([][]string, bs.nDocs)
	totalLen := 0

	for i, doc := range docs {
		bs.terms[i] = tokenize(doc)
		bs.docLens[i] = len(bs.terms[i])
		totalLen += bs.docLens[i]
	}

	if bs.nDocs > 0 {
		bs.avgDl = float64(totalLen) / float64(bs.nDocs)
	}
}

func (bs *BM25Searcher) Search(query string, topK int) []SearchResult {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	queryTerms := tokenize(query)
	idf := bs.computeIDF(queryTerms)

	scores := make([]float64, bs.nDocs)
	for i := 0; i < bs.nDocs; i++ {
		scores[i] = bs.scoreDocument(i, queryTerms, idf)
	}

	results := make([]SearchResult, 0, topK)
	for i, score := range scores {
		if score > 0 {
			results = append(results, SearchResult{
				Index: i,
				Score: score,
				Text:  bs.docs[i],
			})
		}
	}

	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Score > results[j-1].Score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results
}

func (bs *BM25Searcher) computeIDF(queryTerms []string) map[string]float64 {
	idf := make(map[string]float64)
	seen := make(map[string]bool)

	for _, term := range queryTerms {
		if seen[term] {
			continue
		}
		seen[term] = true
		df := 0
		for i := 0; i < bs.nDocs; i++ {
			for _, t := range bs.terms[i] {
				if t == term {
					df++
					break
				}
			}
		}
		if df > 0 {
			idf[term] = (float64(bs.nDocs-df)+0.5)/(float64(df)+0.5) + 1.0
		}
	}
	return idf
}

func (bs *BM25Searcher) scoreDocument(docIdx int, queryTerms []string, idf map[string]float64) float64 {
	score := 0.0
	dl := float64(bs.docLens[docIdx])

	termFreqs := make(map[string]int)
	for _, t := range bs.terms[docIdx] {
		termFreqs[t]++
	}

	for _, term := range queryTerms {
		tf := float64(termFreqs[term])
		idfVal, ok := idf[term]
		if !ok {
			continue
		}
		denom := tf + bs.k1*(1-bs.b+bs.b*dl/bs.avgDl)
		score += idfVal * (tf * (bs.k1 + 1)) / denom
	}

	return score
}

type SearchResult struct {
	Index int
	Score float64
	Text  string
}

func tokenize(s string) []string {
	result := make([]string, 0)
	current := make([]byte, 0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == ',' || c == '.' || c == ';' || c == ':' || c == '!' || c == '?' {
			if len(current) > 0 {
				result = append(result, string(current))
				current = current[:0]
			}
		} else if c >= 'A' && c <= 'Z' {
			current = append(current, c+32)
		} else {
			current = append(current, c)
		}
	}
	if len(current) > 0 {
		result = append(result, string(current))
	}
	return result
}

type TrieSearcher struct {
	root *trieNode
	mu   sync.RWMutex
}

type trieNode struct {
	children map[byte]*trieNode
	isEnd    bool
	data     interface{}
}

func NewTrieSearcher() *TrieSearcher {
	return &TrieSearcher{
		root: &trieNode{children: make(map[byte]*trieNode)},
	}
}

func (ts *TrieSearcher) Insert(key string, data interface{}) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	node := ts.root
	for i := 0; i < len(key); i++ {
		c := key[i]
		if _, ok := node.children[c]; !ok {
			node.children[c] = &trieNode{children: make(map[byte]*trieNode)}
		}
		node = node.children[c]
	}
	node.isEnd = true
	node.data = data
}

func (ts *TrieSearcher) Search(prefix string) []interface{} {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	node := ts.root
	for i := 0; i < len(prefix); i++ {
		c := prefix[i]
		if _, ok := node.children[c]; !ok {
			return nil
		}
		node = node.children[c]
	}

	results := make([]interface{}, 0)
	ts.collect(node, prefix, &results)
	return results
}

func (ts *TrieSearcher) collect(node *trieNode, prefix string, results *[]interface{}) {
	if node.isEnd {
		*results = append(*results, node.data)
	}
	for c, child := range node.children {
		ts.collect(child, prefix+string(c), results)
	}
}

func (ts *TrieSearcher) StartsWith(prefix string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	node := ts.root
	for i := 0; i < len(prefix); i++ {
		c := prefix[i]
		if _, ok := node.children[c]; !ok {
			return false
		}
		node = node.children[c]
	}
	return true
}

func (ts *TrieSearcher) Delete(key string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.delete(ts.root, key, 0)
}

func (ts *TrieSearcher) delete(node *trieNode, key string, depth int) bool {
	if node == nil {
		return false
	}

	if depth == len(key) {
		if !node.isEnd {
			return false
		}
		node.isEnd = false
		return len(node.children) == 0
	}

	c := key[depth]
	child := node.children[c]
	deleted := ts.delete(child, key, depth+1)

	if deleted {
		delete(node.children, c)
		return !node.isEnd && len(node.children) == 0
	}

	return false
}

type IndexSearcher struct {
	index map[string][]int
	docs  []string
	mu    sync.RWMutex
}

func NewIndexSearcher() *IndexSearcher {
	return &IndexSearcher{
		index: make(map[string][]int),
	}
}

func (is *IndexSearcher) AddDocument(doc string) int {
	is.mu.Lock()
	defer is.mu.Unlock()

	idx := len(is.docs)
	is.docs = append(is.docs, doc)

	terms := tokenize(doc)
	seen := make(map[string]bool)
	for _, term := range terms {
		if !seen[term] {
			is.index[term] = append(is.index[term], idx)
			seen[term] = true
		}
	}

	return idx
}

func (is *IndexSearcher) Search(query string) []string {
	is.mu.RLock()
	defer is.mu.RUnlock()

	queryTerms := tokenize(query)
	resultSet := make(map[int]bool)

	for _, term := range queryTerms {
		if docIDs, ok := is.index[term]; ok {
			for _, id := range docIDs {
				resultSet[id] = true
			}
		}
	}

	results := make([]string, 0, len(resultSet))
	for id := range resultSet {
		if id < len(is.docs) {
			results = append(results, is.docs[id])
		}
	}
	return results
}

func (is *IndexSearcher) SearchAnd(query string) []string {
	is.mu.RLock()
	defer is.mu.RUnlock()

	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil
	}

	var resultSets []map[int]bool
	for _, term := range queryTerms {
		set := make(map[int]bool)
		if docIDs, ok := is.index[term]; ok {
			for _, id := range docIDs {
				set[id] = true
			}
		}
		resultSets = append(resultSets, set)
	}

	intersection := resultSets[0]
	for i := 1; i < len(resultSets); i++ {
		newIntersection := make(map[int]bool)
		for id := range intersection {
			if resultSets[i][id] {
				newIntersection[id] = true
			}
		}
		intersection = newIntersection
	}

	results := make([]string, 0, len(intersection))
	for id := range intersection {
		if id < len(is.docs) {
			results = append(results, is.docs[id])
		}
	}
	return results
}

type KNNSearcher struct {
	data     [][]float64
	labels   []string
	distance func(a, b []float64) float64
	mu       sync.RWMutex
}

func NewKNNSearcher(distance func(a, b []float64) float64) *KNNSearcher {
	if distance == nil {
		distance = euclideanDistance
	}
	return &KNNSearcher{distance: distance}
}

func (ks *KNNSearcher) AddPoint(point []float64, label string) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.data = append(ks.data, point)
	ks.labels = append(ks.labels, label)
}

func (ks *KNNSearcher) Search(query []float64, k int) []KNNResult {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if k <= 0 {
		k = 5
	}

	distances := make([]KNNResult, len(ks.data))
	for i, point := range ks.data {
		distances[i] = KNNResult{
			Index:    i,
			Label:    ks.labels[i],
			Distance: ks.distance(query, point),
		}
	}

	for i := 1; i < len(distances); i++ {
		for j := i; j > 0 && distances[j].Distance < distances[j-1].Distance; j-- {
			distances[j], distances[j-1] = distances[j-1], distances[j]
		}
	}

	if k > len(distances) {
		k = len(distances)
	}
	return distances[:k]
}

type KNNResult struct {
	Index    int
	Label    string
	Distance float64
}

func euclideanDistance(a, b []float64) float64 {
	sum := 0.0
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return sum
}
