# Jira

This is needed only if you use Bitbucket, because the Bitbucket API does not allow searching for an unrecognized user email address in order to associate it with a known Atlassian account ID.

## Create an Atlassian User API Token

Create a user API token without scope limitations in: <https://id.atlassian.com/manage-profile/security/api-tokens>

> [!NOTE]
> The following **should** also work, but hasn't been tested yet:
>
> - User API tokens with the classic Jira scopes `read:jira-user` and `read:me`
> - Jira OAuth 2.0 (3LO) integrations, [as described in Thrippy documentation](https://github.com/tzrikka/thrippy/blob/main/docs/atlassian/jira/jira-app-oauth.md)

## Define a Thrippy Link for the User API Token

```shell
$ thrippy create-link --template jira-user-token
New link ID: <Thrippy link ID>

$ thrippy set-creds <Thrippy link ID> \
  --kv "base_url=https://your-domain.atlassian.net" \
  --kv "email=you@example.com" --kv "api_token=..."
```

## Add the Thrippy Link to the Timpani Configuration

Add a `jira` line under the `[thrippy.links]` section in the file `$XDG_CONFIG_HOME/timpani/config.toml`:

```toml
[thrippy.links]
jira = "<Thrippy link ID>"
```

(If `$XDG_CONFIG_HOME` isn't set, the default path per OS is specified [here](https://github.com/tzrikka/xdg/blob/main/README.md#default-paths)).

## Additional Information

<https://github.com/tzrikka/thrippy/tree/main/docs/atlassian/jira/README.md>
