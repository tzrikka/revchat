# RevChat

[![Go Reference](https://pkg.go.dev/badge/github.com/tzrikka/revchat.svg)](https://pkg.go.dev/github.com/tzrikka/revchat)
[![Code Wiki](https://img.shields.io/badge/Code_Wiki-gold?logo=googlegemini)](https://codewiki.google/github.com/tzrikka/revchat)
[![Go Report Card](https://goreportcard.com/badge/github.com/tzrikka/revchat)](https://goreportcard.com/report/github.com/tzrikka/revchat)

**RevChat** creates a seamless integration between Source Code Management platforms (such as GitHub, GitLab, and Bitbucket) and Instant Messaging platforms (such as Slack and Discord) to streamline code reviews and reduce the time to merge.

It automatically manages a dedicated channel for each pull/merge request, mirrors discussions and events between them, and nudges the relevant participants to respond to the latest updates.

## Why?

**RevChat** is a free, secure, self-hosted, open-source solution that enables:

- Real-time, relevant, informative, 2-way updates and discussions
- Easier collaboration and faster execution for teams of any size

It is designed to cut down:

- Delays due to unnoticed comments and asynchronous state changes
- Notification fatigue due to a firehose of details without context
- Questions like "Whose turn is it?" or "When should I look at this again?"

## Channel Creation

When a new PR is created, RevChat initializes a dedicated channel for it. These channels can be either public or private, depending on the organization's preference between searchability and security.

The channel's topic and description are set to the PR's URL and title, respectively, and RevChat posts the PR's description (with automatic fixing of markdown formats and emoji names between systems).

Example:

![New channel](/images/readme/new_channel.png)

Also note the automatic hyperlinking of IDs in PR titles, like in the PR's web UI (to reference related issues and other PRs):

![ID linkification](/images/readme/title_hyperlink_1.png)

![PR linkification](/images/readme/title_hyperlink_2.png)

Hyperlinking tips:

- IDs such as `[ID-123]` are stripped from the channel name (as in the screenshot above)
- To keep an ID in the channel name, don't surround it with `[]`
- PR (and GitHub issue) hyperlink format: `#123` and `repo#123`

## Channel Organization

All channel names follow this pattern: `_pr-(id)_(normalized-and-truncated-title)`

Repository names are not included, to conserve limited space for the PR title.

Slack's lexicographical sorting of channel names ensures that all RevChat channels are grouped together at the top of the "Channels" section (thanks to the `_pr` prefix) and ordered relatively chronologically (thanks to the ID after the prefix):

![Channels list](/images/readme/channels_list.png)

## Channel Bookmarks

Each channel also has a variety of deep links with **auto-updating** labels.

This serves 2 different purposes:

- Real-time snapshot of the PR state without having to leave Slack\
  -- but if needed --
- 1-click switching from the channel to a specific PR view/function

Example for a Bitbucket PR:

![Bitbucket bookmarks](/images/readme/bookmarks.png)

## Reviewers Sync

Adding/removing reviewers in a PR adds/removes them as channel members in real-time:

_(Screenshot)_

Important note 1: only opted-in users are added to these channels! Opted-out users are still mentioned, but not added.

Important note 2: this sync happens only in one direction, joining/leaving a channel does **not** add/remove the user as a reviewer in the PR!

## 2-Way Event Sync

RevChat automatically mirrors PR events in the Slack channel, and vice versa, in real-time:

- Comment threads
- Changes in the PR
- Check status updates, un/approvals
- Merge readiness announcement (only in Slack), to get the attention of authors/mergers and late reviewers
- Auto archiving of closed PRs

Example:

![PR event mirroring](/images/readme/sync_pr_events.png)

Here's another example of seamless 2-way updates of a PR comment thread and a channel message thread:

- New messages in Slack are created automatically in the PR on behalf of the users
- Likewise, comments and replies in the PR are posted (in the relevant thread) in Slack

Message from Slack:

_(Screenshot)_

Reply in the PR to the synchronized message:

_(Screenshot)_

Reaction in Slack to the synchronized reply:

_(Screenshot)_

Note that when synchronizing events between the PR and the channel, RevChat automatically converts user IDs, markdown formats, attachments, and emoji names between systems.

Another subtle but important UX note:

- When RevChat reflects a user **action** in Slack, it uses a profile link that **looks like (but isn't)** a Slack user mention
- When RevChat reflects a user **mention** in Slack, it uses an **actual** Slack user mention

This means that RevChat always identifies users clearly and consistently, but Slack grabs their attention only when they need to know something, not when RevChat merely echoes their own actions!

Commit pushes, check status updates, file comments, and inline comments include deep links - so when you see them in Slack you can jump directly into the right place in the PR's web UI to see the context and respond there:

![Deep links 1](/images/readme/deep_links_1.png)

![Deep links 2](/images/readme/deep_links_2.png)

Example of a resolved inline comment:

![Resolved comment](/images/readme/resolved_comment.png)

Lastly, code suggestions have extra styling in Slack:

![Code suggestion](/images/readme/code_suggestion.png)

## Daily Reminders

Sent as DMs from RevChat on weekdays. The default time is 8:00 AM in the user's timezone, but users may change this time and update the timezone with the `/revchat reminders` slash command (see the [next section](#slash-commands) below).

Reminders summarize the status and sensitivity of PRs:

- Special marking for drafts in the title
- Age (time since creation and last update)
- CI states (green/red), current approvals
- Count of the files that the reminded user owns

Which PRs are listed? Not necessarily all of them! RevChat tries to guess which PRs require your attention. For more details, see the section [Whose Turn Is It Anyway?](#whose-turn-is-it-anyway) below.

Example:

_(Screenshot)_

## Slash Commands

General commands:

- `/revchat opt-in` - opt into being added to PR channels and receiving DMs
- `/revchat opt-out` - opt out of being added to PR channels and receiving DMs
- `/revchat reminders at <time in 12h/24h format>` - on weekdays, using your timezone
- `/revchat status` - show your current PR states, both as an author and a reviewer

More commands inside PR channels:

- `/revchat who` / `whose turn` / `my turn` / `not my turn` / `[un]freeze [turns]`
- `/revchat nudge <1 or more @users or @groups>` / `ping <...>` / `poke <...>`
- `/revchat explain` - who needs to approve each file, and have they?
- `/revchat clean` - remove unnecessary reviewers from the PR
- `/revchat approve` or `lgtm` or `+1`
- `/revchat unapprove` or `-1`

The `status` command has the same output as [daily reminders](#daily-reminders), but users can run it at any time.

The `explain` command analyzes the current code ownership and approvals in a PR channel:

_(Screenshot)_

The `clean` command removes all unnecessary reviewers from a PR: those who do not own any files and did not already approve the PR.

## Whose Turn Is It Anyway?

RevChat tracks the state of each PR, remembers who is the author and who are the reviewers, and their turns to pay attention to the PR. This is reflected in daily reminders and the `/revchat status` slash command.

**Initial state:** if the PR has no reviewers, it's automatically the author's turn.

When one or more reviewers are added to a PR (when it's created or later), the turn switches from the author to them. However, this does not affect the turns of other existing reviewers, and the author in relation to those existing reviewers.

Reviewers of drafts:

- When a PR is marked as a draft, the reviewers who were added **before** that remain in the Slack channel and their turns are still tracked in relation to the author.
- However, reviewers who are added **while** a PR is in draft mode are not added to the Slack channel and to RevChat's turn tracking; they will be added only when the PR is marked as ready to review.

State transitions:

- When a **reviewer** posts one or more new comments / replies / code suggestions, RevChat switches their turn back to the author.
- This switch does not affect the turns of **other** reviewers, every author-reviewer pair has an isolated state, but...
- When the **author** posts one or more new comments / replies / code suggestions, RevChat switches their turn back to all the tracked reviewers.

However, an easier way for authors and reviewers to trigger state transitions without spamming the PR and the channel is with these slash commands **in the PR's Slack channel**:

- `/revchat my turn` - or - `/revchat not my turn`
- `/revchat freeze [turns]` - or - `/revchat unfreeze [turns]`
- `/revchat who` - or - `/revchat whose turn`

Note that pushing commits and retargeting branches has no effect on turns because these actions may be work in progress. Only discussions trigger state transitions.

**Final state:** when a reviewer approves a PR, or is unassigned from it, RevChat stops tracking their turn and switches back to the author permanently.

- When a reviewer is unassigned, they are also removed from the Slack channel.
- When a reviewer approves the PR, they remain in the Slack channel (until they leave manually, or until the PR is closed and the channel is auto-archived).

## End-User Onboarding

1. Slack left panel → Apps → right-click on `⋮` → Manage → Browse apps
2. Find the "RevChat" app → select it → click its "Open App" button
3. Run this slash command in the app's messages tab: `/revchat opt-in`
4. Optional: `/revchat reminders at <time in 12h or 24h format>`\
   (the default is 8:00 AM on weekdays, using your timezone)
