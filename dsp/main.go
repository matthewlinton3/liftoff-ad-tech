package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

type BidRequest struct {
	ID     string       `json:"id"`
	Imp    []Impression `json:"imp"`
	Site   *Site        `json:"site,omitempty"`
	Device *Device      `json:"device,omitempty"`
	User   *User        `json:"user,omitempty"`
	TMax   int          `json:"tmax"`
}
type Impression struct {
	ID       string  `json:"id"`
	Banner   *Banner `json:"banner,omitempty"`
	BidFloor float64 `json:"bidfloor"`
}
type Banner struct{ W, H int }
type Site struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Domain string   `json:"domain"`
	Cat    []string `json:"cat"`
}
type Device struct {
	UA         string `json:"ua"`
	IP         string `json:"ip"`
	Geo        *Geo   `json:"geo,omitempty"`
	DeviceType int    `json:"devicetype"`
	OS         string `json:"os"`
}
type Geo struct {
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
}
type User struct{ ID string `json:"id"` }
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

type ModelMetadata struct {
	Features     []string  `json:"features"`
	ScalerMean   []float64 `json:"scaler_mean"`
	ScalerStd    []float64 `json:"scaler_std"`
	Intercept    float64   `json:"intercept"`
	Coefficients []float64 `json:"coefficients"`
	AUC          float64   `json:"auc"`
	BaseCVR      float64   `json:"base_cvr"`
}

var model *ModelMetadata

func loadModel(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read model file: %w", err)
	}
	var m ModelMetadata
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("could not parse model: %w", err)
	}
	model = &m
	log.Printf("[%s] ML model loaded | AUC=%.4f | base_cvr=%.2f%% | features=%d",
		dspName, m.AUC, m.BaseCVR*100, len(m.Features))
	return nil
}

func predictCVR(br BidRequest) float64 {
	if model == nil {
		return 0.025
	}
	floor := 0.0
	if len(br.Imp) > 0 {
		floor = br.Imp[0].BidFloor
	}
	hour := float64(time.Now().Hour())
	isBusinessHours := 0.0
	if hour >= 9 && hour <= 18 { isBusinessHours = 1.0 }
	isEvening := 0.0
	if hour >= 19 && hour <= 22 { isEvening = 1.0 }
	deviceType := 2.0
	osIOS := 0.0
	osAndroid := 0.0
	if br.Device != nil {
		deviceType = float64(br.Device.DeviceType)
		switch br.Device.OS {
		case "iOS":     osIOS = 1.0
		case "Android": osAndroid = 1.0
		}
	}
	countryTier1 := 0.0
	countryTier2 := 0.0
	if br.Device != nil && br.Device.Geo != nil {
		switch br.Device.Geo.Country {
		case "USA", "CAN", "GBR", "AUS": countryTier1 = 1.0
		case "DEU", "FRA", "JPN", "KOR": countryTier2 = 1.0
		}
	}
	siteTech := 0.0
	siteNews := 0.0
	if br.Site != nil {
		for _, cat := range br.Site.Cat {
			if cat == "IAB19" { siteTech = 1.0 }
			if cat == "IAB1" || cat == "IAB12" { siteNews = 1.0 }
		}
	}
	features := []float64{
		deviceType, osIOS, osAndroid, countryTier1, countryTier2,
		hour, isBusinessHours, isEvening, siteTech, siteNews,
		1.0, 1.0, 1.0, floor,
	}
	scaled := make([]float64, len(features))
	for i, v := range features {
		scaled[i] = (v - model.ScalerMean[i]) / model.ScalerStd[i]
	}
	logit := model.Intercept
	for i, coef := range model.Coefficients {
		logit += coef * scaled[i]
	}
	return 1.0 / (1.0 + math.Exp(-logit))
}

type Strategy string
const (
	Aggressive   Strategy = "aggressive"
	Conservative Strategy = "conservative"
	Smart        Strategy = "smart"
)

var (
	dspName   = "GenericDSP"
	strategy  = Smart
	targetCPA = 10.0
)

func getBidPrice(br BidRequest) (float64, bool) {
	floor := 0.0
	if len(br.Imp) > 0 { floor = br.Imp[0].BidFloor }
	cvr := predictCVR(br)
	baseBid := targetCPA * cvr
	switch strategy {
	case Aggressive:
		baseBid *= 1.3
	case Conservative:
		if cvr < 0.03 { return 0, false }
		baseBid *= 0.9
	default:
		baseBid += (rand.Float64() - 0.5) * 0.1
	}
	if baseBid < floor { return 0, false }
	return math.Round(baseBid*10000) / 10000, true
}

func getCreatives() []struct{ ID, AdM string } {
	return []struct{ ID, AdM string }{
		{"cr-001", fmt.Sprintf(`<div style="width:300px;height:250px;background:linear-gradient(135deg,#667eea,#764ba2);display:flex;align-items:center;justify-content:center;color:white;font-family:sans-serif;border-radius:8px;cursor:pointer"><div style="text-align:center"><div style="font-size:24px;font-weight:bold">%s</div><div style="font-size:14px;margin-top:8px">Click to learn more →</div></div></div>`, dspName)},
		{"cr-002", fmt.Sprintf(`<div style="width:300px;height:250px;background:linear-gradient(135deg,#f093fb,#f5576c);display:flex;align-items:center;justify-content:center;color:white;font-family:sans-serif;border-radius:8px;cursor:pointer"><div style="text-align:center"><div style="font-size:20px;font-weight:bold">Special Offer!</div><div style="font-size:13px;margin-top:8px">From %s</div><div style="margin-top:12px;background:white;color:#f5576c;padding:8px 16px;border-radius:20px;font-weight:bold">Shop Now</div></div></div>`, dspName)},
	}
}

func handleBid(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	var br BidRequest
	if err := json.NewDecoder(r.Body).Decode(&br); err != nil { http.Error(w, "bad request", http.StatusBadRequest); return }
	cvr := predictCVR(br)
	price, shouldBid := getBidPrice(br)
	if !shouldBid {
		log.Printf("[%s] NO BID — cvr=%.2f%% below threshold", dspName, cvr*100)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	creatives := getCreatives()
	creative := creatives[rand.Intn(len(creatives))]
	log.Printf("[%s] BID $%.4f | cvr=%.2f%% | targetCPA=$%.2f | strategy=%s", dspName, price, cvr*100, targetCPA, strategy)
	impID := "1"
	if len(br.Imp) > 0 { impID = br.Imp[0].ID }
	resp := BidResponse{
		ID: br.ID,
		SeatBid: []SeatBid{{
			Bid: []Bid{{
				ID: fmt.Sprintf("bid-%d", time.Now().UnixNano()), ImpID: impID,
				Price: price, AdID: dspName + "-ad-001", AdM: creative.AdM, CrID: creative.ID, W: 300, H: 250,
			}},
			Seat: dspName,
		}},
		Cur: "USD",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	auc := 0.0
	if model != nil { auc = model.AUC }
	json.NewEncoder(w).Encode(map[string]interface{}{"dsp": dspName, "strategy": string(strategy), "target_cpa": targetCPA, "model_auc": auc, "status": "ok"})
}

func main() {
	rand.Seed(time.Now().UnixNano())
	port := "3001"
	if len(os.Args) > 1 { port = os.Args[1] }
	if len(os.Args) > 2 { dspName = os.Args[2] }
	if len(os.Args) > 3 {
		switch os.Args[3] {
		case "aggressive":   strategy = Aggressive
		case "conservative": strategy = Conservative
		default:             strategy = Smart
		}
	}
	if len(os.Args) > 4 {
		if v, err := strconv.ParseFloat(os.Args[4], 64); err == nil { targetCPA = v }
	}
	// Try cwd, then dsp/ (when run from project root via start-all.sh)
	var loadErr error
	for _, path := range []string{"model_metadata.json", "dsp/model_metadata.json"} {
		loadErr = loadModel(path)
		if loadErr == nil {
			break
		}
	}
	if loadErr != nil {
		log.Printf("[%s] WARNING: %s — falling back to base CVR", dspName, loadErr)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/bid", handleBid)
	mux.HandleFunc("/health", handleHealth)
	fmt.Printf("\n📡  %s (%s) running on http://localhost:%s\n", dspName, strategy, port)
	fmt.Printf("    Bidding formula: bid = $%.2f CPA × predicted CVR\n\n", targetCPA)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
