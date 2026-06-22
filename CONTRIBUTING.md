# Contributing

## Git workflow on `main`

```bash
git fetch origin
git pull --rebase origin main
git push origin main
```

**Do not:**
- `git push -f` / `git push --force` / `git push --force-with-lease`
- `git push origin +main`
- `git reset --hard origin/main` then push a different branch onto `main`

**If a push is rejected (non-fast-forward):**
1. `git fetch origin`
2. `git pull --rebase origin main`
3. Resolve conflicts (if any)
4. `git push origin main`

## Before pushing

- `make fmt` (gofmt + goimports)
- `make lint` (golangci-lint)
- `make test` (and `make test-integration` when touching repositories/HTTP)

When in doubt, ask first, then push.
