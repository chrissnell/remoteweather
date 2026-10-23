// Forecast temperature band — the hero plot for both forecast screens.
//
// The 10-day view is one edge-to-edge band: its top edge is the interpolated
// daily high, its bottom edge the daily low, and every pixel column is a
// vertical gradient between the two sampled off a fixed temperature ramp, so
// the fill *is* the temperature across the week. The 24-hour view is the same
// instrument with a single curve and the night hours shaded.
//
// The ramp is shared with the Blue Aspen Observatory LED panel
// (blue-aspen-observatory, src/app/color.rs), so a color means the same
// temperature on the panel and on this page.
//
// The pure helpers below are dual-exported for node --test; the canvas/SVG
// glue only touches the DOM from the browser.

const WeatherForecastBand = (function () {
  'use strict';

  // ---------------------------------------------------------------------
  // Temperature ramp
  // ---------------------------------------------------------------------

  // Readings outside this band clamp to the end stops. Ramp positions are
  // these °F stops normalized onto [0, 1].
  const TEMP_LO = 10;
  const TEMP_HI = 105;

  // Perceptual cold->hot ramp: deep indigo (<=10°F) -> cyan (32°) -> teal-green
  // (50°) -> green-gold (68°) -> amber (82°) -> orange-red (95°) -> crimson
  // (>=105°).
  const TEMP_RAMP = [
    { pos: 0.0000, r: 0x3b, g: 0x2f, b: 0xae }, //  10°F deep indigo
    { pos: 0.2316, r: 0x22, g: 0xb6, b: 0xd6 }, //  32°F cyan
    { pos: 0.4211, r: 0x2f, g: 0xbf, b: 0x71 }, //  50°F teal-green
    { pos: 0.6105, r: 0x9f, g: 0xd2, b: 0x3a }, //  68°F green-gold
    { pos: 0.7579, r: 0xf2, g: 0xb6, b: 0x32 }, //  82°F amber
    { pos: 0.8947, r: 0xf2, g: 0x63, b: 0x2f }, //  95°F orange-red
    { pos: 1.0000, r: 0xd2, g: 0x1f, b: 0x3c }, // 105°F crimson
  ];

  // Normalized position on the ramp -> [r, g, b].
  function sampleRamp(v) {
    const t = Math.min(1, Math.max(0, v));
    let i = 0;
    while (i + 1 < TEMP_RAMP.length && t > TEMP_RAMP[i + 1].pos) i++;
    const a = TEMP_RAMP[i];
    const b = TEMP_RAMP[Math.min(i + 1, TEMP_RAMP.length - 1)];
    const f = Math.min(1, Math.max(0, (t - a.pos) / Math.max(1e-6, b.pos - a.pos)));
    const lerp = (x, y) => Math.round(x + (y - x) * f);
    return [lerp(a.r, b.r), lerp(a.g, b.g), lerp(a.b, b.b)];
  }

  const tempRGB = (f) => sampleRamp((f - TEMP_LO) / (TEMP_HI - TEMP_LO));
  const tempColor = (f) => { const c = tempRGB(f); return `rgb(${c[0]},${c[1]},${c[2]})`; };
  const tempColorAlpha = (f, a) => { const c = tempRGB(f); return `rgba(${c[0]},${c[1]},${c[2]},${a})`; };

  // ---------------------------------------------------------------------
  // Contrast
  // ---------------------------------------------------------------------

  // Theme variables are hex (#2a2a2a / #f5f5f5); ramp colors come back as rgb().
  function parseColor(str) {
    const s = String(str || '').trim();
    if (s[0] === '#') {
      const hex = s.length === 4
        ? s.slice(1).split('').map((c) => c + c).join('')
        : s.slice(1, 7);
      return [0, 2, 4].map((i) => parseInt(hex.substr(i, 2), 16));
    }
    const m = s.match(/[\d.]+/g);
    return m && m.length >= 3 ? [+m[0], +m[1], +m[2]] : [0, 0, 0];
  }

  function relativeLuminance(rgb) {
    const lin = (v) => {
      const x = v / 255;
      return x <= 0.04045 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4);
    };
    return 0.2126 * lin(rgb[0]) + 0.7152 * lin(rgb[1]) + 0.0722 * lin(rgb[2]);
  }

  function contrastRatio(a, b) {
    const la = relativeLuminance(a);
    const lb = relativeLuminance(b);
    return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
  }

  // The ramp is tuned for an LED panel on black; as *text* on a page it has to
  // clear WCAG AA against the card in both themes. Step the ramp color toward
  // the card's black or white end until it does — same hue, same meaning,
  // legible either way.
  function readableTempColor(tempF, backgroundCss) {
    let c = tempRGB(tempF);
    const bg = parseColor(backgroundCss);
    const toward = relativeLuminance(bg) > 0.5 ? [0, 0, 0] : [255, 255, 255];
    for (let i = 0; i < 24 && contrastRatio(c, bg) < 4.5; i++) {
      c = c.map((v, k) => Math.round(v + (toward[k] - v) * 0.08));
    }
    return `rgb(${c[0]},${c[1]},${c[2]})`;
  }

  // ---------------------------------------------------------------------
  // Plot geometry
  // ---------------------------------------------------------------------

  // Center x of column `i` of `n` across `width`, splitting it evenly.
  function columnCenterX(i, n, width) {
    const colW = width / Math.max(1, n);
    return colW * i + colW / 2;
  }

  // The temperature span the plot maps onto its vertical extent, padded so the
  // extremes don't sit exactly on the edges. Returns [min, max], max > min.
  function bandRange(temps) {
    const valid = (temps || []).filter((t) => Number.isFinite(t));
    if (!valid.length) return [0, 1];
    let lo = Math.min.apply(null, valid);
    let hi = Math.max.apply(null, valid);
    const pad = Math.max(3, (hi - lo) * 0.12);
    lo -= pad;
    hi += pad;
    if (hi - lo < 1) hi = lo + 1;
    return [lo, hi];
  }

  // Map a temperature onto the plot's y: `tmax` at `yTop`, `tmin` at `yBot`, so
  // hotter is higher on the page. Clamped into the plot.
  function tempToY(temp, tmin, tmax, yTop, yBot) {
    const f = Math.min(1, Math.max(0, (temp - tmin) / (tmax - tmin)));
    return yBot - f * (yBot - yTop);
  }

  // Monotone cubic (Fritsch-Carlson) interpolation through every point: smooth,
  // but it can never overshoot and invent a high warmer than the warmest day.
  function monotoneSpline(xs, ys) {
    const n = xs.length;
    if (!n) return () => 0;
    if (n === 1) return () => ys[0];

    const dx = [];
    const dy = [];
    const slope = [];
    for (let i = 0; i < n - 1; i++) {
      dx.push(xs[i + 1] - xs[i]);
      dy.push(ys[i + 1] - ys[i]);
      slope.push(dy[i] / dx[i]);
    }

    const tangent = [slope[0]];
    for (let i = 1; i < n - 1; i++) {
      if (slope[i - 1] * slope[i] <= 0) {
        tangent.push(0);
      } else {
        const w1 = 2 * dx[i] + dx[i - 1];
        const w2 = dx[i] + 2 * dx[i - 1];
        tangent.push((w1 + w2) / (w1 / slope[i - 1] + w2 / slope[i]));
      }
    }
    tangent.push(slope[n - 2]);

    return function sampleAt(x) {
      if (x <= xs[0]) return ys[0];
      if (x >= xs[n - 1]) return ys[n - 1];
      let i = 0;
      while (i < n - 2 && x > xs[i + 1]) i++;
      const h = dx[i];
      const t = (x - xs[i]) / h;
      const t2 = t * t;
      const t3 = t2 * t;
      return (2 * t3 - 3 * t2 + 1) * ys[i]
        + (t3 - 2 * t2 + t) * h * tangent[i]
        + (-2 * t3 + 3 * t2) * ys[i + 1]
        + (t3 - t2) * h * tangent[i + 1];
    };
  }

  // Park the 32° marker where the dashed line has the most room: never on top
  // of the band, as far as possible from the columns whose labels crowd it.
  function freezeLabelX(width, freezeY, edges, isInsideBand, centers, labelYs) {
    const busy = centers.filter((_, i) => (labelYs[i] || []).some((y) => Math.abs(y - freezeY) < 26));
    let best = null;
    let bestScore = -1;
    for (let x = 44; x <= width - 44; x += 4) {
      if (isInsideBand(x)) continue;
      const clear = Math.min.apply(null, edges.map((f) => Math.abs(f(x) - freezeY)));
      const nearest = busy.length ? Math.min.apply(null, busy.map((c) => Math.abs(c - x))) : Infinity;
      const score = Math.min(clear, 26) * 2 + Math.min(nearest, 60);
      if (score > bestScore) {
        bestScore = score;
        best = x;
      }
    }
    // The band covers the line everywhere: tuck the chip against the left edge.
    return best === null ? Math.min(44, width / 2) : best;
  }

  // ---------------------------------------------------------------------
  // Formatting
  // ---------------------------------------------------------------------

  // "Next Thursday" -> "THU", "Today" -> "TODAY" (kept whole, it is the anchor).
  function shortDayLabel(name) {
    const s = String(name || '').trim();
    if (!s) return '--';
    if (/^today$/i.test(s)) return 'TODAY';
    const word = s.split(/\s+/).pop();
    return word.slice(0, 3).toUpperCase();
  }

  // Day labels shrink in tiers as the columns narrow: the full name, the name
  // without its "Next" prefix (the date underneath already says which
  // Thursday), the weekday with TODAY kept whole, then plain weekdays. On a
  // phone even TODAY is too wide, and the tinted column already marks it.
  const DAY_LABEL_TIERS = 4;

  function dayLabelAtTier(name, weekday, tier, isToday) {
    const full = String(name || '').trim().toUpperCase();
    if (!full) return '--';
    if (tier <= 0) return full;
    if (tier === 1) return full.replace(/^NEXT\s+/, '');
    if (tier === 2 && isToday) return 'TODAY';
    return weekday || shortDayLabel(name);
  }

  // '0.32"' -> '.32"'. Buys three pixels per column on a phone.
  function stripLeadingZero(amount) {
    return String(amount || '').replace(/^0(?=\.)/, '');
  }

  // "7:14 AM" -> minutes past local midnight; null if unparseable.
  function parseClockMinutes(text) {
    const m = String(text || '').trim().match(/^(\d{1,2}):(\d{2})\s*([AaPp])[Mm]?$/);
    if (!m) return null;
    let hour = parseInt(m[1], 10) % 12;
    const minute = parseInt(m[2], 10);
    if (hour > 11 || minute > 59) return null;
    if (m[3].toLowerCase() === 'p') hour += 12;
    return hour * 60 + minute;
  }

  // Is the hour starting at `minutes` after dark? Handles the usual
  // sunrise-before-sunset day as well as polar cases where one is missing.
  function isNightMinute(minutes, sunriseMin, sunsetMin) {
    if (sunriseMin === null || sunsetMin === null) return false;
    if (sunriseMin === sunsetMin) return false;
    return sunriseMin < sunsetMin
      ? (minutes < sunriseMin || minutes >= sunsetMin)
      : (minutes < sunriseMin && minutes >= sunsetMin);
  }

  // ---------------------------------------------------------------------
  // Rendering (browser only)
  // ---------------------------------------------------------------------

  const SVG_NS = 'http://www.w3.org/2000/svg';
  const FREEZING_F = 32;
  // Room above and below the plot for the high / low labels.
  const LABEL_PAD_TOP = 30;
  const LABEL_PAD_BOTTOM = 26;

  const state = {
    days: null, hours: null,
    sunrise: null, sunset: null, sunriseMin: null, sunsetMin: null,
  };

  const hasDOM = () => typeof document !== 'undefined' && typeof window !== 'undefined';
  const byId = (id) => document.getElementById(id);

  function cssVar(name) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  }

  function svgEl(tag, attrs, text) {
    const node = document.createElementNS(SVG_NS, tag);
    Object.keys(attrs).forEach((k) => node.setAttribute(k, attrs[k]));
    if (text != null) node.textContent = text;
    return node;
  }

  // A plot's canvas + svg sized to its box, in CSS pixels, device-pixel aware.
  function prepareSurface(host) {
    const canvas = host.querySelector('canvas');
    const svg = host.querySelector('svg');
    const width = host.clientWidth;
    const height = host.clientHeight;
    if (!width || !height) return null;

    const dpr = window.devicePixelRatio || 1;
    canvas.width = Math.round(width * dpr);
    canvas.height = Math.round(height * dpr);
    const ctx = canvas.getContext('2d');
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, width, height);

    svg.setAttribute('viewBox', `0 0 ${width} ${height}`);
    while (svg.firstChild) svg.removeChild(svg.firstChild);

    return { ctx, svg, width, height };
  }

  function strokeCurve(ctx, width, yAt, color, lineWidth) {
    ctx.lineWidth = lineWidth;
    ctx.strokeStyle = color;
    ctx.beginPath();
    for (let x = 0; x <= width; x++) {
      if (x === 0) ctx.moveTo(x, yAt(x)); else ctx.lineTo(x, yAt(x));
    }
    ctx.stroke();
  }

  function drawFreezeLine(svg, width, freezeY, edges, isInsideBand, centers, labelYs, text) {
    svg.appendChild(svgEl('line', {
      x1: 0, y1: freezeY, x2: width, y2: freezeY,
      stroke: cssVar('--fc-freeze'), 'stroke-width': 1, 'stroke-dasharray': '5 5',
    }));
    const x = freezeLabelX(width, freezeY, edges, isInsideBand, centers, labelYs);
    const chipWidth = text.length * 6.6 + 12;
    svg.appendChild(svgEl('rect', {
      x: x - chipWidth / 2, y: freezeY - 8, width: chipWidth, height: 16, rx: 8,
      fill: cssVar('--bg-secondary'), stroke: cssVar('--fc-freeze'), 'stroke-width': 0.75, opacity: 0.96,
    }));
    svg.appendChild(svgEl('text', {
      x, y: freezeY + 4, class: 'fc-axis', 'text-anchor': 'middle', fill: cssVar('--text-secondary'),
    }, text));
  }

  // The 10-day band: high edge, low edge, per-column gradient between them.
  function drawWeekBand(days) {
    const host = byId('fc-week-band');
    if (!host) return;
    const surface = prepareSurface(host);
    if (!surface || !days || !days.length) return;

    const { ctx, svg, width, height } = surface;
    const yTop = LABEL_PAD_TOP;
    const yBot = height - LABEL_PAD_BOTTOM;
    const centers = days.map((_, i) => columnCenterX(i, days.length, width));
    const temps = [];
    days.forEach((d) => temps.push(d.highTemp, d.lowTemp));
    const [tmin, tmax] = bandRange(temps);

    const highAt = monotoneSpline(centers, days.map((d) => d.highTemp));
    const lowAt = monotoneSpline(centers, days.map((d) => d.lowTemp));
    const yOf = (t) => tempToY(t, tmin, tmax, yTop, yBot);
    const highY = (x) => yOf(highAt(x));
    const lowY = (x) => yOf(lowAt(x));

    const band = new Path2D();
    for (let x = 0; x <= width; x++) {
      if (x === 0) band.moveTo(x, highY(x)); else band.lineTo(x, highY(x));
    }
    for (let x = width; x >= 0; x--) band.lineTo(x, lowY(x));
    band.closePath();

    // Lift the band off the card.
    ctx.save();
    ctx.shadowColor = 'rgba(0,0,0,.42)';
    ctx.shadowBlur = 20;
    ctx.shadowOffsetY = 7;
    ctx.fillStyle = 'rgba(0,0,0,.85)';
    ctx.fill(band);
    ctx.restore();

    // The fill is the data: each column runs from its high temperature to its
    // low, and the stops in between follow the ramp rather than a straight
    // RGB lerp, so a column spanning 20°-80° really does pass through cyan,
    // teal and green on the way.
    ctx.save();
    ctx.clip(band);
    for (let x = 0; x <= width; x++) {
      const hi = highAt(x);
      const lo = lowAt(x);
      const yHi = yOf(hi);
      const yLo = yOf(lo);
      const grad = ctx.createLinearGradient(0, yHi, 0, Math.max(yLo, yHi + 0.5));
      for (let s = 0; s <= 4; s++) grad.addColorStop(s / 4, tempColor(hi + (lo - hi) * (s / 4)));
      ctx.fillStyle = grad;
      ctx.fillRect(x, yHi - 1, 1.25, (yLo - yHi) + 2);
    }
    strokeCurve(ctx, width, highY, cssVar('--fc-edge-high'), 2.5);
    strokeCurve(ctx, width, lowY, cssVar('--fc-edge-low'), 2.5);
    ctx.restore();

    // "Which nights freeze?" answered without reading ten numbers.
    if (tmin < FREEZING_F && tmax > FREEZING_F) {
      const freezeY = yOf(FREEZING_F);
      drawFreezeLine(
        svg, width, freezeY, [highY, lowY],
        (x) => freezeY >= highY(x) - 6 && freezeY <= lowY(x) + 6,
        centers, days.map((d) => [yOf(d.highTemp) - 11, yOf(d.lowTemp) + 20]),
        width / days.length < 56 ? '32°' : '32° FREEZING',
      );
    }

    // Anchors and labels, each number in its own (contrast-corrected) ramp color.
    const cardBg = cssVar('--bg-secondary');
    days.forEach((day, i) => {
      const cx = centers[i];
      const yHi = yOf(day.highTemp);
      const yLo = yOf(day.lowTemp);
      [[yHi, day.highTemp], [yLo, day.lowTemp]].forEach(([cy, temp]) => {
        svg.appendChild(svgEl('circle', {
          cx, cy, r: 3.4, fill: tempColor(temp),
          stroke: cssVar('--bg-secondary'), 'stroke-width': 1.6,
        }));
      });
      svg.appendChild(svgEl('text', {
        x: cx, y: yHi - 11, class: 'fc-hi', 'text-anchor': 'middle',
        fill: readableTempColor(day.highTemp, cardBg),
      }, `${Math.round(day.highTemp)}°`));
      svg.appendChild(svgEl('text', {
        x: cx, y: yLo + 20, class: 'fc-lo', 'text-anchor': 'middle',
        fill: readableTempColor(day.lowTemp, cardBg),
      }, `${Math.round(day.lowTemp)}°`));
    });
  }

  // The 24-hour view: one temperature curve over a ramp-colored area.
  function drawDayArea(hours) {
    const host = byId('fc-day-band');
    if (!host) return;
    const surface = prepareSurface(host);
    if (!surface || !hours || !hours.length) return;

    const { ctx, svg, width, height } = surface;
    const yTop = LABEL_PAD_TOP;
    const yBot = height - 6;
    const centers = hours.map((_, i) => columnCenterX(i, hours.length, width));
    const [tmin, tmax] = bandRange(hours.map((h) => h.temp));
    const tempAt = monotoneSpline(centers, hours.map((h) => h.temp));
    const yOf = (t) => tempToY(t, tmin, tmax, yTop, yBot);
    const curveY = (x) => yOf(tempAt(x));

    const area = new Path2D();
    area.moveTo(0, yBot);
    for (let x = 0; x <= width; x++) area.lineTo(x, curveY(x));
    area.lineTo(width, yBot);
    area.closePath();

    ctx.save();
    ctx.clip(area);
    for (let x = 0; x <= width; x++) {
      const temp = tempAt(x);
      const y = yOf(temp);
      const grad = ctx.createLinearGradient(0, y, 0, yBot);
      grad.addColorStop(0, tempColorAlpha(temp, 1));
      grad.addColorStop(0.55, tempColorAlpha(temp, 0.5));
      grad.addColorStop(1, tempColorAlpha(temp, 0.12));
      ctx.fillStyle = grad;
      ctx.fillRect(x, y - 1, 1.25, yBot - y + 2);
    }
    ctx.restore();
    strokeCurve(ctx, width, curveY, cssVar('--fc-edge-high'), 2.5);

    if (tmin < FREEZING_F && tmax > FREEZING_F) {
      const freezeY = yOf(FREEZING_F);
      drawFreezeLine(
        svg, width, freezeY, [curveY], (x) => freezeY >= curveY(x) - 6,
        centers, hours.map((h) => [yOf(h.temp) - 10]), '32°',
      );
    }

    // Sunrise / sunset, where the night shading changes hands.
    hours.forEach((hour, i) => {
      if (!hour.sunLabel) return;
      const x = centers[i];
      svg.appendChild(svgEl('line', {
        x1: x, y1: yTop - 20, x2: x, y2: yBot,
        stroke: cssVar('--fc-grid'), 'stroke-width': 1, 'stroke-dasharray': '2 3',
      }));
      svg.appendChild(svgEl('text', { x: x + 5, y: yTop - 12, class: 'fc-axis' }, hour.sunLabel));
    });

    // Every third hour, plus the day's extremes.
    const cardBg = cssVar('--bg-secondary');
    let warmest = 0;
    let coldest = 0;
    hours.forEach((h, i) => {
      if (h.temp > hours[warmest].temp) warmest = i;
      if (h.temp < hours[coldest].temp) coldest = i;
    });
    hours.forEach((hour, i) => {
      const extreme = i === warmest || i === coldest;
      if (i % 3 !== 0 && !extreme) return;
      const cx = centers[i];
      const cy = yOf(hour.temp);
      svg.appendChild(svgEl('circle', {
        cx, cy, r: extreme ? 3.6 : 2.4, fill: tempColor(hour.temp),
        stroke: cssVar('--bg-secondary'), 'stroke-width': 1.5,
      }));
      const label = svgEl('text', {
        x: cx, y: cy - 10, class: 'fc-hi', 'text-anchor': 'middle',
        fill: readableTempColor(hour.temp, cardBg),
      }, `${Math.round(hour.temp)}°`);
      if (!extreme) label.setAttribute('style', 'font-size:12px;font-weight:400;opacity:.85');
      svg.appendChild(label);
    });
  }

  // ---------------------------------------------------------------------
  // Rows around the plot
  // ---------------------------------------------------------------------

  function setColumns(plotId, count) {
    const plot = byId(plotId);
    if (plot) plot.style.setProperty('--fc-cols', count);
  }

  function setHTML(id, html) {
    const node = byId(id);
    if (node) node.innerHTML = html;
  }

  // Bar height = chance of precip, number = expected accumulation: two
  // questions, two channels, neither standing in for the other.
  function precipCell(entry, mode) {
    const narrow = mode === 'narrow';
    const hourly = mode === 'hourly';
    const pop = Math.max(0, Math.min(100, Number(entry.pop) || 0));
    const palette = entry.precipType === 'snow' ? 'snow' : (pop > 0 ? 'rain' : 'dry');
    const bar = `<div class="fc-pbar"><i style="height:${Math.max(3, pop)}%;`
      + `--fc-bar-top:var(--fc-${palette}-top);--fc-bar-bottom:var(--fc-${palette}-bottom);`
      + `opacity:${pop ? 1 : 0.4}"></i></div>`;

    if (hourly) {
      return `<div>${bar}<div class="fc-pamt${pop ? '' : ' is-dry'}">${pop ? `${pop}%` : ''}</div></div>`;
    }

    const amount = narrow ? stripLeadingZero(entry.precipAmount) : (entry.precipAmount || '');
    const glyph = amount && entry.precipIcon ? `<b>${entry.precipIcon}</b>` : '';
    const label = amount || (pop ? `${pop}%` : (narrow ? '' : '—'));
    const chance = amount && !narrow ? `<div class="fc-ppop">${pop}%</div>` : '';
    return `<div>${bar}<div class="fc-pamt${amount ? '' : ' is-dry'}">${glyph}${label}</div>${chance}</div>`;
  }

  // Write the day names at the widest tier that doesn't clip, asking the laid
  // out row rather than guessing at text widths. One tier for the whole row,
  // so the names stay a set.
  function writeDayLabels(days) {
    const row = byId('fc-week-days');
    if (!row) return;
    for (let tier = 0; tier < DAY_LABEL_TIERS; tier++) {
      row.innerHTML = days.map((day, i) => {
        const name = dayLabelAtTier(day.dayName, day.weekday, tier, i === 0);
        const date = day.dateLabel ? `<small>${day.dateLabel}</small>` : '';
        return `<div class="${i === 0 ? 'is-today' : ''}">${name}${date}</div>`;
      }).join('');
      const clipped = Array.prototype.some.call(row.children, (cell) => cell.scrollWidth > cell.clientWidth + 1);
      if (!clipped) break;
    }
  }

  function renderWeekRows(days) {
    const plot = byId('fc-week-plot');
    const columnWidth = (plot ? plot.clientWidth : 1168) / Math.max(1, days.length);
    const tight = columnWidth < 56;

    setColumns('fc-week-plot', days.length);
    setHTML('fc-week-icons', days.map((d) => `<div>${d.icon || ''}</div>`).join(''));
    setHTML('fc-week-cond', days.map((d) => `<div>${d.weather || ''}</div>`).join(''));
    writeDayLabels(days);
    setHTML('fc-week-precip', days.map((d) => precipCell(d, tight ? 'narrow' : 'wide')).join(''));

    const today = byId('fc-week-today');
    if (today) {
      today.style.left = '0%';
      today.style.width = `${100 / days.length}%`;
    }
  }

  function renderDayRows(hours) {
    setColumns('fc-day-plot', hours.length);
    // One icon every other hour: 24 glyphs in a row is noise, not information.
    setHTML('fc-day-icons', hours.map((h, i) => `<div>${i % 2 ? '' : (h.icon || '')}</div>`).join(''));
    setHTML('fc-day-hours', hours.map((h, i) => (
      `<div class="${i === 0 ? 'is-today' : ''}">${i % 3 === 0 ? (h.shortTime || '') : ''}</div>`
    )).join(''));
    setHTML('fc-day-precip', hours.map((h) => precipCell(h, 'hourly')).join(''));

    const nights = byId('fc-day-nights');
    if (!nights) return;
    const blocks = [];
    let start = null;
    hours.forEach((hour, i) => {
      if (hour.isNight && start === null) start = i;
      if (!hour.isNight && start !== null) {
        blocks.push([start, i]);
        start = null;
      }
    });
    if (start !== null) blocks.push([start, hours.length]);
    nights.innerHTML = blocks.map(([from, to]) => (
      `<i style="left:${(from / hours.length) * 100}%;width:${((to - from) / hours.length) * 100}%"></i>`
    )).join('');
  }

  // ---------------------------------------------------------------------
  // Hover readout
  // ---------------------------------------------------------------------

  function weekCard(day) {
    const cardBg = cssVar('--bg-secondary');
    const rows = [
      ['High', `${Math.round(day.highTemp)}°F`, readableTempColor(day.highTemp, cardBg)],
      ['Low', `${Math.round(day.lowTemp)}°F`, readableTempColor(day.lowTemp, cardBg)],
      ['Precip chance', `${Math.max(0, Number(day.pop) || 0)}%`, null],
    ];
    if (day.precipAmount) rows.push([day.precipType === 'snow' ? 'Snow' : 'Rain', day.precipAmount, null]);
    const title = day.dateLabel ? `${day.dayName} · ${day.dateLabel}` : day.dayName;
    return `<h4>${title}</h4>`
      + `<div class="fc-card-cond"><b>${day.icon || ''}</b>${day.weather || ''}</div>`
      + rows.map(([k, v, color]) => (
        `<div class="fc-card-row"><span>${k}</span>`
        + `<span${color ? ` style="color:${color}"` : ''}>${v}</span></div>`
      )).join('');
  }

  function hourCard(hour) {
    const cardBg = cssVar('--bg-secondary');
    return `<h4>${hour.time ? String(hour.time).replace(/<br>/g, ' · ') : ''}</h4>`
      + `<div class="fc-card-cond"><b>${hour.icon || ''}</b>${hour.weather || ''}</div>`
      + `<div class="fc-card-row"><span>Temp</span>`
      + `<span style="color:${readableTempColor(hour.temp, cardBg)}">${Math.round(hour.temp)}°F</span></div>`
      + `<div class="fc-card-row"><span>Precip chance</span><span>${Math.max(0, Number(hour.pop) || 0)}%</span></div>`;
  }

  function wireHover(key, getData, renderCard) {
    const plot = byId(`fc-${key}-plot`);
    const band = byId(`fc-${key}-band`);
    const hover = byId(`fc-${key}-hover`);
    const card = byId(`fc-${key}-card`);
    if (!plot || !band || !hover || !card) return;

    const move = (event) => {
      const data = getData();
      if (!data || !data.length) return;
      const box = plot.getBoundingClientRect();
      const pointerX = event.touches ? event.touches[0].clientX : event.clientX;
      const columnWidth = box.width / data.length;
      const index = Math.min(data.length - 1, Math.max(0, Math.floor((pointerX - box.left) / columnWidth)));

      hover.style.left = `${index * columnWidth}px`;
      hover.style.width = `${columnWidth}px`;
      card.innerHTML = renderCard(data[index]);
      const cardWidth = card.offsetWidth || 190;
      const left = index * columnWidth + columnWidth / 2 - cardWidth / 2;
      card.style.left = `${Math.min(box.width - cardWidth - 2, Math.max(2, left))}px`;
      card.style.top = `${Math.max(2, band.offsetTop - 66)}px`;
      plot.classList.add('is-hovering');
    };

    plot.addEventListener('mousemove', move);
    plot.addEventListener('touchmove', move, { passive: true });
    plot.addEventListener('mouseleave', () => plot.classList.remove('is-hovering'));
    plot.addEventListener('touchend', () => plot.classList.remove('is-hovering'));
  }

  // ---------------------------------------------------------------------
  // Public API
  // ---------------------------------------------------------------------

  function paintRampKeys() {
    const stops = [];
    for (let i = 0; i <= 24; i++) {
      const c = sampleRamp(i / 24);
      stops.push(`rgb(${c[0]},${c[1]},${c[2]}) ${(i / 24) * 100}%`);
    }
    const gradient = `linear-gradient(90deg,${stops.join(',')})`;
    Array.prototype.forEach.call(document.querySelectorAll('.fc-key-bar'), (bar) => {
      bar.style.background = gradient;
    });
  }

  function hideEmptyState(id) {
    const node = byId(id);
    if (node) node.style.display = 'none';
  }

  function renderWeek(days) {
    if (!hasDOM() || !days || !days.length) return;
    state.days = days;
    hideEmptyState('fc-week-empty');
    renderWeekRows(days);
    drawWeekBand(days);
  }

  function renderDay(hours) {
    if (!hasDOM() || !hours || !hours.length) return;
    state.hours = annotateHours(hours);
    hideEmptyState('fc-day-empty');
    renderDayRows(state.hours);
    drawDayArea(state.hours);
  }

  // Tag each hour with its night flag and the sunrise/sunset marker, from the
  // sun times the live-conditions payload already carries.
  function annotateHours(hours) {
    const { sunriseMin, sunsetMin } = state;
    let sunriseShown = false;
    let sunsetShown = false;
    return hours.map((hour) => {
      const minutes = Number.isFinite(hour.minuteOfDay) ? hour.minuteOfDay : null;
      const isNight = minutes === null ? false : isNightMinute(minutes, sunriseMin, sunsetMin);
      let sunLabel = '';
      if (minutes !== null && sunriseMin !== null && !sunriseShown
          && sunriseMin >= minutes && sunriseMin < minutes + 60) {
        sunLabel = `SUNRISE ${state.sunrise}`;
        sunriseShown = true;
      } else if (minutes !== null && sunsetMin !== null && !sunsetShown
          && sunsetMin >= minutes && sunsetMin < minutes + 60) {
        sunLabel = `SUNSET ${state.sunset}`;
        sunsetShown = true;
      }
      return Object.assign({}, hour, { isNight, sunLabel });
    });
  }

  function setSunTimes(sunrise, sunset) {
    state.sunrise = sunrise || null;
    state.sunset = sunset || null;
    state.sunriseMin = parseClockMinutes(sunrise);
    state.sunsetMin = parseClockMinutes(sunset);
    if (state.hours) renderDay(state.hours);
  }

  // Re-run the layout: the rows pick a label density from the column width and
  // the canvas is sized in device pixels, so both have to be rebuilt on resize,
  // on a tab switch (a hidden plot has no width) and on a theme change.
  function refresh() {
    if (!hasDOM()) return;
    if (state.days) {
      renderWeekRows(state.days);
      drawWeekBand(state.days);
    }
    if (state.hours) {
      renderDayRows(state.hours);
      drawDayArea(state.hours);
    }
  }

  function init() {
    if (!hasDOM() || !byId('fc-week-plot')) return;
    paintRampKeys();
    wireHover('week', () => state.days, weekCard);
    wireHover('day', () => state.hours, hourCard);

    let resizeTimer = null;
    window.addEventListener('resize', () => {
      clearTimeout(resizeTimer);
      resizeTimer = setTimeout(refresh, 120);
    });

    // The theme toggle swaps CSS variables; the canvas has already baked the
    // old ones in, so repaint when `data-theme` changes.
    if (typeof MutationObserver !== 'undefined') {
      new MutationObserver(refresh).observe(document.documentElement, {
        attributes: true, attributeFilter: ['data-theme'],
      });
    }

    if (document.fonts && document.fonts.ready) document.fonts.ready.then(refresh);
  }

  if (hasDOM()) {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', init);
    } else {
      init();
    }
  }

  return {
    // Pure helpers, exported for tests.
    TEMP_RAMP,
    sampleRamp,
    tempColor,
    tempColorAlpha,
    parseColor,
    relativeLuminance,
    contrastRatio,
    readableTempColor,
    columnCenterX,
    bandRange,
    tempToY,
    monotoneSpline,
    freezeLabelX,
    shortDayLabel,
    dayLabelAtTier,
    DAY_LABEL_TIERS,
    stripLeadingZero,
    parseClockMinutes,
    isNightMinute,
    // Browser API.
    renderWeek,
    renderDay,
    setSunTimes,
    refresh,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = WeatherForecastBand;
}
