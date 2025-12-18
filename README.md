# Real-Time Stock Tracker

A Go-based real-time stock price tracker and leaderboard app. Fetches stock prices from Alpha Vantage API, stores variations in Redis for ranking, and broadcasts updates to clients via WebSockets.

## Features

- Real-time stock price fetching and updates
- WebSocket-based live notifications
- Redis-backed leaderboard for stock variations
- Simple web UI for monitoring
- Configurable via environment variables

## Prerequisites

- Go 1.21+
- Redis (running on localhost:6379)
- Alpha Vantage API key (free at alphavantage.co)

## Installation

1. Clone or download the project.
2. Install dependencies: `go mod tidy`
3. Set environment variables:
   - `ALPHA_VANTAGE_KEY`: Your Alpha Vantage API key
   - `STOCK_SYMBOLS`: Comma-separated list of symbols (default: AAPL,GOOGL)
   - `POLL_SECONDS`: Polling interval in seconds (default: 60)

## Running

```bash
# Set API key
export ALPHA_VANTAGE_KEY=your_key_here

# Optional: customize symbols and poll interval
export STOCK_SYMBOLS=AAPL,GOOGL,MSFT
export POLL_SECONDS=30

# Run the server
go run main.go
```

Open http://localhost:8081 in your browser.

## API Endpoints

- `GET /`: Serves the web UI
- `GET /ws`: WebSocket endpoint for real-time updates
- `GET /leaderboard`: JSON leaderboard of stock variations

## Architecture

- **Backend**: Go with Gin for HTTP, Gorilla WebSocket for real-time, go-redis for caching.
- **Data Source**: Alpha Vantage Global Quote API.
- **Storage**: Redis sorted set for leaderboard.
- **Frontend**: Vanilla HTML/JS with responsive design.

## License

MIT