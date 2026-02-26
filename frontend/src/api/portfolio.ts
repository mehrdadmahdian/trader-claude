import apiClient from './client'
import type {
  Portfolio,
  Position,
  PortfolioSummary,
  Transaction,
  EquityPoint,
  CreatePortfolioReq,
  AddPositionReq,
  UpdatePositionReq,
  AddTransactionReq,
  PaginatedResponse,
} from '@/types'

// Portfolios
export const fetchPortfolios = () =>
  apiClient.get<{ data: Portfolio[] }>('/api/v1/portfolios').then((r) => r.data.data)

export const fetchPortfolio = (id: number) =>
  apiClient
    .get<{ data: Portfolio; positions: Position[] }>(`/api/v1/portfolios/${id}`)
    .then((r) => r.data)

export const createPortfolio = (req: CreatePortfolioReq) =>
  apiClient.post<{ data: Portfolio }>('/api/v1/portfolios', req).then((r) => r.data.data)

export const updatePortfolio = (id: number, req: Partial<CreatePortfolioReq>) =>
  apiClient.put<{ data: Portfolio }>(`/api/v1/portfolios/${id}`, req).then((r) => r.data.data)

export const deletePortfolio = (id: number) =>
  apiClient.delete(`/api/v1/portfolios/${id}`)

export const fetchPortfolioSummary = (id: number) =>
  apiClient
    .get<{ data: PortfolioSummary }>(`/api/v1/portfolios/${id}/summary`)
    .then((r) => r.data.data)

// Positions
export const addPosition = (portfolioId: number, req: AddPositionReq) =>
  apiClient
    .post<{ data: Position }>(`/api/v1/portfolios/${portfolioId}/positions`, req)
    .then((r) => r.data.data)

export const updatePosition = (portfolioId: number, posId: number, req: UpdatePositionReq) =>
  apiClient
    .put<{ data: Position }>(`/api/v1/portfolios/${portfolioId}/positions/${posId}`, req)
    .then((r) => r.data.data)

export const deletePosition = (portfolioId: number, posId: number) =>
  apiClient.delete(`/api/v1/portfolios/${portfolioId}/positions/${posId}`)

// Transactions
export const addTransaction = (portfolioId: number, req: AddTransactionReq) =>
  apiClient
    .post<{ data: Transaction }>(`/api/v1/portfolios/${portfolioId}/transactions`, req)
    .then((r) => r.data.data)

export const fetchTransactions = (portfolioId: number, page = 1, limit = 20) =>
  apiClient
    .get<PaginatedResponse<Transaction>>(
      `/api/v1/portfolios/${portfolioId}/transactions`,
      { params: { page, limit } },
    )
    .then((r) => r.data)

// Equity curve
export const fetchEquityCurve = (portfolioId: number) =>
  apiClient
    .get<{ points: EquityPoint[] }>(`/api/v1/portfolios/${portfolioId}/equity-curve`)
    .then((r) => r.data.points)
