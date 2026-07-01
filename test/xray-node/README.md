# test/xray-node

Local xray-core node for integration-testing generated configs and (Phase 6) the
sync engine.

## Validate a generated config

The protocol registry's output is validated automatically by:

```
cd backend
go test -tags=integration -run Integration ./internal/protocols/
```

which builds VLESS Reality + VMess + Trojan via the registry and asserts
`xray -test` accepts the assembled `config.json` (using the official
`ghcr.io/xtls/xray-core` image).

## Run a node manually

Put a `config.json` under `./config/` and:

```
docker compose -f test/xray-node/docker-compose.yml up
```
