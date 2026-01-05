package monarch

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TestContextFunctions verifies the context injection and retrieval logic.
// This is a unit test that doesn't require a running MongoDB.
func TestContextFunctions(t *testing.T) {
	t.Run("getDB returns error when no db in context", func(t *testing.T) {
		ctx := context.Background()
		_, err := getDB(ctx)
		if !errors.Is(err, ErrNoDatabase) {
			t.Errorf("expected ErrNoDatabase, got %v", err)
		}
	})

	t.Run("getDB returns db when present", func(t *testing.T) {
		var db *mongo.Database = nil
		ctx := WithContext(context.Background(), db)

		got, err := getDB(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != db {
			t.Errorf("expected db %p, got %p", db, got)
		}
	})
}

// setupTestDB connects to MongoDB and returns a database instance for testing.
// It only skips the test if SKIP_INTEGRATION is set to any non-empty value.
// Returns the database and a cleanup function that should be deferred.
func setupTestDB(t *testing.T) (*mongo.Database, func()) {
	t.Helper()

	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("skipping integration tests (SKIP_INTEGRATION is set)")
	}

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("failed to create MongoDB client: %v", err)
	}

	// Ping to verify connectivity - use a temporary context just for the ping
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		client.Disconnect(context.Background())
		t.Fatalf("failed to connect to MongoDB: %v", err)
	}

	db := client.Database("monarch_test_db")

	cleanup := func() {
		ctx := context.Background()
		db.Drop(ctx)
		client.Disconnect(ctx)
	}

	return db, cleanup
}

// cleanupCollection removes all documents from a collection.
// Use this at the beginning of each test to ensure a clean state.
func cleanupCollection[T any](t *testing.T, ctx context.Context, c Collection[T]) {
	t.Helper()
	c.DeleteMany(ctx, bson.M{})
}

// User is the test model used across integration tests.
type User struct {
	ID   string `bson:"_id,omitempty"`
	Name string `bson:"name"`
	Age  int    `bson:"age"`
}

// Users is the test collection.
var Users Collection[User] = "users"

func TestInsertOneAndFindOne(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	user := User{ID: "user1", Name: "Alice", Age: 30}
	res, err := Users.InsertOne(ctx, user)
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}
	id := res.InsertedID.(string)

	fetched, err := Users.FindOne(ctx, bson.M{"_id": id})
	if err != nil {
		t.Fatalf("FindOne failed: %v", err)
	}
	if fetched.Name != user.Name {
		t.Errorf("expected name %s, got %s", user.Name, fetched.Name)
	}
}

func TestInsertManyAndFind(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "user2", Name: "Bob", Age: 25},
		{ID: "user3", Name: "Charlie", Age: 35},
	}
	insertRes, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}
	if len(insertRes.InsertedIDs) != 2 {
		t.Errorf("InsertMany: expected 2 inserted IDs, got %d", len(insertRes.InsertedIDs))
	}

	results, err := Users.Find(ctx, bson.M{"age": bson.M{"$gte": 25}})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Find: expected 2 results, got %d", len(results))
	}
	names := []string{results[0].Name, results[1].Name}
	if !slices.Contains(names, "Bob") || !slices.Contains(names, "Charlie") {
		t.Errorf("Find: expected Bob and Charlie, got %v", names)
	}
}

func TestFindWithVariousFilters(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "t1", Name: "Alice", Age: 20},
		{ID: "t2", Name: "Bob", Age: 30},
		{ID: "t3", Name: "Charlie", Age: 40},
		{ID: "t4", Name: "David", Age: 50},
	}
	insertRes, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}
	if len(insertRes.InsertedIDs) != 4 {
		t.Errorf("InsertMany: expected 4 inserted IDs, got %d", len(insertRes.InsertedIDs))
	}

	tests := []struct {
		name          string
		filter        bson.M
		expectedCount int
		expectedNames []string
		checkAllNames bool
	}{
		{
			name:          "Find all",
			filter:        bson.M{},
			expectedCount: 4,
		},
		{
			name:          "Find age > 30",
			filter:        bson.M{"age": bson.M{"$gt": 30}},
			expectedCount: 2,
			expectedNames: []string{"Charlie", "David"},
			checkAllNames: true,
		},
		{
			name:          "Find by exact name",
			filter:        bson.M{"name": "Alice"},
			expectedCount: 1,
			expectedNames: []string{"Alice"},
			checkAllNames: true,
		},
		{
			name:          "Find with no matches",
			filter:        bson.M{"name": "Xavier"},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := Users.Find(ctx, tt.filter)
			if err != nil {
				t.Fatalf("Find failed: %v", err)
			}
			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}
			if tt.checkAllNames && len(results) > 0 {
				resultNames := make([]string, len(results))
				for i, r := range results {
					resultNames[i] = r.Name
				}
				for _, expectedName := range tt.expectedNames {
					if !slices.Contains(resultNames, expectedName) {
						t.Errorf("expected to find %s in results, got %v", expectedName, resultNames)
					}
				}
			}
		})
	}
}

func TestCountDocumentsWithMultipleFilters(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "c1", Name: "User1", Age: 20},
		{ID: "c2", Name: "User2", Age: 30},
		{ID: "c3", Name: "User2", Age: 40},
	}
	insertRes, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}
	if len(insertRes.InsertedIDs) != 3 {
		t.Errorf("InsertMany: expected 3 inserted IDs, got %d", len(insertRes.InsertedIDs))
	}

	tests := []struct {
		name          string
		filter        bson.M
		expectedCount int64
	}{
		{
			name:          "Count all documents",
			filter:        bson.M{},
			expectedCount: 3,
		},
		{
			name:          "Count by name User2",
			filter:        bson.M{"name": "User2"},
			expectedCount: 2,
		},
		{
			name:          "Count age > 25",
			filter:        bson.M{"age": bson.M{"$gt": 25}},
			expectedCount: 2,
		},
		{
			name:          "Count no matches",
			filter:        bson.M{"name": "Ghost"},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := Users.CountDocuments(ctx, tt.filter)
			if err != nil {
				t.Fatalf("CountDocuments failed: %v", err)
			}
			if count != tt.expectedCount {
				t.Errorf("expected %d, got %d", tt.expectedCount, count)
			}
		})
	}
}

func TestUpdateOneAndDeleteOne(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	user := User{ID: "ud1", Name: "Original", Age: 20}
	insertRes, err := Users.InsertOne(ctx, user)
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}
	if insertRes.InsertedID.(string) != "ud1" {
		t.Errorf("InsertOne: expected ID ud1, got %v", insertRes.InsertedID)
	}

	updateRes, err := Users.UpdateOne(ctx, bson.M{"_id": "ud1"}, bson.M{"$set": bson.M{"name": "Updated"}})
	if err != nil {
		t.Fatalf("UpdateOne failed: %v", err)
	}
	if updateRes.MatchedCount != 1 {
		t.Errorf("UpdateOne: expected 1 matched, got %d", updateRes.MatchedCount)
	}
	if updateRes.ModifiedCount != 1 {
		t.Errorf("UpdateOne: expected 1 modified, got %d", updateRes.ModifiedCount)
	}

	updated, err := Users.FindOne(ctx, bson.M{"_id": "ud1"})
	if err != nil {
		t.Fatalf("FindOne after update failed: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("FindOne after update: expected name Updated, got %s", updated.Name)
	}

	deleteRes, err := Users.DeleteOne(ctx, bson.M{"_id": "ud1"})
	if err != nil {
		t.Fatalf("DeleteOne failed: %v", err)
	}
	if deleteRes.DeletedCount != 1 {
		t.Errorf("DeleteOne: expected 1 deleted, got %d", deleteRes.DeletedCount)
	}

	_, err = Users.FindOne(ctx, bson.M{"_id": "ud1"})
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("FindOne after delete: expected ErrNoDocuments, got %v", err)
	}
}

func TestAggregateAsWithGrouping(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "agg1", Name: "Alice", Age: 20},
		{ID: "agg2", Name: "Bob", Age: 30},
		{ID: "agg3", Name: "Charlie", Age: 20},
	}
	insertRes, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}
	if len(insertRes.InsertedIDs) != 3 {
		t.Errorf("InsertMany: expected 3 inserted IDs, got %d", len(insertRes.InsertedIDs))
	}

	pipeline := bson.A{
		bson.M{"$group": bson.M{
			"_id":   "$age",
			"count": bson.M{"$sum": 1},
		}},
		bson.M{"$sort": bson.D{{Key: "_id", Value: 1}}},
	}

	type AgeCount struct {
		Age   int `bson:"_id"`
		Count int `bson:"count"`
	}

	results, err := AggregateAs[AgeCount](ctx, Users, pipeline)
	if err != nil {
		t.Fatalf("AggregateAs failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("AggregateAs: expected 2 results, got %d", len(results))
	}

	if results[0].Age != 20 || results[0].Count != 2 {
		t.Errorf("AggregateAs result[0]: expected age 20 count 2, got age %d count %d", results[0].Age, results[0].Count)
	}
	if results[1].Age != 30 || results[1].Count != 1 {
		t.Errorf("AggregateAs result[1]: expected age 30 count 1, got age %d count %d", results[1].Age, results[1].Count)
	}
}

func TestFindSeq(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{{ID: "s1", Name: "A"}, {ID: "s2", Name: "B"}, {ID: "s3", Name: "C"}}
	_, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}

	count := 0
	for item, err := range Users.FindSeq(ctx, bson.M{}) {
		if err != nil {
			t.Fatalf("FindSeq iteration error: %v", err)
		}
		count++
		if item.Name == "" {
			t.Errorf("got empty name for item %v", item)
		}
	}
	if count != 3 {
		t.Errorf("expected 3 items, got %d", count)
	}
}

func TestWithTransaction(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	client := db.Client()
	session, err := client.StartSession()
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	defer session.EndSession(context.Background())

	res, err := WithTransaction(ctx, session, func(ctx context.Context) (string, error) {
		_, err := Users.InsertOne(ctx, User{Name: "Transactor"})
		return "done", err
	})

	if err != nil {
		t.Logf("transaction failed (expected on standalone): %v", err)
	} else {
		if res != "done" {
			t.Errorf("expected 'done', got '%s'", res)
		}
	}
}

func TestWithTransactionPropagation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	client := db.Client()
	session, err := client.StartSession()
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	defer session.EndSession(context.Background())

	val, err := WithTransaction(ctx, session, func(ctx context.Context) (int, error) {
		return 123, nil
	})

	if err == nil {
		if val != 123 {
			t.Errorf("expected 123, got %d", val)
		}
	}
}

func TestUpdateMany(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "um1", Name: "User1", Age: 20},
		{ID: "um2", Name: "User2", Age: 20},
		{ID: "um3", Name: "User3", Age: 30},
	}
	_, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}

	res, err := Users.UpdateMany(ctx, bson.M{"age": 20}, bson.M{"$set": bson.M{"age": 25}})
	if err != nil {
		t.Fatalf("UpdateMany failed: %v", err)
	}
	if res.ModifiedCount != 2 {
		t.Errorf("expected 2 modified, got %d", res.ModifiedCount)
	}

	updated, err := Users.Find(ctx, bson.M{"age": 25})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(updated) != 2 {
		t.Errorf("expected 2 users with age 25, got %d", len(updated))
	}
}

func TestReplaceOne(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	user := User{ID: "r1", Name: "Original", Age: 20}
	_, err := Users.InsertOne(ctx, user)
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}

	replacement := User{ID: "r1", Name: "Replaced", Age: 99}
	res, err := Users.ReplaceOne(ctx, bson.M{"_id": "r1"}, replacement)
	if err != nil {
		t.Fatalf("ReplaceOne failed: %v", err)
	}
	if res.ModifiedCount != 1 {
		t.Errorf("expected 1 modified, got %d", res.ModifiedCount)
	}

	replaced, err := Users.FindOne(ctx, bson.M{"_id": "r1"})
	if err != nil {
		t.Fatalf("FindOne failed: %v", err)
	}
	if replaced.Name != "Replaced" || replaced.Age != 99 {
		t.Errorf("expected Replaced/99, got %s/%d", replaced.Name, replaced.Age)
	}
}

func TestFindOneAndUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	user := User{ID: "fau1", Name: "Before", Age: 20}
	_, err := Users.InsertOne(ctx, user)
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}

	before, err := Users.FindOneAndUpdate(ctx, bson.M{"_id": "fau1"}, bson.M{"$set": bson.M{"name": "After"}})
	if err != nil {
		t.Fatalf("FindOneAndUpdate failed: %v", err)
	}
	if before.Name != "Before" {
		t.Errorf("expected Before, got %s", before.Name)
	}

	after, err := Users.FindOne(ctx, bson.M{"_id": "fau1"})
	if err != nil {
		t.Fatalf("FindOne failed: %v", err)
	}
	if after.Name != "After" {
		t.Errorf("expected After, got %s", after.Name)
	}
}

func TestFindOneAndUpdateWithReturnDocumentAfter(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	user := User{ID: "fau2", Name: "Before", Age: 20}
	_, err := Users.InsertOne(ctx, user)
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}

	after, err := Users.FindOneAndUpdate(ctx,
		bson.M{"_id": "fau2"},
		bson.M{"$set": bson.M{"name": "After"}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	if err != nil {
		t.Fatalf("FindOneAndUpdate failed: %v", err)
	}
	if after.Name != "After" {
		t.Errorf("expected After, got %s", after.Name)
	}
}

func TestFindOneAndReplace(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	user := User{ID: "far1", Name: "Original", Age: 20}
	_, err := Users.InsertOne(ctx, user)
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}

	replacement := User{ID: "far1", Name: "Replacement", Age: 99}
	before, err := Users.FindOneAndReplace(ctx, bson.M{"_id": "far1"}, replacement)
	if err != nil {
		t.Fatalf("FindOneAndReplace failed: %v", err)
	}
	if before.Name != "Original" {
		t.Errorf("expected Original, got %s", before.Name)
	}

	after, err := Users.FindOne(ctx, bson.M{"_id": "far1"})
	if err != nil {
		t.Fatalf("FindOne failed: %v", err)
	}
	if after.Name != "Replacement" || after.Age != 99 {
		t.Errorf("expected Replacement/99, got %s/%d", after.Name, after.Age)
	}
}

func TestFindOneAndDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	user := User{ID: "fad1", Name: "ToDelete", Age: 20}
	_, err := Users.InsertOne(ctx, user)
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}

	deleted, err := Users.FindOneAndDelete(ctx, bson.M{"_id": "fad1"})
	if err != nil {
		t.Fatalf("FindOneAndDelete failed: %v", err)
	}
	if deleted.Name != "ToDelete" {
		t.Errorf("expected ToDelete, got %s", deleted.Name)
	}

	_, err = Users.FindOne(ctx, bson.M{"_id": "fad1"})
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("expected ErrNoDocuments, got %v", err)
	}
}

func TestDeleteMany(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "dm1", Name: "User1", Age: 20},
		{ID: "dm2", Name: "User2", Age: 20},
		{ID: "dm3", Name: "User3", Age: 30},
	}
	_, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}

	res, err := Users.DeleteMany(ctx, bson.M{"age": 20})
	if err != nil {
		t.Fatalf("DeleteMany failed: %v", err)
	}
	if res.DeletedCount != 2 {
		t.Errorf("expected 2 deleted, got %d", res.DeletedCount)
	}

	remaining, err := Users.Find(ctx, bson.M{})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Age != 30 {
		t.Errorf("expected 1 user with age 30, got %d users", len(remaining))
	}
}

func TestAggregateWithoutTransformation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "agg4", Name: "Alice", Age: 20},
		{ID: "agg5", Name: "Bob", Age: 30},
	}
	_, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}

	pipeline := bson.A{
		bson.M{"$match": bson.M{"age": bson.M{"$gte": 20}}},
		bson.M{"$sort": bson.D{{Key: "age", Value: 1}}},
	}

	results, err := Users.Aggregate(ctx, pipeline)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results[0].Age != 20 || results[1].Age != 30 {
		t.Errorf("expected sorted ages 20,30, got %d,%d", results[0].Age, results[1].Age)
	}
}

func TestFindAsWithProjection(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "fa1", Name: "Alice", Age: 20},
		{ID: "fa2", Name: "Bob", Age: 30},
	}
	_, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}

	type NameOnly struct {
		Name string `bson:"name"`
	}

	results, err := FindAs[NameOnly](ctx, Users, bson.M{}, options.Find().SetProjection(bson.M{"name": 1, "_id": 0}))
	if err != nil {
		t.Fatalf("FindAs failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results[0].Name == "" || results[1].Name == "" {
		t.Errorf("expected non-empty names")
	}
}

func TestFindSeqAsWithProjection(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "fsa1", Name: "Alice", Age: 20},
		{ID: "fsa2", Name: "Bob", Age: 30},
	}
	_, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}

	type NameOnly struct {
		Name string `bson:"name"`
	}

	count := 0
	for item, err := range FindSeqAs[NameOnly](ctx, Users, bson.M{}, options.Find().SetProjection(bson.M{"name": 1, "_id": 0})) {
		if err != nil {
			t.Fatalf("FindSeqAs iteration error: %v", err)
		}
		count++
		if item.Name == "" {
			t.Errorf("expected non-empty name")
		}
	}
	if count != 2 {
		t.Errorf("expected 2 items, got %d", count)
	}
}

func TestFindOneAsWithProjection(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	user := User{ID: "foa1", Name: "Alice", Age: 20}
	_, err := Users.InsertOne(ctx, user)
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}

	type NameOnly struct {
		Name string `bson:"name"`
	}

	result, err := FindOneAs[NameOnly](ctx, Users, bson.M{"_id": "foa1"}, options.FindOne().SetProjection(bson.M{"name": 1, "_id": 0}))
	if err != nil {
		t.Fatalf("FindOneAs failed: %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("expected Alice, got %s", result.Name)
	}
}

func TestFindSeqEarlyTermination(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "seq1", Name: "A"},
		{ID: "seq2", Name: "B"},
		{ID: "seq3", Name: "C"},
	}
	_, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}

	count := 0
	for _, err := range Users.FindSeq(ctx, bson.M{}) {
		if err != nil {
			t.Fatalf("FindSeq error: %v", err)
		}
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("expected early termination at 2, got %d", count)
	}
}

func TestErrorHandlingNoDatabaseInContext(t *testing.T) {
	ctxNoDB := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "Find",
			fn: func() error {
				_, err := Users.Find(ctxNoDB, bson.M{})
				return err
			},
		},
		{
			name: "FindOne",
			fn: func() error {
				_, err := Users.FindOne(ctxNoDB, bson.M{})
				return err
			},
		},
		{
			name: "InsertOne",
			fn: func() error {
				_, err := Users.InsertOne(ctxNoDB, User{})
				return err
			},
		},
		{
			name: "UpdateOne",
			fn: func() error {
				_, err := Users.UpdateOne(ctxNoDB, bson.M{}, bson.M{})
				return err
			},
		},
		{
			name: "DeleteOne",
			fn: func() error {
				_, err := Users.DeleteOne(ctxNoDB, bson.M{})
				return err
			},
		},
		{
			name: "CountDocuments",
			fn: func() error {
				_, err := Users.CountDocuments(ctxNoDB, bson.M{})
				return err
			},
		},
		{
			name: "Aggregate",
			fn: func() error {
				_, err := Users.Aggregate(ctxNoDB, bson.A{})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if !errors.Is(err, ErrNoDatabase) {
				t.Errorf("expected ErrNoDatabase, got %v", err)
			}
		})
	}
}

func TestOptionsSupport(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := WithContext(context.Background(), db)

	cleanupCollection(t, ctx, Users)

	users := []User{
		{ID: "opt1", Name: "A", Age: 30},
		{ID: "opt2", Name: "B", Age: 20},
		{ID: "opt3", Name: "C", Age: 40},
	}
	_, err := Users.InsertMany(ctx, users)
	if err != nil {
		t.Fatalf("InsertMany failed: %v", err)
	}

	results, err := Users.Find(ctx, bson.M{},
		options.Find().SetLimit(2).SetSort(bson.D{{Key: "age", Value: 1}}),
	)
	if err != nil {
		t.Fatalf("Find with options failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results[0].Age != 20 || results[1].Age != 30 {
		t.Errorf("expected sorted ages 20,30, got %d,%d", results[0].Age, results[1].Age)
	}

	count, err := Users.CountDocuments(ctx, bson.M{}, options.Count().SetLimit(2))
	if err != nil {
		t.Fatalf("CountDocuments with options failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}
