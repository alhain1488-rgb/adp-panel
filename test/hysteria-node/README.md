# test/hysteria-node

Local Hysteria2 engine (sing-box) node for integration-testing generated configs
and (Phase 6) the sync engine.

## Validate a generated config

```
cd backend
go test -tags=integration -run Integration ./internal/protocols/
```

builds a Hysteria2 inbound via the registry and asserts `sing-box check` accepts
the assembled config (using the official `ghcr.io/sagernet/sing-box` image, with
a generated self-signed cert).

## Run a node manually

Put a sing-box `config.json` under `./config/` and:

```
docker compose -f test/hysteria-node/docker-compose.yml up
```
