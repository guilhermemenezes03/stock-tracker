package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// StockData represents stock price and variation data.
type StockData struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price"`
	Change float64 `json:"change"`
}

// Global variables for Redis client, WebSocket upgrader, connected clients, and update channel.
var (
	redisClient *redis.Client
	upgrader    = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clients     = make(map[*websocket.Conn]bool)
	clientsMux  sync.Mutex
	updates     = make(chan StockData, 128)
)

func init() {
	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

type AlphaVantageResponse struct {
	GlobalQuote map[string]string `json:"Global Quote"`
}

func getSymbols() []string {
	v := strings.TrimSpace(os.Getenv("STOCK_SYMBOLS"))
	if v == "" {
		return []string{"AAPL", "GOOGL"}
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"AAPL", "GOOGL"}
	}
	return out
}

// getPollInterval returns the polling interval in seconds from environment or default.
func getPollInterval() time.Duration {
	v := strings.TrimSpace(os.Getenv("POLL_SECONDS"))
	if v == "" {
		return 60 * time.Second
	}
	seconds, err := strconv.Atoi(v)
	if err != nil || seconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

// fetchStock fetches stock data for a symbol from Alpha Vantage.
func fetchStock(symbol string) (StockData, bool, error) {
	key := strings.TrimSpace(os.Getenv("ALPHA_VANTAGE_KEY"))
	if key == "" {
		return StockData{}, false, fmt.Errorf("missing ALPHA_VANTAGE_KEY")
	}

	url := "https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=" + symbol + "&apikey=" + key
	resp, err := http.Get(url)
	if err != nil {
		return StockData{}, false, err
	}
	defer resp.Body.Close()

	var payload AlphaVantageResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return StockData{}, false, err
	}
	if len(payload.GlobalQuote) == 0 {
		return StockData{}, false, nil
	}

	current := parseFloat(payload.GlobalQuote["05. price"])
	previous := parseFloat(payload.GlobalQuote["08. previous close"])
	if previous == 0 {
		return StockData{}, false, nil
	}
	change := ((current - previous) / previous) * 100
	return StockData{Symbol: symbol, Price: current, Change: change}, true, nil
}

func pushUpdate(data StockData) {
	select {
	case updates <- data:
	default:
	}
}

func broadcaster() {
	for data := range updates {
		clientsMux.Lock()
		for conn := range clients {
			if err := conn.WriteJSON(data); err != nil {
				conn.Close()
				delete(clients, conn)
			}
		}
		clientsMux.Unlock()
	}
}

func fetchLoop() {
	interval := getPollInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	update := func() {
		for _, symbol := range getSymbols() {
			data, ok, err := fetchStock(symbol)
			if err != nil {
				log.Println(err)
				continue
			}
			if !ok {
				continue
			}
			redisClient.ZAdd(context.Background(), "leaderboard", &redis.Z{Score: data.Change, Member: data.Symbol})
			pushUpdate(data)
		}
	}

	update()
	for range ticker.C {
		update()
	}
}

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}
	clientsMux.Lock()
	clients[conn] = true
	clientsMux.Unlock()
	defer func() {
		clientsMux.Lock()
		delete(clients, conn)
		clientsMux.Unlock()
		conn.Close()
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// main sets up the server and starts goroutines.
func main() {
	if strings.TrimSpace(os.Getenv("ALPHA_VANTAGE_KEY")) == "" {
		log.Fatal("missing ALPHA_VANTAGE_KEY")
	}

	go broadcaster()
	go fetchLoop()
	r := gin.Default()
	r.GET("/", func(c *gin.Context) { c.File("index.html") })
	r.GET("/ws", handleWebSocket)
	r.GET("/leaderboard", func(c *gin.Context) {
		result := redisClient.ZRevRangeWithScores(context.Background(), "leaderboard", 0, -1)
		c.JSON(200, result.Val())
	})
	r.Run(":8081")
}
