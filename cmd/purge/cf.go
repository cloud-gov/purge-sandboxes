package main

import (
	"context"

	"github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/cloudfoundry-community/go-cfclient/v3/config"
	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
)

type ApplicationsClient interface {
	Delete(ctx context.Context, guid string) (string, error)
	ListAll(ctx context.Context, opts *client.AppListOptions) ([]*resource.App, error)
}

type OrganizationsClient interface {
	ListAll(ctx context.Context, opts *client.OrganizationListOptions) ([]*resource.Organization, error)
	Single(ctx context.Context, opts *client.OrganizationListOptions) (*resource.Organization, error)
}

type RolesClient interface {
	CreateSpaceRole(ctx context.Context, spaceGUID, userGUID string, roleType resource.SpaceRoleType) (*resource.Role, error)
	ListIncludeUsersAll(ctx context.Context, opts *client.RoleListOptions) ([]*resource.Role, []*resource.User, error)
}

type ServiceInstancesClient interface {
	ListAll(ctx context.Context, opts *client.ServiceInstanceListOptions) ([]*resource.ServiceInstance, error)
}

type SpacesClient interface {
	ListAll(ctx context.Context, opts *client.SpaceListOptions) ([]*resource.Space, error)
	ListUsersAll(ctx context.Context, spaceGUID string, opts *client.UserListOptions) ([]*resource.User, error)
	Create(ctx context.Context, r *resource.SpaceCreate) (*resource.Space, error)
	Delete(ctx context.Context, guid string) (string, error)
	Single(ctx context.Context, opts *client.SpaceListOptions) (*resource.Space, error)
}

type SpaceQuotasClient interface {
	Single(ctx context.Context, opts *client.SpaceQuotaListOptions) (*resource.SpaceQuota, error)
	Apply(ctx context.Context, guid string, spaceGUIDs []string) ([]string, error)
}

type UsersClient interface {
	ListAll(ctx context.Context, opts *client.UserListOptions) ([]*resource.User, error)
}

type JobsClient interface {
	PollComplete(ctx context.Context, jobGUID string, opts *client.PollingOptions) error
}

type cfResourceClient struct {
	Applications     ApplicationsClient
	Organizations    OrganizationsClient
	Roles            RolesClient
	ServiceInstances ServiceInstancesClient
	Spaces           SpacesClient
	SpaceQuotas      SpaceQuotasClient
	Users            UsersClient
	Jobs             JobsClient
}

func newCFClient(
	cfApiUrl string,
	cfApiClientId string,
	cfApiClientSecret string,
) (*cfResourceClient, error) {
	cfg, err := config.NewClientSecret(
		cfApiUrl,
		cfApiClientId,
		cfApiClientSecret,
	)
	if err != nil {
		return nil, err
	}
	cf, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	return &cfResourceClient{
		Applications:     cf.Applications,
		Organizations:    cf.Organizations,
		Roles:            cf.Roles,
		ServiceInstances: cf.ServiceInstances,
		Spaces:           cf.Spaces,
		SpaceQuotas:      cf.SpaceQuotas,
		Users:            cf.Users,
		Jobs:             cf.Jobs,
	}, nil
}
