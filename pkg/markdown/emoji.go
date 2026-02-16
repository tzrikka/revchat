package markdown

import (
	"regexp"
	"strings"
)

var (
	BitbucketToSlackEmojiTwoWay = map[string]string{
		":frame_photo:": ":frame_with_picture:",
		":robot:":       ":robot_face:",

		// Smileys.
		":rofl:":         ":rolling_on_the_floor_laughing:",
		":slight_frown:": ":slightly_frowning_face:",
		":slight_smile:": ":slightly_smiling_face:",
		":upside_down:":  ":upside_down_face:",

		// People (gendered and gender-neutral).
		":man_facepalming:":  ":man-facepalming:",
		":man_gesturing_no:": ":man-gesturing-no:",
		":man_gesturing_ok:": ":man-gesturing-ok:",
		":man_shrugging:":    ":man-shrugging:",

		":woman_facepalming:":  ":woman-facepalming:",
		":woman_gesturing_no:": ":woman-gesturing-no:",
		":woman_gesturing_ok:": ":woman-gesturing-ok:",
		":woman_shrugging:":    ":woman-shrugging:",

		":person_bald:": ":bald_person:",
	}

	SlackToBitbucketEmojiOneWay = map[string]string{
		":face_palm:": ":man_facepalming:",    // No gender-neutral version in Bitbucket.
		":memo:":      ":pencil:",             // Slack supports both, Bitbucket doesn't.
		":no_good:":   ":man_gesturing_no:",   // No gender-neutral version in Bitbucket.
		":ok_woman:":  ":woman_gesturing_ok:", // Slack supports both.
		":shrug:":     ":man_shrugging:",      // No gender-neutral version in Bitbucket.
		":thanks:":    ":pray:",               // Unofficial but common alias in Slack.
	}

	// GitHubToSlackEmojiTwoWay is based on: https://api.github.com/emojis
	GitHubToSlackEmojiTwoWay = map[string]string{
		":framed_picture:": ":frame_with_picture:",
		":robot:":          ":robot_face:",

		// Smileys.
		":rofl:": ":rolling_on_the_floor_laughing:",

		// People (gendered and gender-neutral).
		":man_facepalming:": ":man-facepalming:",
		":no_good_man:":     ":man-gesturing-no:",
		":ok_man:":          ":man-gesturing-ok:",
		":man_shrugging:":   ":man-shrugging:",

		":woman_facepalming:": ":woman-facepalming:",
		":no_good_woman:":     ":woman-gesturing-no:",
		// ":ok_woman:":       ":woman-gesturing-ok:", // Attention: see ":ok_person:" key!
		":woman_shrugging:": ":woman-shrugging:",

		":person_bald:": ":bald_person:",
		":facepalm:":    ":face_palm:",
		// ":ok_person:": ":ok_woman:", // Attention: see ":ok_woman:" key!
	}

	// SlackToGitHubEmojiOneWay is based on: https://api.github.com/emojis
	SlackToGitHubEmojiOneWay = map[string]string{
		":thanks:": ":pray:", // Unofficial but common alias in Slack.
	}

	SlackSkinTonePattern = regexp.MustCompile(`:skin-tone-\d:`)
)

// BitbucketToSlackEmoji fixes small inconsistencies in emoji names between Bitbucket and Slack. It doesn't
// address all of them yet, but eventually it should. It is idempotent, and the inverse of [SlackToBitbucketEmoji].
func BitbucketToSlackEmoji(text string) string {
	for from, to := range BitbucketToSlackEmojiTwoWay {
		text = strings.ReplaceAll(text, from, to)
	}
	return text
}

// SlackToBitbucketEmoji fixes small inconsistencies in emoji names between Bitbucket and Slack. It doesn't
// address all of them yet, but eventually it should. It is idempotent, and the inverse of [BitbucketToSlackEmoji].
// Note that some emojis are supported in Slack but not in Bitbucket, so we adjust them to the closest equivalent.
func SlackToBitbucketEmoji(text string) string {
	for to, from := range BitbucketToSlackEmojiTwoWay {
		text = strings.ReplaceAll(text, from, to)
	}
	for from, to := range SlackToBitbucketEmojiOneWay {
		text = strings.ReplaceAll(text, from, to)
	}
	return trimSlackEmojiSkinTones(text)
}

// GitHubToSlackEmoji fixes small inconsistencies in emoji names between GitHub and Slack. It doesn't
// address all of them yet, but eventually it should. It is idempotent, and the inverse of [SlackToGitHubEmoji].
func GitHubToSlackEmoji(text string) string {
	for from, to := range GitHubToSlackEmojiTwoWay {
		text = strings.ReplaceAll(text, from, to)
	}

	// Special cases related to ":ok_woman:": the order matters, so we can't
	// rely on regular map iteration because it's non-deterministic by design.
	text = strings.ReplaceAll(text, ":ok_woman:", ":woman-gesturing-ok:")
	text = strings.ReplaceAll(text, ":ok_person:", ":ok_woman:")

	return text
}

// SlackToGitHubEmoji fixes small inconsistencies in emoji names between GitHub and Slack. It doesn't
// address all of them yet, but eventually it should. It is idempotent, and the inverse of [GitHubToSlackEmoji].
// Note that some emojis are supported in Slack but not in GitHub, so we adjust them to the closest equivalent.
func SlackToGitHubEmoji(text string) string {
	for to, from := range GitHubToSlackEmojiTwoWay {
		text = strings.ReplaceAll(text, from, to)
	}

	// Special cases related to ":ok_woman:": the order matters, so we can't
	// rely on regular map iteration because it's non-deterministic by design.
	text = strings.ReplaceAll(text, ":ok_woman:", ":ok_person:")
	text = strings.ReplaceAll(text, ":woman-gesturing-ok:", ":ok_woman:")

	for from, to := range SlackToGitHubEmojiOneWay {
		text = strings.ReplaceAll(text, from, to)
	}
	return trimSlackEmojiSkinTones(text)
}

// trimSlackEmojiSkinTones removes skin tone suffixes from emoji names,
// which are supported in Slack but not in Bitbucket or GitHub.
func trimSlackEmojiSkinTones(text string) string {
	return SlackSkinTonePattern.ReplaceAllString(text, "")
}
