# Bitbucket Workflows

## Pull Requests

### PR Created

- If the PR is a draft - abort
- Initialize a new Slack channel
  - Create a normalized version of the PR title
  - Create the Slack channel with the normalized name
    - If the channel already exists, retry with a numeric counter suffix
  - Set the channel's topic (Bitbucker URL)
  - Set the channel's description (PR title)
  - Set the channel's bookmarks (comments, tasks, approvals, commits)
  - Post an intro Slack message: mention PR author and the PR description
  - Post another message with a list of linkified references in the PR title
  - For all opted-in participants (author + reviewers)
    - Add to the Slack channel
    - Send a DM about it

### PR Updated

> [!NOTE]
> Bitbucket events don't pinpoint changes like GitHub, details can be determined only by storing a snapshot of the PR's metadata and comparing it between update events.

- If the PR is a draft - abort (ignore this event)
- If converted into a draft - same as [PR Declined](#pr-declined)
- If the PR is converted from a draft to marked-as-ready - same as [PR Created](#pr-created)
- If the PR doesn't have a Slack channel - abort
- Count the total number of commits in the PR (via API)
- If the PR title is edited
  - Mention the editing user in a Slack message
  - Update the Slack channel's description
  - List linkified references in a Slack message
  - Create a normalized version of the new PR title
  - Rename the Slack channel with the normalized name
    - If the channel already exists, retry with a numeric counter suffix
- If the PR description is deleted/edited
  - Mention the editing user and the new text in a Slack message
  - Update the Slack channel's description
  - List linkified references in a Slack message
- If reviewers are added and/or removed
  - Enumerate added and removed separately
  - Mention the user and the changes in a Slack message
  - Invite added reviewers to the Slack channel
  - Kick removed reviewers from the Slack channel
- If 1 or more commits are pushed to the PR branch
  - Mention the user and the commits in a Slack message
- If the PR destination branch is retargeted
  - Mention and user with name and links of both branches
- In any case, update the Slack channel's bookmarks

### PR Approved

- If the PR is a draft - abort (ignore this event)
- Mention the user and the action in a Slack message
- Update the Slack channel's bookmarks

### PR Unapproved

- Same as [PR Approved](#pr-approved)

### PR Merged

- If the PR is a draft - abort (ignore this event)
- Wait 5 seconds (to handle other asynchronous events, e.g. a PR closure comment)
- Clean up all of RevChat's data for this PR
- Mention the user and the specific action in a Slack message
- Archive the Slack channel

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

- If the PR is a draft - abort (ignore this event)
- If the comment was posted by RevChat (i.e. mirrored from Slack) - abort (don't repost it)
- Convert Bitbucket markdown to Slack markdown
- Post as a Slack message/reply on behalf of the user
- Update the Slack channel's bookmarks

### Comment Updated

- If the PR is a draft - abort (ignore this event)
- Convert Bitbucket markdown to Slack markdown
- Identify the corresponding Slack message/reply, and update it
- Update the Slack channel's bookmarks

### Comment Deleted

- If the PR is a draft - abort (ignore this event)
- Identify the corresponding Slack message/reply, and delete it
- Update the Slack channel's bookmarks

### Comment Resolved

- If the PR is a draft - abort (ignore this event)
- Identify the corresponding Slack message
  - Add an :ok: reaction to the message
  - Mention the user and the action in a reply
- Update the Slack channel's bookmarks

### Comment Reopened

- If the PR is a draft - abort (ignore this event)
- Identify the corresponding Slack message
  - Remove the :ok: reaction from the message
  - Mention the user and the action in a reply
- Update the Slack channel's bookmarks

## Repository

### Commit Comment Created

> [!CAUTION]
> Not implemented yet.

### Build Status Created

> [!CAUTION]
> Not implemented yet.

### Build Status Updated

> [!CAUTION]
> Not implemented yet.
