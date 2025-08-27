package github

import (
	"fmt"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
)

func (c Config) updateMembers(ctx workflow.Context, event PullRequestEvent) error {
	// Already confirmed the Slack channel exists before calling this function.
	channelID, _ := lookupChannel(ctx, event.Action, event.PullRequest)

	// Individual assignee.
	if user := event.Assignee; user != nil {
		switch event.Action {
		case "assigned":
			return c.addChannelMember(ctx, channelID, *user, event.Sender, "assignee")
		case "unassigned":
			return c.removeChannelMember(ctx, channelID, *user, event.Sender, "assignee")
		default:
			return nil
		}
	}

	// Individual reviewer.
	if user := event.RequestedReviewer; user != nil {
		switch event.Action {
		case "review_requested":
			return c.addChannelMember(ctx, channelID, *user, event.Sender, "reviewer")
		case "review_request_removed":
			return c.removeChannelMember(ctx, channelID, *user, event.Sender, "reviewer")
		default:
			return nil
		}
	}

	// Reviewing team.
	action := "added"
	if event.Action == "review_request_removed" {
		action = "removed"
	}
	if team := event.RequestedTeam; team != nil {
		msg := fmt.Sprintf("%s the <%s|%s> team as a reviewer", action, team.HTMLURL, team.Name)
		return c.mentionUserInMsg(ctx, channelID, event.Sender, "%s "+msg)
	}

	return nil
}

func (c Config) addChannelMember(ctx workflow.Context, channelID string, reviewer, sender User, role string) error {
	slackUserID := c.announceUser(ctx, channelID, reviewer, sender, role, "added")
	if slackUserID == "" {
		return nil
	}

	err := slack.InviteUsersToChannelActivity(ctx, c.Cmd, channelID, []string{slackUserID})

	if reviewer.Login == sender.Login {
		return err // No need to also DM the user if they added themselves.
	}

	// Send a DM to the reviewer with a reference to the Slack channel.
	msg := fmt.Sprintf("added you as %s to a PR: <#%s>", role, channelID)
	_ = c.mentionUserInMsg(ctx, slackUserID, sender, "%s "+msg)

	return err
}

func (c Config) removeChannelMember(ctx workflow.Context, channelID string, reviewer, sender User, role string) error {
	slackUserID := c.announceUser(ctx, channelID, reviewer, sender, role, "removed")
	if slackUserID == "" {
		return nil
	}

	return slack.KickUserFromChannelActivity(ctx, c.Cmd, channelID, slackUserID)
}

func (c Config) announceUser(ctx workflow.Context, channelID string, reviewer, sender User, role, action string) string {
	slackRef := users.GitHubToSlackRef(ctx, c.Cmd, reviewer.Login, reviewer.HTMLURL)

	person := slackRef
	if reviewer.Login == sender.Login {
		person = "themselves"
	}
	msg := fmt.Sprintf("%s %s as %s", action, person, withArticle(role))
	_ = c.mentionUserInMsg(ctx, channelID, sender, "%s "+msg)

	if !strings.HasPrefix(slackRef, "<@") {
		return "" // Not a real Slack user ID - can't add it to the Slack channel.
	}

	slackID := strings.TrimSuffix(strings.TrimPrefix(slackRef, "<@"), ">")
	email, err := data.SlackUserEmailByID(slackID)
	if err != nil {
		workflow.GetLogger(ctx).Error("failed to load Slack user email", "error", err, "user_id", slackID)
		return ""
	}
	if email == "" {
		return ""
	}

	optedIn, err := data.IsOptedIn(email)
	if err != nil {
		workflow.GetLogger(ctx).Error("failed to load user opt-in status", "error", err, "email", email)
		return ""
	}
	if !optedIn {
		return ""
	}

	return slackID // Continue to add/remove the user to/from the Slack channel.
}

func withArticle(role string) string {
	article := "a "
	if regexp.MustCompile(`^[AEIOUaeiou]`).MatchString(role) {
		article = "an "
	}
	return article + role
}
