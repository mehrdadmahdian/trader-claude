# Todo List

> Full phase-by-phase plan: `.claude/docs/phases.md`

---

## Status Summary

| Phase | Description | Status |
|---|---|---|
| 1 | Scaffold & Infrastructure | ✅ Complete |
| 2 | Market Data Layer | 🔲 Next |
| 3 | Strategy Engine & Backtesting | 🔲 |
| 4 | Slow-Motion Replay Engine | 🔲 |
| 5 | Technical Indicators on Chart | 🔲 |
| 6 | Portfolio Tracker | 🔲 |
| 7 | News, Events & Alerts | 🔲 |
| 8 | Live Market Monitor & Signal Alerts | 🔲 |
| 9 | Telegram Bot & Social Card Generator | 🔲 |
| 10 | AI Assistant Chatbot | 🔲 |
| 11 | Advanced Analytics | 🔲 |
| 12 | Open Source Polish & CI | 🔲 |

---

## Active Todos

### Technical (all phases)
- [ ] Unit tests for every backend function (`go test ./...` must always pass)
- [ ] Unit tests for every frontend component/hook (`vitest run` must always pass)
- [ ] All values configurable via env vars — no hardcoded values
- [ ] `user_id` column on every model (single-user now, multi-tenant ready later)

### UI (all phases)
- [ ] Card/island design style throughout
- [ ] Responsive — mobile and desktop friendly always
- [ ] WebSocket-first for all real-time updates (no frontend polling)
- [ ] Schema-driven UI for strategy params, indicator params, alert conditions

---

## Up Next: Phase 2

Key deliverables:
- `BinanceAdapter` + `YahooFinanceAdapter`
- `DataService` with gap-filling logic
- Background sync worker (every 5 min)
- Market/candles API endpoints
- Chart page with real data: adapter selector, symbol search, timeframe buttons, CandlestickChart component
