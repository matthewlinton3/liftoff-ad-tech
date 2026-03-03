package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ─── OpenRTB 2.6 Types ───────────────────────────────────────────────────────

type BidRequest struct {
	ID     string        `json:"id"`
	Imp    []Impression  `json:"imp"`
	Site   *Site         `json:"site,omitempty"`
	App    *App          `json:"app,omitempty"`
	Device *Device       `json:"device,omitempty"`
	User   *User         `json:"user,omitempty"`
	AT     int           `json:"at"`   // 1=first price, 2=second price
	TMax   int           `json:"tmax"` // timeout ms
	Cur    []string      `json:"cur"`
}

type Impression struct {
	ID          string   `json:"id"`
	Banner      *Banner  `json:"banner,omitempty"`
	BidFloor    float64  `json:"bidfloor"`
	BidFloorCur string   `json:"bidfloorcur"`
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

type App struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Bundle string   `json:"bundle"`
	Cat    []string `json:"cat"`
}

type Device struct {
	UA         string `json:"ua"`
	IP         string `json:"ip"`
	Geo        *Geo   `json:"geo,omitempty"`
	DeviceType int    `json:"devicetype"` // 1=mobile, 2=PC, 3=TV
	OS         string `json:"os"`
	Language   string `json:"language"`
}

type Geo struct {
	Country string  `json:"country"`
	Region  string  `json:"region"`
	City    string  `json:"city"`
	Lat     float64 `json:"lat,omitempty"`
	Lon     float64 `json:"lon,omitempty"`
}

type User struct {
	ID string `json:"id"`
}

type BidResponse struct {
	ID      string    `json:"id"`
	SeatBid []SeatBid `json:"seatbid"`
	Cur     string    `json:"cur"`
}

type SeatBid struct {
	Bid  []Bid  `json:"bid"`
	Seat string `json:"seat"`
}

type Bid struct {
	ID    string  `json:"id"`
	ImpID string  `json:"impid"`
	Price float64 `json:"price"`
	AdID  string  `json:"adid"`
	AdM   string  `json:"adm"`
	CrID  string  `json:"crid"`
	W     int     `json:"w"`
	H     int     `json:"h"`
}

// ─── Exchange Types ───────────────────────────────────────────────────────────

type DSP struct {
	ID   string
	Name string
	URL  string
}

type CollectedBid struct {
	DSP   DSP
	Bid   *Bid
	Error error
}

type AuctionResult struct {
	Winner        CollectedBid
	ClearingPrice float64
	AllBids       []CollectedBid
	ElapsedMS     int64
}

type ExchangeResponse struct {
	ID      string      `json:"id"`
	SeatBid []SeatBid   `json:"seatbid"`
	Cur     string      `json:"cur"`
	Ext     DebugExt    `json:"ext"`
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

const auctionTimeoutMS = 150

var defaultDSPs = []DSP{
	{ID: "dsp-1", Name: "AlphaDSP", URL: "http://localhost:3001/bid"},
	{ID: "dsp-2", Name: "BetaDSP", URL: "http://localhost:3002/bid"},
	{ID: "dsp-3", Name: "GammaDSP", URL: "http://localhost:3003/bid"},
	{ID: "dsp-4", Name: "DeltaDSP", URL: "http://localhost:3004/bid"},
	{ID: "dsp-5", Name: "EpsilonDSP", URL: "http://localhost:3005/bid"},
	{ID: "dsp-6", Name: "ZetaDSP", URL: "http://localhost:3006/bid"},
	{ID: "dsp-7", Name: "EtaDSP", URL: "http://localhost:3007/bid"},
	{ID: "dsp-8", Name: "ThetaDSP", URL: "http://localhost:3008/bid"},
	{ID: "dsp-9", Name: "IotaDSP", URL: "http://localhost:3009/bid"},
	{ID: "dsp-10", Name: "KappaDSP", URL: "http://localhost:3010/bid"},
}

var registeredDSPs []DSP

// parseDSPURLs parses DSP_URLS (comma-separated URLs) into a slice of DSP.
// Each URL becomes one DSP with ID dsp-N and Name DSP-N.
func parseDSPURLs(env string) []DSP {
	var dsps []DSP
	for i, s := range strings.Split(env, ",") {
		url := strings.TrimSpace(s)
		if url == "" {
			continue
		}
		n := i + 1
		dsps = append(dsps, DSP{
			ID:   fmt.Sprintf("dsp-%d", n),
			Name: fmt.Sprintf("DSP-%d", n),
			URL:  url,
		})
	}
	return dsps
}

var httpClient = &http.Client{
	Timeout: time.Duration(auctionTimeoutMS) * time.Millisecond,
}

// Simulated fallback DSPs when all real DSPs fail to respond.
var simulatedDSPNames = []string{"Nike", "Apple", "American Express", "Coca-Cola", "Samsung", "Toyota"}

// generateSimulatedBids returns 3–5 fake bids from industry-brand DSPs so the auction always has a winner.
func generateSimulatedBids(floor float64, impID string) []CollectedBid {
	minPrice := floor
	if minPrice < 0.50 {
		minPrice = 0.50
	}
	n := 3 + rand.Intn(3) // 3–5 bids
	used := make(map[int]bool)
	out := make([]CollectedBid, 0, n)
	for len(out) < n {
		idx := rand.Intn(len(simulatedDSPNames))
		if used[idx] {
			continue
		}
		used[idx] = true
		price := minPrice + rand.Float64()*(5.00-minPrice)
		price = float64(int(price*100)) / 100
		name := simulatedDSPNames[idx]
		dsp := DSP{ID: "sim-" + fmt.Sprintf("%d", idx+1), Name: name, URL: ""}
		bid := &Bid{
			ID:    fmt.Sprintf("sim-bid-%d", len(out)+1),
			ImpID: impID,
			Price: price,
			AdID:  "sim-ad",
			AdM:   fmt.Sprintf("<div>Simulated ad from %s</div>", name),
			CrID:  "sim-creative",
			W:     300,
			H:     250,
		}
		out = append(out, CollectedBid{DSP: dsp, Bid: bid, Error: nil})
	}
	return out
}

// ─── Auction Logic ────────────────────────────────────────────────────────────

// secondPriceAuction runs a Vickrey (second-price) auction.
// Winner pays the second-highest bid + $0.01.
func secondPriceAuction(bids []CollectedBid) *AuctionResult {
	valid := []CollectedBid{}
	for _, b := range bids {
		if b.Error == nil && b.Bid != nil {
			valid = append(valid, b)
		}
	}
	if len(valid) == 0 {
		return nil
	}

	sort.Slice(valid, func(i, j int) bool {
		return valid[i].Bid.Price > valid[j].Bid.Price
	})

	clearingPrice := valid[0].Bid.Price
	if len(valid) > 1 {
		clearingPrice = valid[1].Bid.Price + 0.01
	}

	return &AuctionResult{
		Winner:        valid[0],
		ClearingPrice: float64(int(clearingPrice*10000)) / 10000,
		AllBids:       valid,
	}
}

// fanOut sends bid requests to all DSPs concurrently and collects responses.
func fanOut(bidRequest BidRequest) []CollectedBid {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(auctionTimeoutMS)*time.Millisecond)
	defer cancel()

	results := make(chan CollectedBid, len(registeredDSPs))
	var wg sync.WaitGroup

	for _, dsp := range registeredDSPs {
		wg.Add(1)
		go func(d DSP) {
			defer wg.Done()
			bid, err := sendBidRequest(ctx, d, bidRequest)
			results <- CollectedBid{DSP: d, Bid: bid, Error: err}
		}(dsp)
	}

	// Close results channel when all goroutines finish
	go func() {
		wg.Wait()
		close(results)
	}()

	collected := []CollectedBid{}
	for r := range results {
		collected = append(collected, r)
	}
	return collected
}

// sendBidRequest sends an OpenRTB bid request to a single DSP.
func sendBidRequest(ctx context.Context, dsp DSP, bidRequest BidRequest) (*Bid, error) {
	body, _ := json.Marshal(bidRequest)

	req, err := http.NewRequestWithContext(ctx, "POST", dsp.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-openrtb-version", "2.6")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, fmt.Errorf("no bid")
	}

	var bidResp BidResponse
	if err := json.NewDecoder(resp.Body).Decode(&bidResp); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	if len(bidResp.SeatBid) == 0 || len(bidResp.SeatBid[0].Bid) == 0 {
		return nil, fmt.Errorf("empty seatbid")
	}

	return &bidResp.SeatBid[0].Bid[0], nil
}

// ─── HTTP Handlers ────────────────────────────────────────────────────────────

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func handleAuction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var bidRequest BidRequest
	if err := json.Unmarshal(body, &bidRequest); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	start := time.Now()
	auctionID := bidRequest.ID
	if auctionID == "" {
		auctionID = fmt.Sprintf("auction-%d", time.Now().UnixNano())
	}

	floor := 0.0
	impID := "1"
	if len(bidRequest.Imp) > 0 {
		floor = bidRequest.Imp[0].BidFloor
		impID = bidRequest.Imp[0].ID
		if impID == "" {
			impID = "1"
		}
	}

	log.Printf("[Exchange] Auction %s | floor=$%.2f", auctionID, floor)

	// Fan out to all DSPs
	bids := fanOut(bidRequest)

	// Filter by floor price
	validBids := []CollectedBid{}
	for _, b := range bids {
		if b.Error != nil {
			log.Printf("[Exchange]   %s: NO BID — %s", b.DSP.Name, b.Error)
			continue
		}
		if b.Bid.Price < floor {
			log.Printf("[Exchange]   %s: $%.4f BELOW FLOOR", b.DSP.Name, b.Bid.Price)
			continue
		}
		log.Printf("[Exchange]   %s: $%.4f", b.DSP.Name, b.Bid.Price)
		validBids = append(validBids, b)
	}

	// Fallback: when all real DSPs fail, use simulated bids from fake industry DSPs
	if len(validBids) == 0 {
		validBids = generateSimulatedBids(floor, impID)
		log.Printf("[Exchange]   → Using %d simulated fallback bids", len(validBids))
		for _, b := range validBids {
			log.Printf("[Exchange]   %s: $%.4f (simulated)", b.DSP.Name, b.Bid.Price)
		}
	}

	result := secondPriceAuction(validBids)
	elapsed := time.Since(start).Milliseconds()

	if result == nil {
		log.Printf("[Exchange]   → No fill (%dms)", elapsed)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	log.Printf("[Exchange]   → WINNER: %s @ $%.4f (%dms)",
		result.Winner.DSP.Name, result.ClearingPrice, elapsed)

	// Build debug info
	allBidsDebug := []BidDebug{}
	for _, b := range result.AllBids {
		allBidsDebug = append(allBidsDebug, BidDebug{DSP: b.DSP.Name, Price: b.Bid.Price})
	}

	winBid := *result.Winner.Bid
	winBid.Price = result.ClearingPrice

	response := ExchangeResponse{
		ID: auctionID,
		SeatBid: []SeatBid{{
			Bid:  []Bid{winBid},
			Seat: result.Winner.DSP.ID,
		}},
		Cur: "USD",
		Ext: DebugExt{Debug: DebugInfo{
			ElapsedMS: elapsed,
			AllBids:   allBidsDebug,
		}},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"service": "exchange",
		"dsps":    len(registeredDSPs),
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"dsps":   len(registeredDSPs),
	})
}

func handleDSPs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"dsps": registeredDSPs})
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	rand.Seed(time.Now().UnixNano())

	if urls := os.Getenv("DSP_URLS"); urls != "" {
		registeredDSPs = parseDSPURLs(urls)
		if len(registeredDSPs) == 0 {
			log.Fatal("[Exchange] DSP_URLS is set but no valid URLs parsed")
		}
	} else {
		registeredDSPs = defaultDSPs
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	addr := "0.0.0.0:" + port

	mux := http.NewServeMux()
	mux.HandleFunc("/", corsMiddleware(handleRoot))
	mux.HandleFunc("/auction", corsMiddleware(handleAuction))
	mux.HandleFunc("/health", corsMiddleware(handleHealth))
	mux.HandleFunc("/dsps", corsMiddleware(handleDSPs))

	fmt.Printf("\n🏦  Ad Exchange running on http://0.0.0.0:%s\n", port)
	fmt.Printf("    Registered DSPs: ")
	for i, d := range registeredDSPs {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(d.Name)
	}
	fmt.Printf("\n    Auction timeout: %dms\n", auctionTimeoutMS)
	fmt.Printf("    Endpoints: POST /auction | GET /health | GET /dsps\n\n")

	log.Fatal(http.ListenAndServe(addr, mux))
}
