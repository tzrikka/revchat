# Slack

## Create a Slack App

Detailed instructions: <https://github.com/tzrikka/thrippy/blob/main/docs/slack/README.md>

### Permission Scopes

- [app_mentions:read](https://docs.slack.dev/reference/scopes/app_mentions.read)
- Bookmarks
  - [bookmarks:read](https://docs.slack.dev/reference/scopes/bookmarks.read)
  - [bookmarks:write](https://docs.slack.dev/reference/scopes/bookmarks.write)
- Channels
  - [channels:history](https://docs.slack.dev/reference/scopes/channels.history)
  - [channels:manage](https://docs.slack.dev/reference/scopes/channels.manage)
  - [channels:read](https://docs.slack.dev/reference/scopes/channels.read)
  - [channels:write.invites](https://docs.slack.dev/reference/scopes/channels.write.invites)
  - [channels:write.topic](https://docs.slack.dev/reference/scopes/channels.write.topic)
- Chat
  - [chat:write](https://docs.slack.dev/reference/scopes/chat.write)
  - [chat:write.customize](https://docs.slack.dev/reference/scopes/chat.write.customize)
  - Optional: [chat:write.public](https://docs.slack.dev/reference/scopes/chat.write.public) (to allow RevChat to respond to Slack slash commands in any public channel, even if it wasn't explicitly added to it)
- [commands](https://docs.slack.dev/reference/scopes/commands)
- Files
  - [files:read](https://docs.slack.dev/reference/scopes/files.read)
  - [files:write](https://docs.slack.dev/reference/scopes/files.write)
- Groups (private channels)
  - [groups:history](https://docs.slack.dev/reference/scopes/groups.history)
  - [groups:read](https://docs.slack.dev/reference/scopes/groups.read)
  - [groups:write](https://docs.slack.dev/reference/scopes/groups.write)
  - [groups:write.invites](https://docs.slack.dev/reference/scopes/groups.write.invites)
  - [groups:write.topic](https://docs.slack.dev/reference/scopes/groups.write.topic)
- IM (direct messages)
  - [im:history](https://docs.slack.dev/reference/scopes/im.history)
  - [im:write](https://docs.slack.dev/reference/scopes/im.write)
- Reactions
  - [reactions:read](https://docs.slack.dev/reference/scopes/reactions.read)
  - [reactions:write](https://docs.slack.dev/reference/scopes/reactions.write)
- User Groups
  - [usergroups:read](https://docs.slack.dev/reference/scopes/usergroups.read)
- Users
  - [users:read](https://docs.slack.dev/reference/scopes/users.read)
  - [users:read.email](https://docs.slack.dev/reference/scopes/users.read.email)
  - [users.profile:read](https://docs.slack.dev/reference/scopes/users.profile.read)

### App Home

In the "Show Tabs" sections, enable "Message Tab" and "Allow users to send Slash commands and messages from the messages tab".

## Define a Thrippy Link for the Slack App

Example - using a Slack app's static bot token:

```shell
$ thrippy create-link --template slack-bot-token
New link ID: <Thrippy link ID>

$ thrippy set-creds <Thrippy link ID> --kv "bot_token=..." --kv "signing_secret=..."
```

## Add the Thrippy Link to the Timpani Configuration

Add a `slack` line under the `[thrippy.links]` section in the file `$XDG_CONFIG_HOME/timpani/config.toml`:

```toml
[thrippy.links]
slack = "<Thrippy link ID>"
```

(If `$XDG_CONFIG_HOME` isn't set, the default path per OS is specified [here](https://github.com/tzrikka/xdg/blob/main/README.md#default-paths)).

## More Slack App Settings After Thrippy & Timpani

### Bot Event Subscriptions

Request URL: `https://ADDRESS/webhook/THRIPPY-LINK-ID`

(`ADDRESS` is Thrippy's [public address for HTTP webhooks](https://github.com/tzrikka/thrippy/blob/main/docs/http_tunnel.md), `THRIPPY-LINK-ID` is the Thrippy link ID that you added to the Timpani configuration file).

- [app_mention](https://docs.slack.dev/reference/events/app_mention)
- [channel_archive](https://docs.slack.dev/reference/events/channel_archive)
- [group_archive](https://docs.slack.dev/reference/events/group_archive)
- [member_joined_channel](https://docs.slack.dev/reference/events/member_joined_channel)
- [member_left_channel](https://docs.slack.dev/reference/events/member_left_channel)
- [message.channels](https://docs.slack.dev/reference/events/message.channels)
- [message.groups](https://docs.slack.dev/reference/events/message.groups)
- [message.im](https://docs.slack.dev/reference/events/message.im)
- [reaction_added](https://docs.slack.dev/reference/events/reaction_added)
- [reaction_removed](https://docs.slack.dev/reference/events/reaction_removed)

### Slash Command

(After configuring Thrippy and Timpani)

- Name: `/revchat`
- Request URL: `https://ADDRESS/webhook/THRIPPY-LINK-ID`
- Short description: `RevChat slash command`
- Usage hint: `help`
- Escape channels, users, and links sent to your app: `yes`
