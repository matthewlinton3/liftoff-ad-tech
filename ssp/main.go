package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

// ─── OpenRTB Types ────────────────────────────────────────────────────────────

type BidRequest struct {
	ID     string       `json:"id"`
	Imp    []Impression `json:"imp"`
	Site   Site         `json:"site"`
	Device Device       `json:"device"`
	User   User         `json:"user"`
	AT     int          `json:"at"`
	TMax   int          `json:"tmax"`
	Cur    []string     `json:"cur"`
}

type Impression struct {
	ID       string  `json:"id"`
	Banner   Banner  `json:"banner"`
	BidFloor float64 `json:"bidfloor"`
}

type Banner struct {
	W int `json:"w"`
	H int `json:"h"`
}

type Site struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Domain string   `json:"domain"`
	Cat    []string `json:"cat"`
	Page   string   `json:"page"`
}

type Device struct {
	UA         string `json:"ua"`
	IP         string `json:"ip"`
	Geo        Geo    `json:"geo"`
	DeviceType int    `json:"devicetype"`
	OS         string `json:"os"`
	Language   string `json:"language"`
}

type Geo struct {
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
}

type User struct {
	ID string `json:"id"`
}

type ExchangeResponse struct {
	ID      string    `json:"id"`
	SeatBid []SeatBid `json:"seatbid"`
	Cur     string    `json:"cur"`
	Ext     DebugExt  `json:"ext"`
}

type SeatBid struct {
	Bid  []Bid  `json:"bid"`
	Seat string `json:"seat"`
}

type Bid struct {
	Price float64 `json:"price"`
	AdM   string  `json:"adm"`
	CrID  string  `json:"crid"`
}

type DebugExt struct {
	Debug DebugInfo `json:"debug"`
}

type DebugInfo struct {
	ElapsedMS int64      `json:"elapsed_ms"`
	AllBids   []BidDebug `json:"all_bids"`
}

type BidDebug struct {
	DSP   string  `json:"dsp"`
	Price float64 `json:"price"`
}

// ─── Config ───────────────────────────────────────────────────────────────────

func getExchangeURL() string {
	if u := os.Getenv("EXCHANGE_URL"); u != "" {
		return u
	}
	return "http://localhost:3000/auction"
}

var userAgents = []string{
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36",
}

var cities = []struct{ City, Region, Country string }{
	{"San Francisco", "CA", "USA"},
	{"New York", "NY", "USA"},
	{"Toronto", "ON", "CAN"},
	{"London", "EN", "GBR"},
	{"Chicago", "IL", "USA"},
}

// ─── SSP Auction Logic ────────────────────────────────────────────────────────

func buildBidRequest(pageURL, userID string) BidRequest {
	ua := userAgents[rand.Intn(len(userAgents))]
	city := cities[rand.Intn(len(cities))]

	os := "Unknown"
	deviceType := 2
	if len(ua) > 20 {
		if contains(ua, "iPhone") || contains(ua, "Android") {
			os = "iOS"
			deviceType = 1
			if contains(ua, "Android") {
				os = "Android"
			}
		} else if contains(ua, "Macintosh") {
			os = "macOS"
		} else if contains(ua, "Windows") {
			os = "Windows"
		}
	}

	return BidRequest{
		ID: fmt.Sprintf("imp-%d", time.Now().UnixNano()),
		Imp: []Impression{{
			ID:       "1",
			Banner:   Banner{W: 300, H: 250},
			BidFloor: 0.50,
		}},
		Site: Site{
			ID:     "pub-001",
			Name:   "TechPulse Media",
			Domain: "techpulse.example.com",
			Cat:    []string{"IAB19"},
			Page:   pageURL,
		},
		Device: Device{
			UA:         ua,
			IP:         fmt.Sprintf("71.%d.%d.%d", rand.Intn(255), rand.Intn(255), rand.Intn(255)),
			Geo:        Geo{Country: city.Country, Region: city.Region, City: city.City},
			DeviceType: deviceType,
			OS:         os,
			Language:   "en",
		},
		User: User{ID: userID},
		AT:   2,
		TMax: 150,
		Cur:  []string{"USD"},
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func callExchange(bidReq BidRequest) (*ExchangeResponse, error) {
	body, _ := json.Marshal(bidReq)
	resp, err := http.Post(getExchangeURL(), "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("exchange unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil // no fill
	}

	var result ExchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid exchange response: %w", err)
	}
	return &result, nil
}

// ─── HTTP Handlers ────────────────────────────────────────────────────────────

func recoverMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[SSP] panic recovered: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
			}
		}()
		next(w, r)
	}
}

func handlePublisherPage(w http.ResponseWriter, r *http.Request) {
	// Try cwd first, then ssp/ (when run from project root via ./bin/ssp)
	var html []byte
	for _, path := range []string{"publisher.html", "ssp/publisher.html"} {
		var err error
		html, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}
	if len(html) == 0 {
		log.Printf("[SSP] WARNING: publisher.html not found (tried publisher.html, ssp/publisher.html)")
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><h1>Publisher page not found</h1><p>Run the SSP from the project root or from the ssp/ directory.</p></body></html>"))
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(html)
}

func handleAdRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	pageURL := r.URL.Query().Get("page")
	userID := r.URL.Query().Get("uid")
	if userID == "" {
		userID = fmt.Sprintf("user-%d", rand.Intn(99999))
	}

	bidReq := buildBidRequest(pageURL, userID)

	log.Printf("[SSP] Ad request | user=%s | page=%s | floor=$%.2f",
		userID, pageURL, bidReq.Imp[0].BidFloor)

	result, err := callExchange(bidReq)
	if err != nil {
		log.Printf("[SSP] Exchange error: %s", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if result == nil {
		log.Printf("[SSP] No fill for user=%s", userID)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(result.SeatBid) == 0 || len(result.SeatBid[0].Bid) == 0 {
		log.Printf("[SSP] No fill for user=%s (empty seatbid)", userID)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	seatBid := &result.SeatBid[0]
	bid := &seatBid.Bid[0]
	log.Printf("[SSP] Filled | winner=%s | price=$%.4f | time=%dms",
		seatBid.Seat, bid.Price, result.Ext.Debug.ElapsedMS)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"adm":      bid.AdM,
		"price":    bid.Price,
		"winner":   seatBid.Seat,
		"elapsed":  result.Ext.Debug.ElapsedMS,
		"all_bids": result.Ext.Debug.AllBids,
		"bid_request": map[string]interface{}{
			"id":      bidReq.ID,
			"user":    bidReq.User.ID,
			"city":    bidReq.Device.Geo.City,
			"country": bidReq.Device.Geo.Country,
			"os":      bidReq.Device.OS,
			"floor":   bidReq.Imp[0].BidFloor,
		},
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "ssp"})
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	rand.Seed(time.Now().UnixNano())

	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}
	addr := "0.0.0.0:" + port

	mux := http.NewServeMux()
	mux.HandleFunc("/", recoverMiddleware(handlePublisherPage))
	mux.HandleFunc("/ad", recoverMiddleware(handleAdRequest))
	mux.HandleFunc("/health", recoverMiddleware(handleHealth))

	fmt.Printf("\n📰  SSP + Publisher running on http://0.0.0.0:%s\n", port)
	fmt.Printf("    Publisher page: http://localhost:%s\n", port)
	fmt.Printf("    Ad endpoint:    http://localhost:%s/ad\n\n", port)

	log.Fatal(http.ListenAndServe(addr, mux))
}
