package bitbucket

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/markdown"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
	tslack "github.com/tzrikka/timpani-api/pkg/slack"
)

func archiveChannel(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no channel to archive.
	pr := event.PullRequest
	channelID, found := lookupChannel(ctx, event.Type, pr)
	if !found {
		return nil
	}

	// Wait for a few seconds to handle other asynchronous events
	// (e.g. a PR closure comment) before archiving the channel.
	_ = workflow.Sleep(ctx, 5*time.Second)

	u := pr.Links["html"].HRef
	cleanupPRData(ctx, u)

	state := "closed this PR"
	switch event.Type {
	case "fulfilled":
		state = "merged this PR"
	case "updated":
		state = "converted this PR to a draft"
	}

	if reason := event.PullRequest.Reason; reason != "" {
		state = fmt.Sprintf("%s with this reason: `%s`", state, reason)
	} else {
		state += "."
	}

	_ = mentionUserInMsg(ctx, channelID, event.Actor, "%s "+state)

	if err := tslack.ConversationsArchiveActivity(ctx, channelID); err != nil {
		state = strings.Replace(state, " this PR", "", 1)
		msg := "Failed to archive this channel, even though its PR was " + state
		_, _ = slack.PostMessage(ctx, channelID, msg)

		return err
	}

	if err := data.LogSlackChannelArchived(channelID, u); err != nil {
		log.Error(ctx, "failed to log Slack channel archived", "error", err, "channel_id", channelID, "pr_url", u)
		// Don't fail the workflow for logging errors.
	}
	return nil
}

// lookupChannel returns the ID of a channel associated
// with a PR, if the PR is active and the channel is found.
func lookupChannel(ctx workflow.Context, eventType string, pr PullRequest) (string, bool) {
	if pr.Draft {
		return "", false
	}

	u := pr.Links["html"].HRef
	channelID, err := data.SwitchURLAndID(u)
	if err != nil {
		log.Error(ctx, "failed to retrieve PR's Slack channel ID", "error", err, "event_type", eventType, "pr_url", u)
		return "", false
	}

	return channelID, channelID != ""
}

// cleanupPRData deletes all data associated with a PR. If there are errors,
// they are logged but ignored, as they do not affect the overall workflow.
func cleanupPRData(ctx workflow.Context, u string) {
	if err := data.DeleteBitbucketPR(u); err != nil {
		log.Error(ctx, "failed to delete Bitbucket PR snapshot", "error", err, "pr_url", u)
	}

	if err := data.DeleteTurn(u); err != nil {
		log.Error(ctx, "failed to delete Bitbucket PR turn-taking state", "error", err, "pr_url", u)
	}

	if err := data.DeleteURLAndIDMapping(u); err != nil {
		log.Error(ctx, "failed to delete PR URL / Slack channel mappings", "error", err, "pr_url", u)
	}
}

// initPRData saves the initial state of a new PR: a snapshot of the
// PR details, and mappings for 2-way syncs between Bitbucket and Slack.
func initPRData(ctx workflow.Context, u string, pr PullRequest, authorAccountID, channelID string) error {
	if err := data.StoreBitbucketPR(u, pr); err != nil {
		log.Error(ctx, "failed to save Bitbucket PR snapshot", "error", err, "channel_id", channelID, "pr_url", u)
		return err
	}

	email, err := users.BitbucketToEmail(ctx, authorAccountID)
	if err != nil {
		return err
	}
	if email == "" {
		log.Error(ctx, "initializing Bitbucket PR data without author's email", "pr_url", u)
	}

	reviewers := []string{}
	for _, r := range pr.Reviewers {
		if e, err := users.BitbucketToEmail(ctx, r.AccountID); err == nil {
			reviewers = append(reviewers, e)
		}
	}

	if err := data.InitTurn(u, email, reviewers); err != nil {
		log.Error(ctx, "failed to initialize Bitbucket PR turn-taking state", "error", err, "channel_id", channelID, "pr_url", u)
		return err
	}

	if err := data.MapURLAndID(u, channelID); err != nil {
		log.Error(ctx, "failed to save PR URL / Slack channel mapping", "error", err, "channel_id", channelID, "pr_url", u)
		return err
	}

	return nil
}

func (c Config) initChannel(ctx workflow.Context, event PullRequestEvent) error {
	pr := event.PullRequest
	u := pr.Links["html"].HRef
	pr.CommitCount = len(commits(ctx, event))

	channelID, err := c.createChannel(ctx, pr)
	if err != nil {
		reportCreationErrorToAuthor(ctx, event.Actor.AccountID, u)
		return err
	}

	if err := initPRData(ctx, u, pr, event.Actor.AccountID, channelID); err != nil {
		reportCreationErrorToAuthor(ctx, event.Actor.AccountID, u)
		cleanupPRData(ctx, u)
		return err
	}

	// Channel cosmetics.
	slack.SetChannelTopic(ctx, channelID, u)
	slack.SetChannelDescription(ctx, channelID, pr.Title, u)
	setChannelBookmarks(ctx, channelID, u, pr)
	postIntroMsg(ctx, channelID, event.Type, pr, event.Actor)
	if msg := c.linkifyIDs(ctx, pr.Title); msg != "" {
		_, _ = slack.PostMessage(ctx, channelID, msg)
	}

	err = slack.InviteUsersToChannel(ctx, channelID, prParticipants(ctx, pr))
	if err != nil {
		reportCreationErrorToAuthor(ctx, event.Actor.AccountID, u)
		cleanupPRData(ctx, u)
		return err
	}

	return nil
}

func (c Config) createChannel(ctx workflow.Context, pr PullRequest) (string, error) {
	title := slack.NormalizeChannelName(pr.Title, c.Cmd.Int("slack-max-channel-name-length"))
	u := pr.Links["html"].HRef

	for i := 1; i < 50; i++ {
		name := fmt.Sprintf("%s-%d_%s", c.Cmd.String("slack-channel-name-prefix"), pr.ID, title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		id, retry, err := slack.CreateChannel(ctx, name, u, c.Cmd.Bool("slack-private-channels"))
		if err != nil {
			if retry {
				continue
			} else {
				return "", err
			}
		}

		if err := data.LogSlackChannelCreated(id, name, u); err != nil {
			log.Error(ctx, "failed to log Slack channel creation", "error", err, "channel_id", id, "pr_url", u)
			// Don't fail the workflow for logging errors.
		}
		return id, nil
	}

	msg := "too many failed attempts to create Slack channel"
	log.Error(ctx, msg, "pr_url", u)
	return "", errors.New(msg)
}

func (c Config) renameChannel(ctx workflow.Context, pr PullRequest, channelID string) error {
	title := slack.NormalizeChannelName(pr.Title, c.Cmd.Int("slack-max-channel-name-length"))

	for i := 1; i < 50; i++ {
		name := fmt.Sprintf("%s-%d_%s", c.Cmd.String("slack-channel-name-prefix"), pr.ID, title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		retry, err := slack.RenameChannel(ctx, channelID, name)
		if retry {
			continue
		}
		if err == nil {
			if logErr := data.LogSlackChannelRenamed(channelID, name); logErr != nil {
				log.Error(ctx, "failed to log Slack channel renaming", "error", logErr, "channel_id", channelID, "new_name", name)
				// Don't fail the workflow for logging errors.
			}
		}
		return err
	}

	msg := "too many failed attempts to rename Slack channel"
	log.Error(ctx, msg, "pr_url", pr.Links["html"].HRef, "channel_id", channelID)
	return errors.New(msg)
}

func reportCreationErrorToAuthor(ctx workflow.Context, accountID, url string) {
	// True = don't send a DM to the user if they're opted-out.
	userID := users.BitbucketToSlackID(ctx, accountID, true)
	if userID == "" {
		return
	}

	_, _ = slack.PostMessage(ctx, userID, "Failed to create Slack channel for "+url)
}

func postIntroMsg(ctx workflow.Context, channelID, eventType string, pr PullRequest, actor Account) {
	action := "created"
	if eventType == "updated" {
		action = "marked as ready for review"
	}

	url := pr.Links["html"].HRef
	msg := fmt.Sprintf("%%s %s %s: `%s`", action, url, pr.Title)
	if text := strings.TrimSpace(pr.Description); text != "" {
		msg += "\n\n" + markdown.BitbucketToSlack(ctx, text, url)
	}

	_ = mentionUserInMsg(ctx, channelID, actor, msg)
}

var linkifyPattern = regexp.MustCompile(`[A-Z]+-\d+`)

// linkifyIDs finds IDs in the text and tries to linkify them based on the configured issue
// trackers. If no IDs are found, or none are recognized, an empty string is returned.
func (c Config) linkifyIDs(ctx workflow.Context, text string) string {
	ids := linkifyPattern.FindAllString(text, -1)
	slices.Sort(ids)
	ids = slices.Compact(ids)

	msg := "> References in the PR:"
	for _, id := range ids {
		if url := c.linkify(ctx, id); url != "" {
			msg += fmt.Sprintf("\n>  •  <%s|%s>", url, id)
		}
	}

	if strings.Count(msg, "\n") == 0 {
		msg = ""
	}
	return msg
}

// linkify can recognize specific case-sensitive keys, as well as a generic "default" key.
func (c Config) linkify(ctx workflow.Context, id string) string {
	lm := c.Cmd.StringSlice("linkification-map")
	keys := map[string]string{}
	for _, kv := range lm {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			log.Error(ctx, "invalid key-value pair in linkification map configuration", "kv", kv)
			continue
		}
		keys[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	key, _, _ := strings.Cut(id, "-")
	if base, found := keys[key]; found {
		return buildURL(ctx, base, id)
	}

	if base, found := keys["default"]; found {
		return buildURL(ctx, base, id)
	}

	return ""
}

func buildURL(ctx workflow.Context, base, id string) string {
	u, err := url.JoinPath(base, id)
	if err != nil {
		log.Error(ctx, "failed to join URL paths", "error", err, "base", base, "id", id)
		return ""
	}
	return u
}

// prParticipants returns a list of opted-in Slack user IDs (author/participants/reviewers).
// The output is guaranteed to be sorted, without teams/apps, and without repetitions.
func prParticipants(ctx workflow.Context, pr PullRequest) []string {
	accounts := append([]Account{pr.Author}, pr.Reviewers...)
	for _, p := range pr.Participants {
		accounts = append(accounts, p.User)
	}

	accountIDs := []string{}
	for _, a := range accounts {
		if a.Type == "user" {
			accountIDs = append(accountIDs, a.AccountID)
		}
	}

	slackIDs := []string{}
	for _, aid := range accountIDs {
		// True = don't include opted-out users. They will still be mentioned
		// in the channel, but as non-members they won't be notified about it.
		if sid := users.BitbucketToSlackID(ctx, aid, true); sid != "" {
			slackIDs = append(slackIDs, sid)
		}
	}

	slices.Sort(slackIDs)
	return slices.Compact(slackIDs)
}
