package zedcache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/efficientgo/e2e"
	cache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/connylabs/zedcache/go-cache"
)

func TestCheckPermission(t *testing.T) {
	if v, ok := os.LookupEnv("E2E"); !ok || !(v == "1" || v == "true") {
		t.Skip("To enable this test, set the E2E environment variable to 1 or true")
	}
	e, err := e2e.NewDockerEnvironment("main-e2e")
	require.NoError(t, err)
	wd, err := os.Getwd()
	require.NoError(t, err)
	r := e.Runnable("authzed").WithPorts(
		map[string]int{
			"grpc": 50051,
		}).Init(e2e.StartOptions{
		Image:     "authzed/spicedb:v1.15.0",
		Command:   e2e.NewCommand("", "serve-testing", "--load-configs=/mnt/authzed.yaml", "--log-level=trace"),
		Readiness: e2e.NewTCPReadinessProbe("grpc"),
		Volumes:   []string{fmt.Sprintf("%s:/mnt", wd)},
	})
	require.NoError(t, r.Start())
	require.NoError(t, r.WaitReady())
	t.Cleanup(func() {
		assert.NoError(t, r.Stop())
	})

	ac, err := authzed.NewClient(r.Endpoint("grpc"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	waitForAuthZed(t, ctx, ac)
	ca := cache.New(cache.NoExpiration, cache.NoExpiration)
	c := New(ac, gocache.New(ca))

	_, err = c.CheckPermission(ctx, &pb.CheckPermissionRequest{
		Permission: "read",
		Resource:   &pb.ObjectReference{ObjectType: "post", ObjectId: "1"},
		Subject: &pb.SubjectReference{
			Object: &pb.ObjectReference{
				ObjectType: "user",
				ObjectId:   "1",
			},
		},
	})
	assert.NoError(t, err)
	getCacheValue(t, ca, "post#1")

	_, err = c.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{
		Updates: []*pb.RelationshipUpdate{
			{
				Operation: pb.RelationshipUpdate_OPERATION_CREATE,
				Relationship: &pb.Relationship{
					Resource: &pb.ObjectReference{ObjectType: "post", ObjectId: "2"},
					Relation: "owner",
					Subject: &pb.SubjectReference{
						Object: &pb.ObjectReference{
							ObjectType: "user",
							ObjectId:   "1",
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)
	getCacheValue(t, ca, "post#2")
	getCacheValue(t, ca, "user#1")

	lrr, err := c.LookupResources(ctx, &pb.LookupResourcesRequest{
		ResourceObjectType: "post",
		Permission:         "read",
		Subject: &pb.SubjectReference{
			Object: &pb.ObjectReference{
				ObjectType: "user",
				ObjectId:   "1",
			},
		},
	})
	assert.NoError(t, err)
	lrrRecv, err := lrr.Recv()
	assert.NoError(t, err)

	getCacheValue(t, ca, "user#1")
	assert.Equal(t, lrrRecv.LookedUpAt.Token, getCacheValue(t, ca, "user#1"))
}

func getCacheValue(t *testing.T, ca *cache.Cache, key string) string {
	t.Helper()

	rawToken, ok := ca.Get(key)
	assert.True(t, ok)
	token, ok := rawToken.(string)
	assert.True(t, ok)
	return token
}

func waitForAuthZed(t *testing.T, ctx context.Context, ac *authzed.Client) {
	t.Helper()

	for i := 0; i*500*int(time.Millisecond) < int(10*time.Second); i++ {
		if _, err := ac.CheckPermission(ctx, &pb.CheckPermissionRequest{
			Permission: "read",
			Resource:   &pb.ObjectReference{ObjectType: "post", ObjectId: "1"},
			Subject: &pb.SubjectReference{
				Object: &pb.ObjectReference{
					ObjectType: "user",
					ObjectId:   "1",
				},
			},
		}); err == nil {
			return
		} else {
			t.Logf("authzed not ready: %s\n", err.Error())
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.Fail(t, "authzed not ready")
}
