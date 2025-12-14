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

// lookupChannel returns the ID of a Slack channel associated with the given PR, if it exists.
func lookupChannel(ctx workflow.Context, eventType string, pr PullRequest) (string, bool) {
	channelID, err := data.SwitchURLAndID(htmlURL(pr.Links))
	if err != nil {
		log.Error(ctx, "failed to retrieve PR's Slack channel ID", "error", err,
			"event_type", eventType, "pr_url", htmlURL(pr.Links))
		return "", false
	}

	return channelID, channelID != ""
}

// lookupSlackFileID returns all the Slack IDs associated with the given PR comment, if they exist.
func lookupSlackFileID(ctx workflow.Context, comment *Comment) (string, bool) {
	fileID, err := data.SwitchURLAndID(htmlURL(comment.Links) + "/slack_file_id")
	if err != nil {
		log.Error(ctx, "failed to retrieve PR comment's Slack file ID",
			"error", err, "pr_url", htmlURL(comment.Links))
		return "", false
	}

	return fileID, fileID != ""
}

func archiveChannel(ctx workflow.Context, event PullRequestEvent) error {
	// If we're not tracking this PR, there's no channel to archive.
	pr := event.PullRequest
	channelID, found := lookupChannel(ctx, event.Type, pr)
	if !found {
		return nil
	}

	// Wait for a few seconds to handle other asynchronous events
	// (e.g. a PR closure comment) before archiving the channel.
	_ = workflow.Sleep(ctx, 3*time.Second)

	state := "closed this PR"
	if event.Type == "fulfilled" {
		state = "merged this PR"
	}
	if reason := event.PullRequest.Reason; reason != "" {
		state = fmt.Sprintf("%s with this reason: `%s`", state, reason)
	} else {
		state += "."
	}
	mentionUserInMsg(ctx, channelID, event.Actor, "%s "+state)

	prURL := htmlURL(pr.Links)
	if err := data.LogSlackChannelArchived(channelID, prURL); err != nil {
		log.Error(ctx, "failed to log Slack channel archived", "error", err, "channel_id", channelID, "pr_url", prURL)
	}

	if err := tslack.ConversationsArchive(ctx, channelID); err != nil {
		log.Error(ctx, "failed to archive Slack channel", "error", err, "channel_id", channelID, "pr_url", prURL)

		state = strings.Replace(state, " this PR", "", 1)
		_, _ = slack.PostMessage(ctx, channelID, "Failed to archive this channel, even though its PR was "+state)

		return err
	}

	data.FullPRCleanup(ctx, prURL)
	return nil
}

// initPRData saves the initial state of a new PR: a snapshot of the PR
// details, and mappings for 2-way syncs between Bitbucket and Slack. If there
// are errors, they are logged but ignored, as we can try to create the data later.
func initPRData(ctx workflow.Context, event PullRequestEvent, channelID string) {
	pr := event.PullRequest
	prURL := htmlURL(pr.Links)

	if err := data.StoreBitbucketPR(prURL, pr); err != nil {
		log.Error(ctx, "failed to save Bitbucket PR snapshot", "error", err, "channel_id", channelID, "pr_url", prURL)
	}

	if err := data.UpdateBitbucketDiffstat(prURL, diffstat(ctx, event)); err != nil {
		log.Error(ctx, "failed to update Bitbucket PR's diffstat", "error", err, "pr_url", prURL)
	}

	email, _ := users.BitbucketToEmail(ctx, event.Actor.AccountID)
	if email == "" {
		log.Error(ctx, "initializing Bitbucket PR data without author's email", "pr_url", prURL)
	}

	reviewers := []string{}
	for _, r := range pr.Reviewers {
		if e, err := users.BitbucketToEmail(ctx, r.AccountID); err == nil {
			reviewers = append(reviewers, e)
		}
	}

	if err := data.InitTurns(prURL, email, reviewers); err != nil {
		log.Error(ctx, "failed to initialize Bitbucket PR turn-taking state",
			"error", err, "channel_id", channelID, "pr_url", prURL)
	}

	if err := data.MapURLAndID(prURL, channelID); err != nil {
		log.Error(ctx, "failed to save PR URL / Slack channel mapping",
			"error", err, "channel_id", channelID, "pr_url", prURL)
	}
}

func (c Config) initChannel(ctx workflow.Context, event PullRequestEvent) error {
	pr := event.PullRequest
	prURL := htmlURL(pr.Links)
	pr.CommitCount = len(commits(ctx, event))

	channelID, err := c.createChannel(ctx, pr)
	if err != nil {
		reportCreationErrorToAuthor(ctx, event.Actor.AccountID, prURL)
		return err
	}

	initPRData(ctx, event, channelID)

	// Channel cosmetics.
	slack.SetChannelTopic(ctx, channelID, prURL)
	slack.SetChannelDescription(ctx, channelID, pr.Title, prURL)
	setChannelBookmarks(ctx, channelID, prURL, pr)

	msg := "%s created this PR: " + c.linkifyTitle(ctx, pr.Title)
	if desc := strings.TrimSpace(pr.Description); desc != "" {
		msg += "\n\n" + markdown.BitbucketToSlack(ctx, desc, prURL)
	}
	mentionUserInMsg(ctx, channelID, event.Actor, msg)

	err = slack.InviteUsersToChannel(ctx, channelID, prParticipants(ctx, pr))
	if err != nil {
		reportCreationErrorToAuthor(ctx, event.Actor.AccountID, prURL)
		_ = tslack.ConversationsArchive(ctx, channelID)
		data.FullPRCleanup(ctx, prURL)
		return err
	}

	return nil
}

func (c Config) createChannel(ctx workflow.Context, pr PullRequest) (string, error) {
	title := slack.NormalizeChannelName(pr.Title, c.Cmd.Int("slack-max-channel-name-length"))
	prURL := htmlURL(pr.Links)

	for i := 1; i < 10; i++ {
		name := fmt.Sprintf("%s-%d_%s", c.Cmd.String("slack-channel-name-prefix"), pr.ID, title)
		if i > 1 {
			name = fmt.Sprintf("%s_%d", name, i)
		}

		id, retry, err := slack.CreateChannel(ctx, name, prURL, c.Cmd.Bool("slack-private-channels"))
		if err != nil {
			if retry {
				continue
			} else {
				return "", err
			}
		}

		if err := data.LogSlackChannelCreated(id, name, prURL); err != nil {
			log.Error(ctx, "failed to log Slack channel creation", "error", err, "channel_id", id, "pr_url", prURL)
			// Don't fail the workflow because of logging errors.
		}

		return id, nil
	}

	msg := "too many failed attempts to create Slack channel"
	log.Error(ctx, msg, "pr_url", prURL)
	return "", errors.New(msg)
}

func (c Config) renameChannel(ctx workflow.Context, pr PullRequest, channelID string) error {
	title := slack.NormalizeChannelName(pr.Title, c.Cmd.Int("slack-max-channel-name-length"))

	for i := 1; i < 10; i++ {
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
	log.Error(ctx, msg, "pr_url", htmlURL(pr.Links), "channel_id", channelID)
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

var linkifyPattern = regexp.MustCompile(`[A-Z]{2,}-\d+`)

// linkifyTitle finds IDs in the text and tries to linkify them based on the configured issue
// trackers. If no IDs are found, or none are recognized, it returns the input text as-is.
func (c Config) linkifyTitle(ctx workflow.Context, title string) string {
	for _, id := range linkifyPattern.FindAllString(title, -1) {
		if url := c.linkifyID(ctx, id); url != "" {
			title = strings.Replace(title, id, fmt.Sprintf("<%s|%s>", url, id), 1)
		}
	}
	return title
}

// linkifyID can recognize specific case-sensitive keys, as well as a generic "default" key.
func (c Config) linkifyID(ctx workflow.Context, id string) string {
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

// prParticipants returns a list of opted-in Slack user IDs (author/participants/reviewers/followers).
// The output is guaranteed to be sorted, without teams/apps, and without repetitions.
func prParticipants(ctx workflow.Context, pr PullRequest) []string {
	accounts := append([]Account{pr.Author}, pr.Reviewers...)
	for _, p := range pr.Participants {
		accounts = append(accounts, p.User)
	}

	accountIDs := make([]string, 0, len(accounts))
	for _, a := range accounts {
		if a.Type == "user" {
			accountIDs = append(accountIDs, a.AccountID)
		}
	}

	slackIDs := make([]string, 0, len(accountIDs))
	for _, aid := range accountIDs {
		// True = don't include opted-out users. They will still be mentioned
		// in the channel, but as non-members they won't be notified about it.
		if sid := users.BitbucketToSlackID(ctx, aid, true); sid != "" {
			slackIDs = append(slackIDs, sid)
		}
	}

	if user, err := data.SelectUserByBitbucketID(pr.Author.AccountID); err == nil {
		slackIDs = append(slackIDs, user.Followers...)
	}

	slices.Sort(slackIDs)
	return slices.Compact(slackIDs)
}
