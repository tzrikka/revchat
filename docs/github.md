# GitHub

Detailed instructions: https://github.com/tzrikka/thrippy/blob/main/docs/github/README.md

## GitHub App

## Thrippy Link

Example - using a GitHub app:

```shell
$ thrippy create-link --template github-app-jwt --client-id "..." --param "app_name=..."
New link ID: <Thrippy link ID>

$ thrippy set-creds <Thrippy link ID> --kv "client_id=..." \
  --kv "private_key=@.../github_app.private-key.pem" --kv "webhook_secret=..."

$ thrippy start-oauth <Thrippy link ID>
Opening a browser with this URL: http://localhost:14470/start?id=<Thrippy link ID>
```
