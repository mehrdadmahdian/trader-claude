# Contributing to trader-claude

Thank you for your interest in trader-claude! This guide explains how to set up your development environment, follow our code style, and contribute changes.

## Development Setup

**Prerequisites:**
- Go 1.24+
- Node.js 20+
- Docker + Docker Compose

```bash
# 1. Clone the repository
git clone https://github.com/mehrdadmahdian/trader-claude.git
cd trader-claude

# 2. Copy environment template
cp .env.example .env
# Edit .env — set DB credentials, API keys, etc.

# 3. Start all services
make up

# 4. Verify everything works
curl http://localhost:8080/health
open http://localhost:5173
```

## Code Style & Formatting

### Backend (Go)

Format your code before committing:

```bash
make backend-fmt
```

This runs `gofmt`, `goimports`, and other linters. All Go code must pass:
- `gofmt` checks
- `golangci-lint` (see `.golangci.yml`)

Naming conventions:
- Package names: `lowercase`, `concise` (e.g., `adapter`, `registry`, not `market_adapters`)
- Interface names: `suffix` with `er` (e.g., `Adapter`, `Handler`)
- Unexported functions: `camelCase` starting lowercase

### Frontend (TypeScript/React)

Format your code before committing:

```bash
make frontend-fmt
```

This runs `prettier` on all `.ts`, `.tsx`, `.json` files.

Lint your code:

```bash
make frontend-lint
```

Naming conventions:
- Components: `PascalCase` (e.g., `DashboardPage.tsx`)
- Hooks: `camelCase` with `use` prefix (e.g., `useTheme`)
- Stores: `camelCase` with `Store` suffix (e.g., `themeStore`)
- Types: `PascalCase` (e.g., `Backtest`, `Signal`)

## Commit Messages

We follow [Conventional Commits](https://conventionalcommits.org). Each commit must start with a type and scope.

**Format:**
```
<type>(<scope>): <description>

[optional body]
```

**Types:**
| Type | Use | Example |
|---|---|---|
| `feat` | New feature | `feat(phase8): add monitor routes` |
| `fix` | Bug fix | `fix(backtest): handle zero-division error` |
| `docs` | Documentation | `docs: update README with API examples` |
| `test` | Tests only | `test(strategy): add RSI edge cases` |
| `refactor` | Code restructuring | `refactor(ws): simplify hub broadcast logic` |
| `perf` | Performance improvement | `perf(candle): optimize index query` |
| `chore` | Maintenance | `chore: upgrade dependencies` |
| `ci` | CI/CD changes | `ci: add GitHub Actions workflow` |

**Scopes (optional but recommended):**
- `phase-N` for phase work (e.g., `feat(phase10): add AI endpoint`)
- `component-name` for specific features (e.g., `fix(monitor): handle ws disconnect`)
- Leave blank for general changes

**Examples:**
```
feat(phase10): add AI chat endpoint

- Integrate OpenAI client
- Add prompt templating for backtest context
- Wire /api/v1/ai/chat POST handler

Closes #123
```

```
fix(backtest): handle MACD cross on same candle
```

## Branch Naming

```
feature/phase-N-description
fix/issue-description
docs/topic
refactor/component-name
chore/maintenance-task
```

**Examples:**
```
feature/phase9-telegram-notifications
fix/monitor-ws-reconnect
docs/extending-adapters
refactor/strategy-registry-cleanup
```

## Pull Request Process

1. **Create a branch** from `master`:
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes** and commit with conventional messages:
   ```bash
   git add backend/internal/api/my_handler.go
   git commit -m "feat(phase11): add analytics endpoint"
   ```

3. **Run all tests locally**:
   ```bash
   make backend-test
   make frontend-test
   make backend-lint
   make frontend-lint
   ```

4. **Push your branch**:
   ```bash
   git push origin feature/my-feature
   ```

5. **Open a pull request** against `master` with:
   - Clear title (same as first commit message)
   - Description of changes
   - Test plan (what you tested)
   - Any breaking changes or migration steps

6. **Respond to code review** — maintainers will review and request changes if needed.

## Testing

### Backend Tests

```bash
# Run all tests
make backend-test

# Run specific package
cd backend && go test ./internal/adapter/...

# Run with coverage
cd backend && go test -cover ./...

# Run specific test
cd backend && go test -run TestBinanceAdapter ./internal/adapter/...
```

**Guidelines:**
- Write tests for all public functions
- Use table-driven tests for multiple scenarios
- Mock external dependencies (HTTP, Redis, MySQL)
- Tests requiring MySQL are skipped if `DB_HOST` is unreachable

### Frontend Tests

```bash
# Run all tests
make frontend-test

# Run tests in watch mode
cd frontend && npm run test:watch

# Run with coverage
cd frontend && npm run test:coverage
```

**Guidelines:**
- Test components with React Testing Library (not Enzyme)
- Mock API calls with `jest.mock`
- Focus on behavior, not implementation details

## Adding Features

### Adding a Market Adapter

Want to add support for Kraken, Interactive Brokers, or another data source?

→ See **[Adding a Market Adapter](.claude/docs/adding-a-market.md)**

### Adding a Trading Strategy

Want to implement a Bollinger Bands, Stochastic, or custom strategy?

→ See **[Adding a Trading Strategy](.claude/docs/adding-a-strategy.md)**

## Git Workflow

**Never:**
- Force push to `master` (use `--force-with-lease` only if absolutely necessary)
- Commit `.env` files or sensitive credentials
- Skip pre-commit hooks
- Use `git rebase -i` in public PRs

**Always:**
- Fetch `master` before starting new work: `git fetch origin master:master`
- Create a new branch from `master`: `git checkout master && git pull && git checkout -b my-branch`
- Rebase on latest master before pushing: `git rebase origin/master`

## Documentation

If you add a feature, update the relevant docs:

- **API Changes**: Update `.claude/docs/api.md`
- **Database Changes**: Update `.claude/docs/database.md` + `backend/migrations/`
- **WebSocket Changes**: Update `.claude/docs/websocket.md`
- **Architecture Changes**: Update `.claude/docs/architecture.md`

## Troubleshooting

### Docker Issues

```bash
# Restart all services
make down && make up

# View logs
make logs

# Check container status
docker compose ps
```

### Database Issues

```bash
# Reset database (WARNING: deletes all data)
make down-v && make up

# Access MySQL CLI
make db-shell

# Re-run migrations
make migrate
```

### Go Module Issues

```bash
# Update dependencies
cd backend && go get -u ./...
go mod tidy

# Verify go.mod
go mod verify
```

### Node Module Issues

```bash
# Clear node_modules and reinstall
cd frontend && rm -rf node_modules package-lock.json
npm install
```

## Performance Considerations

- **Database**: Use composite indexes (`symbol, market, timeframe, timestamp` on candles)
- **Cache**: Use Redis for hot data (recent candles, session state)
- **Worker Pool**: Backtest jobs are queued; don't block the main thread
- **WebSocket**: Use channels for fan-out (broadcasts to multiple clients)
- **API**: Paginate large result sets (limit 100, offset support)

## Security Considerations

- **Secrets**: Never commit `.env` or API keys; use environment variables
- **SQL Injection**: Always use GORM parameterized queries, never string concat
- **CORS**: Whitelist origins in `config.go`
- **JWT**: All authenticated routes check JWT tokens (coming in Phase 5)
- **Rate Limiting**: Consider adding middleware for public endpoints

## Questions?

1. Check the **[Architecture Guide](.claude/docs/architecture.md)** for system design
2. Review the **[API Reference](.claude/docs/api.md)** for endpoint details
3. Look at existing code for patterns (e.g., `internal/adapter/binance.go`)
4. Open an issue or discussion on GitHub

---

**Thanks for contributing!** Every bug fix, feature, and documentation improvement makes trader-claude better.
