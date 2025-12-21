# RevChat

[![Go Reference](https://pkg.go.dev/badge/github.com/tzrikka/revchat.svg)](https://pkg.go.dev/github.com/tzrikka/revchat)
[![Go Report Card](https://goreportcard.com/badge/github.com/tzrikka/revchat)](https://goreportcard.com/report/github.com/tzrikka/revchat)

RevChat creates a seamless integration between Source Code Management platforms (such as GitHub, GitLab, and Bitbucket) and Instant Messaging platforms (such as Slack and Discord) to streamline code reviews and reduce the time to merge.

It automatically manages a dedicated channel for each pull/merge request, mirrors discussions and events between platforms, and nudges the relevant participants to responded to the latest updates.

It is a free, secure, and open-source solution that provides:

- Real-time, relevant, informative, 2-way updates and discussions
- Easier collaboration and faster execution for teams of any size

No more:

- Delays due to unnoticed comments and asynchronous state changes
- Notification fatigue due to a firehose of details without context
- Questions like "Whose turn is it?" or "When should I look at this again?"

## Daily Reminders

Sent as DMs from RevChat on weekdays. The default time is 8am in the user's timezone, but users may change this time and update the timezone with the `/revchat reminders` slash command (see the [next section](#slash-commands) below).

Reminders reflect the status and urgency of PRs:

- Special marking for drafts in the title
- Age (time since creation and last update)
- CI states (green/red), current approvals
- Count of the files that the reminded user owns

Which PRs are listed? Not necessarily all of them! RevChat tries to guess which PRs require your attention. For more details, see the section [Whose Turn Is It Anyway?](#whose-turn-is-it-anyway) below.

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

Caveat:

- RevChat will be notified about every `/revchat` slash command no matter where it was entered, but will be able to respond to it only in its own app messages tab, and in PR channels that it created, and any other channel where it was added in advance.
- It's possible to enable RevChat to respond in any public channel even if it wasn't added to it, but that requires a special [Slack app scope](https://docs.slack.dev/reference/scopes/chat.write.public).

The `status` command has the same output as daily reminders (see above), but users can run it at any time.

The `explain` command analyzes the current code ownership and approvals in a PR channel:

(TODO: screenshot/s)

The `clean` command removes all unnecessary reviewers from a PR: those who do not own any files, were not added manually, and did not already approve the PR.

## Whose Turn Is It Anyway?

RevChat tracks the state of each PR, remembers who is the author and who are the reviewers, and their turns to pay attention to the PR. This is reflected in daily reminders and the `/revchat status` Slack slash command.

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
