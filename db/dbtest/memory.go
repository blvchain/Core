// Package dbtest provides an in-memory implementation of db.Collection.
//
// It supports the small subset of the MongoDB query language that the node
// actually uses ($and, $or, dotted field paths, comparison operators, sort,
// skip and limit), which is enough to exercise the storage layer and every
// package built on top of it without a running mongod.
package dbtest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Memory is a goroutine-safe, in-memory db.Collection.
type Memory struct {
	mu   sync.Mutex
	docs []bson.M
}

// New returns an empty collection.
func New() *Memory { return &Memory{} }

// Len returns the number of stored documents.
func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.docs)
}

// Reset removes every document.
func (m *Memory) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs = nil
}

// Seed stores documents directly, bypassing InsertOne semantics. Useful for
// arranging test fixtures.
func (m *Memory) Seed(docs ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, doc := range docs {
		normalized, err := normalize(doc)
		if err != nil {
			panic(fmt.Sprintf("dbtest: cannot seed document: %v", err))
		}
		m.docs = append(m.docs, normalized)
	}
}

func normalize(doc interface{}) (bson.M, error) {
	raw, err := bson.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var out bson.M
	if err := bson.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (m *Memory) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	normalized, err := normalize(document)
	if err != nil {
		return nil, err
	}

	if id, ok := normalized["_id"]; ok {
		for _, existing := range m.docs {
			if reflect.DeepEqual(existing["_id"], id) {
				return nil, errors.New("dbtest: duplicate key error: _id already exists")
			}
		}
	}

	m.docs = append(m.docs, normalized)
	return &mongo.InsertOneResult{InsertedID: normalized["_id"]}, nil
}

func (m *Memory) InsertMany(ctx context.Context, documents []interface{}, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ordered := true
	for _, opt := range opts {
		if opt != nil && opt.Ordered != nil {
			ordered = *opt.Ordered
		}
	}

	ids := make([]interface{}, 0, len(documents))
	var firstErr error

	for _, doc := range documents {
		normalized, err := normalize(doc)
		if err != nil {
			if ordered {
				return nil, err
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		id, hasID := normalized["_id"]
		if hasID {
			duplicate := false
			for _, existing := range m.docs {
				if reflect.DeepEqual(existing["_id"], id) {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
		}

		m.docs = append(m.docs, normalized)
		if hasID {
			ids = append(ids, id)
		}
	}

	return &mongo.InsertManyResult{InsertedIDs: ids}, firstErr
}

func (m *Memory) FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) *mongo.SingleResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, doc := range m.docs {
		if matchFilter(doc, filter) {
			return mongo.NewSingleResultFromDocument(doc, nil, nil)
		}
	}
	// NewSingleResultFromDocument treats a nil document as ErrNilDocument, so
	// pass an empty document together with ErrNoDocuments to surface the error
	// the real driver would return.
	return mongo.NewSingleResultFromDocument(bson.M{}, mongo.ErrNoDocuments, nil)
}

func (m *Memory) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	matched := make([]bson.M, 0, len(m.docs))
	for _, doc := range m.docs {
		if matchFilter(doc, filter) {
			matched = append(matched, doc)
		}
	}

	var findOpts *options.FindOptions
	if len(opts) > 0 && opts[0] != nil {
		findOpts = opts[0]
	}
	sortDocuments(matched, findOpts)

	skip, limit := int64(0), int64(0)
	if findOpts != nil {
		if findOpts.Skip != nil {
			skip = *findOpts.Skip
		}
		if findOpts.Limit != nil {
			limit = *findOpts.Limit
		}
	}
	if skip > 0 {
		if skip >= int64(len(matched)) {
			matched = matched[:0]
		} else {
			matched = matched[skip:]
		}
	}
	if limit > 0 && int64(len(matched)) > limit {
		matched = matched[:limit]
	}

	docs := make([]interface{}, len(matched))
	for i, doc := range matched {
		docs[i] = doc
	}

	return mongo.NewCursorFromDocuments(docs, nil, nil)
}

// ---------------------------------------------------------------------------
// Query matching
// ---------------------------------------------------------------------------

func matchFilter(doc bson.M, filter interface{}) bool {
	switch f := filter.(type) {
	case nil:
		return true
	case bson.M:
		return matchDocument(doc, f)
	case bson.D:
		return matchDocument(doc, toMap(f))
	default:
		// Fall back to a BSON round-trip so callers can pass typed structs.
		normalized, err := normalize(f)
		if err != nil {
			return false
		}
		return matchDocument(doc, normalized)
	}
}

func matchDocument(doc bson.M, filter bson.M) bool {
	for key, cond := range filter {
		switch key {
		case "$and":
			for _, sub := range toSlice(cond) {
				if !matchFilter(doc, sub) {
					return false
				}
			}
		case "$or":
			any := false
			for _, sub := range toSlice(cond) {
				if matchFilter(doc, sub) {
					any = true
					break
				}
			}
			if !any {
				return false
			}
		case "$nor":
			for _, sub := range toSlice(cond) {
				if matchFilter(doc, sub) {
					return false
				}
			}
		case "$not":
			if matchFilter(doc, cond) {
				return false
			}
		default:
			value, exists := lookupPath(doc, key)
			if !matchCondition(value, exists, cond) {
				return false
			}
		}
	}
	return true
}

func matchCondition(value interface{}, exists bool, cond interface{}) bool {
	if m, ok := cond.(bson.M); ok {
		for op, operand := range m {
			if !matchOperator(value, exists, op, operand) {
				return false
			}
		}
		return true
	}
	if d, ok := cond.(bson.D); ok {
		for _, item := range d {
			if !matchOperator(value, exists, item.Key, item.Value) {
				return false
			}
		}
		return true
	}
	return exists && equalValues(value, cond)
}

func matchOperator(value interface{}, exists bool, op string, operand interface{}) bool {
	switch op {
	case "$eq":
		return exists && equalValues(value, operand)
	case "$ne":
		return !exists || !equalValues(value, operand)
	case "$gt":
		return exists && compareValues(value, operand) > 0
	case "$gte":
		return exists && compareValues(value, operand) >= 0
	case "$lt":
		return exists && compareValues(value, operand) < 0
	case "$lte":
		return exists && compareValues(value, operand) <= 0
	case "$in":
		for _, candidate := range toSlice(operand) {
			if exists && equalValues(value, candidate) {
				return true
			}
		}
		return false
	case "$nin":
		for _, candidate := range toSlice(operand) {
			if exists && equalValues(value, candidate) {
				return false
			}
		}
		return true
	case "$exists":
		want, ok := operand.(bool)
		if !ok {
			return false
		}
		return exists == want
	default:
		return false
	}
}

func lookupPath(doc bson.M, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var current interface{} = doc
	for _, part := range parts {
		m, ok := current.(bson.M)
		if !ok {
			if d, isD := current.(bson.D); isD {
				m = toMap(d)
				ok = true
			}
		}
		if !ok {
			return nil, false
		}
		value, exists := m[part]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

func equalValues(a, b interface{}) bool {
	if af, ok := toFloat(a); ok {
		if bf, ok := toFloat(b); ok {
			return af == bf
		}
	}
	return reflect.DeepEqual(a, b)
}

func compareValues(a, b interface{}) int {
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// toSlice normalises the operand of $and/$or/$in/$nin. It uses reflection so
// that typed slices ([]bson.M, []string, bson.A, ...) are all handled.
func toSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return []interface{}{v}
	}

	out := make([]interface{}, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out[i] = rv.Index(i).Interface()
	}
	return out
}

func toMap(d bson.D) bson.M {
	m := make(bson.M, len(d))
	for _, item := range d {
		m[item.Key] = item.Value
	}
	return m
}

// ---------------------------------------------------------------------------
// Sorting
// ---------------------------------------------------------------------------

func sortDocuments(docs []bson.M, opts *options.FindOptions) {
	if opts == nil || opts.Sort == nil {
		return
	}

	var keys []string
	var directions []int

	switch s := opts.Sort.(type) {
	case bson.M:
		// Map iteration order is random, so walk the keys in a stable order.
		for _, k := range sortedKeys(s) {
			keys = append(keys, k)
			directions = append(directions, directionOf(s[k]))
		}
	case bson.D:
		for _, item := range s {
			keys = append(keys, item.Key)
			directions = append(directions, directionOf(item.Value))
		}
	default:
		return
	}

	if len(keys) == 0 {
		return
	}

	sort.SliceStable(docs, func(i, j int) bool {
		for idx, key := range keys {
			left, _ := lookupPath(docs[i], key)
			right, _ := lookupPath(docs[j], key)
			cmp := compareValues(left, right)
			if cmp == 0 {
				continue
			}
			if directions[idx] < 0 {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
}

func directionOf(v interface{}) int {
	if n, ok := toFloat(v); ok && n < 0 {
		return -1
	}
	return 1
}

func sortedKeys(m bson.M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
