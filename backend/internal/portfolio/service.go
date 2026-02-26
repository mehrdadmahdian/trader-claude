package portfolio

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

// PriceFetcher is the minimal interface required by PortfolioService
type PriceFetcher interface {
	GetPrice(ctx context.Context, adapterID, symbol string) (float64, error)
}

// EquityPoint is a single value snapshot in the equity curve
type EquityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// PortfolioSummary holds aggregated stats for the summary cards
type PortfolioSummary struct {
	PortfolioID int64   `json:"portfolio_id"`
	TotalValue  float64 `json:"total_value"`
	TotalCost   float64 `json:"total_cost"`
	TotalPnL    float64 `json:"total_pnl"`
	TotalPnLPct float64 `json:"total_pnl_pct"`
}

// Request types

type CreatePortfolioReq struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Type        models.PortfolioType `json:"type"`
	Currency    string               `json:"currency"`
	InitialCash float64              `json:"initial_cash"`
}

type UpdatePortfolioReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Currency    string `json:"currency"`
}

type AddPositionReq struct {
	AdapterID string    `json:"adapter_id"`
	Symbol    string    `json:"symbol"`
	Market    string    `json:"market"`
	Quantity  float64   `json:"quantity"`
	AvgCost   float64   `json:"avg_cost"`
	OpenedAt  time.Time `json:"opened_at"`
}

type UpdatePositionReq struct {
	Quantity float64 `json:"quantity"`
	AvgCost  float64 `json:"avg_cost"`
}

type AddTransactionReq struct {
	PositionID *int64                 `json:"position_id,omitempty"`
	Type       models.TransactionType `json:"type"`
	AdapterID  string                 `json:"adapter_id"`
	Symbol     string                 `json:"symbol"`
	Quantity   float64                `json:"quantity"`
	Price      float64                `json:"price"`
	Fee        float64                `json:"fee"`
	Notes      string                 `json:"notes"`
	ExecutedAt time.Time              `json:"executed_at"`
}

// Service handles all portfolio business logic
type Service struct {
	db    *gorm.DB
	price PriceFetcher
}

func NewService(db *gorm.DB, price PriceFetcher) *Service {
	return &Service{db: db, price: price}
}

// --- Portfolio CRUD ---

func (s *Service) CreatePortfolio(ctx context.Context, req CreatePortfolioReq) (*models.Portfolio, error) {
	if req.Currency == "" {
		req.Currency = "USD"
	}
	if req.Type == "" {
		req.Type = models.PortfolioTypeManual
	}
	p := &models.Portfolio{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Currency:    req.Currency,
		InitialCash: req.InitialCash,
		CurrentCash: req.InitialCash,
		IsActive:    true,
	}
	if err := s.db.WithContext(ctx).Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) GetPortfolio(ctx context.Context, id int64) (*models.Portfolio, error) {
	var p models.Portfolio
	if err := s.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) ListPortfolios(ctx context.Context) ([]*models.Portfolio, error) {
	var portfolios []*models.Portfolio
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Find(&portfolios).Error; err != nil {
		return nil, err
	}
	return portfolios, nil
}

func (s *Service) UpdatePortfolio(ctx context.Context, id int64, req UpdatePortfolioReq) (*models.Portfolio, error) {
	var p models.Portfolio
	if err := s.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.Currency != "" {
		p.Currency = req.Currency
	}
	if err := s.db.WithContext(ctx).Save(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) DeletePortfolio(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&models.Portfolio{}, id).Error
}

func (s *Service) GetPortfolioWithPositions(ctx context.Context, id int64) (*models.Portfolio, []models.Position, error) {
	p, err := s.GetPortfolio(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	var positions []models.Position
	if err := s.db.WithContext(ctx).Where("portfolio_id = ?", id).Find(&positions).Error; err != nil {
		return nil, nil, err
	}
	return p, positions, nil
}

func (s *Service) GetSummary(ctx context.Context, id int64) (*PortfolioSummary, error) {
	_, positions, err := s.GetPortfolioWithPositions(ctx, id)
	if err != nil {
		return nil, err
	}
	var totalValue, totalCost float64
	for _, pos := range positions {
		totalValue += pos.CurrentValue
		totalCost += pos.Quantity * pos.AvgCost
	}
	var totalPnL, totalPnLPct float64
	if totalCost > 0 {
		totalPnL = totalValue - totalCost
		totalPnLPct = (totalPnL / totalCost) * 100
	}
	return &PortfolioSummary{
		PortfolioID: id,
		TotalValue:  totalValue,
		TotalCost:   totalCost,
		TotalPnL:    totalPnL,
		TotalPnLPct: totalPnLPct,
	}, nil
}

// --- Position CRUD ---

func (s *Service) AddPosition(ctx context.Context, portfolioID int64, req AddPositionReq) (*models.Position, error) {
	pos := &models.Position{
		PortfolioID: portfolioID,
		AdapterID:   req.AdapterID,
		Symbol:      req.Symbol,
		Market:      req.Market,
		Quantity:    req.Quantity,
		AvgCost:     req.AvgCost,
		OpenedAt:    req.OpenedAt,
	}
	if err := s.db.WithContext(ctx).Create(pos).Error; err != nil {
		return nil, err
	}
	return pos, nil
}

func (s *Service) UpdatePosition(ctx context.Context, positionID int64, req UpdatePositionReq) (*models.Position, error) {
	var pos models.Position
	if err := s.db.WithContext(ctx).First(&pos, positionID).Error; err != nil {
		return nil, err
	}
	pos.Quantity = req.Quantity
	pos.AvgCost = req.AvgCost
	if err := s.db.WithContext(ctx).Save(&pos).Error; err != nil {
		return nil, err
	}
	return &pos, nil
}

func (s *Service) DeletePosition(ctx context.Context, positionID int64) error {
	return s.db.WithContext(ctx).Delete(&models.Position{}, positionID).Error
}

// --- Transaction CRUD ---

func (s *Service) AddTransaction(ctx context.Context, portfolioID int64, req AddTransactionReq) (*models.Transaction, error) {
	tx := &models.Transaction{
		PortfolioID: portfolioID,
		PositionID:  req.PositionID,
		Type:        req.Type,
		AdapterID:   req.AdapterID,
		Symbol:      req.Symbol,
		Quantity:    req.Quantity,
		Price:       req.Price,
		Fee:         req.Fee,
		Notes:       req.Notes,
		ExecutedAt:  req.ExecutedAt,
	}
	if err := s.db.WithContext(ctx).Create(tx).Error; err != nil {
		return nil, err
	}
	return tx, nil
}

func (s *Service) ListTransactions(ctx context.Context, portfolioID int64, page, limit int) ([]models.Transaction, int64, error) {
	var txs []models.Transaction
	var total int64

	q := s.db.WithContext(ctx).Model(&models.Transaction{}).Where("portfolio_id = ?", portfolioID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := q.Order("executed_at DESC").Offset(offset).Limit(limit).Find(&txs).Error; err != nil {
		return nil, 0, err
	}
	return txs, total, nil
}

// --- Analytics ---

// RecalculatePortfolio fetches current prices for all positions, updates PnL fields,
// and sets Portfolio.CurrentValue to the sum of all position values.
func (s *Service) RecalculatePortfolio(ctx context.Context, portfolioID int64) error {
	var positions []models.Position
	if err := s.db.WithContext(ctx).Where("portfolio_id = ?", portfolioID).Find(&positions).Error; err != nil {
		return err
	}

	var totalValue float64
	for i := range positions {
		pos := &positions[i]
		currentPrice, err := s.price.GetPrice(ctx, pos.AdapterID, pos.Symbol)
		if err != nil {
			currentPrice = pos.CurrentPrice // fall back to last known price on fetch failure
		}
		pos.CurrentPrice = currentPrice
		pos.CurrentValue = pos.Quantity * currentPrice
		costBasis := pos.Quantity * pos.AvgCost
		pos.UnrealizedPnL = pos.CurrentValue - costBasis
		if costBasis > 0 {
			pos.UnrealizedPnLPct = (pos.UnrealizedPnL / costBasis) * 100
		}
		totalValue += pos.CurrentValue
		if err := s.db.WithContext(ctx).Save(pos).Error; err != nil {
			return err
		}
	}

	return s.db.WithContext(ctx).Model(&models.Portfolio{}).Where("id = ?", portfolioID).
		Update("current_value", totalValue).Error
}

// GetEquityCurve replays transactions chronologically to produce a value-over-time series.
func (s *Service) GetEquityCurve(ctx context.Context, portfolioID int64) ([]EquityPoint, error) {
	var txs []models.Transaction
	if err := s.db.WithContext(ctx).
		Where("portfolio_id = ?", portfolioID).
		Order("executed_at ASC").
		Find(&txs).Error; err != nil {
		return nil, err
	}

	var runningValue float64
	points := make([]EquityPoint, 0, len(txs))

	for _, tx := range txs {
		switch tx.Type {
		case models.TransactionTypeDeposit:
			runningValue += tx.Price * tx.Quantity
		case models.TransactionTypeWithdrawal:
			runningValue -= tx.Price * tx.Quantity
		case models.TransactionTypeBuy:
			// No net change at transaction time (cash out = asset value in at cost)
		case models.TransactionTypeSell:
			runningValue += tx.Price*tx.Quantity - tx.Fee
		}
		points = append(points, EquityPoint{Timestamp: tx.ExecutedAt, Value: runningValue})
	}

	return points, nil
}
