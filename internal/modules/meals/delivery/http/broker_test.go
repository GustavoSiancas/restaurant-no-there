package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/modules/meals/domain"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestBrokerSendsClaimedOrdersSnapshotOnConnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker := NewBroker([]string{"*"})
	photoURL := "https://cdn.example.com/workers/worker-1.jpg"
	orders := []domain.MealOrder{{Claim: domain.Claim{ID: "claim-1", Status: domain.ClaimClaimed}, Worker: domain.ClaimPreviewWorker{PhotoURL: &photoURL}}}
	router := gin.New()
	router.GET("/ws", func(c *gin.Context) { broker.Serve(c, orders) })
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	var event struct {
		Type string             `json:"type"`
		Data []domain.MealOrder `json:"data"`
	}
	if err = conn.ReadJSON(&event); err != nil {
		t.Fatalf("read initial event: %v", err)
	}
	if event.Type != "CLAIMED_ORDERS" || len(event.Data) != 1 || event.Data[0].ID != "claim-1" || event.Data[0].Status != domain.ClaimClaimed {
		t.Fatalf("unexpected initial event: %+v", event)
	}
	if event.Data[0].Worker.PhotoURL == nil || *event.Data[0].Worker.PhotoURL != photoURL {
		t.Fatalf("worker photo was not included in WebSocket event: %+v", event.Data[0].Worker)
	}
}
