const { test } = require('node:test');
const assert = require('node:assert');
const band = require('./weather-forecast-band.js');

const DARK_CARD = '#2a2a2a';
const LIGHT_CARD = '#f5f5f5';

test('ramp stops are sorted and span the full range', () => {
  assert.strictEqual(band.TEMP_RAMP[0].pos, 0);
  assert.strictEqual(band.TEMP_RAMP[band.TEMP_RAMP.length - 1].pos, 1);
  for (let i = 1; i < band.TEMP_RAMP.length; i++) {
    assert.ok(band.TEMP_RAMP[i].pos > band.TEMP_RAMP[i - 1].pos, 'stops must strictly increase');
  }
});

test('ramp hits the named end stops and clamps past them', () => {
  assert.strictEqual(band.tempColor(10), 'rgb(59,47,174)', '10°F deep indigo');
  assert.strictEqual(band.tempColor(105), 'rgb(210,31,60)', '105°F crimson');
  assert.strictEqual(band.tempColor(-40), band.tempColor(10), 'below the band clamps to indigo');
  assert.strictEqual(band.tempColor(130), band.tempColor(105), 'above the band clamps to crimson');
});

test('hotter is redder and less blue', () => {
  const cold = band.parseColor(band.tempColor(20));
  const hot = band.parseColor(band.tempColor(95));
  assert.ok(hot[0] > cold[0], 'red rises with temperature');
  assert.ok(hot[2] < cold[2], 'blue falls with temperature');
});

test('parseColor reads both hex and rgb()', () => {
  assert.deepStrictEqual(band.parseColor('#2a2a2a'), [42, 42, 42]);
  assert.deepStrictEqual(band.parseColor('#abc'), [170, 187, 204]);
  assert.deepStrictEqual(band.parseColor('rgb(12, 34, 56)'), [12, 34, 56]);
  assert.deepStrictEqual(band.parseColor('rgba(12, 34, 56, 0.5)'), [12, 34, 56]);
});

test('label colors clear WCAG AA against the card in both themes', () => {
  for (const card of [DARK_CARD, LIGHT_CARD]) {
    const bg = band.parseColor(card);
    for (let t = -20; t <= 120; t += 5) {
      const label = band.parseColor(band.readableTempColor(t, card));
      const ratio = band.contrastRatio(label, bg);
      assert.ok(ratio >= 4.5, `${t}°F on ${card} is only ${ratio.toFixed(2)}:1`);
    }
  }
});

test('label colors move away from the card, not toward it', () => {
  const lum = (css) => band.relativeLuminance(band.parseColor(css));
  // A 95° orange-red is bright: on a white card it has to darken.
  assert.ok(lum(band.readableTempColor(95, LIGHT_CARD)) < lum(band.tempColor(95)),
    'darkens against a light card');
  // A 10° indigo is dark: on a near-black card it has to lighten.
  assert.ok(lum(band.readableTempColor(10, DARK_CARD)) > lum(band.tempColor(10)),
    'lightens against a dark card');
  // A color that already clears AA is left exactly as the ramp drew it.
  assert.strictEqual(band.readableTempColor(95, DARK_CARD), band.tempColor(95),
    'no correction when the ramp color already passes');
});

test('columns split the width evenly and stay inside it', () => {
  for (const n of [3, 7, 10, 24]) {
    for (let i = 0; i < n; i++) {
      const x = band.columnCenterX(i, n, 1168);
      assert.ok(x > 0 && x < 1168, `column ${i}/${n} at ${x} escaped the plot`);
      if (i > 0) assert.ok(x > band.columnCenterX(i - 1, n, 1168), 'columns increase');
    }
  }
  assert.strictEqual(band.columnCenterX(0, 0, 100), 50, 'a zero column count cannot divide by zero');
});

test('band range brackets every reading with padding', () => {
  const [lo, hi] = band.bandRange([78, 47, 38, 19]);
  assert.ok(lo < 19 && hi > 78, `range must pad past the extremes: ${lo}..${hi}`);
  const [elo, ehi] = band.bandRange([]);
  assert.ok(Number.isFinite(elo) && Number.isFinite(ehi) && ehi > elo, 'empty input stays finite');
  const [flo, fhi] = band.bandRange([50, 50]);
  assert.ok(fhi > flo, 'a flat forecast still has height');
});

test('hotter maps higher on the page and clamps into the plot', () => {
  const hot = band.tempToY(85, 30, 90, 26, 212);
  const cold = band.tempToY(35, 30, 90, 26, 212);
  assert.ok(hot < cold, 'hotter is a smaller y');
  assert.strictEqual(band.tempToY(1000, 30, 90, 26, 212), 26, 'clamps to the top');
  assert.strictEqual(band.tempToY(-1000, 30, 90, 26, 212), 212, 'clamps to the bottom');
});

test('monotone spline passes through every point', () => {
  const xs = [10, 50, 90];
  const ys = [78, 48, 63];
  const at = band.monotoneSpline(xs, ys);
  xs.forEach((x, i) => assert.ok(Math.abs(at(x) - ys[i]) < 1e-9, `missed point ${i}`));
});

test('monotone spline holds flat past the ends', () => {
  const at = band.monotoneSpline([10, 50, 90], [78, 48, 63]);
  assert.strictEqual(at(-100), 78);
  assert.strictEqual(at(500), 63);
});

test('monotone spline never overshoots the data', () => {
  // A spike the forecast does not contain must not appear between days: a
  // plain cubic would overshoot past 80 on the way up.
  const xs = [0, 50, 100, 150];
  const ys = [40, 40, 80, 80];
  const at = band.monotoneSpline(xs, ys);
  for (let x = 0; x <= 150; x += 0.5) {
    const y = at(x);
    assert.ok(y >= 40 - 1e-9 && y <= 80 + 1e-9, `overshot to ${y} at x=${x}`);
  }
});

test('monotone spline survives degenerate input', () => {
  assert.strictEqual(band.monotoneSpline([], [])(5), 0);
  assert.strictEqual(band.monotoneSpline([10], [72])(999), 72);
});

test('the freeze chip never lands on the band', () => {
  const width = 1168;
  const freezeY = 150;
  const high = () => 60;
  const low = (x) => (x > 400 && x < 700 ? 200 : 100); // the band dips over the line midway
  const inside = (x) => freezeY >= high(x) - 6 && freezeY <= low(x) + 6;
  const centers = [100, 300, 500, 700, 900];
  const labels = centers.map(() => [40, 220]);
  const x = band.freezeLabelX(width, freezeY, [high, low], inside, centers, labels);
  assert.ok(!inside(x), `chip at ${x} sits on the band`);
  assert.ok(x >= 44 && x <= width - 44, 'chip stays on the plot');
});

test('the freeze chip falls back when the band covers the whole line', () => {
  const x = band.freezeLabelX(600, 100, [() => 100], () => true, [300], [[90]]);
  assert.ok(x >= 0 && x <= 600, 'still produces a usable position');
});

test('day labels abbreviate to three letters, today stays whole', () => {
  assert.strictEqual(band.shortDayLabel('Wednesday'), 'WED');
  assert.strictEqual(band.shortDayLabel('Next Thursday'), 'THU');
  assert.strictEqual(band.shortDayLabel('Today'), 'TODAY');
  assert.strictEqual(band.shortDayLabel(''), '--');
});

test('day labels shrink by tier, shedding the "Next" prefix first', () => {
  assert.strictEqual(band.dayLabelAtTier('Next Thursday', 'THU', 0, false), 'NEXT THURSDAY');
  assert.strictEqual(band.dayLabelAtTier('Next Thursday', 'THU', 1, false), 'THURSDAY');
  assert.strictEqual(band.dayLabelAtTier('Next Thursday', 'THU', 2, false), 'THU');
  // A name with no prefix to shed is unchanged until the weekday tier.
  assert.strictEqual(band.dayLabelAtTier('Tomorrow', 'FRI', 1, false), 'TOMORROW');
  assert.strictEqual(band.dayLabelAtTier('Tomorrow', 'FRI', 2, false), 'FRI');
  // Today keeps its name one tier longer than the rest, then gives it up too.
  assert.strictEqual(band.dayLabelAtTier('Today', 'THU', 2, true), 'TODAY');
  assert.strictEqual(band.dayLabelAtTier('Today', 'THU', 3, true), 'THU');
  // Without a supplied weekday the last tiers derive one.
  assert.strictEqual(band.dayLabelAtTier('Wednesday', null, 3, false), 'WED');
  assert.strictEqual(band.dayLabelAtTier('', 'MON', 0, false), '--');
  // The tiers really do get shorter, which is what the fitting loop relies on.
  let previous = Infinity;
  for (let tier = 0; tier < band.DAY_LABEL_TIERS; tier++) {
    const length = band.dayLabelAtTier('Next Thursday', 'THU', tier, false).length;
    assert.ok(length <= previous, `tier ${tier} is not shorter`);
    previous = length;
  }
});

test('narrow columns drop the leading zero only from a decimal', () => {
  assert.strictEqual(band.stripLeadingZero('0.32"'), '.32"');
  assert.strictEqual(band.stripLeadingZero('6.2"'), '6.2"');
  assert.strictEqual(band.stripLeadingZero('0"'), '0"');
  assert.strictEqual(band.stripLeadingZero(''), '');
});

test('clock strings parse to minutes past midnight', () => {
  assert.strictEqual(band.parseClockMinutes('7:14 AM'), 434);
  assert.strictEqual(band.parseClockMinutes('6:02 PM'), 1082);
  assert.strictEqual(band.parseClockMinutes('12:00 AM'), 0);
  assert.strictEqual(band.parseClockMinutes('12:30 PM'), 750);
  assert.strictEqual(band.parseClockMinutes('--'), null);
  assert.strictEqual(band.parseClockMinutes(null), null);
});

test('night runs from sunset to sunrise', () => {
  const sunrise = 434;  // 7:14 AM
  const sunset = 1082;  // 6:02 PM
  assert.strictEqual(band.isNightMinute(0, sunrise, sunset), true, 'midnight is night');
  assert.strictEqual(band.isNightMinute(420, sunrise, sunset), true, '7:00 AM is still dark');
  assert.strictEqual(band.isNightMinute(480, sunrise, sunset), false, '8:00 AM is day');
  assert.strictEqual(band.isNightMinute(1080, sunrise, sunset), false, '6:00 PM is day');
  assert.strictEqual(band.isNightMinute(1140, sunrise, sunset), true, '7:00 PM is night');
  assert.strictEqual(band.isNightMinute(300, null, sunset), false, 'no sun times, no shading');
});
