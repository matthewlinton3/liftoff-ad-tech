package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

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
type Banner struct{ W, H int }
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
type User struct{ ID string `json:"id"` }

type ExchangeResponse struct {
	ID      string    `json:"id"`
	SeatBid []SeatBid `json:"seatbid"`
	Cur     string    `json:"cur"`
	Ext     struct {
		Debug struct {
			ElapsedMS int64 `json:"elapsed_ms"`
			AllBids   []struct {
				DSP   string  `json:"dsp"`
				Price float64 `json:"price"`
			} `json:"all_bids"`
		} `json:"debug"`
	} `json:"ext"`
}
type SeatBid struct {
	Bid  []Bid  `json:"bid"`
	Seat string `json:"seat"`
}
type Bid struct {
	Price float64 `json:"price"`
	CrID  string  `json:"crid"`
}

func main() {
	req := BidRequest{
		ID: fmt.Sprintf("test-%d", time.Now().UnixMilli()),
		Imp: []Impression{{
			ID:       "1",
			Banner:   Banner{W: 300, H: 250},
			BidFloor: 0.50,
		}},
		Site: Site{
			ID:     "site-001",
			Name:   "Example Publisher",
			Domain: "example.com",
			Cat:    []string{"IAB19"},
			Page:   "https://example.com/article/ai-trends",
		},
		Device: Device{
			UA:         "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)",
			IP:         "71.123.45.67",
			Geo:        Geo{Country: "USA", Region: "CA", City: "San Francisco"},
			DeviceType: 1,
			OS:         "iOS",
			Language:   "en",
		},
		User: User{ID: "user-abc123"},
		AT:   2,
		TMax: 150,
		Cur:  []string{"USD"},
	}

	body, _ := json.Marshal(req)

	fmt.Println("\n📤 Sending bid request to exchange...")
	fmt.Printf("   Imp: 300x250 banner | Floor: $%.2f\n", req.Imp[0].BidFloor)
	fmt.Printf("   User: %s in %s\n", req.Device.OS, req.Device.Geo.City)

	resp, err := http.Post("http://localhost:3000/auction", "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Println("\n❌ Could not reach exchange:", err)
		fmt.Println("   (Make sure the exchange is running: go run main.go in the exchange/ folder)")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		fmt.Println("\n❌ No fill — no DSPs bid above floor price")
		return
	}

	data, _ := io.ReadAll(resp.Body)
	var result ExchangeResponse
	json.Unmarshal(data, &result)

	if len(result.SeatBid) == 0 || len(result.SeatBid[0].Bid) == 0 {
		fmt.Println("\n❌ Unexpected empty response")
		return
	}

	bid := result.SeatBid[0].Bid[0]
	fmt.Println("\n✅ AUCTION COMPLETE")
	fmt.Printf("   Winner:         %s\n", result.SeatBid[0].Seat)
	fmt.Printf("   Clearing price: $%.4f\n", bid.Price)
	fmt.Printf("   Creative ID:    %s\n", bid.CrID)
	fmt.Printf("   Auction time:   %dms\n", result.Ext.Debug.ElapsedMS)
	fmt.Println("\n   All bids:")
	for _, b := range result.Ext.Debug.AllBids {
		flag := "  "
		if b.DSP == result.SeatBid[0].Seat {
			flag = "🏆"
		}
		fmt.Printf("   %s %s: $%.4f\n", flag, b.DSP, b.Price)
	}
	fmt.Println()
}
