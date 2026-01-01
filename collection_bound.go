package monarch

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// BoundCollection represents a collection with a pre-bound database.
//
// This provides an alternative API where the database doesn't need to be passed
// on every method call. All methods have the same behavior as Collection methods,
// but with the database parameter omitted.
type BoundCollection[T any] struct {
	db *mongo.Database
	c  Collection[T]
}

// Bind creates a BoundCollection by binding a database to a collection.
//
// This simplifies code when working with a single database, as you don't need
// to pass the database parameter on every operation.
func Bind[T any](db *mongo.Database, collection Collection[T]) BoundCollection[T] {
	return BoundCollection[T]{db, collection}
}

// Find returns all documents matching the filter as a slice.
// See Collection.Find for details.
func (bc BoundCollection[T]) Find(ctx context.Context, filter any, opts ...options.Lister[options.FindOptions]) ([]T, error) {
	return bc.c.Find(ctx, bc.db, filter, opts...)
}

// FindSeq returns an iterator over documents matching the filter.
// See Collection.FindSeq for details.
func (bc BoundCollection[T]) FindSeq(ctx context.Context, filter any, opts ...options.Lister[options.FindOptions]) func(yield func(T, error) bool) {
	return bc.c.FindSeq(ctx, bc.db, filter, opts...)
}

// FindOne returns a single document matching the filter.
// See Collection.FindOne for details.
func (bc BoundCollection[T]) FindOne(ctx context.Context, filter any, opts ...options.Lister[options.FindOneOptions]) (T, error) {
	return bc.c.FindOne(ctx, bc.db, filter, opts...)
}

// FindOneAndUpdate atomically finds and updates a single document.
// See Collection.FindOneAndUpdate for details.
func (bc BoundCollection[T]) FindOneAndUpdate(ctx context.Context, filter any, update any, opts ...options.Lister[options.FindOneAndUpdateOptions]) (T, error) {
	return bc.c.FindOneAndUpdate(ctx, bc.db, filter, update, opts...)
}

// FindOneAndReplace atomically finds and replaces a single document.
// See Collection.FindOneAndReplace for details.
func (bc BoundCollection[T]) FindOneAndReplace(ctx context.Context, filter any, replacement T, opts ...options.Lister[options.FindOneAndReplaceOptions]) (T, error) {
	return bc.c.FindOneAndReplace(ctx, bc.db, filter, replacement, opts...)
}

// FindOneAndDelete atomically finds and deletes a single document.
// See Collection.FindOneAndDelete for details.
func (bc BoundCollection[T]) FindOneAndDelete(ctx context.Context, filter any, opts ...options.Lister[options.FindOneAndDeleteOptions]) (T, error) {
	return bc.c.FindOneAndDelete(ctx, bc.db, filter, opts...)
}

// InsertOne inserts a single document into the collection.
// See Collection.InsertOne for details.
func (bc BoundCollection[T]) InsertOne(ctx context.Context, document T, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error) {
	return bc.c.InsertOne(ctx, bc.db, document, opts...)
}

// InsertMany inserts multiple documents into the collection.
// See Collection.InsertMany for details.
func (bc BoundCollection[T]) InsertMany(ctx context.Context, documents []T, opts ...options.Lister[options.InsertManyOptions]) (*mongo.InsertManyResult, error) {
	return bc.c.InsertMany(ctx, bc.db, documents, opts...)
}

// UpdateOne updates a single document matching the filter.
// See Collection.UpdateOne for details.
func (bc BoundCollection[T]) UpdateOne(ctx context.Context, filter any, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	return bc.c.UpdateOne(ctx, bc.db, filter, update, opts...)
}

// UpdateMany updates all documents matching the filter.
// See Collection.UpdateMany for details.
func (bc BoundCollection[T]) UpdateMany(ctx context.Context, filter any, update any, opts ...options.Lister[options.UpdateManyOptions]) (*mongo.UpdateResult, error) {
	return bc.c.UpdateMany(ctx, bc.db, filter, update, opts...)
}

// ReplaceOne replaces a single document matching the filter.
// See Collection.ReplaceOne for details.
func (bc BoundCollection[T]) ReplaceOne(ctx context.Context, filter any, replacement T, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	return bc.c.ReplaceOne(ctx, bc.db, filter, replacement, opts...)
}

// DeleteOne deletes a single document matching the filter.
// See Collection.DeleteOne for details.
func (bc BoundCollection[T]) DeleteOne(ctx context.Context, filter any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	return bc.c.DeleteOne(ctx, bc.db, filter, opts...)
}

// DeleteMany deletes all documents matching the filter.
// See Collection.DeleteMany for details.
func (bc BoundCollection[T]) DeleteMany(ctx context.Context, filter any, opts ...options.Lister[options.DeleteManyOptions]) (*mongo.DeleteResult, error) {
	return bc.c.DeleteMany(ctx, bc.db, filter, opts...)
}

// CountDocuments returns the number of documents matching the filter.
// See Collection.CountDocuments for details.
func (bc BoundCollection[T]) CountDocuments(ctx context.Context, filter any, opts ...options.Lister[options.CountOptions]) (int64, error) {
	return bc.c.CountDocuments(ctx, bc.db, filter, opts...)
}

// Aggregate executes an aggregation pipeline and returns results as type T.
// See Collection.Aggregate for details.
func (bc BoundCollection[T]) Aggregate(ctx context.Context, pipeline any, opts ...options.Lister[options.AggregateOptions]) ([]T, error) {
	return bc.c.Aggregate(ctx, bc.db, pipeline, opts...)
}
