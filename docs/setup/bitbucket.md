# Bitbucket

## Create a Bitbucket OAuth Consumer

1. [Bitbucket Workspaces](https://bitbucket.org/account/workspaces/) > Manage

2. Left Sidebar > Apps and Features > OAuth Consumers

3. Click the "Add consumer" button

   - Callback URL: `https://ADDRESS/callback`\
     (`ADDRESS` is Thrippy's [public address for HTTP webhooks](https://github.com/tzrikka/thrippy/blob/main/docs/http_tunnel.md))
   - Permissions
     - Account: read
     - Workspace membership: read
     - Pull requests: write
   - Click the "Save" button

## App Details to Copy

- Click the consumer name to see its details
  - Key (client ID)
  - Secret

## Configure Webhook Triggers

- Pull request (all)
  - Created
  - Updated
  - Approved
  - Approval removed
  - Changes Request created
  - Changes Request removed
  - Merged
  - Declined
  - Comment created
  - Comment updated
  - Comment deleted
  - Comment resolved
  - Comment reopened
- Repo
  - Commit comment created
  - Build status created
  - Build status updated
  - Issue created

## Define a Thrippy Link for the OAuth Consumer

```shell
$ thrippy create-link --template bitbucket-app-oauth --client-id "..." --client-secret "..."
New link ID: <Thrippy link ID>

$ thrippy set-creds <Thrippy link ID> --kv "webhook_secret=..."

$ thrippy start-oauth <Thrippy link ID>
Opening a browser with this URL: http://localhost:14470/start?id=<Thrippy link ID>
```

## Add the Thrippy Link to the Timpani Configuration

Add a `bitbucket` line under the `[thrippy.links]` section in the file `$XDG_CONFIG_HOME/timpani/config.toml`:

```toml
[thrippy.links]
bitbucket = "<Thrippy link ID>"
```

(If `$XDG_CONFIG_HOME` isn't set, the default path per OS is specified [here](https://github.com/tzrikka/xdg/blob/main/README.md#default-paths)).

## Known Issues

> [!IMPORTANT]
> Bitbucket has a few known issues which affect RevChat functionality:
>
> 1. Bitbucket sends a webhook event when a user edits a PR comment only if [10 minutes or more](https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Comment-updated) have passed since the comment was created or last updated (workaround: temporary polling after comments are created or updated, until the duration passes or the comment is deleted)
> 2. Bitbucket does not send a webhook event when a user creates/updates a task (workaround: check PR counter after every other PR event, when updating the channel's bookmarks)
> 3. Bitbucket does not send a webhook event when a user un/likes a PR/file/commit comment/reply

## Additional Information

<https://github.com/tzrikka/thrippy/tree/main/docs/atlassian/bitbucket/README.md>
