package activities

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/users"
	"github.com/tzrikka/timpani-api/pkg/slack"
)

// AlertError posts a Slack message about an error, if the Slack channel is configured.
// This function returns the original error for convenience, so it can be used inline.
func AlertError(ctx workflow.Context, channelName, prefix string, err error, details ...any) error {
	if channelName == "" || err == nil {
		return err
	}
	if prefix != "" {
		prefix += ": "
	}
	return alert(ctx, channelName, prefix, err, details...)
}

// AlertWarn posts a Slack message with a warning message, if the Slack channel is configured.
func AlertWarn(ctx workflow.Context, channelName, warning string, details ...any) {
	if channelName == "" || warning == "" {
		return
	}
	_ = alert(ctx, channelName, warning, nil, details...)
}

func alert(ctx workflow.Context, channelName, text string, err error, details ...any) error {
	msg := new(strings.Builder)
	if err != nil {
		fmt.Fprintf(msg, ":x: Error: %s%v", text, err)
	} else {
		fmt.Fprintf(msg, ":warning: %s", text)
	}

	// Workflow info.
	info := workflow.GetInfo(ctx)
	t := info.WorkflowStartTime
	fmt.Fprintf(msg, "\n\nWorkflow:\n  •  ID = `%s`", info.WorkflowExecution.ID)
	fmt.Fprintf(msg, "\n  •  Start = <!date^%d^{date_long_pretty} {time_secs}|%s>", t.Unix(), t.UTC().Format(time.RFC3339))

	// Extra details (optional).
	if len(details) > 0 {
		msg.WriteString("\n\nExtra details:")
	}
	for i := 0; i < len(details); i += 2 {
		fmt.Fprintf(msg, "\n  •  %v", details[i])
		if i+1 < len(details) {
			fmt.Fprintf(msg, " = %v", details[i+1])
		}
	}

	// Stack trace.
	pcs := make([]uintptr, 10)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	var frame runtime.Frame
	more := n > 0
	if more {
		msg.WriteString("\n\nStack trace:\n```")
	}
	for more {
		frame, more = frames.Next()
		fn := strings.Split(frame.Function, "/")
		fmt.Fprintf(msg, "%s - line %d\n    %s\n", frame.File, frame.Line, fn[len(fn)-1])
	}
	if n > 0 {
		msg.WriteString("```")
	}

	return errors.Join(err, PostMessage(ctx, channelName, msg.String()))
}

func DeleteMessage(ctx workflow.Context, channelID, timestamp string) error {
	if err := slack.ChatDelete(ctx, channelID, timestamp); err != nil {
		logger.From(ctx).Error("failed to delete Slack message", slog.Any("error", err),
			slog.String("channel_id", channelID), slog.String("msg_ts", timestamp))
		return err
	}
	return nil
}

func PostEphemeralMessage(ctx workflow.Context, channelID, userID, msg string) error {
	req := slack.ChatPostEphemeralRequest{Channel: channelID, User: userID, Text: msg}
	if err := slack.ChatPostEphemeral(ctx, req); err != nil {
		if e := err.Error(); strings.Contains(e, "channel_not_found") || strings.Contains(e, "not_in_channel") {
			err = PostMessage(ctx, userID, fmt.Sprintf("Couldn't send you this message in <#%s>:\n\n%s", channelID, msg))
		} else {
			logger.From(ctx).Error("failed to post Slack ephemeral message", slog.Any("error", err),
				slog.String("channel_id", channelID), slog.String("user_id", userID))
		}
		return err
	}
	return nil
}

func PostMessage(ctx workflow.Context, channelID, msg string) error {
	_, err := PostReplyAsUser(ctx, channelID, "", "", "", msg)
	return err
}

func PostReply(ctx workflow.Context, channelID, timestamp, msg string) (*slack.ChatPostMessageResponse, error) {
	return PostReplyAsUser(ctx, channelID, timestamp, "", "", msg)
}

func PostReplyAsUser(ctx workflow.Context, channelID, timestamp, name, icon, msg string) (*slack.ChatPostMessageResponse, error) {
	resp, err := slack.ChatPostMessage(ctx, slack.ChatPostMessageRequest{
		Channel:  channelID,
		ThreadTS: timestamp,
		Username: strings.TrimPrefix(name, "@"),
		IconURL:  icon,
		Text:     msg,
	})
	if err != nil {
		// If the channel is archived but we still store data for it, clean it up.
		if strings.Contains(err.Error(), "is_archived") {
			url, _ := data.SwitchURLAndID(ctx, channelID)
			data.CleanupPRData(ctx, channelID, url)
		}
		logger.From(ctx).Error("failed to post Slack message", slog.Any("error", err),
			slog.String("channel_id", channelID), slog.String("thread_ts", timestamp))
		return nil, err
	}
	return resp, nil
}

func PostDMWithImage(ctx workflow.Context, senderID, recipientID, msg, imageURL, altText string) error {
	name := users.SlackIDToDisplayName(ctx, senderID)

	if imageURL == "" {
		_, err := PostReplyAsUser(ctx, recipientID, "", name, users.SlackIDToIcon(ctx, senderID), msg)
		return err
	}

	_, err := slack.ChatPostMessage(ctx, slack.ChatPostMessageRequest{
		Channel:  recipientID,
		Username: strings.TrimPrefix(name, "@"),
		IconURL:  users.SlackIDToIcon(ctx, senderID),
		Blocks: []map[string]any{
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": msg,
				},
			},
			{
				"type":      "image",
				"image_url": imageURL,
				"alt_text":  altText,
			},
		},
	})
	if err != nil {
		logger.From(ctx).Error("failed to post Slack DM with image", slog.Any("error", err),
			slog.String("user_id", recipientID), slog.String("image_url", imageURL))
		return err
	}
	return nil
}

func UpdateMessage(ctx workflow.Context, channelID, timestamp, msg string) error {
	req := slack.ChatUpdateRequest{Channel: channelID, TS: timestamp, Text: msg}
	if err := slack.ChatUpdate(ctx, req); err != nil {
		logger.From(ctx).Error("failed to update Slack message", slog.Any("error", err),
			slog.String("channel_id", channelID), slog.String("msg_ts", timestamp))
		return err
	}
	return nil
}
