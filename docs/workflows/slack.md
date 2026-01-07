# Slack Workflows

## Messages

### Message Created

- If the channel isn't mapped to a PR - ignore this event
- Determine who created the message, and load their Bitbucket/GitHub auth token (abort on errors)
- Convert Slack markdown to Bitbucket/GitHub markdown
- Append an invisible watermark to show that RevChat synced this message (to prevent an endless sync loop when RevChat receives a subsequent Bitbucket/GitHub comment creation event)
- Create a PR comment on behalf of the user
- Save a 2-way mapping between the Slack channel/thread/message IDs and the PR comment's URL
- (The subsequent Bitbucket/GitHub comment event will trigger bookmark updates in the channel)

### Message Changed

- If the channel isn't mapped to a PR - ignore this event
- Determine who edited the message, and load their Bitbucket/GitHub auth token (abort on errors)
- Convert Slack markdown to Bitbucket or GitHub markdown
- Identify the corresponding PR comment, and update it

### Message Deleted

- If the channel isn't mapped to a PR - ignore this event
- Determine who deleted the message, and load their Bitbucket/GitHub auth token (abort on errors)
- Identify the corresponding PR comment, and delete it
- Delete the 2-way mapping between the Slack channel/thread/message IDs and the PR comment's URL
- (The subsequent Bitbucket/GitHub comment event will trigger bookmark updates in the channel)

## Channel

### Channel Archived

- If the event was triggered by RevChat, or the channel isn't mapped to a PR - ignore it
- Clean up all of RevChat's data about this channel's PR

### Member Joined

- If the joining user isn't opted-in (i.e. added to the channel by someone other than RevChat), send them a DM with opt-in instructions

## Slash Command

### Opt-In

- If the user is already opted in - inform them and abort
- Create a new Thrippy OAuth link for Bitbucket or GitHub (depending on RevChat configuration)
- Send a message to the user with a link to start an OAuth 2.0 3-legged flow for this Thrippy link
- Wait up to 1 minute for this OAuth 2.0 flow to complete - otherwise abort, and inform the user to retry
- Save/update a mapping between the user's email address and Bitbucket/GitHub/Slack IDs
- Also associate the new Thrippy link with the user, to use these credentials in the future
- Set for the user a default reminder at 8:00 AM (in the user's timezone) on weekdays
- Inform the user how to change the time of this daily reminder (with a slash command)
- On any error or timeout before completion, delete the new Thrippy link

### Opt-Out

- If the user is already opted out - inform them and abort
- Delete the mapping between the user and their Thrippy link
- Delete this Thrippy link

### Set Weekday Reminder Time

- Parse, normalize, and check the specified time (RevChat supports several 12h and 24h formats)
  - `1` = `01` = `1:00` = `01:00` = `1a` = `1am` = `01:00 AM`
  - `13` = `13:00` = `1:00 p` = `01:00pm`, etc.
- Get the user's current timezone from their Slack profile
- Save the time and the current timezone

### Status

- Almost the same as [Scheduled Reminders](#scheduled-reminders), but triggered manually and only for the user running this command

## Scheduled Reminders

- Run this workflow every 30 minutes (with a jitter of 0-10 seconds)
  - Load the attention sets of all the PRs that RevChat tracks (a stateful mapping of PRs to reviewers)
  - Invert this into a mapping of RevChat users to the PRs in which it's their turn to take action
  - Load all the reminder times of all the RevChat users
  - Intersect these 2 mappings to keep only the users whose reminder time is now
  - For each such user, construct and send a Slack DM summarizing the details of the PRs in which it's their turn to take action
    - Title + PR link
    - Slack channel reference
    - PR details
      - Creation and last update times
      - Latest check results (if there are any)
      - Approvals (if there are any)
    - User-specific details
      - When was the last time you reviewed this PR?
      - Does it contain any files for which you are a code owner?
      - Does it contain any high-risk files?

## App Rate Limited
