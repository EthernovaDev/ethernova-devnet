# Ethernova VPS Bundle (Linux amd64)

Este bundle esta pensado para correr un nodo de **Ethernova** en una **VPS Ubuntu** (o cualquier Linux amd64).

Incluye:
- `ethernova` (binario Linux amd64)
- `genesis-mainnet.json` / `genesis-dev.json`
- `networks/` (bootnodes + static-nodes)
- `scripts/` (init/run/test)

## Requisitos
- Linux amd64 (Ubuntu 20.04/22.04/24.04 recomendado)
- `curl` (para `scripts/ethernova-test-rpc.sh`)
- Puertos P2P abiertos para mainnet: `30303/tcp` y `30303/udp`

## Quickstart (mainnet)

1) Extrae el tarball del release:
```bash
tar -xzf ethernova-<tag>-linux-amd64-vps.tar.gz
cd ethernova-<tag>-linux-amd64-vps
```

2) Inicializa el datadir (no borra datos si ya existe):
```bash
./scripts/ethernova-init.sh mainnet
```

3) Arranca el nodo (foreground):
```bash
./scripts/ethernova-run.sh mainnet
```

4) Verifica RPC:
```bash
./scripts/ethernova-test-rpc.sh mainnet http://127.0.0.1:8545
```

## Seguridad (RPC/WS)
- Por defecto los scripts atan RPC/WS a `127.0.0.1` (recomendado).
- NO expongas `8545/8546` publicamente. Si necesitas acceso remoto, usa SSH tunnel/VPN o firewall estricto.

## RPC 403 (geth/core-geth) con Docker/WSL / Host header
Si corres un explorer en Docker (Blockscout) apuntando a `http://host.docker.internal:8545`, geth/core-geth puede responder **HTTP 403: invalid host specified** por `--http.vhosts`.

Los scripts arrancan con:
```text
--http.vhosts=localhost,127.0.0.1,host.docker.internal
```

Dev-only (menos seguro):
```text
--http.vhosts=*
```

Opcional (si usas WS desde browsers, dev-only):
```text
--ws.origins=*
```

## Mining / Pool mode (opcional)
Por defecto `scripts/ethernova-run.sh` NO mina.

Ejemplo (mainnet):
```bash
export MINE=true
export ETHERBASE=0xYourPoolAddress
./scripts/ethernova-run.sh mainnet
```

## Archivos importantes
- Mainnet genesis: `genesis-mainnet.json` (chainId 121525; EVM compatibility fork at block 105000)
- Dev genesis: `genesis-dev.json` (chainId 77778)
- Bootnodes: `networks/mainnet/bootnodes.txt`
- Static peers: `networks/mainnet/static-nodes.json` (se copia al datadir en init)
