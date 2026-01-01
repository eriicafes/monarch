// Package monarch provides a type-safe MongoDB wrapper for Go.
//
// Monarch offers compile-time type safety for MongoDB operations using Go generics
// while maintaining full compatibility with the native MongoDB driver.
// It uses the official MongoDB driver's bson and options packages directly,
// providing only a thin type-safety layer with zero abstraction overhead.
//
// # Quick Start
//
// Define your document types and collections:
//
//	type User struct {
//	    ID     string `bson:"_id"`
//	    Name   string `bson:"name"`
//	    Email  string `bson:"email"`
//	    Status string `bson:"status"`
//	}
//
//	var Users Collection[User] = "users"
//
// Use with native MongoDB driver APIs:
//
//	import (
//	    "github.com/eriicafes/monarch"
//	    "go.mongodb.org/mongo-driver/v2/bson"
//	    "go.mongodb.org/mongo-driver/v2/mongo/options"
//	)
//
//	// Find with native bson filters
//	users, err := Users.Find(ctx, db, bson.M{"status": "active"})
//
//	// Update with native bson operations
//	result, err := Users.UpdateOne(ctx, db,
//	    bson.M{"_id": "user123"},
//	    bson.D{
//	        {"$set", bson.M{"status": "inactive"}},
//	        {"$inc", bson.M{"loginCount": 1}},
//	    },
//	)
//
//	// Use native options
//	users, err := Users.Find(ctx, db,
//	    bson.M{"age": bson.M{"$gte": 18}},
//	    options.Find().SetLimit(10).SetSort(bson.D{{"createdAt", -1}}),
//	)
//
// # BoundCollection
//
// For cleaner code, bind the database once:
//
//	users := monarch.Bind(db, Users)
//	results, err := users.Find(ctx, bson.M{"status": "active"})
//
// # Type Transformation
//
// When aggregations or projections change the document structure,
// use the *As functions:
//
//	type UserStats struct {
//	    Role  string `bson:"_id"`
//	    Count int    `bson:"count"`
//	}
//
//	pipeline := bson.A{
//	    bson.M{"$match": bson.M{"status": "active"}},
//	    bson.M{"$group": bson.M{
//	        "_id":   "$role",
//	        "count": bson.M{"$sum": 1},
//	    }},
//	}
//
//	stats, err := monarch.AggregateAs[UserStats](ctx, db, Users, pipeline)
//
// Similarly, use FindAs, FindSeqAs, and FindOneAs for projections that
// change the document structure.
//
// # Error Handling
//
// All MongoDB driver errors pass through unchanged:
//
//	user, err := Users.FindOne(ctx, db, bson.M{"_id": "nonexistent"})
//	if errors.Is(err, mongo.ErrNoDocuments) {
//	    // Handle not found
//	}
//
// # Streaming Results
//
// Use FindSeq for memory-efficient iteration over large result sets:
//
//	for user, err := range Users.FindSeq(ctx, db, bson.M{"status": "active"}) {
//	    if err != nil {
//	        // Handle error, continue or break as needed
//	        continue
//	    }
//	    // Process user
//	}
package monarch

import (
	"context"
	"iter"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Collection represents a type-safe MongoDB collection.
//
// The generic type parameter T specifies the document structure for this collection,
// ensuring all operations return correctly typed results without manual type assertions.
//
// Collections are defined as typed string constants representing the collection name.
// The string value must match the actual collection name in MongoDB.
type Collection[T any] string

// Find executes a find command and returns all documents matching the filter as a slice.
//
// All results are loaded into memory. For large result sets, use FindSeq instead.
// The filter parameter accepts any valid MongoDB filter document (typically bson.M or bson.D).
// An empty filter (bson.D{} or bson.M{}) matches all documents in the collection.
// Options can be provided using the native mongo/options package.
//
// See [mongo.Collection.Find] for more details.
func (c Collection[T]) Find(ctx context.Context, db *mongo.Database, filter any, opts ...options.Lister[options.FindOptions]) ([]T, error) {
	collection := db.Collection(string(c))
	cursor, err := collection.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []T
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// FindSeq executes a find command and returns an iterator over documents matching the filter.
//
// This provides memory-efficient streaming for large result sets without loading
// all documents into memory at once. The iterator yields both the document and any
// error encountered during iteration. Users control iteration flow by returning
// true to continue or false to stop.
//
// See [mongo.Collection.Find] for more details on the underlying operation.
func (c Collection[T]) FindSeq(ctx context.Context, db *mongo.Database, filter any, opts ...options.Lister[options.FindOptions]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		collection := db.Collection(string(c))
		cursor, err := collection.Find(ctx, filter, opts...)
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		defer cursor.Close(ctx)

		for cursor.Next(ctx) {
			var result T
			err := cursor.Decode(&result)
			if !yield(result, err) {
				return
			}
		}

		if err := cursor.Err(); err != nil {
			var zero T
			yield(zero, err)
		}
	}
}

// FindOne executes a find command and returns a single document matching the filter.
//
// Returns mongo.ErrNoDocuments if no document matches the filter.
// If multiple documents match the filter, only the first document is returned
// according to the collection's natural order or the sort order if specified in options.
//
// See [mongo.Collection.FindOne] for more details.
func (c Collection[T]) FindOne(ctx context.Context, db *mongo.Database, filter any, opts ...options.Lister[options.FindOneOptions]) (T, error) {
	var result T
	collection := db.Collection(string(c))
	err := collection.FindOne(ctx, filter, opts...).Decode(&result)
	return result, err
}

// FindOneAndUpdate executes a findAndModify command to update at most one document.
//
// This operation is atomic - the document is updated and returned in a single operation,
// preventing race conditions. By default, returns the document before the update was applied.
// Use options.FindOneAndUpdate().SetReturnDocument(options.After) to return the updated
// document instead. Returns mongo.ErrNoDocuments if no document matches the filter.
//
// The update parameter must contain update operators (e.g., $set, $inc).
// Use FindOneAndReplace if you want to replace the entire document.
//
// See [mongo.Collection.FindOneAndUpdate] for more details.
func (c Collection[T]) FindOneAndUpdate(ctx context.Context, db *mongo.Database, filter any, update any, opts ...options.Lister[options.FindOneAndUpdateOptions]) (T, error) {
	var result T
	collection := db.Collection(string(c))
	err := collection.FindOneAndUpdate(ctx, filter, update, opts...).Decode(&result)
	return result, err
}

// FindOneAndReplace executes a findAndModify command to replace at most one document.
//
// This operation is atomic - the document is replaced and returned in a single operation,
// preventing race conditions. By default, returns the document before the replacement
// was applied. Use options.FindOneAndReplace().SetReturnDocument(options.After) to
// return the new document instead. Returns mongo.ErrNoDocuments if no document matches.
//
// The replacement document must not contain update operators. Use FindOneAndUpdate
// if you want to use update operators.
//
// See [mongo.Collection.FindOneAndReplace] for more details.
func (c Collection[T]) FindOneAndReplace(ctx context.Context, db *mongo.Database, filter any, replacement T, opts ...options.Lister[options.FindOneAndReplaceOptions]) (T, error) {
	var result T
	collection := db.Collection(string(c))
	err := collection.FindOneAndReplace(ctx, filter, replacement, opts...).Decode(&result)
	return result, err
}

// FindOneAndDelete executes a findAndModify command to delete at most one document.
//
// This operation is atomic - the document is deleted and returned in a single operation,
// preventing race conditions. Returns the document that was deleted.
// Returns mongo.ErrNoDocuments if no document matches the filter.
//
// See [mongo.Collection.FindOneAndDelete] for more details.
func (c Collection[T]) FindOneAndDelete(ctx context.Context, db *mongo.Database, filter any, opts ...options.Lister[options.FindOneAndDeleteOptions]) (T, error) {
	var result T
	collection := db.Collection(string(c))
	err := collection.FindOneAndDelete(ctx, filter, opts...).Decode(&result)
	return result, err
}

// InsertOne executes an insert command to insert a single document into the collection.
//
// If the document does not have an _id field, one will be generated automatically.
// Returns the inserted document's ID in InsertOneResult.InsertedID.
//
// See [mongo.Collection.InsertOne] for more details.
func (c Collection[T]) InsertOne(ctx context.Context, db *mongo.Database, value T, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error) {
	collection := db.Collection(string(c))
	return collection.InsertOne(ctx, value, opts...)
}

// InsertMany executes an insert command to insert multiple documents into the collection.
//
// If any document does not have an _id field, one will be generated automatically.
// Returns the inserted document IDs in InsertManyResult.InsertedIDs.
//
// By default, documents are inserted in order and the operation stops on the first error.
// Use options.InsertMany().SetOrdered(false) for unordered bulk inserts, which continue
// inserting even if one document fails.
//
// See [mongo.Collection.InsertMany] for more details.
func (c Collection[T]) InsertMany(ctx context.Context, db *mongo.Database, values []T, opts ...options.Lister[options.InsertManyOptions]) (*mongo.InsertManyResult, error) {
	collection := db.Collection(string(c))
	docs := make([]any, len(values))
	for i, v := range values {
		docs[i] = v
	}
	return collection.InsertMany(ctx, docs, opts...)
}

// UpdateOne executes an update command to update at most one document matching the filter.
//
// The update parameter must contain update operators (e.g., $set, $inc, $push).
// Use ReplaceOne if you want to replace the entire document without operators.
//
// Returns UpdateResult with MatchedCount (number of documents matched) and
// ModifiedCount (number of documents actually changed) fields.
// Use options.UpdateOne().SetUpsert(true) to insert a new document if no match is found.
//
// See [mongo.Collection.UpdateOne] for more details.
func (c Collection[T]) UpdateOne(ctx context.Context, db *mongo.Database, filter any, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	collection := db.Collection(string(c))
	return collection.UpdateOne(ctx, filter, update, opts...)
}

// UpdateMany executes an update command to update all documents matching the filter.
//
// The update parameter must contain update operators (e.g., $set, $inc, $push).
// Returns UpdateResult with MatchedCount (number of documents matched) and
// ModifiedCount (number of documents actually changed) fields.
//
// See [mongo.Collection.UpdateMany] for more details.
func (c Collection[T]) UpdateMany(ctx context.Context, db *mongo.Database, filter any, update any, opts ...options.Lister[options.UpdateManyOptions]) (*mongo.UpdateResult, error) {
	collection := db.Collection(string(c))
	return collection.UpdateMany(ctx, filter, update, opts...)
}

// ReplaceOne executes an update command to replace at most one document matching the filter.
//
// The entire document is replaced with the provided replacement document, except for the _id field.
// The replacement document must not contain update operators. Use UpdateOne if you need operators.
//
// Returns UpdateResult with MatchedCount (number of documents matched) and
// ModifiedCount (number of documents actually changed) fields.
// Use options.Replace().SetUpsert(true) to insert the replacement if no match is found.
//
// See [mongo.Collection.ReplaceOne] for more details.
func (c Collection[T]) ReplaceOne(ctx context.Context, db *mongo.Database, filter any, replacement T, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	collection := db.Collection(string(c))
	return collection.ReplaceOne(ctx, filter, replacement, opts...)
}

// DeleteOne executes a delete command to delete at most one document matching the filter.
//
// If multiple documents match the filter, only the first document according to the
// collection's natural order or the sort order is deleted.
//
// Returns DeleteResult with DeletedCount field indicating how many documents were
// deleted (0 or 1).
//
// See [mongo.Collection.DeleteOne] for more details.
func (c Collection[T]) DeleteOne(ctx context.Context, db *mongo.Database, filter any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	collection := db.Collection(string(c))
	return collection.DeleteOne(ctx, filter, opts...)
}

// DeleteMany executes a delete command to delete all documents matching the filter.
//
// An empty filter (bson.D{} or bson.M{}) will delete all documents in the collection.
// Returns DeleteResult with DeletedCount field indicating how many documents were deleted.
//
// See [mongo.Collection.DeleteMany] for more details.
func (c Collection[T]) DeleteMany(ctx context.Context, db *mongo.Database, filter any, opts ...options.Lister[options.DeleteManyOptions]) (*mongo.DeleteResult, error) {
	collection := db.Collection(string(c))
	return collection.DeleteMany(ctx, filter, opts...)
}

// CountDocuments executes a count command and returns the number of documents matching the filter.
//
// This method uses an aggregation pipeline to count documents, which provides an accurate
// count but may be slower for large collections. An empty filter (bson.D{} or bson.M{})
// counts all documents in the collection.
//
// For large collections, consider using options.Count().SetLimit() to avoid scanning
// the entire collection.
//
// See [mongo.Collection.CountDocuments] for more details.
func (c Collection[T]) CountDocuments(ctx context.Context, db *mongo.Database, filter any, opts ...options.Lister[options.CountOptions]) (int64, error) {
	collection := db.Collection(string(c))
	return collection.CountDocuments(ctx, filter, opts...)
}

// Aggregate executes an aggregation pipeline and returns results as type T.
//
// The pipeline parameter is typically bson.A containing aggregation stages
// (e.g., $match, $group, $sort, $limit). All results are loaded into memory.
//
// If the pipeline transforms the document structure (e.g., $group, $project),
// use AggregateAs instead to specify the result type. An empty pipeline (bson.A{})
// returns all documents in the collection.
//
// See [mongo.Collection.Aggregate] for more details.
func (c Collection[T]) Aggregate(ctx context.Context, db *mongo.Database, pipeline any, opts ...options.Lister[options.AggregateOptions]) ([]T, error) {
	collection := db.Collection(string(c))
	cursor, err := collection.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []T
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// AggregateAs executes an aggregation pipeline and returns results as type R.
//
// Use this when the aggregation pipeline transforms the document structure,
// such as with $group, $project, $lookup, or other stages that change the result shape.
// The result type R must match the structure of documents returned by the pipeline.
//
// This provides type safety when your aggregation produces a different document
// structure than the collection's base type T.
//
// See [mongo.Collection.Aggregate] for more details.
func AggregateAs[R, T any](ctx context.Context, db *mongo.Database, c Collection[T], pipeline any, opts ...options.Lister[options.AggregateOptions]) ([]R, error) {
	collection := db.Collection(string(c))
	cursor, err := collection.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []R
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// FindAs executes a find command and returns documents matching the filter as type R.
//
// Use this when projections change the document structure. The result type R must
// match the projected document structure. All results are loaded into memory.
//
// This provides type safety when using projections that select or transform specific
// fields, resulting in a different structure than the collection's base type T.
//
// See [mongo.Collection.Find] for more details.
func FindAs[R, T any](ctx context.Context, db *mongo.Database, c Collection[T], filter any, opts ...options.Lister[options.FindOptions]) ([]R, error) {
	collection := db.Collection(string(c))
	cursor, err := collection.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []R
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// FindSeqAs executes a find command and returns an iterator over documents as type R.
//
// Use this for memory-efficient streaming when projections change the document structure.
// The result type R must match the projected document structure.
//
// This combines the benefits of FindSeq (memory efficiency) with FindAs (type transformation)
// for large result sets with projected fields.
//
// See [mongo.Collection.Find] for more details.
func FindSeqAs[R, T any](ctx context.Context, db *mongo.Database, c Collection[T], filter any, opts ...options.Lister[options.FindOptions]) iter.Seq2[R, error] {
	return func(yield func(R, error) bool) {
		collection := db.Collection(string(c))
		cursor, err := collection.Find(ctx, filter, opts...)
		if err != nil {
			var zero R
			yield(zero, err)
			return
		}
		defer cursor.Close(ctx)

		for cursor.Next(ctx) {
			var result R
			err := cursor.Decode(&result)
			if !yield(result, err) {
				return
			}
		}

		if err := cursor.Err(); err != nil {
			var zero R
			yield(zero, err)
		}
	}
}

// FindOneAs executes a find command and returns a single document as type R.
//
// Use this when projections change the document structure. The result type R must
// match the projected document structure. Returns mongo.ErrNoDocuments if no document
// matches the filter.
//
// This provides type safety when using projections to select or transform specific
// fields for a single document.
//
// See [mongo.Collection.FindOne] for more details.
func FindOneAs[R, T any](ctx context.Context, db *mongo.Database, c Collection[T], filter any, opts ...options.Lister[options.FindOneOptions]) (R, error) {
	var result R
	collection := db.Collection(string(c))
	err := collection.FindOne(ctx, filter, opts...).Decode(&result)
	return result, err
}
