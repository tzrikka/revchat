package commands

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
)

func Nudge(ctx workflow.Context, event SlashCommandEvent, imagesHTTPServer string) error {
	url := prDetailsFromChannel(ctx, event)
	if url == nil {
		return nil // Not a server error as far as we're concerned.
	}

	users := extractAtLeastOneUserID(ctx, event)
	if len(users) == 0 {
		return nil
	}

	parts := strings.SplitN(event.Text, " ", 2) // parts[0] = "nudge", "ping", or "poke".
	if len(users) == 1 && users[0] == event.UserID {
		msg := ":confused: Why are you trying to %s yourself? Treating this as a `%s my turn` command..."
		activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, fmt.Sprintf(msg, parts[0], event.Command))
		return MyTurn(ctx, event)
	}

	done := make([]string, 0, len(users))
	for _, userID := range users {
		// Check that the user is eligible to be nudged.
		if !checkUserBeforeNudging(ctx, event, url[0], userID) {
			continue
		}

		msg := fmt.Sprintf(":pleading_face: Please take a look at <#%s> :pray:", event.ChannelID)
		altText := "Tip: click the collapse arrow above this image to hide it, as a self-reminder after completing this task"
		if err := activities.PostDMWithImage(ctx, userID, msg, imageURL(ctx, imagesHTTPServer), altText); err != nil {
			PostEphemeralError(ctx, event, fmt.Sprintf("failed to send a %s to <@%s>.", parts[0], userID))
			continue
		}

		done = append(done, userID)
	}

	if len(done) == 0 {
		return nil
	}

	msg := "Sent " + parts[0] // "Nudge", "ping", or "poke".
	if len(done) > 1 {
		msg += "s"
	}
	msg = fmt.Sprintf("%s to: <@%s>.", msg, strings.Join(done, ">, <@"))
	return activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
}

// checkUserBeforeNudging ensures that the user exists, is opted-in, and is a reviewer of the PR.
func checkUserBeforeNudging(ctx workflow.Context, event SlashCommandEvent, url, userID string) bool {
	if userID == event.UserID {
		// Silently ignore self-nudges when there are additional users
		// (likely caused by nudging an entire group that includes the user).
		return false
	}

	user, optedIn, err := UserDetails(ctx, event, userID)
	if err != nil {
		return false
	}
	if !optedIn {
		msg := fmt.Sprintf(":no_bell: <@%s> isn't opted-in to use RevChat.", userID)
		_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
		return false
	}

	ok, err := data.Nudge(url, user.Email)
	if err != nil {
		logger.From(ctx).Error("failed to nudge user", slog.Any("error", err),
			slog.String("pr_url", url), slog.String("user_id", userID))
		PostEphemeralError(ctx, event, fmt.Sprintf("internal data error while nudging <@%s>.", userID))
		return ok // May be true despite the error: a valid reviewer, but failed to save it.
	}
	if !ok {
		msg := fmt.Sprintf(":no_good: <@%s> is not a tracked participant in this PR.", userID)
		_ = activities.PostEphemeralMessage(ctx, event.ChannelID, event.UserID, msg)
	}

	return ok
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

func imageURL(ctx workflow.Context, httpServer string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(nudgeImageFiles))))
	if err != nil {
		logger.From(ctx).Error("crypto.rand.Int() failed", slog.Any("error", err))
		return ""
	}
	// https://github.com/tzrikka/timpani/blob/main/pkg/http/webhooks/server.go
	return fmt.Sprintf("https://%s/nudge/%s", httpServer, nudgeImageFiles[n.Int64()])
}
