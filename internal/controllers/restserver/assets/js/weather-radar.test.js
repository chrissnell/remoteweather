const { test } = require('node:test');
const assert = require('node:assert');
const { zoomForWidthMiles, buildDbzFillColor, parseManifestFrames } = require('./weather-radar.js');

test('zoomForWidthMiles ~200mi at 40N lands around z7-8', () => {
  const z = zoomForWidthMiles(200, 40.5, 800);
  assert.ok(z > 6.5 && z < 8.5, `z=${z}`);
});

test('zoomForWidthMiles clamps to [3,11]', () => {
  assert.strictEqual(zoomForWidthMiles(20000, 0, 800), 3);
  assert.strictEqual(zoomForWidthMiles(0.001, 0, 800), 11);
});

test('buildDbzFillColor is a step expression keyed on dbz', () => {
  const e = buildDbzFillColor();
  assert.strictEqual(e[0], 'step');
  assert.deepStrictEqual(e[1], ['get', 'dbz']);
  assert.strictEqual(e[2], '#c5f0f7');          // base = lowest band color
  assert.strictEqual(e[e.length - 2], 75);       // last stop input
  assert.strictEqual(e[e.length - 1], '#00ff00');// last stop color
});

test('parseManifestFrames reverses newest-first to oldest-first', () => {
  const frames = parseManifestFrames({
    schema_version: 1,
    frames: [{ ts: 200, iso: 'b' }, { ts: 100, iso: 'a' }],
  });
  assert.deepStrictEqual(frames.map(f => f.ts), [100, 200]);
});

test('parseManifestFrames returns [] for a bad manifest', () => {
  assert.deepStrictEqual(parseManifestFrames({ schema_version: 2 }), []);
  assert.deepStrictEqual(parseManifestFrames(null), []);
});
