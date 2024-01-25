package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
)

type mockApplications struct {
	listAppsErr error
	apps        []*resource.App
	spaceGUID   string
}

func (a *mockApplications) ListAll(ctx context.Context, opts *client.AppListOptions) ([]*resource.App, error) {
	if a.listAppsErr != nil {
		return nil, a.listAppsErr
	}
	expectedOpts := &client.AppListOptions{
		SpaceGUIDs: client.Filter{
			Values: []string{a.spaceGUID},
		},
	}
	if !cmp.Equal(opts, expectedOpts) {
		return nil, fmt.Errorf(cmp.Diff(opts, expectedOpts))
	}
	return a.apps, nil
}

func (a *mockApplications) Delete(ctx context.Context, guid string) (string, error) {
	return "", nil
}

type mockRoles struct {
	listRolesErr error
	roles        []*resource.Role
	spaceGUID    string
}

func (r *mockRoles) CreateSpaceRole(ctx context.Context, spaceGUID, userGUID string, roleType resource.SpaceRoleType) (*resource.Role, error) {
	return nil, nil
}

func (r *mockRoles) ListAll(ctx context.Context, opts *client.RoleListOptions) ([]*resource.Role, error) {
	if r.listRolesErr != nil {
		return nil, r.listRolesErr
	}
	expectedOpts := &client.RoleListOptions{
		SpaceGUIDs: client.Filter{
			Values: []string{r.spaceGUID},
		},
		Include: resource.RoleIncludeUser,
	}
	if !cmp.Equal(opts, expectedOpts) {
		return nil, fmt.Errorf(cmp.Diff(opts, expectedOpts))
	}
	return r.roles, nil
}

type mockSpaces struct {
	listUsersAllErr error
	users           []*resource.User
	spaceGUID       string
}

func (s *mockSpaces) ListUsersAll(ctx context.Context, spaceGUID string, opts *client.UserListOptions) ([]*resource.User, error) {
	if s.listUsersAllErr != nil {
		return nil, s.listUsersAllErr
	}
	if spaceGUID != s.spaceGUID {
		return nil, fmt.Errorf("expected %s, got %s", spaceGUID, s.spaceGUID)
	}
	return s.users, nil
}

func (s *mockSpaces) ListAll(ctx context.Context, opts *client.SpaceListOptions) ([]*resource.Space, error) {
	return nil, nil
}

func (s *mockSpaces) Create(ctx context.Context, r *resource.SpaceCreate) (*resource.Space, error) {
	return nil, nil
}

func (s *mockSpaces) Delete(ctx context.Context, guid string) (string, error) {
	return "", nil
}

type mockMailSender struct{}

func (m *mockMailSender) sendMail(
	opts SMTPOptions,
	sender string,
	subject string,
	body string,
	recipients []string,
) error {
	return nil
}

func TestPurgeAndRecreateSpace(t *testing.T) {
	userGUIDs := map[string]bool{
		"user-1": true,
	}

	err := purgeAndRecreateSpace(
		context.Background(),
		&cfResourceClient{
			Applications: &mockApplications{},
			Roles: &mockRoles{
				spaceGUID: "space-1",
				roles: []*resource.Role{
					{
						Type: resource.SpaceRoleManager.String(),
						Relationships: resource.RoleSpaceUserOrganizationRelationships{
							Space: resource.ToOneRelationship{
								Data: &resource.Relationship{
									GUID: "space-1",
								},
							},
							User: resource.ToOneRelationship{
								Data: &resource.Relationship{
									GUID: "user-1",
								},
							},
						},
					},
				},
			},
			Spaces: &mockSpaces{
				spaceGUID: "space-1",
				users: []*resource.User{
					{
						GUID:     "user-1",
						Username: "foo@bar.gov",
					},
				},
			},
		},
		Options{
			DryRun: false,
		},
		userGUIDs,
		&resource.Organization{
			GUID: "org-1",
		},
		SpaceDetails{
			Space: &resource.Space{
				GUID: "space-1",
			},
		},
		&mockMailSender{},
	)

	if err != nil {
		t.Fatal(err)
	}
}
