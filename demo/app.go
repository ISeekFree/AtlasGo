package main

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"iseekfree.com/common/sdk/gomvc/common"
	demov1 "iseekfree.com/common/sdk/gomvc/demo/gen/demo/v1"
	clawgrpc "iseekfree.com/common/sdk/gomvc/grpc"
	clawmongo "iseekfree.com/common/sdk/gomvc/mongo"
	clawredis "iseekfree.com/common/sdk/gomvc/redis"
	"iseekfree.com/common/sdk/gomvc/web"
)

type optionalClients struct {
	mongo         *clawmongo.Registry
	mongoDatabase string
	redis         *goredis.Client
	redisKey      clawredis.KeyBuilder
}

type mongoConnectionTest struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name" mongo:"index=idx_connection_test_name"`
	CreatedAt time.Time          `bson:"createdAt" mongo:"index=ttl_connection_test,expire=3600"`
}

func (mongoConnectionTest) MongoDatastore() string  { return "demo" }
func (mongoConnectionTest) MongoCollection() string { return "connection_tests" }

func newDemoApp(ctx context.Context, config Config) (*gin.Engine, func(context.Context) error, error) {
	grpcConfig := defaultDemoGRPCConfig()
	if config.GRPC != nil {
		grpcConfig = *config.GRPC
	}
	listener, err := net.Listen("tcp", grpcConfig.Server.ListenAddress())
	if err != nil {
		return nil, nil, err
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(clawgrpc.UnaryServerAuthInterceptor(grpcConfig.Server.AuthOptions())),
	)
	demov1.RegisterDemoServiceServer(grpcServer, demoServiceServer{})
	go func() {
		_ = grpcServer.Serve(listener)
	}()

	clients, err := setupOptionalClients(ctx, config)
	if err != nil {
		grpcServer.Stop()
		_ = listener.Close()
		return nil, nil, err
	}

	clientOptions := grpcConfig.Client
	clientOptions.Channels = cloneChannels(clientOptions.Channels)
	local := clientOptions.Channels["local"]
	if local.Target == "" || grpcConfig.Server.Port == 0 {
		local.Target = listener.Addr().String()
	}
	clientOptions.Channels["local"] = local
	channelFactory := clawgrpc.NewChannelFactory(clientOptions)

	engine := gin.New()
	sdk := web.New(web.Options{
		ContextCustomizers: []web.ContextCustomizer{
			func(wc *web.Context, c *gin.Context) {
				traceID := c.GetHeader("x-demo-trace-id")
				if traceID != "" {
					wc.SetAttribute("demoTraceId", traceID)
				}
			},
		},
	})
	sdk.Install(engine)

	engine.GET("/demo/public", func(c *gin.Context) {
		wc := web.MustCurrent(c)
		traceID, _ := wc.GetAttribute("demoTraceId")
		c.JSON(http.StatusOK, gin.H{"ip": wc.IP, "traceId": stringValue(traceID)})
	})
	engine.GET("/demo/options", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"mongo": clients.mongo != nil,
			"redis": clients.redis != nil,
			"grpc":  true,
		})
	})
	engine.GET("/demo/me", sdk.RequireAuth(web.Domains("app.demo"), web.Permissions("demo:read")), func(c *gin.Context) {
		wc := web.MustCurrent(c)
		c.JSON(http.StatusOK, gin.H{"uid": wc.UID, "domain": wc.Domain})
	})
	grpcAuth := sdk.RequireAuth(web.Domains("app.demo"))
	engine.GET("/demo/grpc", grpcAuth, func(c *gin.Context) {
		client := demoGRPCClient(c, channelFactory)
		response, err := client.Echo(c.Request.Context(), &demov1.EchoRequest{Message: "ping"})
		if err != nil {
			panic(common.WrapError(500, "gRPC Echo failed", err))
		}
		c.String(http.StatusOK, response.GetMessage())
	})
	engine.GET("/demo/grpc/echo", grpcAuth, func(c *gin.Context) {
		client := demoGRPCClient(c, channelFactory)
		response, err := client.Echo(c.Request.Context(), &demov1.EchoRequest{Message: c.DefaultQuery("message", "ping")})
		if err != nil {
			panic(common.WrapError(500, "gRPC Echo failed", err))
		}
		c.JSON(http.StatusOK, gin.H{"message": response.GetMessage(), "userId": response.GetUserId()})
	})
	engine.GET("/demo/grpc/me", grpcAuth, func(c *gin.Context) {
		client := demoGRPCClient(c, channelFactory)
		response, err := client.CurrentUser(c.Request.Context(), &demov1.CurrentUserRequest{})
		if err != nil {
			panic(common.WrapError(500, "gRPC CurrentUser failed", err))
		}
		c.JSON(http.StatusOK, gin.H{
			"userId": response.GetUserId(), "domain": response.GetDomain(), "permissions": response.GetPermissions(),
		})
	})
	engine.GET("/demo/stream", sdk.RequireAuth(web.Domains("app.demo")), func(c *gin.Context) {
		wc := web.MustCurrent(c)
		c.Header("Content-Type", "text/event-stream")
		c.SSEvent("message", "uid:"+wc.UID)
		c.SSEvent("message", "domain:"+wc.Domain)
	})
	if clients.mongo != nil {
		engine.GET("/demo/mongo/status", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"database": clients.mongoDatabase})
		})
		engine.GET("/demo/mongo/basic", func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), shortTimeout())
			defer cancel()

			collection, err := clients.mongo.CollectionFor(mongoConnectionTest{})
			if err != nil {
				panic(common.WrapError(500, "Mongo entity mapping failed", err))
			}
			id := primitive.NewObjectID()
			if _, err := collection.InsertOne(ctx, bson.M{
				"_id":       id,
				"name":      "claw-sdk-demo",
				"createdAt": time.Now(),
			}); err != nil {
				panic(common.WrapError(500, "Mongo insert failed", err))
			}
			var found bson.M
			if err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&found); err != nil {
				panic(common.WrapError(500, "Mongo find failed", err))
			}
			if _, err := collection.UpdateByID(ctx, id, bson.M{"$set": bson.M{"status": "ok"}}); err != nil {
				panic(common.WrapError(500, "Mongo update failed", err))
			}
			deleted, err := collection.DeleteOne(ctx, bson.M{"_id": id})
			if err != nil {
				panic(common.WrapError(500, "Mongo delete failed", err))
			}
			c.JSON(http.StatusOK, gin.H{
				"id":      id.Hex(),
				"name":    found["name"],
				"deleted": deleted.DeletedCount,
			})
		})
	}
	if clients.redis != nil {
		engine.GET("/demo/redis/key/:name", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"key": clients.redisKey.Of(c.Param("name"))})
		})
		engine.GET("/demo/redis/basic", func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), shortTimeout())
			defer cancel()

			key := clients.redisKey.Of("connection-test")
			value := "claw-sdk-demo"
			if err := clients.redis.Set(ctx, key, value, time.Minute).Err(); err != nil {
				panic(common.WrapError(500, "Redis set failed", err))
			}
			got, err := clients.redis.Get(ctx, key).Result()
			if err != nil {
				panic(common.WrapError(500, "Redis get failed", err))
			}
			deleted, err := clients.redis.Del(ctx, key).Result()
			if err != nil {
				panic(common.WrapError(500, "Redis del failed", err))
			}
			c.JSON(http.StatusOK, gin.H{"key": key, "value": got, "deleted": deleted})
		})
	}

	cleanup := func(ctx context.Context) error {
		grpcServer.Stop()
		_ = listener.Close()
		var firstErr error
		if err := channelFactory.Close(); err != nil {
			firstErr = err
		}
		if clients.mongo != nil {
			if err := clients.mongo.Close(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if clients.redis != nil {
			if err := clients.redis.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
	return engine, cleanup, nil
}

func demoGRPCClient(c *gin.Context, channelFactory *clawgrpc.ChannelFactory) demov1.DemoServiceClient {
	conn, err := channelFactory.Channel(c.Request.Context(), "local")
	if err != nil {
		panic(common.WrapError(500, "gRPC channel failed", err))
	}
	return demov1.NewDemoServiceClient(conn)
}

func defaultDemoGRPCConfig() clawgrpc.Config {
	return clawgrpc.Config{
		Server: clawgrpc.ServerOptions{
			Addr: "127.0.0.1:0",
			Auth: clawgrpc.ServerAuthConfig{Required: true},
		},
		Client: clawgrpc.ClientOptions{
			Channels: map[string]clawgrpc.ChannelOptions{
				"local": {Plaintext: true, DialTimeout: shortTimeout()},
			},
		},
	}
}

func cloneChannels(input map[string]clawgrpc.ChannelOptions) map[string]clawgrpc.ChannelOptions {
	output := make(map[string]clawgrpc.ChannelOptions, len(input)+1)
	for name, channel := range input {
		output[name] = channel
	}
	return output
}

func setupOptionalClients(ctx context.Context, config Config) (optionalClients, error) {
	clients := optionalClients{}
	if config.Mongo != nil && config.Mongo.IsEnabled() {
		mongoConfig := *config.Mongo
		mongoConfig.Entities = append(mongoConfig.Entities, clawmongo.EntityMapping{Model: mongoConnectionTest{}})
		registry, err := clawmongo.Connect(ctx, mongoConfig)
		if err != nil {
			return clients, err
		}
		clients.mongo = registry
		database, err := registry.RequireDatabase("demo")
		if err != nil {
			_ = registry.Close(context.Background())
			return clients, err
		}
		clients.mongoDatabase = database.Name()
	}
	if config.Redis != nil && config.Redis.IsEnabled() {
		clients.redis = clawredis.NewClient(*config.Redis)
		clients.redisKey = clawredis.NewKeyBuilder(config.Redis.KeyPrefix)
	}
	return clients, nil
}
