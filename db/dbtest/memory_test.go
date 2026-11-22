package dbtest

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type doc struct {
	ID   string `bson:"_id"`
	Name string `bson:"name"`
	Role int64  `bson:"role"`
	Meta meta   `bson:"meta"`
}

type meta struct {
	TS int64 `bson:"ts"`
}

func seedDocs() *Memory {
	m := New()
	m.Seed(
		doc{ID: "a", Name: "alice", Role: 1, Meta: meta{TS: 100}},
		doc{ID: "b", Name: "bob", Role: 2, Meta: meta{TS: 200}},
		doc{ID: "c", Name: "carol", Role: 1, Meta: meta{TS: 300}},
	)
	return m
}

func findAll(t *testing.T, m *Memory, filter any, opts ...*options.FindOptions) []doc {
	t.Helper()

	cursor, err := m.Find(context.Background(), filter, opts...)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	defer cursor.Close(context.Background())

	var out []doc
	if err := cursor.All(context.Background(), &out); err != nil {
		t.Fatalf("cursor.All: %v", err)
	}
	return out
}

func TestMemoryFindEquality(t *testing.T) {
	m := seedDocs()

	got := findAll(t, m, bson.M{"name": "bob"})
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("equality filter returned %+v", got)
	}
}

func TestMemoryFindDottedPath(t *testing.T) {
	m := seedDocs()

	got := findAll(t, m, bson.M{"meta.ts": bson.M{"$gte": 200}})
	if len(got) != 2 {
		t.Fatalf("dotted path filter returned %d docs, want 2", len(got))
	}
}

func TestMemoryFindOperators(t *testing.T) {
	m := seedDocs()

	cases := []struct {
		name   string
		filter bson.M
		want   int
	}{
		{"$and", bson.M{"$and": []bson.M{{"role": 1}, {"meta.ts": bson.M{"$lt": 300}}}}, 1},
		{"$or", bson.M{"$or": []bson.M{{"name": "alice"}, {"name": "bob"}}}, 2},
		{"$ne", bson.M{"name": bson.M{"$ne": "alice"}}, 2},
		{"$gt", bson.M{"role": bson.M{"$gt": 1}}, 1},
		{"$gte", bson.M{"role": bson.M{"$gte": 1}}, 3},
		{"$lt", bson.M{"role": bson.M{"$lt": 2}}, 2},
		{"$lte", bson.M{"role": bson.M{"$lte": 2}}, 3},
		{"$in", bson.M{"name": bson.M{"$in": []string{"alice", "carol"}}}, 2},
		{"$nin", bson.M{"name": bson.M{"$nin": []string{"alice", "carol"}}}, 1},
		{"$exists true", bson.M{"name": bson.M{"$exists": true}}, 3},
		{"$exists false", bson.M{"missing": bson.M{"$exists": false}}, 3},
		{"missing field", bson.M{"missing": "x"}, 0},
		{"nil filter", nil, 3},
	}

	for _, tc := range cases {
		got := findAll(t, m, tc.filter)
		if len(got) != tc.want {
			t.Errorf("%s: got %d docs, want %d", tc.name, len(got), tc.want)
		}
	}
}

func TestMemorySort(t *testing.T) {
	m := seedDocs()

	desc := findAll(t, m, bson.M{}, options.Find().SetSort(bson.M{"meta.ts": -1}))
	if len(desc) != 3 || desc[0].ID != "c" || desc[2].ID != "a" {
		t.Fatalf("descending sort = %+v", desc)
	}

	asc := findAll(t, m, bson.M{}, options.Find().SetSort(bson.D{{Key: "meta.ts", Value: 1}}))
	if len(asc) != 3 || asc[0].ID != "a" || asc[2].ID != "c" {
		t.Fatalf("ascending sort = %+v", asc)
	}
}

func TestMemorySkipAndLimit(t *testing.T) {
	m := seedDocs()

	page := findAll(t, m, bson.M{}, options.Find().SetSort(bson.M{"meta.ts": 1}).SetSkip(1).SetLimit(1))
	if len(page) != 1 || page[0].ID != "b" {
		t.Fatalf("skip/limit = %+v", page)
	}

	beyond := findAll(t, m, bson.M{}, options.Find().SetSkip(10))
	if len(beyond) != 0 {
		t.Fatalf("skip beyond the end returned %d docs", len(beyond))
	}
}

func TestMemoryFindOne(t *testing.T) {
	m := seedDocs()

	var found doc
	if err := m.FindOne(context.Background(), bson.M{"name": "carol"}).Decode(&found); err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if found.ID != "c" {
		t.Fatalf("FindOne returned %+v", found)
	}

	if err := m.FindOne(context.Background(), bson.M{"name": "nobody"}).Decode(&found); err != mongo.ErrNoDocuments {
		t.Fatalf("missing document error = %v, want mongo.ErrNoDocuments", err)
	}
}

func TestMemoryInsertOneDuplicate(t *testing.T) {
	m := New()

	if _, err := m.InsertOne(context.Background(), doc{ID: "a"}); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := m.InsertOne(context.Background(), doc{ID: "a"}); err == nil {
		t.Fatal("duplicate _id was accepted")
	}
	if m.Len() != 1 {
		t.Fatalf("Len = %d, want 1", m.Len())
	}
}

func TestMemoryInsertManyUnordered(t *testing.T) {
	m := New()

	_, err := m.InsertMany(
		context.Background(),
		[]interface{}{doc{ID: "a"}, doc{ID: "b"}, doc{ID: "a"}},
		options.InsertMany().SetOrdered(false),
	)
	if err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
	if m.Len() != 2 {
		t.Fatalf("Len = %d, want 2", m.Len())
	}
}

func TestMemoryReset(t *testing.T) {
	m := seedDocs()
	if m.Len() != 3 {
		t.Fatalf("Len = %d, want 3", m.Len())
	}

	m.Reset()
	if m.Len() != 0 {
		t.Fatalf("Len after Reset = %d, want 0", m.Len())
	}
	if got := findAll(t, m, bson.M{}); len(got) != 0 {
		t.Fatalf("Reset left %d docs", len(got))
	}
}

func TestMemoryConcurrentAccess(t *testing.T) {
	m := New()

	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				id := string(rune('a' + n))
				if _, err := m.InsertOne(context.Background(), doc{ID: id}); err != nil {
					continue
				}
				cursor, err := m.Find(context.Background(), bson.M{})
				if err != nil {
					continue
				}
				_ = cursor.Close(context.Background())
			}
		}(i)
	}
	for i := 0; i < 8; i++ {
		<-done
	}

	if m.Len() != 8 {
		t.Fatalf("Len = %d, want 8", m.Len())
	}
}
