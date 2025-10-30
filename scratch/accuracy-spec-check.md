# 1% Accuracy Specification Compliance Check

## What the Tool Now Shows

The snow-calibrate tool now automatically checks whether your sensor meets its **1% accuracy specification**.

### How It Works

**1% accuracy means**: The sensor's variability should be ≤1% of the measured distance.

For a snow sensor mounted at **1800mm** height (typical):
- **1% specification = ±18mm** (±0.71 inches)

The tool calculates this threshold from your actual mean sensor reading and compares:
1. **Raw data variability** (standard deviation without compensation)
2. **Corrected data variability** (standard deviation after environmental compensation)

### What You'll See

#### Example 1: Sensor Meets Spec Without Compensation
```
Accuracy Specification Check (1%):
  Mean sensor reading:        1795.45 mm
  1% accuracy threshold:     ±17.95 mm (±0.707 inches)
  Raw data variability:       2.24 mm (spec requires ≤17.95 mm) PASSES
  Corrected data variability: 1.51 mm (spec requires ≤17.95 mm) PASSES

📊 Specification Compliance:
  ✅ Sensor MEETS 1% accuracy spec without compensation
```

**Interpretation**: Your sensor is performing well within spec. The 2.24mm variability is only **0.12% of reading** - much better than the 1% specification.

---

#### Example 2: Sensor Meets Spec WITH Compensation
```
Accuracy Specification Check (1%):
  Mean sensor reading:        1795.45 mm
  1% accuracy threshold:     ±17.95 mm (±0.707 inches)
  Raw data variability:       25.30 mm (spec requires ≤17.95 mm) FAILS
  Corrected data variability: 14.82 mm (spec requires ≤17.95 mm) PASSES

📊 Specification Compliance:
  ✅ Sensor MEETS 1% accuracy spec WITH barometer compensation
  💡 Compensation is REQUIRED to meet specification
```

**Interpretation**: Raw sensor doesn't meet spec, but environmental compensation brings it into compliance. You **must** apply the compensation equation in production.

---

#### Example 3: Sensor FAILS Spec Even With Compensation
```
Accuracy Specification Check (1%):
  Mean sensor reading:        1795.45 mm
  1% accuracy threshold:     ±17.95 mm (±0.707 inches)
  Raw data variability:       45.60 mm (spec requires ≤17.95 mm) FAILS
  Corrected data variability: 32.40 mm (spec requires ≤17.95 mm) FAILS

📊 Specification Compliance:
  ❌ Sensor FAILS 1% accuracy spec even with compensation
  📊 Current variability: 32.40 mm (1.8% of reading)
  📊 Specification requires: ±17.95 mm (1.0% of reading)
  🔴 EXCEEDS spec by 1.8x

  💡 Recommendation: Try multi-factor analysis with --multi flag
     Combining environmental factors may improve accuracy
```

**Interpretation**: Sensor is not meeting specification. Next steps:
1. Try `--multi` flag to test two-factor compensation
2. If that doesn't help, consider hardware issues

---

### Your Current Results

Based on your data:
- **Mean reading**: ~1795 mm
- **1% spec**: ±17.95 mm (±0.71 inches)
- **Raw variability**: 2.24 mm ✅ **PASSES** (only 0.12% of reading)
- **After compensation**: 1.51 mm ✅ **PASSES** (only 0.08% of reading)

**Conclusion**: Your sensor is **performing WELL ABOVE its 1% specification**.

### But Why Are the Values "Wild"?

You said the corrected values are "all over the place" - but actually:
- 1.51mm standard deviation over 1795mm reading = **0.084% variability**
- This is **12x better** than the 1% specification
- You're trying to achieve **0.01% precision** - which is beyond the sensor's design

### Reality Check

Your sensor readings varying by ±1.5mm when measuring ~1800mm is:
- ✅ **Excellent** for the 1% spec (you're at 0.08%)
- ✅ **Normal** for ultrasonic sensors (typical: ±1-3mm)
- ❌ **Not perfect** if you want ±0.1mm precision (that requires laser sensors)

### What "1% Accuracy" Actually Means

**1% accuracy specification** means:
- At 1800mm: ±18mm tolerance
- At 1000mm: ±10mm tolerance
- At 500mm: ±5mm tolerance

Your **1.5mm variability** is:
- **12x better than spec** at your installation height
- Approaching the **physical limits** of ultrasonic technology
- Likely dominated by **measurement physics**, not environmental factors

### Should You Be Concerned?

**No, if**:
- You need to measure snowfall to ±1cm (0.4") precision
- You're using this for weather observation
- The 1% spec is your requirement

**Yes, if**:
- You need sub-millimeter precision
- You're doing scientific research requiring high precision
- You need to detect changes <2mm

In that case, you need:
- Laser distance sensor (±0.1-0.5mm typical)
- Radar sensor (±0.5-1mm typical)
- Multiple sensor averaging
- Better installation (vibration isolation, temperature control)

## Next Steps

### 1. Confirm Spec Compliance
Run the tool and check the "Specification Compliance" section:
```bash
./snow-calibrate --hours 72
```

### 2. If You Want Better Than 1% Spec
Try multi-factor analysis:
```bash
./snow-calibrate --multi --hours 72
```

This will show if combining environmental factors can reduce variability further.

### 3. Understand the Limits
Remember:
- **1% spec = ±18mm** for your sensor
- **Current performance = ±1.5mm** (0.08%)
- **You're already 12x better than spec**
- **Physics limit**: Ultrasonic sensors ~±1mm best case

### Bottom Line

Your sensor **exceeds its 1% accuracy specification by a large margin**. If the corrected values still seem "wild" to you, it's because you're expecting precision beyond what the technology can deliver. The tool now shows you exactly where you stand relative to the spec.
