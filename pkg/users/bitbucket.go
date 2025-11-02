package users

import (
	"errors"
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/timpani-api/pkg/bitbucket"
	"github.com/tzrikka/timpani-api/pkg/jira"
)

// BitbucketToEmail converts a Bitbucket account ID into an email address.
// This depends on the user's email address being the same in both systems.
// This function uses data caching, and API calls as a fallback.
func BitbucketToEmail(ctx workflow.Context, accountID string) (string, error) {
	email, err := data.BitbucketUserEmailByID(accountID)
	if err != nil {
		log.Error(ctx, "failed to load Bitbucket user email", "error", err, "account_id", accountID)
		return "", err
	}

	if email != "" {
		return email, nil
	}

	// Fallback: lookup Bitbucket user in Jira to get their email address,
	// because Bitbucket API does not expose email addresses directly.
	jiraUser, err := jira.UsersGetActivity(ctx, accountID)
	if err != nil {
		log.Error(ctx, "failed to retrieve Jira user info", "error", err, "account_id", accountID)
		return "", err
	}

	email = jiraUser.Email
	if err := data.AddBitbucketUser(accountID, email); err != nil {
		log.Error(ctx, "failed to save Bitbucket account ID/email mapping", "error", err, "account_id", accountID, "email", email)
		// Don't abort - we have the email address, even if we failed to save it.
	}

	return email, nil
}

// BitbucketToSlackID converts a Bitbucket account ID into a Slack user ID.
// This depends on the user's email address being the same in both systems.
// This function returns an empty string if the account ID is not found.
func BitbucketToSlackID(ctx workflow.Context, accountID string, checkOptIn bool) string {
	email, err := BitbucketToEmail(ctx, accountID)
	if err != nil {
		return ""
	}

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

// BitbucketToSlackRef converts a Bitbucket account ID into a Slack user reference.
// This depends on the user's email address being the same in both systems.
// This function returns the Bitbucket display name if the user is not found in Slack.
func BitbucketToSlackRef(ctx workflow.Context, accountID, displayName string) string {
	id := BitbucketToSlackID(ctx, accountID, false)
	if id != "" {
		return fmt.Sprintf("<@%s>", id)
	}

	if displayName == "" {
		user, err := bitbucket.UsersGetActivity(ctx, accountID, "")
		if err != nil {
			log.Error(ctx, "failed to retrieve Bitbucket user info", "error", err, "account_id", accountID)
			return accountID // Fallback: return the original Bitbucket account ID.
		}

		displayName = user.DisplayName
	}

	return displayName
}

// EmailToBitbucketID retrieves a Bitbucket user's account ID based on their
// email address. This function uses data caching, and API calls as a fallback.
func EmailToBitbucketID(ctx workflow.Context, workspace, email string) (string, error) {
	if email == "" {
		return "", errors.New("empty email address")
	}

	id, err := data.BitbucketUserIDByEmail(email)
	if err != nil {
		log.Error(ctx, "failed to load Bitbucket account ID", "error", err, "email", email)
		// Don't abort - try to use the Bitbucket API as a fallback.
	}
	if id != "" {
		return id, nil
	}

	users, err := jira.UsersSearchActivity(ctx, email)
	if err != nil {
		log.Error(ctx, "failed to search Jira user by email", "error", err, "email", email)
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
