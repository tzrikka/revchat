# Bitbucket Workflows

## Pull Requests

### PR Created

- Initialize a new Slack channel
  - Construct a normalized version of the PR title
  - Create the Slack channel with the normalized name
    - If the channel already exists, retry with a numeric counter suffix
  - Set the channel's topic (to the Bitbucket URL)
  - Set the channel's description (to the PR title)
  - Set the channel's bookmarks (reviewers, comments, tasks, approvals, commits, files)
  - Post an intro Slack message: mention the PR author and the PR description
  - Post another message with a list of linkified references from the PR title
  - For all **opted-in** participants (author + reviewers)
    - Add to the Slack channel
    - Send a DM about it

### PR Updated

> [!NOTE]
> Bitbucket events don't pinpoint changes like GitHub, details can be determined only by storing a snapshot of the PR's metadata and comparing it between update events (or with Bitbucket API calls, but we're avoiding doing that whenever possible).

- If the PR is marked as a draft / ready to review - announce it
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
  - Update RevChat's snapshot of the PR files
- If the PR destination branch is retargeted
  - Mention and user with name and links of both branches
- In any case, update the Slack channel's bookmarks

### PR Approved

- If the PR doesn't have a Slack channel - abort (ignore this event)
- Mention the user and the action in a Slack message
- Update the Slack channel's bookmarks

### PR Unapproved

- Same as [PR Approved](#pr-approved)

### PR Merged

- If the PR doesn't have a Slack channel - abort (ignore this event)
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

- If the PR doesn't have a Slack channel - abort (ignore this event)
- If the comment was posted by RevChat (i.e. mirrored from Slack) - abort (don't repost it)
- Convert Bitbucket markdown to Slack markdown
- Post as a Slack message/reply on behalf of the user
- If the comment contains a code suggestion block
  - Generate a diff between the latest version of the file and the suggestion
  - Upload the diff as a text file to Slack
  - Replace the code suggestion block with the Slack file's permalink
- Update the Slack channel's bookmarks

### Comment Updated

- If the PR doesn't have a Slack channel - abort (ignore this event)
- Convert Bitbucket markdown to Slack markdown
- Identify the corresponding Slack message/reply, and update it
- Update the Slack channel's bookmarks

### Comment Deleted

- If the PR doesn't have a Slack channel - abort (ignore this event)
- Identify the corresponding Slack message/reply, and delete it
- Update the Slack channel's bookmarks

### Comment Resolved

- If the PR doesn't have a Slack channel - abort (ignore this event)
- Identify the corresponding Slack message
  - Add an :ok: reaction to the message
  - Mention the user and the action in a reply
- Update the Slack channel's bookmarks

### Comment Reopened

- If the PR doesn't have a Slack channel - abort (ignore this event)
- Identify the corresponding Slack message
  - Remove the :ok: reaction from the message
  - Mention the user and the action in a reply
- Update the Slack channel's bookmarks

## Repository

### Commit Comment Created

> [!CAUTION]
> Not implemented yet.

### Build Status Created

- Find the commit hash from the event in RevChat's collection of PR snapshots
  - We could use a Bitbucket API call, but we're avoiding doing that whenever possible
  - Finding a match in RevChat's data instead of using the Bitbucket API also ensures that the PR is being tracked (i.e. not a draft), and that the commit's status is relevant (i.e. this commit is still the latest in the branch)
- Update RevChat's snapshot of PR build results
  - If RevChat's snaphot references a different commit hash, forget the current results (they are obsolete)
- Post a message in the Slack channel
- Update the Slack channel's bookmarks, if needed

### Build Status Updated

- Same as [Build Status Created](#build-status-created)
