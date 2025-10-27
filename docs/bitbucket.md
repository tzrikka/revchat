# Bitbucket

Detailed instructions: <https://github.com/tzrikka/thrippy/tree/main/docs/atlassian/bitbucket/README.md>

## OAuth Consumer Permissions

- Account: read
- Workspace membership: read
- Pull requests: write
- Webhooks: read and write

## Webhook Triggers

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

## Thrippy Link

Example - using a Bitbucket workspace's OAuth consumer:

```shell
$ thrippy create-link --template bitbucket-app-oauth --client-id "..." --client-secret "..."
New link ID: <Thrippy link ID>

$ thrippy set-creds <Thrippy link ID> --kv "webhook_secret=..."

$ thrippy start-oauth <Thrippy link ID>
Opening a browser with this URL: http://localhost:14470/start?id=<Thrippy link ID>
```

## Known Issues

> [!IMPORTANT]
> Bitbucket has a few known issues which affect RevChat functionality:
>
> 1. Bitbucket does not send a webhook event when a user edits
>    - **PR** comments (as opposed to a file/commit comments)
>    - **Replies** to PR/file/commit comments
> 2. Bitbucket does not send a webhook event when a user un/likes a PR/file/commit comment/reply
> 3. Bitbucket does not send a webhook event when a user create/updates a task
