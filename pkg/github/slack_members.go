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

func (g GitHub) updateMembersWorkflow(ctx workflow.Context, event PullRequestEvent) error {
	// Already confirmed the Slack channel exists before starting this child workflow.
	channelID, _ := lookupChannel(ctx, event.Action, event.PullRequest)

	// Individual assignee.
	if user := event.Assignee; user != nil {
		switch event.Action {
		case "assigned":
			g.addChannelMember(ctx, channelID, *user, event.Sender, "assignee")
		case "unassigned":
			g.removeChannelMember(ctx, channelID, *user, event.Sender, "assignee")
		}
		return nil
	}

	// Individual reviewer.
	if user := event.RequestedReviewer; user != nil {
		switch event.Action {
		case "review_requested":
			g.addChannelMember(ctx, channelID, *user, event.Sender, "reviewer")
		case "review_request_removed":
			g.removeChannelMember(ctx, channelID, *user, event.Sender, "reviewer")
		}
		return nil
	}

	// Reviewing team.
	action := "added"
	if event.Action == "review_request_removed" {
		action = "removed"
	}
	if team := event.RequestedTeam; team != nil {
		msg := fmt.Sprintf("%s the <%s|%s> team as a reviewer", action, team.HTMLURL, team.Name)
		g.mentionUserInMsgAsync(ctx, channelID, event.Sender, "%s "+msg)
	}
	return nil
}

func (g GitHub) addChannelMember(ctx workflow.Context, channelID string, reviewer, sender User, role string) {
	slackUserID := g.announceUser(ctx, channelID, reviewer, sender, role, "added")
	if slackUserID == "" {
		return
	}

	_ = slack.InviteUsersToChannelActivity(ctx, g.cmd, channelID, []string{slackUserID})

	if reviewer.Login == sender.Login {
		return // No need to also DM the user if they added themselves.
	}

	// Send a DM to the reviewer with a reference to the Slack channel.
	msg := fmt.Sprintf("added you as %s to a PR: <#%s>", role, channelID)
	g.mentionUserInMsgAsync(ctx, slackUserID, sender, "%s "+msg)
}

func (g GitHub) removeChannelMember(ctx workflow.Context, channelID string, reviewer, sender User, role string) {
	if slackUserID := g.announceUser(ctx, channelID, reviewer, sender, role, "removed"); slackUserID != "" {
		_ = slack.KickUserFromChannelActivity(ctx, g.cmd, channelID, slackUserID)
	}
}

func (g GitHub) announceUser(ctx workflow.Context, channelID string, reviewer, sender User, role, action string) string {
	slackRef := users.GitHubToSlackRef(ctx, g.cmd, reviewer.Login, reviewer.HTMLURL)

	person := slackRef
	if reviewer.Login == sender.Login {
		person = "themselves"
	}
	msg := fmt.Sprintf("%s %s as %s", action, person, withArticle(role))
	_, _ = g.mentionUserInMsg(ctx, channelID, sender, "%s "+msg)

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
