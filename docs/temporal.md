# Temporal

RevChat depends on a [custom Temporal search attribute](https://docs.temporal.io/search-attribute#custom-search-attribute):

- Name: `WaitingForSignals`
- Type: `KeywordList`

Instructions to create it in your Temporal server: https://docs.temporal.io/self-hosted-guide/visibility#create-custom-search-attributes

For example, to create it in a self-hosted server, run this command:

```shell
temporal operator search-attribute create --name WaitingForSignals --type KeywordList
```
