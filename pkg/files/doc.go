// Package files checks if a reviewer is a code owner of any files in a PR,
// and if a PR touches high-risk files.
//
// This functionality is based on comparing the PR's diffstat against the
// "CODEOWNERS" and "highrisk.txt" files in the PR's destination branch.
//
// This is used in Slack: daily reminders, the status slash command,
// and ready-to-merge notifications.
package files
