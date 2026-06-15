// Live weather radar for the station website. Reuses the Graywolf maps service
// (https://maps.nw5w.com): Americana basemap + per-frame NEXRAD vector contours.
//
// The pure helpers below are dual-exported for node --test; the MapLibre glue
// runs only in the browser.

const DBZ_BANDS = [5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 60, 65, 70, 75];
const DBZ_COLORS = {
  5: '#c5f0f7', 10: '#a8e6f0', 15: '#88ddee', 20: '#00a3e0', 25: '#0077aa',
  30: '#005588', 35: '#ffee00', 40: '#ffaa00', 45: '#ff4400', 50: '#c10000',
  55: '#ffaaff', 60: '#ff77ff', 65: '#ffffff', 70: '#ffffff', 75: '#00ff00',
};

// MapLibre `step` expression: polygon `dbz` -> NWS/RainViewer ramp.
function buildDbzFillColor() {
  const expr = ['step', ['get', 'dbz'], DBZ_COLORS[DBZ_BANDS[0]]];
  for (let i = 1; i < DBZ_BANDS.length; i++) expr.push(DBZ_BANDS[i], DBZ_COLORS[DBZ_BANDS[i]]);
  return expr;
}

// Zoom so a `widthPx`-wide map spans `miles` at latitude `latDeg`.
// Web-Mercator: meters/px = 156543.03 * cos(lat) / 2^z.
function zoomForWidthMiles(miles, latDeg, widthPx) {
  const meters = miles * 1609.344;
  const lat = (latDeg * Math.PI) / 180;
  const z = Math.log2((156543.03 * Math.cos(lat) * widthPx) / meters);
  return Math.max(3, Math.min(z, 11));
}

// Radar manifest -> oldest-first [{ts, iso}]. Manifest `frames` is newest-first.
function parseManifestFrames(json) {
  if (!json || typeof json !== 'object') return [];
  if (json.schema_version !== 1 || !Array.isArray(json.frames)) return [];
  const frames = [];
  for (const f of json.frames) {
    if (!f || typeof f.ts !== 'number' || typeof f.iso !== 'string') continue;
    frames.push({ ts: f.ts, iso: f.iso });
  }
  frames.reverse();
  return frames;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { DBZ_BANDS, DBZ_COLORS, buildDbzFillColor, zoomForWidthMiles, parseManifestFrames };
}

// ---- Browser glue (skipped under node --test) -----------------------------
if (typeof window !== 'undefined' && typeof document !== 'undefined') {
  const TILE_HOST = 'maps.nw5w.com';
  const STYLE_URL = `https://${TILE_HOST}/style/americana-roboto/style.json`;
  const MANIFEST_URL = `https://${TILE_HOST}/radar/manifest.json`;
  const FRAME_MS = 500;        // per-frame dwell
  const HOLD_MS = 1500;        // longer hold on the newest frame
  const POLL_MS = 60000;       // manifest refresh
  const DESKTOP_MAX_FRAMES = 36;
  const MOBILE_MAX_FRAMES = 12;

  function initRadar() {
    const cfgEl = document.getElementById('radar-config');
    const mapEl = document.getElementById('radar-map');
    if (!cfgEl || !mapEl || !window.maplibregl) return;
    const cfg = JSON.parse(cfgEl.textContent);
    const isTouch = window.matchMedia('(pointer: coarse)').matches;
    const maxFrames = isTouch ? MOBILE_MAX_FRAMES : DESKTOP_MAX_FRAMES;

    const tokenParam = (url) => {
      const sep = url.includes('?') ? '&' : '?';
      return `${url}${sep}t=${encodeURIComponent(cfg.token)}`;
    };

    const map = new maplibregl.Map({
      container: 'radar-map',
      style: STYLE_URL,
      center: [cfg.lon, cfg.lat],
      zoom: zoomForWidthMiles(cfg.widthMiles || 200, cfg.lat, mapEl.clientWidth || 800),
      attributionControl: { compact: true },
      cooperativeGestures: isTouch,
      transformRequest: (url) => {
        if (url.startsWith(`https://${TILE_HOST}/`) && !url.startsWith(`https://${TILE_HOST}/style/`)) {
          return { url: tokenParam(url) };
        }
        return { url };
      },
    });
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right');
    map.addControl(new maplibregl.ScaleControl({ unit: 'imperial' }), 'bottom-left');

    // Keep ~200mi width across resizes/orientation.
    let resizeTimer = null;
    window.addEventListener('resize', () => {
      clearTimeout(resizeTimer);
      resizeTimer = setTimeout(() => {
        map.setZoom(zoomForWidthMiles(cfg.widthMiles || 200, cfg.lat, mapEl.clientWidth || 800));
      }, 200);
    });

    map.on('load', () => {
      new maplibregl.Marker().setLngLat([cfg.lon, cfg.lat]).addTo(map);
      startRadar(map, tokenParam, maxFrames, !isTouch);
    });
    // Single-tile fetch errors are benign; only log.
    map.on('error', (e) => console.warn('[radar]', e && e.error ? e.error.message : e));
  }

  function startRadar(map, tokenParam, maxFrames, autoplay) {
    const sourceId = (ts) => `radar-${ts}`;
    const layerId = (ts) => `radar-fill-${ts}`;
    const mounted = new Set();
    let frames = [];        // oldest-first [{ts, iso}]
    let index = 0;
    let curTs = null;
    let playing = false;
    let timer = null;
    const fillColor = buildDbzFillColor();

    const firstSymbolId = () => (map.getStyle().layers.find((l) => l.type === 'symbol') || {}).id;

    function ensureFrame(ts, visible) {
      if (!map.getSource(sourceId(ts))) {
        map.addSource(sourceId(ts), {
          type: 'vector', minzoom: 3, maxzoom: 8,
          tiles: [tokenParam(`https://maps.nw5w.com/radar/${ts}/{z}/{x}/{y}.pbf`)],
          attribution: 'NEXRAD via NWS / Iowa State Mesonet',
        });
      }
      if (!map.getLayer(layerId(ts))) {
        map.addLayer({
          id: layerId(ts), type: 'fill', source: sourceId(ts), 'source-layer': 'radar',
          paint: { 'fill-color': fillColor, 'fill-antialias': false, 'fill-opacity': visible ? 0.75 : 0 },
        }, firstSymbolId());
      }
      mounted.add(ts);
    }

    function removeFrame(ts) {
      if (map.getLayer(layerId(ts))) map.removeLayer(layerId(ts));
      if (map.getSource(sourceId(ts))) map.removeSource(sourceId(ts));
      mounted.delete(ts);
    }

    function showFrame(ts) {
      if (ts === curTs) return;
      const prev = curTs;
      curTs = ts;
      ensureFrame(ts, true);
      if (prev != null && map.getLayer(layerId(prev))) map.setPaintProperty(layerId(prev), 'fill-opacity', 0);
      if (map.getLayer(layerId(ts))) map.setPaintProperty(layerId(ts), 'fill-opacity', 0.75);
      updateUI();
    }

    function scheduleNext() {
      clearTimeout(timer);
      if (!playing || frames.length <= 1) return;
      const atNewest = index >= frames.length - 1;
      timer = setTimeout(() => {
        index = (index + 1) % frames.length;
        showFrame(frames[index].ts);
        scheduleNext();
      }, atNewest ? HOLD_MS : FRAME_MS);
    }

    function play() { if (frames.length > 1) { playing = true; scheduleNext(); updateUI(); } }
    function pause() { playing = false; clearTimeout(timer); updateUI(); }
    function reset() { pause(); index = frames.length - 1; if (frames[index]) showFrame(frames[index].ts); }
    function seek(i) { pause(); index = Math.max(0, Math.min(i, frames.length - 1)); if (frames[index]) showFrame(frames[index].ts); }

    async function poll() {
      try {
        const res = await fetch(tokenParam(MANIFEST_URL), { cache: 'no-store' });
        if (!res.ok) return;
        let next = parseManifestFrames(await res.json());
        if (next.length > maxFrames) next = next.slice(next.length - maxFrames);
        if (!next.length) return;
        const keep = new Set(next.map((f) => f.ts));
        for (const ts of [...mounted]) if (!keep.has(ts) && ts !== curTs) removeFrame(ts);
        next.forEach((f) => ensureFrame(f.ts, false));
        frames = next;
        if (curTs == null || !keep.has(curTs)) { index = frames.length - 1; showFrame(frames[index].ts); }
        else { index = frames.findIndex((f) => f.ts === curTs); }
        updateUI();
      } catch (e) { console.warn('[radar] manifest poll failed', e); }
    }

    // ---- controls ----
    const btnPlay = document.getElementById('radar-play');
    const btnReset = document.getElementById('radar-reset');
    const slider = document.getElementById('radar-scrubber');
    const tsLabel = document.getElementById('radar-timestamp');
    if (btnPlay) btnPlay.addEventListener('click', () => (playing ? pause() : play()));
    if (btnReset) btnReset.addEventListener('click', reset);
    if (slider) slider.addEventListener('input', (e) => seek(Number(e.target.value)));

    function updateUI() {
      if (btnPlay) btnPlay.textContent = playing ? 'Pause' : 'Play';
      if (slider) { slider.max = String(Math.max(0, frames.length - 1)); slider.value = String(index); }
      if (tsLabel && frames[index]) {
        tsLabel.textContent = new Date(frames[index].ts * 1000)
          .toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
      }
    }

    poll().then(() => { if (autoplay) play(); });
    setInterval(poll, POLL_MS);
  }

  // Lazy init: build the map only when the section scrolls into view.
  document.addEventListener('DOMContentLoaded', () => {
    const section = document.getElementById('radar-section');
    if (!section) return;
    if (!('IntersectionObserver' in window)) { initRadar(); return; }
    const io = new IntersectionObserver((entries, obs) => {
      if (entries.some((e) => e.isIntersecting)) { obs.disconnect(); initRadar(); }
    }, { rootMargin: '200px' });
    io.observe(section);
  });
}
