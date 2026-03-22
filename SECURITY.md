# Security Policy

This security policy applies to **Ethernova (CoreGeth fork)** in this repository.

---

## Supported Versions

We support **only versions published in this repository’s Releases**:

- **Latest release**: recommended for production.
- Older releases: support status is **UNKNOWN** (define whether you support “N-1” or “latest only”).

Releases:
- https://github.com/EthernovaDev/ethernova-coregeth/releases

> Note: Builds from `master/main` may change without notice and are not considered stable for production deployments.

---

## Scope

This policy covers:
- Execution client binaries / `cmd/`
- Ethernova network configuration (genesis, chain config, forks)
- Operational tooling included in this repo (PowerShell scripts, CI helpers, docs)

Out of scope (unless proven otherwise):
- External infrastructure (hosting providers, third-party pools, public RPC endpoints)
- Contracts/apps deployed on the network (unless included in this repository)

---

## Reporting a Vulnerability

**Do NOT open a public GitHub Issue** and do not share details publicly.

Report via one of the following channels:

1) **Email (preferred):** (ethernovacoin@gmail.com)  

Please include:
- Exact version / commit hash
- OS + environment details (Windows build, etc.)
- Expected impact (DoS, RCE, consensus, funds, bypass, data leak)
- Steps to reproduce / PoC (if available)
- Relevant logs (do **not** include private keys)

---

## Disclosure Process

After receiving a report:

- We will acknowledge receipt
- We will investigate and prepare a fix/mitigation
- We will coordinate a responsible disclosure timeline
- We will publish:
  - A release containing the fix
  - A GitHub Security Advisory (when applicable)
  - Release notes describing impact and upgrade steps
