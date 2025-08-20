package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestWaitForSpaceDeletion(t *testing.T) {
	pollErr := errors.New("polling error")
	testCases := map[string]struct {
		cfClient      *cfResourceClient
		deleteJobGUID string
		expectedErr   error
	}{
		"success": {
			cfClient: &cfResourceClient{
				Jobs: &mockJobs{
					expectedJobGUID: "delete-1",
				},
			},
			deleteJobGUID: "delete-1",
		},
		"no job GUID": {
			cfClient: &cfResourceClient{
				Jobs: &mockJobs{},
			},
			expectedErr: ErrNoSpaceDeleteJobGUID,
		},
		"error": {
			cfClient: &cfResourceClient{
				Jobs: &mockJobs{
					pollErr:         pollErr,
					expectedJobGUID: "delete-1",
				},
			},
			deleteJobGUID: "delete-1",
			expectedErr:   pollErr,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			err := waitForSpaceDeletion(
				context.Background(),
				test.cfClient,
				test.deleteJobGUID,
			)

			if !errors.Is(err, test.expectedErr) {
				t.Fatal(err)
			}
		})
	}
}

func TestPurgeAndRecreateSpace(t *testing.T) {
	email := "foo@bar.gov"
	email2 := "foo2@bar.gov"

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
							Resource: resource.Resource{
								GUID: "user-1",
							},
							Username: &email,
						},
					},
				},
				Spaces: &mockSpaces{
					spaceGUID: "space-1-guid",
					users: []*resource.User{
						{
							Resource: resource.Resource{
								GUID: "user-1",
							},
							Username: &email,
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
						Resource: resource.Resource{
							GUID: "new-space-1-guid",
						},
						Name: "space-1",
					},
					deleteJobGUID: "delete-space-1",
				},
				SpaceQuotas: &mockSpaceQuotas{
					orgGUID:        "org-1",
					spaceQuotaName: "quota-1",
					quota: &resource.SpaceQuota{
						Resource: resource.Resource{
							GUID: "quota-guid-1",
						},
					},
				},
				Jobs: &mockJobs{
					expectedJobGUID: "delete-space-1",
				},
				ServiceInstances: &mockServiceInstances{},
			},
			userGUIDs: map[string]bool{
				"user-1": true,
			},
			options: Options{
				DryRun:           false,
				SandboxQuotaName: "quota-1",
			},
			organization: &resource.Organization{
				Resource: resource.Resource{
					GUID: "org-1",
				},
			},
			spaceDetails: SpaceDetails{
				Space: &resource.Space{
					Resource: resource.Resource{
						GUID: "space-1-guid",
					},
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
							Resource: resource.Resource{
								GUID: "user-1",
							},
							Username: &email,
						},
						{
							Resource: resource.Resource{
								GUID: "user-2",
							},
							Username: &email2,
						},
					},
				},
				Spaces: &mockSpaces{
					spaceGUID: "space-1-guid",
					users: []*resource.User{
						{
							Resource: resource.Resource{
								GUID: "user-1",
							},
							Username: &email,
						},
						{
							Resource: resource.Resource{
								GUID: "user-2",
							},
							Username: &email2,
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
						Resource: resource.Resource{
							GUID: "new-space-1-guid",
						},
						Name: "space-1",
					},
					deleteJobGUID: "space-delete-1",
				},
				SpaceQuotas: &mockSpaceQuotas{
					orgGUID:        "org-1",
					spaceQuotaName: "quota-1",
					quota: &resource.SpaceQuota{
						Resource: resource.Resource{
							GUID: "quota-guid-1",
						},
					},
				},
				Jobs: &mockJobs{
					expectedJobGUID: "space-delete-1",
				},
				ServiceInstances: &mockServiceInstances{},
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
				Resource: resource.Resource{
					GUID: "org-1",
				},
			},
			spaceDetails: SpaceDetails{
				Space: &resource.Space{
					Resource: resource.Resource{
						GUID: "space-1-guid",
					},
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
							Resource: resource.Resource{
								GUID: "user-1",
							},
							Username: &email,
						},
						{
							Resource: resource.Resource{
								GUID: "user-2",
							},
							Username: &email2,
						},
					},
				},
				Spaces: &mockSpaces{
					spaceGUID: "space-1-guid",
					users: []*resource.User{
						{
							Resource: resource.Resource{
								GUID: "user-1",
							},
							Username: &email,
						},
						{
							Resource: resource.Resource{
								GUID: "user-2",
							},
							Username: &email2,
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
						Resource: resource.Resource{
							GUID: "new-space-1-guid",
						},
						Name: "space-1",
					},
					deleteJobGUID: "space-delete-1",
				},
				SpaceQuotas: &mockSpaceQuotas{
					spaceQuotaName: "quota-1",
					orgGUID:        "org-1",
					quota: &resource.SpaceQuota{
						Name: "quota-1",
						Resource: resource.Resource{
							GUID: "quota-guid-1",
						},
					},
				},
				Jobs: &mockJobs{
					expectedJobGUID: "space-delete-1",
				},
				ServiceInstances: &mockServiceInstances{},
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
				Resource: resource.Resource{
					GUID: "org-1",
				},
			},
			spaceDetails: SpaceDetails{
				Space: &resource.Space{
					Resource: resource.Resource{
						GUID: "space-1-guid",
					},
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
