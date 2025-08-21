package main

import (
	"context"
	"fmt"
	"strings"
)

func getValidUsersMap(
	ctx context.Context,
	cfClient *cfResourceClient,
) (map[string]bool, error) {
	users, err := cfClient.Users.ListAll(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting users: %s", err.Error())
	}
	validUsersMap := map[string]bool{}
	for _, user := range users {
		if user.Username != nil && strings.Contains(*user.Username, "@") {
			validUsersMap[user.GUID] = true
		}
	}
	return validUsersMap, nil
}
