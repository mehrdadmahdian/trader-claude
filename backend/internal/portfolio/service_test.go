package portfolio

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/trader-claude/backend/internal/models"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.Portfolio{}, &models.Position{}, &models.Transaction{}); err != nil {
		t.Fatalf("auto-migrate failed: %v", err)
	}
	return db
}

type stubPriceService struct {
	prices map[string]float64
}

func (s *stubPriceService) GetPrice(_ context.Context, _, symbol string) (float64, error) {
	if p, ok := s.prices[symbol]; ok {
		return p, nil
	}
	return 0, errors.New("price not found")
}

const testUserID = int64(1)

func TestCreateAndGetPortfolio(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, &stubPriceService{})

	p, err := svc.CreatePortfolio(context.Background(), testUserID, CreatePortfolioReq{
		Name:        "Test Portfolio",
		Description: "desc",
		Type:        models.PortfolioTypeManual,
		Currency:    "USD",
		InitialCash: 10000,
	})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if p.CurrentCash != 10000 {
		t.Errorf("expected CurrentCash=10000, got %f", p.CurrentCash)
	}

	got, err := svc.GetPortfolio(context.Background(), p.ID, testUserID)
	if err != nil {
		t.Fatalf("GetPortfolio: %v", err)
	}
	if got.Name != "Test Portfolio" {
		t.Errorf("expected 'Test Portfolio', got %q", got.Name)
	}
}

func TestAddPositionAndRecalculate(t *testing.T) {
	db := newTestDB(t)
	stub := &stubPriceService{prices: map[string]float64{"BTCUSDT": 50000.0}}
	svc := NewService(db, stub)

	p, err := svc.CreatePortfolio(context.Background(), testUserID, CreatePortfolioReq{
		Name: "BTC Portfolio", Type: models.PortfolioTypeManual, InitialCash: 100000, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}

	pos, err := svc.AddPosition(context.Background(), p.ID, AddPositionReq{
		AdapterID: "binance",
		Symbol:    "BTCUSDT",
		Market:    "crypto",
		Quantity:  2.0,
		AvgCost:   40000.0,
		OpenedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("AddPosition: %v", err)
	}
	if pos.Symbol != "BTCUSDT" {
		t.Errorf("expected BTCUSDT, got %s", pos.Symbol)
	}

	if err := svc.RecalculatePortfolio(context.Background(), p.ID); err != nil {
		t.Fatalf("RecalculatePortfolio: %v", err)
	}

	updated, err := svc.GetPortfolio(context.Background(), p.ID, testUserID)
	if err != nil {
		t.Fatalf("GetPortfolio after recalculate: %v", err)
	}
	// 2 BTC at $50k = $100k
	if updated.CurrentValue != 100000.0 {
		t.Errorf("expected CurrentValue=100000, got %f", updated.CurrentValue)
	}

	var pos2 models.Position
	if err := db.WithContext(context.Background()).First(&pos2, pos.ID).Error; err != nil {
		t.Fatalf("fetch updated position: %v", err)
	}
	// UnrealizedPnL = (50000 - 40000) * 2 = 20000
	if pos2.UnrealizedPnL != 20000.0 {
		t.Errorf("expected UnrealizedPnL=20000, got %f", pos2.UnrealizedPnL)
	}
	// UnrealizedPnLPct = 20000 / 80000 * 100 = 25
	if pos2.UnrealizedPnLPct != 25.0 {
		t.Errorf("expected UnrealizedPnLPct=25, got %f", pos2.UnrealizedPnLPct)
	}
}

func TestGetEquityCurve(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, &stubPriceService{})

	p, err := svc.CreatePortfolio(context.Background(), testUserID, CreatePortfolioReq{
		Name: "EQ Portfolio", Type: models.PortfolioTypeManual, InitialCash: 0, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}

	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(24 * time.Hour)

	// Two deposits of $5000 each
	svc.AddTransaction(context.Background(), p.ID, AddTransactionReq{
		Type: models.TransactionTypeDeposit, Price: 5000, Quantity: 1, ExecutedAt: t1,
	})
	svc.AddTransaction(context.Background(), p.ID, AddTransactionReq{
		Type: models.TransactionTypeDeposit, Price: 5000, Quantity: 1, ExecutedAt: t2,
	})

	curve, err := svc.GetEquityCurve(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetEquityCurve: %v", err)
	}
	if len(curve) != 2 {
		t.Errorf("expected 2 equity points, got %d", len(curve))
	}
	if curve[0].Value != 5000 {
		t.Errorf("expected first point=5000, got %f", curve[0].Value)
	}
	if curve[1].Value != 10000 {
		t.Errorf("expected second point=10000, got %f", curve[1].Value)
	}
}

func TestListTransactions_Pagination(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, &stubPriceService{})

	p, err := svc.CreatePortfolio(context.Background(), testUserID, CreatePortfolioReq{
		Name: "TX Portfolio", Type: models.PortfolioTypeManual, InitialCash: 0, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}

	for i := 0; i < 5; i++ {
		svc.AddTransaction(context.Background(), p.ID, AddTransactionReq{
			Type:       models.TransactionTypeDeposit,
			Price:      100,
			Quantity:   1,
			ExecutedAt: time.Now().Add(time.Duration(i) * time.Hour),
		})
	}

	txs, total, err := svc.ListTransactions(context.Background(), p.ID, 1, 3)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(txs) != 3 {
		t.Errorf("expected 3 results on page 1, got %d", len(txs))
	}

	txs2, _, err := svc.ListTransactions(context.Background(), p.ID, 2, 3)
	if err != nil {
		t.Fatalf("ListTransactions page 2: %v", err)
	}
	if len(txs2) != 2 {
		t.Errorf("expected 2 results on page 2, got %d", len(txs2))
	}
}
