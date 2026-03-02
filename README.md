# Ad Tech Stack — Go Implementation

A minimal but realistic programmatic advertising supply chain built in Go.
Implements OpenRTB 2.6 with a second-price auction.

## Architecture

```
Publisher Site
     │
     ▼
  SSP (coming next)
     │  sends bid request
     ▼
 Exchange  ◄── POST /auction (OpenRTB bid request)
     │
     ├──► AlphaDSP :3001  (aggressive bidder)
     ├──► BetaDSP  :3002  (conservative bidder)
     └──► GammaDSP :3003  (smart bidder)
          │
          └── scoreImpression()  ◄── ML model plugs in here
```

## Setup

### 1. Install Go
Download from https://go.dev/dl/ — just run the installer, default settings.

### 2. Folder structure
```
adtech/
├── exchange/        ← the auction engine
├── dsp/             ← DSP bidder (runs as 3 instances)
├── test-auction/    ← fires a test bid request
├── start-all.sh     ← starts everything (Mac/Linux)
└── README.md
```

## Running

### Mac / Linux
```bash
chmod +x start-all.sh
./start-all.sh
```

Then in a **new terminal**:
```bash
./bin/test-auction
```

### Windows
Open 4 separate terminals in the `adtech/` folder:

**Terminal 1 — Exchange:**
```
cd exchange
go run main.go
```

**Terminal 2 — AlphaDSP:**
```
cd dsp
go run main.go 3001 AlphaDSP aggressive
```

**Terminal 3 — BetaDSP:**
```
cd dsp
go run main.go 3002 BetaDSP conservative
```

**Terminal 4 — GammaDSP:**
```
cd dsp
go run main.go 3003 GammaDSP smart
```

**Terminal 5 — Test:**
```
cd test-auction
go run main.go
```

## Example output

```
📤 Sending bid request to exchange...
   Imp: 300x250 banner | Floor: $0.50
   User: iOS in San Francisco

✅ AUCTION COMPLETE
   Winner:         dsp-1
   Clearing price: $3.8401
   Creative ID:    cr-002
   Auction time:   8ms

   All bids:
   🏆 AlphaDSP: $5.2500
      GammaDSP: $3.8400
      BetaDSP:  $2.1000
```

## Where the ML model plugs in

In `dsp/main.go`, find the `scoreImpression()` function.
This is currently a rule-based scorer. Replace it with a trained
CTR prediction model (Python → ONNX → loaded in Go) for a real DSP.

## API

### POST /auction
Accepts an OpenRTB 2.6 bid request, returns the winning bid.

### GET /health
Returns exchange status.

### GET /dsps
Lists registered DSPs.
