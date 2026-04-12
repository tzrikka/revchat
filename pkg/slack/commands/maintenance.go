package commands

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"

	bitbucket "github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/data"
	github "github.com/tzrikka/revchat/pkg/github/activities"
	slack "github.com/tzrikka/revchat/pkg/slack/activities"
)

// CleanPRData is a hidden (but perfectly safe) maintenance command that deletes outdated PR data.
// This is not needed under normal operation, only after outages that caused the system to miss PR closure events.
func CleanPRData(ctx workflow.Context, _ client.Options, event SlashCommandEvent, alertsChannel string) error {
	if err := cleanURLs(ctx, event, alertsChannel); err != nil {
		return err
	}

	if err := cleanChannels(ctx, event, alertsChannel); err != nil {
		return err
	}

	// PR build/diffstat/snapshot/turn data.

	return nil
}

func isPROpen(ctx workflow.Context, url string) (bool, error) {
	workflow.Sleep(ctx, 3*time.Second) // Avoid hitting API rate limits when processing many PRs.

	if strings.HasPrefix(url, "https://bitbucket.org/") {
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

func cleanURLs(ctx workflow.Context, event SlashCommandEvent, alertsChannel string) error {
	results, err := data.ReadAllURLsOrChannels(ctx, data.URLs)
	if err != nil {
		PostEphemeralError(ctx, event, "failed to read all "+data.URLs)
		return err
	}

	cleaned, failed := 0, 0
	for _, url := range results {
		var open bool
		open, err = isPROpen(ctx, url)
		if err != nil {
			err = errors.Join(err, slack.AlertError(ctx, alertsChannel, "failed to fetch the state of a PR", err,
				"URL", url, "Initiator", fmt.Sprintf("<@%s>", event.UserID)))
			failed++
			continue
		}
		if open {
			continue
		}

		err = errors.Join(err, slack.PostMessage(ctx, alertsChannel, "Need to remove data for closed PR: "+url))
		cleaned++
	}

	summary := "Summary: cleaned up data for %d closed PRs, failed to check/clean-up %d PRs."
	return errors.Join(err, slack.PostMessage(ctx, alertsChannel, fmt.Sprintf(summary, cleaned, failed)))
}

func cleanChannels(ctx workflow.Context, event SlashCommandEvent, _ string) error {
	_, err := data.ReadAllURLsOrChannels(ctx, data.Channels)
	if err != nil {
		PostEphemeralError(ctx, event, "failed to read all "+data.Channels)
		return err
	}

	return nil
}
