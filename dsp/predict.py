#!/usr/bin/env python3
"""
predict.py — called by Go DSP to score an impression
Usage: python predict.py <features_json>
Returns: CVR probability as float
"""
import sys, json, joblib, numpy as np

model = joblib.load('xgb_model.joblib')
meta  = json.load(open('xgb_metadata.json'))

features = json.loads(sys.argv[1])
x = np.array([[
    features.get('device_type', 2),
    features.get('os', 2),
    features.get('country', 6),
    features.get('hour', 12),
    features.get('day_of_week', 0),
    features.get('is_business_hours', 0),
    features.get('is_evening', 0),
    features.get('is_weekend', 0),
    features.get('site_cat', 5),
    features.get('ad_size', 0),
    features.get('user_recency', 1.0),
    features.get('frequency', 1),
    features.get('bid_floor', 1.0),
    features.get('connection_type', 0),
    features.get('app_installs', 10),
    features.get('session_depth', 5),
]])
prob = model.predict_proba(x)[0][1]
print(f"{prob:.6f}")
