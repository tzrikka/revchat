package commands

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	bitbucket "github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/data"
	github "github.com/tzrikka/revchat/pkg/github/activities"
	slack "github.com/tzrikka/revchat/pkg/slack/activities"
)

// CleanPRData is a hidden (but perfectly safe) maintenance command that deletes outdated PR data.
// This is not needed under normal operation, only after outages that caused the system to miss PR closure events.
func CleanPRData(ctx workflow.Context, event SlashCommandEvent, alertsChannel string) error {
	if err := cleanChannels(ctx, event, alertsChannel); err != nil {
		return err
	}

	if err := cleanURLs(ctx, event, alertsChannel); err != nil {
		return err
	}

	return nil
}

func cleanChannels(ctx workflow.Context, event SlashCommandEvent, alertsChannel string) error {
	results, err := data.ReadAllURLsOrChannels(ctx, data.Channels)
	if err != nil {
		PostEphemeralError(ctx, event, "failed to read all "+data.Channels)
		return err
	}

	cleaned, failed := 0, 0
	var contErr error
	for _, channelID := range results {
		contErr = continueAsNew(ctx, event)
		if contErr != nil {
			break
		}

		info, err := slack.ChannelInfo(ctx, channelID, false, false)
		if err != nil {
			_ = slack.AlertError(ctx, alertsChannel, "failed to fetch Slack channel info", err, "Channel",
				fmt.Sprintf("`%s` (<#%s>)", channelID, channelID), "Initiator", fmt.Sprintf("<@%s>", event.UserID))
			failed++
			continue
		}

		prURL, err := determineURL(ctx, alertsChannel, event.UserID, channelID, info)
		if err != nil {
			failed++
			continue
		}

		// Scenario 1: channel is archived - no need to check PR state.
		if isArchived, ok := info["is_archived"].(bool); ok && isArchived {
			msg := fmt.Sprintf("Deleting outdated data for already-archived Slack channel: `%s`", channelID)
			if prURL != "" {
				msg = fmt.Sprintf("%s (%s)", msg, prURL)
			}
			_ = slack.PostMessage(ctx, alertsChannel, msg)
			data.CleanupPRData(ctx, channelID, prURL)
			cleaned++
			continue
		}
		if prURL == "" {
			failed++
			continue
		}

		// Scenario 2: PR is closed.
		isOpen, err := isPROpen(ctx, prURL)
		if err != nil {
			_ = slack.AlertError(ctx, alertsChannel, "failed to fetch PR state",
				err, "PR", prURL, "Initiator", fmt.Sprintf("<@%s>", event.UserID))
			failed++
			continue
		}
		if isOpen {
			continue
		}

		if err := slack.ArchiveChannel(ctx, channelID, prURL); err != nil {
			_ = slack.AlertError(ctx, alertsChannel, "failed to archive zombie Slack channel for closed PR", err, "Channel",
				fmt.Sprintf("`%s` (<#%s>)", channelID, channelID), "PR", prURL, "Initiator", fmt.Sprintf("<@%s>", event.UserID))
			failed++
			continue
		}

		msg := "Archived zombie Slack channel: `%s`\nDeleting outdated data for closed PR: %s"
		_ = slack.PostMessage(ctx, alertsChannel, fmt.Sprintf(msg, channelID, prURL))
		data.CleanupPRData(ctx, channelID, prURL)
		cleaned++
	}

	msg := "Summary: deleted outdated data for `%d` Slack channels, failed to check/archive/clean-up `%d`"
	_ = slack.PostMessage(ctx, alertsChannel, fmt.Sprintf(msg, cleaned, failed))
	return contErr
}

func cleanURLs(ctx workflow.Context, event SlashCommandEvent, alertsChannel string) error {
	results, err := data.ReadAllURLsOrChannels(ctx, data.URLs)
	if err != nil {
		PostEphemeralError(ctx, event, "failed to read all "+data.URLs)
		return err
	}

	cleaned, failed := 0, 0
	var contErr error
	for _, prURL := range results {
		contErr = continueAsNew(ctx, event)
		if contErr != nil {
			break
		}

		isOpen, err := isPROpen(ctx, prURL)
		if err != nil {
			_ = slack.AlertError(ctx, alertsChannel, "failed to fetch PR state",
				err, "PR", prURL, "Initiator", fmt.Sprintf("<@%s>", event.UserID))
			failed++
			continue
		}
		if isOpen {
			continue
		}

		channelID, err := determineChannel(ctx, alertsChannel, event.UserID, prURL)
		if err != nil {
			failed++
			continue
		}

		_ = slack.PostMessage(ctx, alertsChannel, "Deleting outdated data for closed PR: "+prURL)
		data.CleanupPRData(ctx, channelID, prURL)
		cleaned++
	}

	msg := "Summary: deleted outdated data for `%d` closed PRs, failed to check/clean-up `%d`"
	_ = slack.PostMessage(ctx, alertsChannel, fmt.Sprintf(msg, cleaned, failed))
	return contErr
}

// continueAsNew helps the [CleanPRData] slash command, as a potentially long-running
// workflow, to play nicely with Temporal's history limits, as well as API rate limits.
func continueAsNew(ctx workflow.Context, event SlashCommandEvent) error {
	if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
		return workflow.NewContinueAsNewError(ctx, "slack.events.slash_command", event)
	}

	_ = workflow.Sleep(ctx, 3*time.Second)
	return nil
}

func determineChannel(ctx workflow.Context, alertsChannel, userID, prURL string) (string, error) {
	channelID, err := data.SwitchURLAndID(ctx, prURL)
	if err != nil {
		return "", slack.AlertError(ctx, alertsChannel, "failed to get mapping between PR URLs and Slack IDs",
			err, "PR", prURL, "Initiator", fmt.Sprintf("<@%s>", userID))
	}
	if channelID == "" {
		_ = slack.PostMessage(ctx, alertsChannel, "Dangling PR URL without Slack channel: "+prURL)
	}

	return channelID, nil
}

func determineURL(ctx workflow.Context, alertsChannel, userID, channelID string, info map[string]any) (string, error) {
	if topic, ok := info["topic"].(map[string]any); ok {
		if value, ok := topic["value"].(string); ok {
			if prURL := PullRequestURLPattern.FindString(value); prURL != "" {
				return prURL, nil
			}
		}
	}

	prURL, err := data.SwitchURLAndID(ctx, channelID)
	if err != nil {
		return "", slack.AlertError(ctx, alertsChannel, "failed to get mapping between PR URLs and Slack IDs", err,
			"Channel", fmt.Sprintf("`%s` (<#%s>)", channelID, channelID), "Initiator", fmt.Sprintf("<@%s>", userID))
	}
	if prURL == "" {
		_ = slack.PostMessage(ctx, alertsChannel, fmt.Sprintf("Dangling Slack channel without PR URL: `%s`", channelID))
	}

	return prURL, nil
}

func isBitbucketPR(url string) bool {
	return strings.HasPrefix(url, "https://bitbucket.org/")
}

func isPROpen(ctx workflow.Context, url string) (bool, error) {
	if isBitbucketPR(url) {
		pr, err := bitbucket.GetPullRequest(ctx, "", url)
		if err != nil {
			return false, err
		}
		state, found := pr["state"]
		return found && state == "OPEN", nil
	}

	// GitHub.
	pr, err := github.GetPullRequest(ctx, "", url)
	if err != nil {
		return false, err
	}
	return pr.State == "open", nil
}
