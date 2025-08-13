# Bitbucket

Detailed instructions: https://github.com/tzrikka/thrippy/tree/main/docs/atlassian/bitbucket/README.md

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

$ thrippy start-oauth <Thrippy link ID>
Opening a browser with this URL: http://localhost:14470/start?id=<Thrippy link ID>
```

## Known Limitations

Bitbucket has the following issues, which affect RevChat:

- There is no webhook event when a user edits a **reply** to a PR/file/commit comment
- There is no webhook event when a user un/likes a PR/file/commit comment/reply
