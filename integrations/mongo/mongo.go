package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type CRUD[T any] struct {
	collection *mongodriver.Collection
}

func NewCRUD[T any](collection *mongodriver.Collection) *CRUD[T] {
	return &CRUD[T]{collection: collection}
}

func (c *CRUD[T]) Collection() *mongodriver.Collection {
	return c.collection
}

func (c *CRUD[T]) Insert(ctx context.Context, entity *T) (*mongodriver.InsertOneResult, error) {
	return c.collection.InsertOne(ctx, entity)
}

func (c *CRUD[T]) FindByID(ctx context.Context, id any) (T, error) {
	var out T
	err := c.collection.FindOne(ctx, bson.M{"_id": normalizeID(id)}).Decode(&out)
	return out, err
}

func (c *CRUD[T]) UpdateByID(ctx context.Context, id any, update any) (*mongodriver.UpdateResult, error) {
	return c.collection.UpdateByID(ctx, normalizeID(id), update)
}

func (c *CRUD[T]) DeleteByID(ctx context.Context, id any) (*mongodriver.DeleteResult, error) {
	return c.collection.DeleteOne(ctx, bson.M{"_id": normalizeID(id)})
}

func (c *CRUD[T]) Find(ctx context.Context, filter any, opts ...*options.FindOptions) ([]T, error) {
	cursor, err := c.collection.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []T
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func normalizeID(id any) any {
	if value, ok := id.(string); ok {
		if objectID, err := primitive.ObjectIDFromHex(value); err == nil {
			return objectID
		}
	}
	return id
}
