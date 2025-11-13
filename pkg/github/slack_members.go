package github

import (
	"fmt"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack"
	"github.com/tzrikka/revchat/pkg/users"
	tslack "github.com/tzrikka/timpani-api/pkg/slack"
)

func (c Config) updateMembers(ctx workflow.Context, event PullRequestEvent) error {
	// Already confirmed the Slack channel exists before calling this function.
	channelID, _ := lookupChannel(ctx, event.Action, event.PullRequest)
	var err error

	// Individual assignee.
	if user := event.Assignee; user != nil {
		switch event.Action {
		case "assigned":
			err = c.addChannelMember(ctx, channelID, *user, event.Sender, "assignee")
		case "unassigned":
			err = c.removeChannelMember(ctx, channelID, *user, event.Sender, "assignee")
		}
	}

	// Individual reviewer.
	if user := event.RequestedReviewer; user != nil {
		switch event.Action {
		case "review_requested":
			err = c.addChannelMember(ctx, channelID, *user, event.Sender, "reviewer")
		case "review_request_removed":
			err = c.removeChannelMember(ctx, channelID, *user, event.Sender, "reviewer")
		}
	}

	// Reviewing team.
	action := "added"
	if event.Action == "review_request_removed" {
		action = "removed"
	}
	if team := event.RequestedTeam; team != nil {
		msg := fmt.Sprintf("%s the <%s|%s> team as a reviewer", action, team.HTMLURL, team.Name)
		err = c.mentionUserInMsg(ctx, channelID, event.Sender, "%s "+msg)
	}

	return err
}

func (c Config) addChannelMember(ctx workflow.Context, channelID string, reviewer, sender User, role string) error {
	slackUserID := c.announceUser(ctx, channelID, reviewer, sender, role, "added")
	if slackUserID == "" {
		return nil
	}

	err := slack.InviteUsersToChannel(ctx, channelID, []string{slackUserID})

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

	return tslack.ConversationsKickActivity(ctx, channelID, slackUserID)
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
	user, err := data.SelectUserBySlackID(slackID)
	if err != nil {
		log.Error(ctx, "failed to load user by Slack ID", "error", err, "user_id", slackID)
		return ""
	}

	if !data.IsOptedIn(user) {
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
