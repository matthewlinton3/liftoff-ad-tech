#!/bin/bash
# start-all.sh — starts the exchange + 10 DSPs + SSP
# Usage: ./start-all.sh
# Stop:  Ctrl+C

set -e

echo ""
echo "🚀 Building and starting all services..."
echo ""

mkdir -p bin

# Build everything first
(cd exchange     && go build -o ../bin/exchange     .) && echo "✅ Exchange built"
(cd dsp          && go build -o ../bin/dsp          .) && echo "✅ DSP built"
(cd ssp          && go build -o ../bin/ssp          .) && echo "✅ SSP built"
(cd test-auction && go build -o ../bin/test-auction .) && echo "✅ Test tool built"

echo ""
echo "Starting services..."
echo ""

# Start exchange
./bin/exchange &
EXCHANGE_PID=$!

sleep 0.3

# Start 10 DSPs: port name strategy
./bin/dsp 3001 AlphaDSP aggressive &
DSP1_PID=$!
./bin/dsp 3002 BetaDSP conservative &
DSP2_PID=$!
./bin/dsp 3003 GammaDSP smart &
DSP3_PID=$!
./bin/dsp 3004 DeltaDSP aggressive &
DSP4_PID=$!
./bin/dsp 3005 EpsilonDSP conservative &
DSP5_PID=$!
./bin/dsp 3006 ZetaDSP smart &
DSP6_PID=$!
./bin/dsp 3007 EtaDSP aggressive &
DSP7_PID=$!
./bin/dsp 3008 ThetaDSP conservative &
DSP8_PID=$!
./bin/dsp 3009 IotaDSP smart &
DSP9_PID=$!
./bin/dsp 3010 KappaDSP aggressive &
DSP10_PID=$!

sleep 0.3

# Start SSP
./bin/ssp &
SSP_PID=$!

echo ""
echo "All services running!"
echo "  Exchange:  http://localhost:3000"
echo "  AlphaDSP:  http://localhost:3001"
echo "  BetaDSP:   http://localhost:3002"
echo "  GammaDSP:  http://localhost:3003"
echo "  DeltaDSP:  http://localhost:3004"
echo "  EpsilonDSP: http://localhost:3005"
echo "  ZetaDSP:   http://localhost:3006"
echo "  EtaDSP:    http://localhost:3007"
echo "  ThetaDSP:  http://localhost:3008"
echo "  IotaDSP:   http://localhost:3009"
echo "  KappaDSP:  http://localhost:3010"
echo "  SSP:       http://localhost:4000"
echo ""
echo "Run a test auction in a new terminal:"
echo "  ./bin/test-auction"
echo "Open publisher page: http://localhost:4000"
echo ""
echo "Press Ctrl+C to stop all services"

# Cleanup on exit
trap "kill $EXCHANGE_PID $DSP1_PID $DSP2_PID $DSP3_PID $DSP4_PID $DSP5_PID $DSP6_PID $DSP7_PID $DSP8_PID $DSP9_PID $DSP10_PID $SSP_PID 2>/dev/null; echo ''; echo 'All services stopped.'" EXIT

wait
