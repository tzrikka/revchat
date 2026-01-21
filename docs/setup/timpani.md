# Timpani

> [!IMPORTANT]
> [Temporal](./temporal.md) and [Thrippy](./thrippy.m) are prerequisites for this section!

## Simplest Setup Procedure

1. Download the latest Timpani server for your platform: <https://github.com/tzrikka/timpani/releases>

2. Extract the `timpani` executable file from the downloaded archive and add it to your PATH

3. Verification:

   ```shell
   timpani -v
   ```

4. Point [Thrippy's HTTP tunnel](https://github.com/tzrikka/thrippy/blob/main/docs/http_tunnel.md) to Timpani (port 14480) instead of Thrippy (port 14470)

5. Verification: <http://localhost:14480/healthz> replies with an HTTP 200 status
