# Thrippy

## Simplest Setup Procedure

1. Download the latest Thrippy CLI for your platform: <https://github.com/tzrikka/thrippy/releases>

2. Extract the `thrippy` executable file from the downloaded archive and add it to your PATH

3. Verification:

   ```shell
   thrippy -v
   ```

4. Set up an HTTP tunnel: <https://github.com/tzrikka/thrippy/blob/main/docs/http_tunnel.md>

5. Start a dev server:

   ```shell
   thrippy server --dev --secrets.provider=file
   ```

6. Verification: <http://localhost:14470/healthz> replies with an HTTP 200 status

## Production Deployment

1. Configure Thrippy to use TLS or mTLS: <https://github.com/tzrikka/thrippy/tree/main/x509>

2. Configure Thrippy to use a real secrets manager, e.g. [HashiCorp Vault](https://developer.hashicorp.com/vault/install) or
   [AWS Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html):
   see <https://github.com/tzrikka/thrippy/blob/main/pkg/secrets/manager.go>

3. Run `thrippy server` **without** the flags in step 5 above
