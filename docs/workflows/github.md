# GitHub Workflows

## Pull Requests

### PR Opened

- Initialize a new Slack channel
  - Construct a normalized version of the PR title
  - Create a new channel with the normalized name
  - Set the channel's topic (to the Bitbucket URL)
  - Set the channel's description (to the PR title)
  - Set the channel's bookmarks
  - Post an introduction message, containing:
    - Mention of the PR author
    - PR title, with optional hyperlinking of IDs (e.g. to reference issues and other PRs)
    - PR description (with markdown support)
  - Add all the **opted-in** participants (author + reviewers) as members
- Initialize RevChat's data about this PR
  - 2-way mapping between the PR's URL and Slack channel ID
  - GitHub PR diffstat (to analyze files)
  - Author and reviewers engagement for user reminders

### PR Closed

- If the PR doesn't have a Slack channel - ignore this event
- Wait a few seconds (to handle other asynchronous events, e.g. a PR closure comment)
- Post a Slack message mentioning the closing user and the type of action (merge / close)
- Archive the Slack channel
- Clean up all of RevChat's data about this PR
  - 2-way mappings between PR/comment URLs and Slack channel/thread/message IDs
  - GitHub PR diffstat (to count and analyze files)
  - Author and reviewers engagement for user reminders

### PR Reopened

Same as [PR Opened](#pr-opened)

> [!NOTE]
> Why the same? See <https://docs.slack.dev/reference/methods/conversations.unarchive>:
> bot tokens (`xoxb-...`) cannot currently be used to unarchive conversations. Use a
> user token (`xoxp-...`) to unarchive conversations rather than a bot token.
>
> Partial workaround: treat this event type as a new PR. Drawback: losing pre-archiving channel history.

### PR Marked as a Draft

- If the PR doesn't have a Slack channel - ignore this event
- Post a Slack message mentioning the triggering user and this action
- Update the Slack channel's bookmarks

### PR Marked as Ready for Review

- Same as [PR Marked as a Draft](#pr-marked-as-a-draft)
- Add to the Slack channel any **opted-in** reviewers who were added to the PR while it was a draft

### PR Review Requested

### PR Review Request Removed

### PR Assigned

### PR Unassigned

### PR Edited

- If the PR doesn't have a Slack channel - ignore this event
- If the PR title is edited
  - Post a Slack message mentioning the editing user, the new text, and any hyperlinked IDs in it
  - Update the Slack channel's description
  - Create a normalized version of the new PR title
  - Rename the Slack channel with the normalized name
    - If the channel already exists, retry with a numeric counter suffix
- If the PR description body is deleted/edited
  - Post a Slack message mentioning the editing user, and the new text (with markdown support)
- If the PR base branch is changed
  - Post a Slack message mentioning the triggering user, with names and links of both branches
- In any case, update the Slack channel's bookmarks

### PR Synchronized

- If the PR head branch is changed
  - Post a Slack message mentioning the triggering user, with names and links of both branches
- If 1 or more commits are pushed to the PR branch
  - Post a Slack message mentioning the committing user and their commits
  - Update RevChat's snapshot of the PR diffstat
- In any case, update the Slack channel's bookmarks

## Pull Request Reviews

### PR Review Submitted

### PR Review Edited

### PR Review Dismissed

## Pull Request Review Comments

### PR Review Comment Created

### PR Review Comment Edited

### PR Review Comment Deleted

## Pull Request Review Threads

### PR Review Thread Resolved

### PR Review Thread Unresolved

## Issue Comments

### Issue Comment Created

### Issue Comment Edited

### Issue Comment Deleted
