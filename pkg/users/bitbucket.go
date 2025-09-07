package users

import (
	"fmt"

	"github.com/urfave/cli/v3"
	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
)

// BitbucketToSlackRef converts a Bitbucket account ID into a Slack user reference.
// This depends on the user's email address being the same in both systems.
// This function returns the Bitbucket display name if the user is not found in Slack.
func BitbucketToSlackRef(ctx workflow.Context, cmd *cli.Command, accountID, displayName string) string {
	id := BitbucketToSlackID(ctx, cmd, accountID, false)
	if id != "" {
		return fmt.Sprintf("<@%s>", id)
	}

	if displayName == "" {
		displayName = "Display Name"
	}

	return displayName
}

// BitbucketToSlackID converts a Bitbucket account ID into a Slack user ID.
// This depends on the user's email address being the same in both systems.
// This function returns an empty string if the account ID is not found.
func BitbucketToSlackID(ctx workflow.Context, cmd *cli.Command, accountID string, checkOptIn bool) string {
	email, err := data.BitbucketUserEmailByID(accountID)
	if err != nil {
		log.Error(ctx, "failed to load Bitbucket user email", "error", err, "account_id", accountID)
		return ""
	}

	// Note: unlike GitHub, we can't see user emails unless we know them in advance.

	if email == "" || email == "bot" {
		return ""
	}

	if checkOptIn {
		optedIn, err := data.IsOptedIn(email)
		if err != nil {
			log.Error(ctx, "failed to load user opt-in status", "error", err, "email", email)
			return ""
		}
		if !optedIn {
			return ""
		}
	}

	return EmailToSlackID(ctx, email)
}

// EmailToBitbucketID retrieves a Bitbucket user's account ID based on their
// email address. This function uses data caching, and API calls as a fallback.
func EmailToBitbucketID(ctx workflow.Context, workspace, email string) (string, error) {
	id, err := data.BitbucketUserIDByEmail(email)
	if err != nil {
		log.Error(ctx, "failed to load Bitbucket account ID", "error", err, "email", email)
		// Don't abort - try to use the Bitbucket API as a fallback.
	}
	if id != "" {
		return id, nil
	}

	users, err := bitbucket.WorkspacesListMembersActivity(ctx, workspace, []string{email})
	if err != nil {
		log.Error(ctx, "failed to search Bitbucket user by email", "error", err, "email", email)
		return "", err
	}
	if len(users) == 0 {
		log.Error(ctx, "Bitbucket user not found", "email", email)
		return "", fmt.Errorf("bitbucket user account not found for %q", email)
	}
	if len(users) > 1 {
		log.Warn(ctx, "multiple Bitbucket users found", "email", email, "count", len(users))
		return "", fmt.Errorf("multiple (%d) Bitbucket accounts found for %q", len(users), email)
	}

	id = users[0].AccountID
	if err := data.AddBitbucketUser(users[0].AccountID, email); err != nil {
		log.Error(ctx, "failed to save Bitbucket account ID/email mapping", "error", err, "account_id", id, "email", email)
	}

	return id, nil
}
