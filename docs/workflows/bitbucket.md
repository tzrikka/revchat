# Bitbucket Workflows

## Pull Requests

### PR Created

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
  - Bitbucket PR details (to identify future update details)
  - Bitbucket PR diffstat (to count and analyze files)
  - Author and reviewers engagement for user reminders

### PR Updated

> [!NOTE]
> Bitbucket events don't pinpoint changes like GitHub, details can be determined only by storing a snapshot of the PR details and comparing it between update events (or with Bitbucket API calls, but we're avoiding doing that whenever possible).

- If the PR doesn't have a Slack channel - ignore this event
- If the PR is marked as a draft
  - Post a Slack message mentioning the triggering user and this action
- If the PR is marked as a ready for review
  - Post a Slack message mentioning the triggering user and this action
  - Add to the Slack channel any **opted-in** reviewers who were added to the PR while it was a draft
- Count the total number of commits in the PR (via API)
- If the PR title is edited
  - Post a Slack message mentioning the editing user, the new text, and any hyperlinked IDs in it
  - Update the Slack channel's description
  - Create a normalized version of the new PR title
  - Rename the Slack channel with the normalized name
    - If the channel already exists, retry with a numeric counter suffix
- If the PR description is deleted/edited
  - Post a Slack message mentioning the editing user, and the new text (with markdown support)
- If reviewers are added and/or removed
  - Enumerate added and removed separately
  - Post a Slack message mentioning the triggering user and the changes
  - If the PR isn't a draft, invite all the added **opted-in** reviewers to the Slack channel
  - Regardless of PR state, kick all the removed reviewers from the Slack channel
- If the PR destination branch is retargeted
  - Post a Slack message mentioning the triggering user, with names and links of both branches
- If 1 or more commits are pushed to the PR branch
  - Post a Slack message mentioning the committing user and their commits
  - Update RevChat's snapshot of the PR diffstat
- In any case, update the Slack channel's bookmarks

### PR Approved

- If the PR doesn't have a Slack channel - ignore this event
- Mention the user and the action in a Slack message
- Update the Slack channel's bookmarks

### PR Unapproved

- Same as [PR Approved](#pr-approved)

### PR Merged

- If the PR doesn't have a Slack channel - ignore this event
- Wait a few seconds (to handle other asynchronous events, e.g. a PR closure comment)
- Post a Slack message mentioning the closing user and the type of action (merge / decline)
- Archive the Slack channel
- Clean up all of RevChat's data about this PR
  - 2-way mappings between PR/comment URLs and Slack channel/thread/message IDs
  - Bitbucket PR details (to identify future update details)
  - Bitbucket PR diffstat (to count and analyze files)
  - Author and reviewers engagement for user reminders

### PR Declined

- Same as [PR Merged](#pr-merged)

## Change Requests

### Changes Request Created

- Same as [PR Approved](#pr-approved)

### Changes Request Removed

- Do nothing (ignore this event)

> [!CAUTION]
> Test the UX.

## Comments

### Comment Created

- If the PR doesn't have a Slack channel - ignore this event
- If the comment was posted by RevChat (i.e. mirrored from Slack) - ignore this event (don't repost it)
- Convert Bitbucket markdown to Slack markdown
- Post a Slack message/reply on behalf of the user
- If the comment contains a code suggestion block
  - Generate a diff between the latest version of the file and the suggestion
  - Upload the diff as a text file to Slack
  - Replace the code suggestion block with the Slack file's permalink
- Save a 2-way mapping between the PR comment's URL and the Slack channel/thread/message IDs
- Update the Slack channel's bookmarks

### Comment Updated

> [!CAUTION]
> According to [Atlassian's documentation](https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-updated),
> Bitbucket sends this webhook event only if 10 minutes or more have passed since the comment was created or last updated.
> Workaround: temporary polling after comments are created or updated, until the duration passes or the comment is deleted.

- If the PR doesn't have a Slack channel - ignore this event
- Convert Bitbucket markdown to Slack markdown
- Identify the corresponding Slack message/reply, and update it
- Update the Slack channel's bookmarks

### Comment Deleted

- If the PR doesn't have a Slack channel - ignore this event
- Identify the corresponding Slack message/reply, and delete it
- Delete the 2-way mapping between the PR comment's URL and the Slack channel/thread/message IDs
- Update the Slack channel's bookmarks

### Comment Resolved

- If the PR doesn't have a Slack channel - ignore this event
- Identify the corresponding Slack message
  - Add an :ok: reaction to the message
  - Mention the user and the action in a reply
- Update the Slack channel's bookmarks

### Comment Reopened

- If the PR doesn't have a Slack channel - ignore this event
- Identify the corresponding Slack message
  - Remove the :ok: reaction from the message
  - Mention the user and the action in a reply
- Update the Slack channel's bookmarks

## Repository

### Build Status Created

- Find the commit hash from the event in RevChat's collection of PR snapshots
  - We could use a Bitbucket API call, but we're avoiding doing that whenever possible
  - Finding a match in RevChat's data instead of using the Bitbucket API also ensures that the PR is being tracked, and that the commit's status is relevant (i.e. this commit is still the latest in the branch)
- Update RevChat's snapshot of PR build results
  - If RevChat's snaphot references a different commit hash, forget the current results (they are obsolete)
- Post a message in the Slack channel
- Update the Slack channel's bookmarks, if needed

### Build Status Updated

- Same as [Build Status Created](#build-status-created)
