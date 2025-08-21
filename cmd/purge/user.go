package main

import (
	"context"
	"fmt"
	"strings"
)

func getAllUsersWithEmails(
	ctx context.Context,
	cfClient *cfResourceClient,
) (map[string]bool, error) {
	users, err := cfClient.Users.ListAll(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting users: %s", err.Error())
	}
	userGUIDs := map[string]bool{}
	for _, user := range users {
		if user.Username != nil && strings.Contains(*user.Username, "@") {
			userGUIDs[user.GUID] = true
		}
	}
	return userGUIDs, nil
}
