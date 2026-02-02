package commands

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	"github.com/tzrikka/revchat/pkg/users"
)

func Nudge(ctx workflow.Context, event SlashCommandEvent, imagesHTTPServer string) error {
	url, err := prDetailsFromChannel(ctx, event)
	if url == nil {
		return err // May or may not be nil.
	}

	users := extractAtLeastOneUserID(ctx, event)
	if len(users) == 0 {
		return nil
	}

	cmd := strings.SplitN(event.Text, " ", 2) // cmd[0] = "nudge", "ping", or "poke".
	if len(users) == 1 && users[0] == event.UserID {
		msg := ":confused: Why are you trying to %s yourself? Treating this as a `%s my turn` command..."
		_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, fmt.Sprintf(msg, cmd[0], event.Command))
		return MyTurn(ctx, event)
	}

	// Also ignore the PR's author when there are additional users. The likely scenario is
	// that the nudge is for a group which they are also a member of, but if the group is
	// nudged and not just them then it means the author is not the intended target.
	if len(users) > 1 {
		author := authorSlackID(ctx, url[0])
		if i := slices.Index(users, author); i != -1 {
			action, _ := strings.CutSuffix(cmd[0], "e")
			msg := ":see_no_evil: Ignoring the PR author (<@%s>) when %sing multiple users."
			_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, fmt.Sprintf(msg, author, action))
			users = slices.Delete(users, i, i+1)
		}
	}

	done := make([]string, 0, len(users))
	for _, userID := range users {
		// Check that the user is eligible to be nudged.
		if !checkAndNudgeUser(ctx, event, url[0], userID) {
			continue
		}

		msg := fmt.Sprintf(":pleading_face: Please take a look at <#%s> :pray:", event.ChannelID)
		altText := "Tip: click the collapse arrow above this image to hide it, as a self-reminder after completing this task"
		if err := activities.PostDMWithImage(ctx, event.UserID, userID, msg, imageURL(ctx, imagesHTTPServer), altText); err != nil {
			PostEphemeralError(ctx, event, fmt.Sprintf("failed to send a %s to <@%s>.", cmd[0], userID))
			continue
		}

		done = append(done, userID)
	}

	if len(done) == 0 {
		return nil
	}

	msg := "Sent " + cmd[0] // "nudge", "ping", or "poke".
	if len(done) > 1 {
		msg += "s"
	}
	msg = fmt.Sprintf("%s to: <@%s>.", msg, strings.Join(done, ">, <@"))
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

func authorSlackID(ctx workflow.Context, prURL string) string {
	pr, err := data.LoadBitbucketPR(ctx, prURL)
	if err != nil {
		return ""
	}

	author, ok := pr["author"].(map[string]any)
	if !ok {
		return ""
	}
	accountID, ok := author["account_id"].(string)
	if !ok {
		return ""
	}

	return users.BitbucketIDToSlackID(ctx, accountID, true)
}

// checkAndNudgeUser ensures that the user exists, is opted-in, and is a reviewer of the PR.
// It returns true if the attention state was updated. If not, it also posts an explanation,
// with the exception of self-nudges which are silently ignored (the user is nudging a group
// that they are also part of, when the user nudges only themselves this function isn't called).
func checkAndNudgeUser(ctx workflow.Context, event SlashCommandEvent, url, userID string) bool {
	// Silently ignore self-nudges.
	if userID == event.UserID {
		return false
	}

	// Check other conditions, send error messages as needed.
	user, optedIn, err := UserDetails(ctx, event, userID)
	if err != nil {
		return false
	}
	if !optedIn {
		msg := fmt.Sprintf(":no_bell: <@%s> isn't opted-in to use RevChat.", userID)
		_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
		return false
	}

	// Update the PR's attention state.
	ok, approved, err := data.Nudge(ctx, url, user.Email)
	if err != nil {
		PostEphemeralError(ctx, event, fmt.Sprintf("internal data error while nudging <@%s>.", userID))
		return ok // May be true despite the error: a valid reviewer, but failed to save it.
	}
	if ok {
		return true
	}

	// Not a tracked participant in this PR - but why?
	msg := fmt.Sprintf(":see_no_evil: <@%s> is not a tracked participant in this PR.", userID)
	if approved {
		msg = fmt.Sprintf(":+1: <@%s> already approved this PR.", userID)
	}
	_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	return false
}

// https://github.com/tzrikka/timpani/tree/main/images/nudge
var nudgeImageFiles = []string{
	"agnes_despicable_me_1.gif",
	"agnes_despicable_me_2.webp",
	"bobsponge.gif",
	"cat.gif",
	"dog_anim.webp",
	"fingers_crossed.gif",
	"fred_flintstone.gif",
	"homer_simpson.webp",
	"hyper_rpg.gif",
	"jimmy_fallon.webp",
	"kitten_anim.webp",
	"kitten.gif",
	"lilo_and_stitch.webp",
	"lion_king.webp",
	"lupita_nyongo.webp",
	"marshall_himym.webp",
	"monalng.gif",
	"mouse.gif",
	"please.gif",
	"puss_in_boots.gif",
	"true_bartleby.webp",
	"workaholics.gif",
}

// imageURL selects a random nudge image and returns its full URL.
// This function uses [workflow.SideEffect] to enforce determinism.
func imageURL(ctx workflow.Context, httpServer string) string {
	encoded := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(nudgeImageFiles))))
		if err != nil {
			logger.From(ctx).Error("crypto.rand.Int() failed", slog.Any("error", err))
			return ""
		}
		// https://github.com/tzrikka/timpani/blob/main/pkg/http/webhooks/server.go
		return fmt.Sprintf("https://%s/nudge/%s", httpServer, nudgeImageFiles[n.Int64()])
	})

	var url string
	if err := encoded.Get(&url); err != nil {
		logger.From(ctx).Error("failed to get side effect result", slog.Any("error", err))
		return ""
	}
	return url
}
