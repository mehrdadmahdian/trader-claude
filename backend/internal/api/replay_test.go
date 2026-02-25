package api

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/trader-claude/backend/internal/replay"
)

// setupReplayApp builds a minimal Fiber app with just the replay routes
// and a pre-seeded manager.
func setupReplayApp(mgr *replay.Manager) *fiber.App {
	app := fiber.New()
	rh := &replayHandler{db: nil, ds: nil, manager: mgr}
	app.Post("/api/v1/backtest/runs/:id/replay", rh.createReplay)
	app.Post("/api/v1/replay/bookmarks", rh.createBookmark)
	app.Get("/api/v1/replay/bookmarks", rh.listBookmarks)
	app.Delete("/api/v1/replay/bookmarks/:id", rh.deleteBookmark)
	return app
}

func TestCreateReplay_InvalidID(t *testing.T) {
	mgr := replay.NewManager()
	app := setupReplayApp(mgr)

	req := httptest.NewRequest("POST", "/api/v1/backtest/runs/not-a-number/replay", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateReplay_RequiresDB(t *testing.T) {
	// Without a DB, createReplay should return non-200 (can't load backtest).
	mgr := replay.NewManager()
	app := setupReplayApp(mgr)

	req := httptest.NewRequest("POST", "/api/v1/backtest/runs/1/replay", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == fiber.StatusOK {
		t.Error("expected non-200 when db is nil")
	}
}

func TestListBookmarks_NoDB_Returns500(t *testing.T) {
	mgr := replay.NewManager()
	app := setupReplayApp(mgr)

	req := httptest.NewRequest("GET", "/api/v1/replay/bookmarks?run_id=1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == fiber.StatusOK {
		t.Error("expected non-200 when db is nil")
	}
}

func TestDeleteBookmark_InvalidID(t *testing.T) {
	mgr := replay.NewManager()
	app := setupReplayApp(mgr)

	req := httptest.NewRequest("DELETE", "/api/v1/replay/bookmarks/not-a-number", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestResponseShape_CreateBookmark_NoBody(t *testing.T) {
	mgr := replay.NewManager()
	app := setupReplayApp(mgr)

	req := httptest.NewRequest("POST", "/api/v1/replay/bookmarks", nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	var res map[string]interface{}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("response is not JSON: %s", string(body))
	}
	if _, ok := res["error"]; !ok {
		t.Error("expected 'error' key in response")
	}
}
