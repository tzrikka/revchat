package workflows

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/slack/commands"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

var groupMentionPattern = regexp.MustCompile(`<!subteam\^(\w+)`)

// triggerNudge examines new message events in pre-configured non-RevChat channels.
// If they contain at least one pre-configured Slack group and at least one PR link, it
// sends nudges to the specified group(s) about the specified PR(s) on behalf of the user.
func (c *Config) triggerNudge(ctx workflow.Context, event messageEventWrapper, senderID string) error {
	// Sanity checks.
	switch {
	case event.InnerEvent.Subtype != "" && event.InnerEvent.Subtype != "thread_broadcast":
		return nil
	case !slices.Contains(c.NudgeChannels, event.InnerEvent.Channel):
		return nil
	case senderID == "":
		logger.From(ctx).Error("could not determine who triggered a Slack message event")
		err := errors.New("could not determine who triggered a Slack message event")
		return activities.AlertError(ctx, c.AlertsChannel, "", err)
	case selfTriggeredEvent(ctx, event.Authorizations, senderID):
		return nil
	}

	recipients := c.extractAndExpandUserIDs(ctx, event.InnerEvent, senderID)
	if len(recipients) == 0 {
		return nil
	}

	var err error
	for _, prURL := range extractPullRequestURLs(event.InnerEvent.Text) {
		err = errors.Join(err, nudge(ctx, event.InnerEvent, recipients, senderID, prURL, c.ThrippyHTTPAddress))
	}

	return err
}

func (c *Config) extractAndExpandUserIDs(ctx workflow.Context, event MessageEvent, senderID string) []string {
	// Extract valid (pre-configured) group IDs.
	matches := groupMentionPattern.FindAllStringSubmatch(event.Text, -1)
	groupIDs := make([]string, len(matches))
	for i, match := range matches {
		groupIDs[i] = match[1]
	}

	groupIDs = intersect(groupIDs, c.NudgeGroups)
	if len(groupIDs) == 0 {
		return nil
	}

	// Expand groups to member IDs.
	var users []string
	for _, id := range groupIDs {
		members, err := slack.UserGroupsUsersList(ctx, strings.ToUpper(id), false)
		if err != nil {
			logger.From(ctx).Error("failed to expand Slack user group", slog.Any("error", err), slog.String("subteam_id", id))
			postEphemeralError(ctx, event, senderID, fmt.Sprintf("failed to expand the user group `<!subteam^%s>`.", id))
			continue
		}
		users = append(users, members...)
	}

	// Sort and deduplicate.
	slices.Sort(users)
	users = slices.Compact(users)

	// Remove the sender from the list to avoid self-nudges.
	if i := slices.Index(users, senderID); i != -1 {
		users = slices.Delete(users, i, i+1)
	}

	return users
}

func intersect[S ~[]E, E cmp.Ordered](slice1, slice2 S) S {
	set := make(map[E]bool, len(slice1))
	for _, elem := range slice1 {
		set[elem] = true
	}

	var result S
	for _, elem := range slice2 {
		if set[elem] {
			result = append(result, elem)
		}
	}

	slices.Sort(result)
	return slices.Compact(result)
}

func extractPullRequestURLs(text string) []string {
	matches := urlPattern.FindAllStringSubmatch(text, -1)

	urls := make([]string, len(matches))
	for i, parts := range matches {
		switch len(parts) {
		case 7, 8:
			urls[i] = strings.TrimSuffix(parts[0], parts[6]) // Full URL - comment suffix = PR URL.
		default:
			urls[i] = parts[0] // PR URL without any suffix that needs trimming.
		}
	}

	slices.Sort(urls)
	return slices.Compact(urls)
}

func nudge(ctx workflow.Context, event MessageEvent, recipients []string, senderID, prURL, imagesHTTPServer string) error {
	done := make([]string, 0, len(recipients))
	for _, userID := range recipients {
		// Check that the user is eligible to be nudged.
		if !checkAndNudgeUser(ctx, event, prURL, userID) {
			continue
		}

		imageURL := commands.NudgeImageURL(ctx, imagesHTTPServer)
		msg := fmt.Sprintf(":pleading_face: Please take a look at %s :pray:", prURL)
		altText := "Tip: click the collapse arrow above this image to hide it, as a self-reminder after completing this task"
		if err := activities.PostDMWithImage(ctx, senderID, userID, msg, imageURL, altText); err != nil {
			postEphemeralError(ctx, event, senderID, fmt.Sprintf("failed to send a nudge to <@%s>.", userID))
			continue
		}

		done = append(done, userID)
	}

	if len(done) == 0 {
		return nil
	}

	msg := "Sent nudge"
	if len(done) > 1 {
		msg += "s"
	}
	msg = fmt.Sprintf("%s to: <@%s> about %s", msg, strings.Join(done, ">, <@"), prURL)
	return activities.PostEphemeralMessage(ctx, event.Channel, senderID, msg)
}

// checkAndNudgeUser ensures that the recipient exists, is opted-in, and is a reviewer of the PR.
// It returns true if the attention state was updated. If not, it also posts an explanation.
func checkAndNudgeUser(ctx workflow.Context, event MessageEvent, prURL, userID string) bool {
	// Check other conditions, send error messages as needed.
	user, optedIn, err := userDetails(ctx, event, userID)
	if err != nil {
		return false
	}
	if !optedIn {
		msg := fmt.Sprintf(":no_bell: <@%s> isn't opted-in to use RevChat.", userID)
		_ = activities.PostEphemeralMessage(ctx, event.Channel, userID, msg)
		return false
	}

	// Update the PR's attention state.
	ok, approved, err := data.Nudge(ctx, prURL, user.Email)
	if err != nil {
		postEphemeralError(ctx, event, userID, fmt.Sprintf("internal data error while nudging <@%s>.", userID))
		return ok // May be true despite the error: a valid reviewer, but failed to save it.
	}
	if ok {
		return true
	}

	// Not a tracked participant in this PR - but why?
	msg := fmt.Sprintf(":see_no_evil: <@%s> isn't a participant in this PR, add them to enable nudging.", userID)
	if approved {
		msg = fmt.Sprintf(":+1: <@%s> already approved this PR.", userID)
	}
	_ = activities.PostEphemeralMessage(ctx, event.Channel, userID, msg)
	return false
}

// userDetails retrieves the user details from internal data based on
// their Slack ID, and (based on that) whether they are opted-in or not.
func userDetails(ctx workflow.Context, event MessageEvent, userID string) (data.User, bool, error) {
	user, optedIn, err := data.SelectUserBySlackID(ctx, userID)
	if err != nil {
		postEphemeralError(ctx, event, userID, fmt.Sprintf("failed to read internal data about <@%s>.", userID))
		return data.User{}, false, err
	}
	return user, optedIn, nil
}

func postEphemeralError(ctx workflow.Context, event MessageEvent, userID, msg string) {
	// We're already reporting another error, there's nothing to do if this fails.
	_ = activities.PostEphemeralMessage(ctx, event.Channel, userID, ":warning: Error: "+msg)
}
