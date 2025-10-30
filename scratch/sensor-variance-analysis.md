# Snow Sensor Variance Analysis

## Current Situation

**Raw sensor data:**
- Standard deviation: 2.24 mm (0.088 inches)
- Range: 1791-1798 mm (7 mm spread)

**After barometric compensation (best single factor):**
- Standard deviation: 1.51 mm (0.059 inches)
- Variance reduction: **32.7%** (Moderate)
- Range: ~6mm spread in corrected values

## Problem Assessment

The corrected values are still "wild" - varying significantly despite compensation. This indicates:

1. **Barometric pressure explains only ~33% of variance**
2. **Other factors (unmeasured or uncorrelated) cause ~67% of variance**
3. **Environmental compensation alone won't stabilize the readings**

## Possible Causes of Remaining Variance

### 1. Measurement Noise (Sensor Precision)
- Ultrasonic sensors typically have ±1-3mm precision
- Your 1.51mm std dev after compensation might be **at the sensor's precision limit**
- **No amount of environmental compensation can fix inherent sensor noise**

### 2. Time-Based Drift
- Sensor may drift slowly over time (hours/days) independent of environment
- Could be thermal equilibrium, electronics aging, mounting stress
- Solution: Add time-based component to model

### 3. Multiple Simultaneous Factors
- Temperature AND humidity together (not individually)
- Wind vibration + temperature
- Solution: Multi-factor regression

### 4. Unmeasured Factors
- Direct sunlight on sensor
- Sensor heating from electronics
- Mounting structure thermal expansion
- Ice/frost formation
- Solution: Add more sensors or physical fixes

## Advanced Statistical Approaches

### Option 1: Multi-Factor Regression
Test combinations of environmental factors:
```
drift = c0 + c1×temp + c2×humidity + c3×barometer + c4×wind
```

**Pros:**
- Can capture interactions between factors
- May explain more variance

**Cons:**
- Risk of overfitting with many parameters
- Need more data points (>100-200 hours)
- Diminishing returns if sensor noise dominates

### Option 2: Grid Search Optimization
Instead of least-squares regression, directly optimize coefficients to minimize variance:
```
Find coefficients that minimize: std_dev(corrected_distances)
```

**Pros:**
- Directly optimizes your goal (flat corrected values)
- Can find non-obvious relationships

**Cons:**
- Computationally expensive
- Can overfit to specific data period
- May not generalize to different conditions

### Option 3: Time-Series Analysis
Model sensor drift as:
```
drift = environmental_component + time_trend + noise
```

**Pros:**
- Captures slow sensor drift
- Better for long-term calibration

**Cons:**
- Requires longer data periods (weeks/months)
- Trend may not be stable

### Option 4: Machine Learning (Neural Networks, Random Forest)
Use ML to find complex patterns:

**Pros:**
- Can find non-linear interactions
- May discover patterns humans miss

**Cons:**
- **Massive overkill for this problem**
- Requires huge training dataset (thousands of points)
- Black box - can't generate simple equation
- Won't help if variance is sensor noise

## Recommendations

### Immediate Actions

1. **Check if 1.5mm variance is acceptable for your use case**
   - For snowfall measurement, is ±1.5mm precision enough?
   - If yes, **you're done** - this may be sensor limit

2. **Try multi-factor regression** (`--multi` flag)
   - Test temp+humidity, temp+barometer, etc.
   - See if you can get >50% variance reduction
   - Diminishing returns likely beyond this

3. **Collect more data**
   - Current analysis: 18 hours
   - Try: 72+ hours covering different conditions
   - More data = better model fitting

### Physical Solutions (May Be Better Than Math)

1. **Sensor shield/housing**
   - Protect from direct sunlight
   - Reduce temperature swings
   - Block wind vibration

2. **Use multiple sensors**
   - Average 2-3 identical sensors
   - √n reduction in random noise
   - Costs money but simple physics

3. **Longer averaging periods**
   - Current: 1-hour averages
   - Try: 3-hour or 6-hour averages
   - Trades time resolution for stability

### Reality Check

**If variance doesn't drop below ~1mm:**
- You've hit sensor physics limits
- Environmental factors aren't the issue
- Solution is hardware, not math

**Your 32.7% reduction is actually decent** - it shows barometric pressure does affect the sensor. But getting to 5-10% remaining variance might be impossible with this sensor design.

## Implemented Advanced Features

### ✅ Multi-Factor Regression
**Usage**: `./snow-calibrate --multi`

Tests ALL two-factor combinations:
- Temperature + Humidity
- Temperature + Windspeed
- Temperature + Barometer
- Humidity + Windspeed
- Humidity + Barometer
- Windspeed + Barometer

**How it works**:
```
drift = c0 + c1×factor1 + c2×factor2
```

Uses multiple linear regression to find coefficients that best explain sensor drift based on TWO environmental factors simultaneously.

**What you'll see**:
- Table showing all combinations ranked by variance reduction
- Best combination highlighted
- Equation for the best two-factor model
- Direct comparison to single-factor results

**Example output**:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
MULTI-FACTOR ANALYSIS (Two-Factor Combinations)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Testing all two-factor combinations on 156 data points

Factor Combination            |       R² |  Var Red % | Corr StdDev | Status
-------------------------------+----------+------------+------------+-----------------
Barometer + Temperature       |   0.4521 |      45.2% |    1.23 mm | Moderate
Temperature + Humidity        |   0.4103 |      41.0% |    1.32 mm | Moderate
Barometer + Humidity          |   0.3567 |      35.7% |    1.44 mm | Moderate
...

🏆 BEST TWO-FACTOR COMBINATION:
  Factors: Barometer + Temperature
  Equation: drift = 125.4321 + (-0.0234)×barometer + 0.0512×temperature
  R² = 0.4521
  Variance reduction: 45.2%
  Corrected std dev: 1.23 mm (0.048 inches)
```

### Key Insights

**If two-factor variance reduction is >50%**:
- Environmental factors explain most variance
- You can achieve stable readings with compensation
- Implement the two-factor equation in production

**If two-factor variance reduction is 25-50%**:
- Environmental factors help but don't explain everything
- Consider testing 3-factor models (temp+humidity+barometer)
- Remaining variance may be sensor noise or unmeasured factors

**If two-factor variance reduction is <35%**:
- Environmental compensation has limited benefit
- Remaining variance is likely:
  - Inherent sensor precision limits (~1-2mm for ultrasonic)
  - Unmeasured physical factors (sunlight, ice, vibration)
  - Electronic noise
- **Solution: Hardware improvements, not better math**

## What to Try Next

### 1. Run Multi-Factor Analysis
```bash
./snow-calibrate --multi --hours 72
```

This will:
- Test all two-factor combinations
- Show you the best achievable variance reduction
- Tell you if you've hit sensor limits

### 2. Collect More Data
Current analysis uses limited hours. For better results:
```bash
./snow-calibrate --multi --hours 168  # 1 week of data
```

More data = more reliable coefficient fitting, especially for multi-factor models.

### 3. Interpret the Results

**Best case scenario** (>50% variance reduction):
- Implement the two-factor compensation equation
- Your corrected readings will be stable
- Problem solved!

**Moderate case** (35-50% variance reduction):
- Two factors help significantly
- Remaining ~1mm variance might be acceptable
- Decision: Is 1mm precision good enough for your needs?

**Worst case** (<35% variance reduction):
- Environmental factors aren't the main issue
- You've hit sensor physics limits
- Next step: Hardware solutions, not math

## Hardware Solutions (When Math Isn't Enough)

If multi-factor regression doesn't get you below 1mm variance:

1. **Better Sensor**
   - Upgrade to laser or radar-based snow sensor
   - Typical precision: ±0.1-0.5mm
   - Cost: $$$$

2. **Multiple Sensors + Averaging**
   - Use 2-3 identical ultrasonic sensors
   - Average their readings
   - Variance reduces by √n (30-40% improvement)
   - Cost: $-$$

3. **Physical Shielding**
   - Sun shield (prevents heating)
   - Wind shield (reduces vibration)
   - Temperature insulation
   - Cost: $

4. **Longer Averaging Windows**
   - Current: 1-hour averages
   - Try: 3-6 hour averages
   - Trades time resolution for stability
   - Cost: Free

## Bottom Line

**Target variance**: What precision do you actually need?
- **±2mm (0.08")**: Achievable with single-factor compensation
- **±1mm (0.04")**: May need two-factor compensation
- **±0.5mm (0.02")**: Likely requires hardware upgrade

Run `--multi` and see what you get. The variance reduction percentage will tell you if you've hit the mathematical limit or if there's room for improvement.
