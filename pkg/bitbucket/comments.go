package bitbucket

import (
	"bytes"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/bitbucket/activities"
	"github.com/tzrikka/revchat/pkg/data"
)

// LookupSlackFileID returns all the Slack IDs associated with a PR comment, if they exist.
func LookupSlackFileID(ctx workflow.Context, comment *Comment) (string, bool) {
	fileID, err := data.SwitchURLAndID(HTMLURL(comment.Links) + "/slack_file_id")
	if err != nil {
		logger.From(ctx).Error("failed to retrieve PR comment's Slack file ID",
			slog.Any("error", err), slog.String("pr_url", HTMLURL(comment.Links)))
		return "", false
	}

	return fileID, fileID != ""
}

// BeautifyInlineComment adds an informative prefix to the comment's text.
// If the comment contains a suggestion code block, it removes that block
// and also generates a diff snippet to attach to the Slack message instead.
func BeautifyInlineComment(ctx workflow.Context, comment *Comment, msg string) (string, []byte) {
	msg = inlineCommentPrefix(HTMLURL(comment.Links), comment.Inline) + msg
	msg = strings.TrimSpace(strings.TrimSuffix(msg, "\u200c"))

	suggestion, ok := extractSuggestionBlock(comment.Content.Raw)
	if !ok {
		return msg, nil
	}

	src := sourceFile(ctx, comment.Links["code"].HRef, comment.Inline.SrcRev)
	if src == "" {
		return msg, nil
	}

	diff := spliceSuggestion(ctx, comment.Inline, suggestion, src)
	if diff == nil {
		return msg, nil
	}

	if suggestion != "" {
		suggestion += "\n"
	}
	msg = strings.Replace(msg, "```suggestion\n"+suggestion, "```\n"+string(diff), 1)

	return msg, diff
}

// inlineCommentPrefix constructs a prefix to a PR comment,
// indicating its type (file/inline) and location (path and line/s).
func inlineCommentPrefix(commentURL string, in *Inline) string {
	var line1 int
	if in.StartFrom != nil {
		line1 = *in.StartFrom
		if in.StartTo != nil && *in.StartTo < line1 {
			line1 = *in.StartTo
		}
	} else if in.StartTo != nil {
		line1 = *in.StartTo
	}

	var line2 int
	if in.From != nil {
		line2 = *in.From
		if in.To != nil && *in.To > line2 {
			line2 = *in.To
		}
	} else if in.To != nil {
		line2 = *in.To
	}

	if line1 == 0 {
		line1 = line2
	}
	if line2 == 0 {
		line2 = line1
	}

	subject := "Inline"
	location := "in"
	switch line1 {
	case 0: // No line info.
		subject = "File"
	case line2: // Single line.
		location = fmt.Sprintf("in line %d in", line1)
	default: // Multiple lines.
		location = fmt.Sprintf("in lines %d-%d in", line1, line2)
	}

	return fmt.Sprintf("<%s|%s comment> %s `%s`:\n", commentURL, subject, location, in.Path)
}

// extractSuggestionBlock extracts the suggestion code block from a PR inline comment.
func extractSuggestionBlock(raw string) (string, bool) {
	_, s, ok := strings.Cut(raw, "```suggestion\n")
	if !ok {
		return "", false
	}

	i := strings.LastIndex(s, "```")
	if i == -1 {
		return "", false
	}

	return strings.TrimSuffix(s[:i], "\n"), true
}

var diffURLPattern = regexp.MustCompile(`/([^/]+)/([^/]+)/diff/([^?]+)(\?path=(.*))?$`)

func sourceFile(ctx workflow.Context, diffURL, hash string) string {
	matches := diffURLPattern.FindStringSubmatch(diffURL)
	if len(matches) < 6 {
		logger.From(ctx).Error("failed to parse Bitbucket diff URL", slog.String("diff_url", diffURL))
		return ""
	}

	file, err := activities.GetSourceFile(ctx, matches[1], matches[2], "", hash, matches[5])
	if err != nil {
		return ""
	}
	return file
}

// spliceSuggestion splices the suggestion into the source
// file content, and returns the result as a diff snippet.
func spliceSuggestion(ctx workflow.Context, in *Inline, suggestion, srcFile string) []byte {
	var firstLine, lastLine int
	if in.From != nil {
		firstLine, lastLine = *in.From, *in.From
	}
	if in.StartFrom != nil {
		firstLine = *in.StartFrom
	}

	if in.To != nil {
		firstLine, lastLine = *in.To, *in.To
	}
	if in.StartTo != nil {
		firstLine = *in.StartTo
	}

	lenFrom := lastLine - firstLine + 1
	lenTo := 0
	if suggestion != "" {
		lenTo = strings.Count(suggestion, "\n") + 1
	}

	// If the calculations above don't match the source or
	// the suggestion, fall back to a minimalistic code block.
	srcLines := strings.Split(srcFile, "\n")
	numLines := len(srcLines)
	if firstLine < 1 || lastLine < 1 || firstLine > numLines || lastLine > numLines || lenFrom <= 0 || lenTo < 0 {
		logger.From(ctx).Warn("mistake in generating pretty diff")
		return nil
	}

	var diff bytes.Buffer
	diff.WriteString(fmt.Sprintf("@@ -%d,%d ", firstLine, lenFrom))
	if lenTo > 0 {
		diff.WriteString(fmt.Sprintf("+%d,%d ", firstLine, lenTo))
	}
	diff.WriteString("@@\n")

	for _, line := range srcLines[firstLine-1 : lastLine] {
		diff.WriteString("-" + line + "\n")
	}

	if suggestion == "" {
		return diff.Bytes()
	}

	for line := range strings.SplitSeq(suggestion, "\n") {
		diff.WriteString("+" + line + "\n")
	}

	return diff.Bytes()
}
