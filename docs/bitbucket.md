# Bitbucket

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

## Issues

Bitbucket has the following issues, which affect RevChat:

- There is no webhook event when a user edits a **reply** to a PR/file/commit comment
- There is no webhook event when a user un/likes a PR/file/commit comment/reply
