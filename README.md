# Monarch

### Type-safe MongoDB wrapper for Go.

Monarch provides type safe operations for MongoDB using Go generics while maintaining full compatibility with the native MongoDB driver.

## Installation

```sh
go get github.com/eriicafes/monarch
```

## Quick Start

### Connect to MongoDB

```go
import (
    "context"

    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
    ctx := context.Background()

    client, err := mongo.Connect(options.Client().ApplyURI("mongodb://localhost:27017"))
    if err != nil {
        panic(err)
    }
    defer client.Disconnect(ctx)

    db := client.Database("myapp")
}
```

## Collections

Collections are defined as typed constants representing the collection name:

```go
import (
    "github.com/eriicafes/monarch"
    "go.mongodb.org/mongo-driver/v2/bson"
)

type User struct {
    ID     string `bson:"_id"`
    Name   string `bson:"name"`
    Email  string `bson:"email"`
    Age    int    `bson:"age"`
    Status string `bson:"status"`
}

var Users monarch.Collection[User] = "users"

// Find all active users
activeUsers, err := Users.Find(ctx, db, bson.M{"status": "active"})
```

Now all operations on `Users` return `User` types automatically.

### BoundCollection

Bind database to a collection once to skip passing it on every call:

```go
import (
    "github.com/eriicafes/monarch"
    "go.mongodb.org/mongo-driver/v2/bson"
)

users := monarch.Bind(db, Users)

// Now operations are simpler
activeUsers, err := users.Find(ctx, bson.M{"status": "active"})
```

All examples below show the `Collection` API. For `BoundCollection`, simply omit the `db` parameter.

## Find

Find all documents matching a filter:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
)

users, err := Users.Find(ctx, db, bson.M{"status": "active"})
```

With options:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
)

users, err := Users.Find(ctx, db,
    bson.M{
        "age":    bson.M{"$gte": 18},
        "status": "active",
    },
    options.Find().
        SetSort(bson.D{{"createdAt", -1}}).
        SetLimit(10).
        SetSkip(20).
        SetProjection(bson.M{
            "name":  1,
            "email": 1,
        }),
)
```

## FindSeq

Stream large result sets using an iterator:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
)

for user, err := range Users.FindSeq(ctx, db, bson.M{"status": "active"}) {
    if err != nil {
        // Handle error
        continue
    }
    // Process user
}
```

## FindOne

Find a single document:

```go
import (
    "errors"

    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
)

user, err := Users.FindOne(ctx, db, bson.M{"_id": "user123"})

if errors.Is(err, mongo.ErrNoDocuments) {
    // Handle not found
}
```

## FindOneAndUpdate

Atomically find and update a document:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Returns original document by default
user, err := Users.FindOneAndUpdate(ctx, db,
    bson.M{"_id": "user123"},
    bson.D{
        {"$inc", bson.M{"loginCount": 1}},
    },
)

// Return the updated document
after := options.After
user, err := Users.FindOneAndUpdate(ctx, db,
    bson.M{"_id": "user123"},
    bson.D{
        {"$inc", bson.M{"loginCount": 1}},
    },
    options.FindOneAndUpdate().SetReturnDocument(after),
)
```

## FindOneAndReplace

Atomically find and replace a document:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
)

user, err := Users.FindOneAndReplace(ctx, db,
    bson.M{"_id": "user123"},
    User{Name: "New Name", Email: "new@example.com"},
)
```

## FindOneAndDelete

Atomically find and delete a document:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
)

user, err := Users.FindOneAndDelete(ctx, db, bson.M{"_id": "user123"})
```

## InsertOne

Insert a single document:

```go
result, err := Users.InsertOne(ctx, db, User{
    Name:  "Alice",
    Email: "alice@example.com",
})
// result.InsertedID
```

## InsertMany

Insert multiple documents:

```go
users := []User{
    {Name: "Alice"},
    {Name: "Bob"},
    {Name: "Charlie"},
}
result, err := Users.InsertMany(ctx, db, users)
// result.InsertedIDs
```

## UpdateOne

Update a single document:

```go
import (
    "time"

    "go.mongodb.org/mongo-driver/v2/bson"
)

result, err := Users.UpdateOne(ctx, db,
    bson.M{"_id": "user123"},
    bson.D{
        {"$inc", bson.M{"loginCount": 1}},
        {"$set", bson.M{"lastLogin": time.Now()}},
        {"$currentDate", bson.M{"updatedAt": true}},
    },
)
// result.MatchedCount, result.ModifiedCount
```

## UpdateMany

Update multiple documents:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
)

result, err := Users.UpdateMany(ctx, db,
    bson.M{"status": "inactive"},
    bson.D{
        {"$set", bson.M{"status": "archived"}},
    },
)
// result.MatchedCount, result.ModifiedCount
```

## ReplaceOne

Replace an entire document:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
)

result, err := Users.ReplaceOne(ctx, db,
    bson.M{"_id": "user123"},
    User{Name: "Updated Name"},
)
// result.MatchedCount, result.ModifiedCount
```

## DeleteOne

Delete a single document:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
)

result, err := Users.DeleteOne(ctx, db, bson.M{"_id": "user123"})
// result.DeletedCount
```

## DeleteMany

Delete multiple documents:

```go
import (
    "time"

    "go.mongodb.org/mongo-driver/v2/bson"
)

result, err := Users.DeleteMany(ctx, db,
    bson.M{
        "lastLogin": bson.M{"$lt": time.Now().AddDate(0, -6, 0)},
    },
)
// result.DeletedCount
```

## CountDocuments

Count documents matching a filter:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
)

count, err := Users.CountDocuments(ctx, db,
    bson.M{
        "status": "active",
        "age":    bson.M{"$gte": 18},
    },
)
```

## Aggregate

Run aggregation pipelines:

```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
)

pipeline := bson.A{
    bson.M{"$match": bson.M{"status": "active"}},
    bson.M{"$group": bson.M{
        "_id":   "$role",
        "count": bson.M{"$sum": 1},
    }},
    bson.M{"$sort": bson.D{{"count", -1}}},
}

results, err := Users.Aggregate(ctx, db, pipeline)
```

### Type transformation

When aggregations transform the document structure, use `AggregateAs`:

```go
import (
    "github.com/eriicafes/monarch"
    "go.mongodb.org/mongo-driver/v2/bson"
)

type UserStats struct {
    Role  string `bson:"_id"`
    Count int    `bson:"count"`
}

pipeline := bson.A{
    bson.M{"$match": bson.M{"status": "active"}},
    bson.M{"$group": bson.M{
        "_id":   "$role",
        "count": bson.M{"$sum": 1},
    }},
}

stats, err := monarch.AggregateAs[UserStats](ctx, db, Users, pipeline)
```

Similarly, use `FindAs`, `FindSeqAs`, and `FindOneAs` when projections change the document structure.

## Transactions

Use MongoDB transactions with the session context:

```go
import (
    "go.mongodb.org/mongo-driver/v2/mongo"
)

session, err := client.StartSession()
if err != nil {
    panic(err)
}
defer session.EndSession(ctx)

err = mongo.WithSession(ctx, session, func(sessCtx mongo.SessionContext) error {
    err := session.StartTransaction()
    if err != nil {
        return err
    }

    _, err = Users.InsertOne(sessCtx, db, newUser)
    if err != nil {
        session.AbortTransaction(sessCtx)
        return err
    }

    _, err = Posts.InsertOne(sessCtx, db, newPost)
    if err != nil {
        session.AbortTransaction(sessCtx)
        return err
    }

    return session.CommitTransaction(sessCtx)
})
```

## Error Handling

All MongoDB driver errors pass through unchanged:

```go
import (
    "errors"

    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
)

user, err := Users.FindOne(ctx, db, bson.M{"_id": "nonexistent"})

if errors.Is(err, mongo.ErrNoDocuments) {
    // Handle document not found
}
```

## License

MIT
