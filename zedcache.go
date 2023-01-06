package zedcache

import (
	"context"
	"fmt"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/grpc"

	"github.com/connylabs/zedcache/cache"
)

// Options allows to pass extra options to the PermissionsServiceClient implementation.
type Option func(*permissionClient)

// WithLogger can overwrite the default Noop logger.
// The logger implements "github.com/go-kit/log"'s Logger interface.
func WithLogger(l log.Logger) Option {
	return func(pc *permissionClient) {
		pc.l = l
	}
}

// New is a helper function to add a cache to the authzed.Client's PermissionsServiceClient implementation.
func New(c *authzed.Client, ca cache.Cache, opts ...Option) *authzed.Client {
	return &authzed.Client{
		PermissionsServiceClient: NewPermissionServiceClient(c.PermissionsServiceClient, ca, opts...),
		SchemaServiceClient:      c.SchemaServiceClient,
		WatchServiceClient:       c.WatchServiceClient,
	}
}

// NewPermissionServiceClient add a cache to the given PermissionsServiceClient.
// Note that unlike the original interface the default Consistency is FullyConsistent.
func NewPermissionServiceClient(c pb.PermissionsServiceClient, ca cache.Cache, opts ...Option) pb.PermissionsServiceClient {
	pc := &permissionClient{
		PermissionsServiceClient: c,
		ca:                       ca,
		l:                        log.NewNopLogger(),
	}
	for _, o := range opts {
		o(pc)
	}

	return pc
}

type permissionClient struct {
	pb.PermissionsServiceClient

	ca cache.Cache
	l  log.Logger
}

type permissionsService_ReadRelationshipsClient struct {
	pb.PermissionsService_ReadRelationshipsClient

	c                cache.Cache
	recourceCacheKey string
	cached           bool
	l                log.Logger
}

func (rrc *permissionsService_ReadRelationshipsClient) Recv() (*pb.ReadRelationshipsResponse, error) {
	ret, err := rrc.PermissionsService_ReadRelationshipsClient.Recv()
	if err != nil || rrc.cached {
		return ret, err
	}
	if ret.ReadAt != nil {
		if err := rrc.c.Set(rrc.recourceCacheKey, ret.ReadAt.Token); err != nil {
			level.Error(rrc.l).Log("msg", "failed to write cache entry", "err", err.Error())
		}
	}

	rrc.cached = true

	return ret, err
}

// ReadRelationships reads a set of the relationships matching one or more
// filters.
func (c *permissionClient) ReadRelationships(ctx context.Context, in *pb.ReadRelationshipsRequest, opts ...grpc.CallOption) (pb.PermissionsService_ReadRelationshipsClient, error) {
	if in.Consistency == nil {
		in.Consistency = &pb.Consistency{}
	}
	if in.Consistency.Requirement == nil {
		in.Consistency.Requirement = &pb.Consistency_FullyConsistent{FullyConsistent: true}

		// Not sure how to cache zedtoken if we the caller does not specify a resource id.
		if in.RelationshipFilter.OptionalResourceId != "" {
			t, err := c.ca.Get(fmt.Sprintf("%s#%s", in.RelationshipFilter.ResourceType, in.RelationshipFilter.OptionalResourceId))
			if err == nil {
				in.Consistency.Requirement = &pb.Consistency_AtLeastAsFresh{AtLeastAsFresh: &pb.ZedToken{Token: t}}
			}
		}

	}

	ret, err := c.PermissionsServiceClient.ReadRelationships(ctx, in, opts...)
	// Not sure how to cache zedtoken if we the caller does not specify a resource id.
	if err != nil || in.RelationshipFilter.OptionalResourceId == "" {
		return ret, err
	}

	return &permissionsService_ReadRelationshipsClient{
		PermissionsService_ReadRelationshipsClient: ret,
		c:                c.ca,
		recourceCacheKey: fmt.Sprintf("%s#%s", in.RelationshipFilter.ResourceType, in.RelationshipFilter.OptionalResourceId),
		l:                c.l,
	}, err
}

// CheckPermission determines for a given resource whether a subject computes
// to having a permission or is a direct member of a particular relation.
func (c *permissionClient) CheckPermission(ctx context.Context, in *pb.CheckPermissionRequest, opts ...grpc.CallOption) (*pb.CheckPermissionResponse, error) {
	if in.Consistency == nil {
		in.Consistency = &pb.Consistency{}
	}
	if in.Consistency.Requirement == nil {
		in.Consistency.Requirement = &pb.Consistency_FullyConsistent{FullyConsistent: true}

		t, err := c.ca.Get(sprintObjectReference(in.Resource))
		if err == nil {
			in.Consistency.Requirement = &pb.Consistency_AtLeastAsFresh{AtLeastAsFresh: &pb.ZedToken{Token: t}}
		}
	}
	ret, err := c.PermissionsServiceClient.CheckPermission(ctx, in, opts...)
	if err != nil {
		return ret, err
	}
	fmt.Println(sprintObjectReference(in.Resource))
	if err := c.ca.Set(sprintObjectReference(in.Resource), ret.CheckedAt.Token); err != nil {
		level.Error(c.l).Log("msg", "failed to write cache entry", "err", err.Error())
	}

	return ret, err
}

// ExpandPermissionTree reveals the graph structure for a resource's
// permission or relation. This RPC does not recurse infinitely deep and may
// require multiple calls to fully unnest a deeply nested graph.
func (c *permissionClient) ExpandPermissionTree(ctx context.Context, in *pb.ExpandPermissionTreeRequest, opts ...grpc.CallOption) (*pb.ExpandPermissionTreeResponse, error) {
	if in.Consistency == nil {
		in.Consistency = &pb.Consistency{}
	}
	if in.Consistency.Requirement == nil {
		in.Consistency.Requirement = &pb.Consistency_FullyConsistent{FullyConsistent: true}

		t, err := c.ca.Get(sprintObjectReference(in.Resource))
		if err == nil {
			in.Consistency.Requirement = &pb.Consistency_AtLeastAsFresh{AtLeastAsFresh: &pb.ZedToken{Token: t}}
		}
	}
	ret, err := c.PermissionsServiceClient.ExpandPermissionTree(ctx, in, opts...)
	if err != nil {
		return ret, err
	}

	if ret.ExpandedAt != nil {
		if err := c.ca.Set(sprintObjectReference(in.Resource), ret.ExpandedAt.Token); err != nil {
			level.Error(c.l).Log("msg", "failed to write cache entry", "err", err.Error())
		}
	}

	return ret, err
}

type permissionsService_LookupResourcesClient struct {
	pb.PermissionsService_LookupResourcesClient

	c                    cache.Cache
	parentObjectCacheKey string
	cached               bool
	l                    log.Logger
}

func (lrc *permissionsService_LookupResourcesClient) Recv() (*pb.LookupResourcesResponse, error) {
	ret, err := lrc.PermissionsService_LookupResourcesClient.Recv()
	if err != nil || lrc.cached {
		return ret, err
	}
	if ret.LookedUpAt != nil {
		if err := lrc.c.Set(lrc.parentObjectCacheKey, ret.LookedUpAt.Token); err != nil {
			level.Error(lrc.l).Log("msg", "failed to write cache entry", "err", err.Error())
		}
	}

	lrc.cached = true

	return ret, err
}

// LookupResources returns all the resources of a given type that a subject
// can access whether via a computed permission or relation membership.
func (c *permissionClient) LookupResources(ctx context.Context, in *pb.LookupResourcesRequest, opts ...grpc.CallOption) (pb.PermissionsService_LookupResourcesClient, error) {
	if in.Consistency == nil {
		in.Consistency = &pb.Consistency{}
	}
	if in.Consistency.Requirement == nil {
		in.Consistency.Requirement = &pb.Consistency_FullyConsistent{FullyConsistent: true}

		t, err := c.ca.Get(sprintSubjectReference(in.Subject))
		if err == nil {
			in.Consistency.Requirement = &pb.Consistency_AtLeastAsFresh{AtLeastAsFresh: &pb.ZedToken{Token: t}}
		}
	}
	ret, err := c.PermissionsServiceClient.LookupResources(ctx, in, opts...)
	if err != nil {
		return ret, err
	}

	nr := &permissionsService_LookupResourcesClient{
		PermissionsService_LookupResourcesClient: ret,
		c:                                        c.ca,
		parentObjectCacheKey:                     sprintSubjectReference(in.Subject),
		l:                                        c.l,
	}
	return nr, err
}

type permissionsService_LookupSubjectsClient struct {
	pb.PermissionsService_LookupSubjectsClient

	c                 cache.Cache
	parentResourceKey string
	cached            bool
	l                 log.Logger
}

func (lsc *permissionsService_LookupSubjectsClient) Recv() (*pb.LookupSubjectsResponse, error) {
	ret, err := lsc.PermissionsService_LookupSubjectsClient.Recv()
	if err != nil {
		return ret, err
	}
	// TODO: does it make sense to cache the token along the resource here?
	if !lsc.cached {
		if err := lsc.c.Set(lsc.parentResourceKey, ret.LookedUpAt.Token); err != nil {
			level.Error(lsc.l).Log("msg", "failed to write cache entry", "err", err.Error())
		}
		lsc.cached = true
	}

	return ret, err
}

// LookupSubjects returns all the subjects of a given type that
// have access whether via a computed permission or relation membership.
func (c *permissionClient) LookupSubjects(ctx context.Context, in *pb.LookupSubjectsRequest, opts ...grpc.CallOption) (pb.PermissionsService_LookupSubjectsClient, error) {
	if in.Consistency == nil {
		in.Consistency = &pb.Consistency{}
	}
	if in.Consistency.Requirement == nil {
		in.Consistency.Requirement = &pb.Consistency_FullyConsistent{FullyConsistent: true}

		t, err := c.ca.Get(sprintObjectReference(in.Resource))
		if err == nil {
			in.Consistency.Requirement = &pb.Consistency_AtLeastAsFresh{AtLeastAsFresh: &pb.ZedToken{Token: t}}
		}
	}
	ret, err := c.PermissionsServiceClient.LookupSubjects(ctx, in, opts...)
	if err != nil {
		return ret, err
	}

	return &permissionsService_LookupSubjectsClient{
		PermissionsService_LookupSubjectsClient: ret,
		c:                                       c.ca,
		parentResourceKey:                       sprintObjectReference(in.Resource),
		l:                                       c.l,
	}, err
}

// WriteRelationships atomically writes and/or deletes a set of specified
// relationships. An optional set of preconditions can be provided that must
// be satisfied for the operation to commit.
// The zedtoken will always be cached along the resource.
// This is not ideal in the case of adding or removing resources: https://authzed.com/docs/reference/zedtokens-and-zookies#when-adding-or-removing-a-resource
// The authors suggest to only save the zedtoken along the parent resource.
// However it is not clear how to determine whether a resource was added/removed or a relation was added/removed.
func (c *permissionClient) WriteRelationships(ctx context.Context, in *pb.WriteRelationshipsRequest, opts ...grpc.CallOption) (*pb.WriteRelationshipsResponse, error) {
	{
		// delete all relevant cached zed token to avoid the "New Enimy" problem.
		keys := make([]string, 2*len(in.Updates))
		for i, r := range in.Updates {
			keys[i] = sprintObjectReference(r.Relationship.Resource)
			keys[2*i] = sprintSubjectReference(r.Relationship.Subject)
		}
		if err := c.ca.Del(keys...); err != nil {
			return nil, fmt.Errorf("failed to clear cache: %w", err)
		}
	}
	res, err := c.PermissionsServiceClient.WriteRelationships(ctx, in, opts...)
	if err != nil {
		return res, err
	}

	g := multierror.Group{}
	for _, r := range in.Updates {
		rKey := sprintObjectReference(r.Relationship.Resource)
		fmt.Println(rKey)
		g.Go(func() error {
			return c.ca.Set(rKey, res.WrittenAt.Token)
		})
		sKey := sprintSubjectReference(r.Relationship.Subject)
		fmt.Println(sKey)
		g.Go(func() error {
			return c.ca.Set(sKey, res.WrittenAt.Token)
		})
	}
	// We don't need block for writing the updated valued to the cache here, because we already deleted the relevant cache entries.
	// But it would make the testing more difficult, so no async writes here at first.
	//	go func() {
	if err := g.Wait().ErrorOrNil(); err != nil {
		level.Error(c.l).Log("msg", "failed to write cache entry", "err", err.Error())
	}
	//	}()
	return res, nil
}

func sprintObjectReference(r *pb.ObjectReference) string {
	if r == nil {
		panic("can not print nil object reference")
	}

	return fmt.Sprintf("%s#%s", r.ObjectType, r.ObjectId)
}

func sprintSubjectReference(r *pb.SubjectReference) string {
	if r == nil || r.Object == nil {
		panic("can not print nil object reference")
	}

	return sprintObjectReference(r.Object)
}
