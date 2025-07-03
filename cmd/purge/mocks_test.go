package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
)

type mockApplications struct {
	listAppsErr     error
	apps            []*resource.App
	deleteCallCount int
	deleteErr       error
}

func (a *mockApplications) ListAll(ctx context.Context, opts *client.AppListOptions) ([]*resource.App, error) {
	return a.apps, a.listAppsErr
}

func (a *mockApplications) Delete(ctx context.Context, guid string) (string, error) {
	a.deleteCallCount += 1
	return "", a.deleteErr
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
		return nil, nil, errors.New(cmp.Diff(opts, expectedOpts))
	}
	return r.roles, r.users, nil
}

type mockSpaces struct {
	listUsersAllErr            error
	users                      []*resource.User
	spaceGUID                  string
	expectedSpaceCreateRequest *resource.SpaceCreate
	space                      *resource.Space
	deleteJobGUID              string
	deleteErr                  error
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
	return s.deleteJobGUID, s.deleteErr
}

func (s *mockSpaces) Single(ctx context.Context, opts *client.SpaceListOptions) (*resource.Space, error) {
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

type mockJobs struct {
	expectedJobGUID string
	pollErr         error
}

func (j *mockJobs) PollComplete(ctx context.Context, jobGUID string, opts *client.PollingOptions) error {
	if j.expectedJobGUID != jobGUID {
		return fmt.Errorf("expected job GUID: %s, received: %s", j.expectedJobGUID, jobGUID)
	}
	return j.pollErr
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

type mockServiceInstances struct {
	listAllServiceInstances     []*resource.ServiceInstance
	deleteServiceInstanceErr    error
	getServiceInstanceErr       error
	getServiceInstanceCallCount int
}

func (s *mockServiceInstances) ListAll(ctx context.Context, opts *client.ServiceInstanceListOptions) ([]*resource.ServiceInstance, error) {
	return s.listAllServiceInstances, nil
}

func (s *mockServiceInstances) Get(ctx context.Context, guid string) (*resource.ServiceInstance, error) {
	s.getServiceInstanceCallCount++
	return nil, s.getServiceInstanceErr
}

func (s *mockServiceInstances) Delete(ctx context.Context, guid string) (string, error) {
	return "", s.deleteServiceInstanceErr
}
