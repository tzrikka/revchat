# Slack Commands

## General Commands

RevChat offers various general-purpose commands for settings and information:

- `/revchat opt-in` - opt into being added to PR channels and receiving DMs
- `/revchat opt-out` - opt out of being added to PR channels and receiving DMs
- `/revchat reminders at <time in 12h or 24h format>` - on weekdays, using your timezone\
  &nbsp;
- `/revchat follow <1 or more @users or @groups>` - auto add yourself to PRs they create
- `/revchat unfollow <1 or more @users or @groups>` - stop following their PR channels\
  &nbsp;
- `/revchat status` - all the PRs you need to look at, as an author or a reviewer
  - Same output as [daily reminders](/README.md#daily-reminders), but you can run it at any time
  - The output of this command is visible only to the calling user\
    &nbsp;
- `/revchat status <1 or more @users or @groups>` - similar to `/revchat status` but:
  - Lists all the open PRs that the **mentioned users** created or need to review (not the calling user)
  - The output of this command is visible to the entire channel (not just the calling user)
  - This is the only command that doesn't require the user to opt-in
  - **Optional flags** (anywhere in the command after `status`):
    - `authors` - show only PRs that the specified user(s) created
    - `reviewers` - show only PRs that the user(s) need to review
    - `drafts` - show draft PRs too (which are hidden by default)
    - `tasks` - show a list of active tasks per PR (Bitbucket only)

> [!NOTE]
> The commands above can run in:
>
> - The RevChat app's messages tab
> - A PR channel that RevChat created
> - Any other public/private channel that the RevChat app was added to

## Inside PR Channels

These commands operate in the context of a specific PR, so you can run them only in PR channels:

- `/revchat who` - or - `/revchat whose turn`
- `/revchat my turn`
- `/revchat not my turn`\
  &nbsp;
- `/revchat freeze` - or - `/revchat freeze turns`
- `/revchat unfreeze`- or - `/revchat unfreeze turns`\
  &nbsp;
- `/revchat nudge <1 or more @users or @groups>`
  - `ping` or `poke` are also acceptable aliases for `nudge`\
    &nbsp;
- `/revchat explain` - who needs to approve each file, and have they?
- `/revchat clean` - remove unnecessary reviewers from the PR\
  &nbsp;
- `/revchat approve` or `lgtm` or `+1`
- `/revchat unapprove` or `-1`

The `explain` command analyzes the current code ownership and approvals in a PR channel:

_(Screenshot)_

The `clean` command removes all unnecessary reviewers from a PR: those who do not own any files and did not already approve the PR.
