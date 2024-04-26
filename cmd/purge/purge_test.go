package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

type spaceCreatedRole struct {
	SpaceGUID string
	UserGUID  string
	RoleType  resource.SpaceRoleType
}

type mockRoles struct {
	listRolesErr      error
	roles             []*resource.Role
	spaceGUID         string
	users             []*resource.User
	createdSpaceRoles []spaceCreatedRole
}

func (r *mockRoles) CreateSpaceRole(ctx context.Context, spaceGUID, userGUID string, roleType resource.SpaceRoleType) (*resource.Role, error) {
	r.createdSpaceRoles = append(r.createdSpaceRoles, spaceCreatedRole{
		SpaceGUID: spaceGUID,
		UserGUID:  userGUID,
		RoleType:  roleType,
	})
	return nil, nil
}

func (r *mockRoles) ListIncludeUsersAll(ctx context.Context, opts *client.RoleListOptions) ([]*resource.Role, []*resource.User, error) {
	if r.listRolesErr != nil {
		return nil, nil, r.listRolesErr
	}
	expectedOpts := &client.RoleListOptions{
		SpaceGUIDs: client.Filter{
			Values: []string{r.spaceGUID},
		},
	}
	if !cmp.Equal(opts.SpaceGUIDs, expectedOpts.SpaceGUIDs) {
		return nil, nil, fmt.Errorf(cmp.Diff(opts, expectedOpts))
	}
	return r.roles, r.users, nil
}

type mockSpaceSingleReturnValues struct {
	space *resource.Space
	err   error
}

type mockSpaces struct {
	listUsersAllErr            error
	users                      []*resource.User
	spaceGUID                  string
	expectedSpaceCreateRequest *resource.SpaceCreate
	space                      *resource.Space
	singleCallCount            int
	mockSingleReturnValues     map[int]mockSpaceSingleReturnValues
	singleErr                  error
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
	if !cmp.Equal(r, s.expectedSpaceCreateRequest) {
		return nil, fmt.Errorf("expected creation params do not match: %s", cmp.Diff(r, s.expectedSpaceCreateRequest))
	}
	return s.space, nil
}

func (s *mockSpaces) Delete(ctx context.Context, guid string) (string, error) {
	return "", nil
}

func (s *mockSpaces) Single(ctx context.Context, opts *client.SpaceListOptions) (*resource.Space, error) {
	s.singleCallCount += 1
	if s.singleErr != nil {
		return nil, s.singleErr
	}
	if s.mockSingleReturnValues != nil {
		return s.mockSingleReturnValues[s.singleCallCount].space, s.mockSingleReturnValues[s.singleCallCount].err
	}
	return nil, nil
}

type mockSpaceQuotas struct {
	spaceQuotaName string
	orgGUID        string
	quota          *resource.SpaceQuota
}

func (q *mockSpaceQuotas) Single(ctx context.Context, opts *client.SpaceQuotaListOptions) (*resource.SpaceQuota, error) {
	expectedOptions := client.NewSpaceQuotaListOptions()
	if q.spaceQuotaName != "" {
		expectedOptions.Names.EqualTo(q.spaceQuotaName)
	}
	if q.orgGUID != "" {
		expectedOptions.OrganizationGUIDs.EqualTo(q.orgGUID)
	}
	if !cmp.Equal(opts, expectedOptions) {
		return nil, fmt.Errorf(cmp.Diff(opts, expectedOptions))
	}
	return q.quota, nil
}

func (q *mockSpaceQuotas) Apply(ctx context.Context, guid string, spaceGUIDs []string) ([]string, error) {
	return []string{}, nil
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

func TestWaitUntilSpaceIsFullyDeleted(t *testing.T) {
	singleErr := errors.New("single error")
	testCases := map[string]struct {
		cfClient                *cfResourceClient
		organization            *resource.Organization
		spaceName               string
		expectedSingleCallCount int
		expectedErr             error
		maxAttempts             int
	}{
		"success": {
			cfClient: &cfResourceClient{
				// These mocks are defined elsewhere
				Spaces: &mockSpaces{},
			},
			organization: &resource.Organization{
				GUID: "org-guid-1",
			},
			spaceName:               "space-1",
			expectedSingleCallCount: 1,
			maxAttempts:             3,
		},
		"success with retry": {
			cfClient: &cfResourceClient{
				// These mocks are defined elsewhere
				Spaces: &mockSpaces{
					mockSingleReturnValues: map[int]mockSpaceSingleReturnValues{
						1: {
							space: &resource.Space{GUID: "space-guid-1"},
							err:   nil,
						},
						2: {
							space: nil,
							err:   nil,
						},
					},
				},
			},
			organization: &resource.Organization{
				GUID: "org-guid-1",
			},
			spaceName:               "space-1",
			expectedSingleCallCount: 2,
			maxAttempts:             3,
		},
		"error": {
			cfClient: &cfResourceClient{
				// These mocks are defined elsewhere
				Spaces: &mockSpaces{
					singleErr: singleErr,
				},
			},
			organization: &resource.Organization{
				GUID: "org-guid-1",
			},
			spaceName:               "space-1",
			expectedSingleCallCount: 1,
			expectedErr:             singleErr,
			maxAttempts:             3,
		},
		"error on retry": {
			cfClient: &cfResourceClient{
				// These mocks are defined elsewhere
				Spaces: &mockSpaces{
					mockSingleReturnValues: map[int]mockSpaceSingleReturnValues{
						1: {
							space: &resource.Space{GUID: "space-guid-1"},
							err:   nil,
						},
						2: {
							space: nil,
							err:   singleErr,
						},
					},
				},
			},
			organization: &resource.Organization{
				GUID: "org-guid-1",
			},
			spaceName:               "space-1",
			expectedSingleCallCount: 2,
			expectedErr:             singleErr,
			maxAttempts:             3,
		},
		"gives up after maximum allowed retries": {
			cfClient: &cfResourceClient{
				// These mocks are defined elsewhere
				Spaces: &mockSpaces{
					mockSingleReturnValues: map[int]mockSpaceSingleReturnValues{
						1: {
							space: &resource.Space{GUID: "space-guid-1"},
							err:   nil,
						},
						2: {
							space: &resource.Space{GUID: "space-guid-1"},
							err:   nil,
						},
						3: {
							space: &resource.Space{GUID: "space-guid-1"},
							err:   nil,
						},
					},
				},
			},
			organization: &resource.Organization{
				GUID: "org-guid-1",
			},
			spaceName:               "space-1",
			expectedSingleCallCount: 3,
			expectedErr:             ErrMaximumAttemptsReached,
			maxAttempts:             3,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			err := waitUntilSpaceIsFullyDeleted(
				context.Background(),
				test.cfClient,
				test.organization,
				test.spaceName,
				test.maxAttempts,
			)

			if !errors.Is(err, test.expectedErr) {
				t.Fatalf(
					"expected err %s, got %s",
					test.expectedErr,
					err,
				)
			}

			if spacesClient, ok := test.cfClient.Spaces.(*mockSpaces); ok {
				if test.expectedSingleCallCount != spacesClient.singleCallCount {
					t.Fatalf(
						"expected %d calls to Spaces.Single(), got %d",
						test.expectedSingleCallCount,
						spacesClient.singleCallCount,
					)
				}
			}
		})
	}
}

func TestPurgeAndRecreateSpace(t *testing.T) {
	testCases := map[string]struct {
		cfClient                *cfResourceClient
		userGUIDs               map[string]bool
		options                 Options
		organization            *resource.Organization
		spaceDetails            SpaceDetails
		expectSpaceCreatedRoles []spaceCreatedRole
	}{
		"success with one org manager": {
			cfClient: &cfResourceClient{
				Applications: &mockApplications{},
				Roles: &mockRoles{
					spaceGUID: "space-1-guid",
					roles: []*resource.Role{
						{
							Type: resource.SpaceRoleManager.String(),
							Relationships: resource.RoleSpaceUserOrganizationRelationships{
								Space: resource.ToOneRelationship{
									Data: &resource.Relationship{
										GUID: "space-1-guid",
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
					users: []*resource.User{
						{
							GUID:     "user-1",
							Username: "foo@bar.gov",
						},
					},
				},
				Spaces: &mockSpaces{
					spaceGUID: "space-1-guid",
					users: []*resource.User{
						{
							GUID:     "user-1",
							Username: "foo@bar.gov",
						},
					},
					expectedSpaceCreateRequest: &resource.SpaceCreate{
						Name: "space-1",
						Relationships: &resource.SpaceRelationships{
							Organization: &resource.ToOneRelationship{
								Data: &resource.Relationship{
									GUID: "org-1",
								},
							},
						},
					},
					space: &resource.Space{
						GUID: "new-space-1-guid",
						Name: "space-1",
					},
				},
				SpaceQuotas: &mockSpaceQuotas{
					orgGUID:        "org-1",
					spaceQuotaName: "quota-1",
					quota: &resource.SpaceQuota{
						GUID: "quota-guid-1",
					},
				},
			},
			userGUIDs: map[string]bool{
				"user-1": true,
			},
			options: Options{
				DryRun:           false,
				SandboxQuotaName: "quota-1",
			},
			organization: &resource.Organization{
				GUID: "org-1",
			},
			spaceDetails: SpaceDetails{
				Space: &resource.Space{
					GUID: "space-1-guid",
					Name: "space-1",
					Relationships: &resource.SpaceRelationships{
						Organization: &resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "org-1",
							},
						},
					},
				},
			},
			expectSpaceCreatedRoles: []spaceCreatedRole{
				{
					SpaceGUID: "new-space-1-guid",
					UserGUID:  "user-1",
					RoleType:  resource.SpaceRoleManager,
				},
			},
		},
		"success with one org manager and one dev": {
			cfClient: &cfResourceClient{
				Applications: &mockApplications{},
				Roles: &mockRoles{
					spaceGUID: "space-1-guid",
					roles: []*resource.Role{
						{
							Type: resource.SpaceRoleManager.String(),
							Relationships: resource.RoleSpaceUserOrganizationRelationships{
								Space: resource.ToOneRelationship{
									Data: &resource.Relationship{
										GUID: "space-1-guid",
									},
								},
								User: resource.ToOneRelationship{
									Data: &resource.Relationship{
										GUID: "user-1",
									},
								},
							},
						},
						{
							Type: resource.SpaceRoleDeveloper.String(),
							Relationships: resource.RoleSpaceUserOrganizationRelationships{
								Space: resource.ToOneRelationship{
									Data: &resource.Relationship{
										GUID: "space-1-guid",
									},
								},
								User: resource.ToOneRelationship{
									Data: &resource.Relationship{
										GUID: "user-2",
									},
								},
							},
						},
					},
					users: []*resource.User{
						{
							GUID:     "user-1",
							Username: "foo@bar.gov",
						},
						{
							GUID:     "user-2",
							Username: "foo2@bar.gov",
						},
					},
				},
				Spaces: &mockSpaces{
					spaceGUID: "space-1-guid",
					users: []*resource.User{
						{
							GUID:     "user-1",
							Username: "foo@bar.gov",
						},
						{
							GUID:     "user-2",
							Username: "foo2@bar.gov",
						},
					},
					expectedSpaceCreateRequest: &resource.SpaceCreate{
						Name: "space-1",
						Relationships: &resource.SpaceRelationships{
							Organization: &resource.ToOneRelationship{
								Data: &resource.Relationship{
									GUID: "org-1",
								},
							},
						},
					},
					space: &resource.Space{
						GUID: "new-space-1-guid",
						Name: "space-1",
					},
				},
				SpaceQuotas: &mockSpaceQuotas{
					orgGUID:        "org-1",
					spaceQuotaName: "quota-1",
					quota: &resource.SpaceQuota{
						GUID: "quota-guid-1",
					},
				},
			},
			userGUIDs: map[string]bool{
				"user-1": true,
				"user-2": true,
			},
			options: Options{
				DryRun:           false,
				SandboxQuotaName: "quota-1",
			},
			organization: &resource.Organization{
				GUID: "org-1",
			},
			spaceDetails: SpaceDetails{
				Space: &resource.Space{
					GUID: "space-1-guid",
					Name: "space-1",
					Relationships: &resource.SpaceRelationships{
						Organization: &resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "org-1",
							},
						},
					},
				},
			},
			expectSpaceCreatedRoles: []spaceCreatedRole{
				{
					SpaceGUID: "new-space-1-guid",
					UserGUID:  "user-1",
					RoleType:  resource.SpaceRoleManager,
				},
				{
					SpaceGUID: "new-space-1-guid",
					UserGUID:  "user-2",
					RoleType:  resource.SpaceRoleDeveloper,
				},
			},
		},
		"success with space quota found": {
			cfClient: &cfResourceClient{
				Applications: &mockApplications{},
				Roles: &mockRoles{
					spaceGUID: "space-1-guid",
					roles: []*resource.Role{
						{
							Type: resource.SpaceRoleManager.String(),
							Relationships: resource.RoleSpaceUserOrganizationRelationships{
								Space: resource.ToOneRelationship{
									Data: &resource.Relationship{
										GUID: "space-1-guid",
									},
								},
								User: resource.ToOneRelationship{
									Data: &resource.Relationship{
										GUID: "user-1",
									},
								},
							},
						},
						{
							Type: resource.SpaceRoleDeveloper.String(),
							Relationships: resource.RoleSpaceUserOrganizationRelationships{
								Space: resource.ToOneRelationship{
									Data: &resource.Relationship{
										GUID: "space-1-guid",
									},
								},
								User: resource.ToOneRelationship{
									Data: &resource.Relationship{
										GUID: "user-2",
									},
								},
							},
						},
					},
					users: []*resource.User{
						{
							GUID:     "user-1",
							Username: "foo@bar.gov",
						},
						{
							GUID:     "user-2",
							Username: "foo2@bar.gov",
						},
					},
				},
				Spaces: &mockSpaces{
					spaceGUID: "space-1-guid",
					users: []*resource.User{
						{
							GUID:     "user-1",
							Username: "foo@bar.gov",
						},
						{
							GUID:     "user-2",
							Username: "foo2@bar.gov",
						},
					},
					expectedSpaceCreateRequest: &resource.SpaceCreate{
						Name: "space-1",
						Relationships: &resource.SpaceRelationships{
							Organization: &resource.ToOneRelationship{
								Data: &resource.Relationship{
									GUID: "org-1",
								},
							},
						},
					},
					space: &resource.Space{
						GUID: "new-space-1-guid",
						Name: "space-1",
					},
				},
				SpaceQuotas: &mockSpaceQuotas{
					spaceQuotaName: "quota-1",
					orgGUID:        "org-1",
					quota: &resource.SpaceQuota{
						Name: "quota-1",
						GUID: "quota-guid-1",
					},
				},
			},
			userGUIDs: map[string]bool{
				"user-1": true,
				"user-2": true,
			},
			options: Options{
				DryRun:           false,
				SandboxQuotaName: "quota-1",
			},
			organization: &resource.Organization{
				GUID: "org-1",
			},
			spaceDetails: SpaceDetails{
				Space: &resource.Space{
					GUID: "space-1-guid",
					Name: "space-1",
					Relationships: &resource.SpaceRelationships{
						Organization: &resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "org-1",
							},
						},
						Quota: &resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "quota-1-guid",
							},
						},
					},
				},
			},
			expectSpaceCreatedRoles: []spaceCreatedRole{
				{
					SpaceGUID: "new-space-1-guid",
					UserGUID:  "user-1",
					RoleType:  resource.SpaceRoleManager,
				},
				{
					SpaceGUID: "new-space-1-guid",
					UserGUID:  "user-2",
					RoleType:  resource.SpaceRoleDeveloper,
				},
			},
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			err := purgeAndRecreateSpace(
				context.Background(),
				test.cfClient,
				test.options,
				test.userGUIDs,
				test.organization,
				test.spaceDetails,
				&mockMailSender{},
			)

			if err != nil {
				t.Fatal(err)
			}

			if mockRolesClient, ok := test.cfClient.Roles.(*mockRoles); ok {
				if !cmp.Equal(
					mockRolesClient.createdSpaceRoles,
					test.expectSpaceCreatedRoles,
					cmpopts.SortSlices(func(a spaceCreatedRole, b spaceCreatedRole) bool { return a.UserGUID < b.UserGUID }),
				) {
					t.Fatal(fmt.Errorf(cmp.Diff(mockRolesClient.createdSpaceRoles, test.expectSpaceCreatedRoles)))
				}
			}
		})
	}
}
