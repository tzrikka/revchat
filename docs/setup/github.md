# GitHub

## Create a GitHub App

Detailed instructions: <https://github.com/tzrikka/thrippy/blob/main/docs/github/README.md>

### Repository Permissions

- [Actions](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps?apiVersion=2022-11-28#repository-permissions-for-actions) - read only
- [Checks](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps?apiVersion=2022-11-28#repository-permissions-for-checks) - read only
- [Issues](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps?apiVersion=2022-11-28#repository-permissions-for-issues) - read & write
- [Metadata](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps?apiVersion=2022-11-28#repository-permissions-for-metadata) - read only (mandatory)
- [Pull requests](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps?apiVersion=2022-11-28#repository-permissions-for-pull-requests) - read & write

### Subscribe to Events

- [Issue comment](https://docs.github.com/en/webhooks/webhook-events-and-payloads#issue_comment)
- [Pull request](https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request)
- [Pull request review](https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review)
- [Pull request review comment](https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_comment)
- [Pull request review thread](https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request_review_thread)

## Define a Thrippy Link for the GitHub App

```shell
$ thrippy create-link --template github-app-jwt --client-id "..." --param "app_name=..."
New link ID: <Thrippy link ID>

$ thrippy set-creds <Thrippy link ID> --kv "client_id=..." \
  --kv "private_key=@.../github_app.private-key.pem" --kv "webhook_secret=..."

$ thrippy start-oauth <Thrippy link ID>
Opening a browser with this URL: http://localhost:14470/start?id=<Thrippy link ID>
```

## Add the Thrippy Link to the Timpani Configuration

Add a `github` line under the `[thrippy.links]` section in the file `$XDG_CONFIG_HOME/timpani/config.toml`:

```toml
[thrippy.links]
github = "<Thrippy link ID>"
```

(If `$XDG_CONFIG_HOME` isn't set, the default path per OS is specified [here](https://github.com/tzrikka/xdg/blob/main/README.md#default-paths)).
