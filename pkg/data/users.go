package data

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/cache"
	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data/internal"
)

type User = internal.User

var usersCache = cache.New[User](10*time.Minute, cache.DefaultCleanupInterval)

func UpsertUser(ctx workflow.Context, email, realName, bitbucketID, githubID, slackID, thrippyLink string) error {
	email = strings.ToLower(email)

	if ctx == nil { // For unit tests.
		_, err := internal.UpsertUser(context.Background(), email, realName, bitbucketID, githubID, slackID, thrippyLink) //workflowcheck:ignore
		if err != nil {
			return err
		}
		return nil
	}

	var user User
	err := executeLocalActivity(ctx, internal.UpsertUser, &user, email, realName, bitbucketID, githubID, slackID, thrippyLink)
	if err != nil {
		logger.From(ctx).Error("failed to upsert user data", slog.Any("error", err),
			slog.String("email", email), slog.String("real_name", realName),
			slog.String("bitbucket_id", bitbucketID), slog.String("github_id", githubID),
			slog.String("slack_id", slackID), slog.String("thrippy_link", thrippyLink))
		return fmt.Errorf("failed to upsert user data: %w", err)
	}

	// Now that the user is fully updated and persisted, also cache the new version.
	if email != "" && email != "bot" {
		usersCache.Set(email, user, cache.DefaultExpiration)
	}
	if bitbucketID != "" {
		usersCache.Set(bitbucketID, user, cache.DefaultExpiration)
	}
	if githubID != "" {
		usersCache.Set(githubID, user, cache.DefaultExpiration)
	}
	if slackID != "" {
		usersCache.Set(slackID, user, cache.DefaultExpiration)
	}

	return nil
}

func FollowUser(ctx workflow.Context, followerSlackID, followedSlackID string) bool {
	return followOrUnfollowUser(ctx, internal.FollowUser, followerSlackID, followedSlackID)
}

func UnfollowUser(ctx workflow.Context, followerSlackID, followedSlackID string) bool {
	return followOrUnfollowUser(ctx, internal.UnfollowUser, followerSlackID, followedSlackID)
}

type followUnfollowFunc func(context.Context, string, string) (User, error)

func followOrUnfollowUser(ctx workflow.Context, fn followUnfollowFunc, followerSlackID, followedSlackID string) bool {
	if ctx == nil { // For unit tests.
		_, err := fn(context.Background(), followerSlackID, followedSlackID) //workflowcheck:ignore
		return err == nil
	}

	var user User
	if err := executeLocalActivity(ctx, fn, &user, followerSlackID, followedSlackID); err != nil {
		logger.From(ctx).Error("failed to un/follow user", slog.Any("error", err),
			slog.String("follower_id", followerSlackID), slog.String("followed_id", followedSlackID))
		return false
	}

	// Now that the user is fully updated and persisted, also cache the new version.
	if user.Email != "" && user.Email != "bot" {
		usersCache.Set(user.Email, user, cache.DefaultExpiration)
	}
	if user.BitbucketID != "" {
		usersCache.Set(user.BitbucketID, user, cache.DefaultExpiration)
	}
	if user.GitHubID != "" {
		usersCache.Set(user.GitHubID, user, cache.DefaultExpiration)
	}
	if user.SlackID != "" {
		usersCache.Set(user.SlackID, user, cache.DefaultExpiration)
	}

	return true
}

func RemoveFollower(ctx workflow.Context, followerSlackID string) {
	if ctx == nil { // For unit tests.
		_ = internal.RemoveFollower(context.Background(), followerSlackID) //workflowcheck:ignore
		return
	}

	if err := executeLocalActivity(ctx, internal.RemoveFollower, nil, followerSlackID); err != nil {
		logger.From(ctx).Error("failed to remove follower", slog.Any("error", err),
			slog.String("slack_id", followerSlackID))
	}
}

func SelectUserByBitbucketID(ctx workflow.Context, accountID string) User {
	user, err := selectUser(ctx, internal.IndexByBitbucketID, accountID, true)
	if err != nil {
		logger.From(ctx).Warn("unexpected but not critical: failed to read user data by Bitbucket ID",
			slog.Any("error", err), slog.String("account_id", accountID))
		return User{}
	}
	return user
}

func SelectUserByEmail(ctx workflow.Context, email string) User {
	email = strings.ToLower(email)
	if email == "bot" {
		return User{}
	}

	user, err := selectUser(ctx, internal.IndexByEmail, email, true)
	if err != nil {
		logger.From(ctx).Error("failed to read user data by email",
			slog.Any("error", err), slog.String("email", email))
		return User{}
	}
	return user
}

func SelectUserByGitHubID(ctx workflow.Context, login string) User {
	user, err := selectUser(ctx, internal.IndexByGitHubID, login, true)
	if err != nil {
		logger.From(ctx).Error("failed to read user data by GitHub ID",
			slog.Any("error", err), slog.String("login", login))
		return User{}
	}
	return user
}

func SelectUserByRealName(ctx workflow.Context, realName string) User {
	user, err := selectUser(ctx, internal.IndexByRealName, realName, false)
	if err != nil {
		logger.From(ctx).Error("failed to read user data by real name",
			slog.Any("error", err), slog.String("real_name", realName))
		return User{}
	}
	return user
}

func SelectUserBySlackID(ctx workflow.Context, userID string) (User, bool, error) {
	user, err := selectUser(ctx, internal.IndexBySlackID, userID, true)
	if err != nil {
		logger.From(ctx).Error("failed to read user data by Slack ID",
			slog.Any("error", err), slog.String("slack_id", userID))
		return User{}, false, err
	}
	return user, user.IsOptedIn(), nil
}

func selectUser(ctx workflow.Context, indexType int, id string, useCache bool) (User, error) {
	if id == "" {
		return User{}, nil
	}

	if ctx == nil { // For unit tests.
		return internal.SelectUser(context.Background(), indexType, id) //workflowcheck:ignore
	}

	if useCache { // Minimize Temporal history without risking staleness or nondeterminism.
		if user, found := usersCache.Get(id); found {
			return user, nil
		}
	}

	var user User
	if err := executeLocalActivity(ctx, internal.SelectUser, &user, indexType, id); err != nil {
		return User{}, err
	}

	if !user.Created.IsZero() && useCache {
		usersCache.Set(id, user, cache.DefaultExpiration)
	}
	if user.Created.IsZero() {
		workflow.SideEffect(ctx, func(_ workflow.Context) any { return []string{"User not found", id} })
	}
	return user, nil
}
