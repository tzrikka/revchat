# Temporal

[Temporal](https://temporal.io/) is an open-source platform for building reliable applications.

RevChat depends on a [custom Temporal search attribute](https://docs.temporal.io/search-attribute#custom-search-attribute):

- Name: `WaitingForSignals`
- Type: `KeywordList`

Instructions to create it: <https://docs.temporal.io/self-hosted-guide/visibility#create-custom-search-attributes>

Example command line:

```shell
temporal operator search-attribute create --name WaitingForSignals --type KeywordList
```

## Simplest Setup Procedure

1. Download and install the latest Temporal CLI for your platform: <https://temporal.io/setup/install-temporal-cli>

2. Verification:

   ```shell
   temporal -v
   ```

3. Start a dev server with a persistent SQLite database and the necessary search attribute:

   ```shell
   temporal server start-dev --db-filename ~/sqlite.db --search-attribute WaitingForSignals=KeywordList
   ```

   (Note: without the `--db-filename` argument, the server will use a temporary in-memory database)

4. Verification: see the dev server's web UI here: <http://localhost:8233/>

## Production Deployment

- Self hosted: <https://docs.temporal.io/self-hosted-guide>

- Temporal Cloud: <https://docs.temporal.io/cloud/overview>
