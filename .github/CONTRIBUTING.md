## Contributing

Thanks for considering a contribution to **Ethernova CoreGeth** — even small fixes (docs, scripts, CI tweaks) help a lot.

### Where to contribute

- **Ethernova-specific changes** (genesis, chain config, scripts, docs, CI, releases): open a PR **in this repo**.
- **Generic fixes that benefit upstream** (CoreGeth / go-ethereum behavior not specific to Ethernova): consider opening the PR upstream too, then we’ll vendor/merge it here as needed.

> If you’re unsure where a change belongs, open an issue in this repo and describe the goal and the files touched.

### How to submit a PR

1. Fork the repo
2. Create a branch from `master` (or `main` if this repo uses `main`)  
3. Make your change
4. Run formatting/tests (see below)
5. Open a pull request with a clear description + steps to verify

### Coding guidelines

- Go code must follow official Go formatting (`gofmt`).
- Keep changes **small and focused** where possible.
- Prefer clear commit messages (include the area you touched), e.g.:
  - `scripts: fix init flags`
  - `docs: clarify mainnet fingerprint verification`
  - `rpc: make getWork smoke test stricter`
- If you modify scripts/docs, include **copy/paste commands** to reproduce and verify.

### Tests & verification

- Use the included scripts for quick validation on Windows:
  - `scripts/test-rpc.ps1`
  - `scripts/verify-mainnet.ps1`
  - `scripts/smoke-test-fees.ps1`

If you add new behavior, include:
- What changed
- How to test it
- Expected output

### Feature requests

Before requesting a feature, check if it can be solved via:
- CLI flags
- Chain config
- Existing scripts (or a small script addition)

Open an issue with:
- Use case
- Expected behavior
- Why existing tooling isn’t enough

### Developer documentation

- Ethernova operational docs: `docs/LAUNCH.md`
- Project docs: `docs/` (add links as they are created/expanded)
