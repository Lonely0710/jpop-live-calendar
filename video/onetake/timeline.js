// The beat sheet as numbers, and every moving thing as a pure function of t.
// Beats (s): hook/type 0–2.3 · body 2.3–3.0 · rest 3.1–4.5 · parse hits 4.6–6.5 · whip 6.5–7.2 ·
// four shows fly 7.3–9.6 · stub 10.1–11.3 · map 11.4–13.6 · trace 14.6–16.3, rest to 17.3 · rain 17.8–19.9 ·
// prefectures 20.5–21.4 · rest · gather 22.4 · lockup 23.5–26.3 · hold to 30.
const DUR = 30;
const { clamp, lerp, seg, sstep, ssstep, ease, spring } = OM;
const T = {
  TYPE0: 0.35, CPS: 24, BODY: 2.25, MAP_IN: 11.0,
  HIT0: 4.6, ROW_DT: 0.38, SPAN_DT: 0.12, PER: 6.3,
  FLY: [7.35, 7.85, 8.25, 8.55], FLY_DUR: 0.62,
  STUB0: 10.15, STUB_DUR: 0.7,
  PILL: [11.65, 11.9, 12.15, 12.35], PILL_DUR: 0.55,
  PULSE0: 14.6, PULSE_DT: 0.14, RING: 15.9, TRACE_OUT: 17.3,
  RAIN0: 17.8, RAIN1: 19.85, PREF0: 20.55, PREF_DT: 0.09,
  G: 22.4,
};
T.LAND = T.FLY.map(t => t + T.FLY_DUR);
T.STUB1 = T.STUB0 + T.STUB_DUR;
T.PLAND = T.PILL.map(t => t + T.PILL_DUR);
const G = T.G, G_MORPH0 = G + 0.15, G_MORPH1 = G + 1.0, G_SWAP = G + 1.12, G_SWAP1 = G + 1.32, G_IMPACT = G + 1.3, G_MARK = G + 1.45;

// Highlight hits: per row the date, venue, start; then the period.
const HITS = [];
for (let r = 0; r < 4; r++) for (let k = 0; k < 3; k++) HITS.push({ r, k, t: T.HIT0 + r * T.ROW_DT + k * T.SPAN_DT });
HITS.push({ per: true, t: T.PER });

// ── geometry measured once (measure() in film.js fills R) ──
const R = {};
const ctr = b => [b.x + b.w / 2, b.y + b.h / 2];
const rectAt = (A, B, k, arc) => { const [ax, ay] = ctr(A), [bx, by] = ctr(B);
  const w = lerp(A.w, B.w, k), h = lerp(A.h, B.h, k), cx = lerp(ax, bx, k), cy = lerp(ay, by, k) - Math.sin(Math.PI * k) * arc;
  return { x: cx - w / 2, y: cy - h / 2, w, h, k }; };

// A date token lifts off its row and lands as a chip in its cell.
const flyK = (i, t) => ease.quintInOut(seg(t, T.FLY[i], T.LAND[i]));
const flyRect = (i, t) => rectAt(R.tok[i], FLY[i], flyK(i, t), 160);
// The period lifts off and lands in the header slot; at the gather it becomes the icon's ticket.
const stubRect = t => {
  if (t < G_MORPH0 + 0.1) return rectAt(R.per, R.slot, ease.quintInOut(seg(t, T.STUB0, T.STUB1)), 120);
  return rectAt(R.slot, R.ticket, ease.expoInOut(seg(t, G_MORPH0 + 0.1, G_MORPH1)), 0);
};
// A prefecture pill pops out of its chip and lands on the capital as the badge.
const pillRect = (i, t) => { const c = FLY[i], cap = PREF[c.s.pref].cap, B = { x: cap[0] - 22, y: cap[1] - 22, w: 44, h: 44 };
  const A = { x: c.x + 10, y: c.y + 4, w: 70, h: 34 };
  return rectAt(A, B, ease.quintInOut(seg(t, T.PILL[i], T.PLAND[i])), 110); };
// The calendar frame becomes the icon's body.
const calRect = t => rectAt({ x: CX, y: CY, w: CW, h: CH }, R.body, ease.expoInOut(seg(t, G_MORPH0, G_MORPH1)), 0);
// Tokyo's badge hops into the icon's note square.
const noteRect = t => { const b = PREF['東京'].cap;
  return rectAt({ x: b[0] - 22, y: b[1] - 22, w: 44, h: 44 }, R.note, ease.expoInOut(seg(t, G_MORPH0 + 0.05, G_MORPH1 + 0.05)), 90); };

// ── the camera: keyed to what each beat frames; chase keys land a little ahead ──
let KEYS = [];
const fit = (b, m = 1.12) => ({ x: b.x + b.w / 2, y: b.y + b.h / 2, zoom: Math.min(1920 / (b.w * m), 1080 / (b.h * m)) });
const union = (...bs) => { const x0 = Math.min(...bs.map(b => b.x)), y0 = Math.min(...bs.map(b => b.y)), x1 = Math.max(...bs.map(b => b.x + b.w)), y1 = Math.max(...bs.map(b => b.y + b.h));
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 }; };
function cameraKeys() {
  const a = R.ann, tt = R.title, cal = { x: CX, y: CY, w: CW, h: CH }, map = { x: MX, y: MY, w: MW, h: MH };
  const rows = union(...R.rows), cells = union(...FLY.map(f => ({ x: f.cell.x, y: f.cell.y, w: COLW, h: ROWH })));
  const caps = PILLS.map(p => PREF[p.pref].cap), capBox = union(...caps.map(c => ({ x: c[0] - 40, y: c[1] - 40, w: 80, h: 80 })));
  const k = (t, b, extra = {}) => Object.assign({ t }, b, extra);
  const endY = ICON.y + 175;
  return [
    k(0, { x: tt.x + 430, y: tt.y + 10, zoom: 1.7 }),
    k(1.25, { x: tt.x + 600, y: tt.y + 34, zoom: 1.85 }, { curve: ease.sineInOut }),
    k(2.15, { x: tt.x + 520, y: tt.y + 62, zoom: 1.75 }, { curve: ease.sineInOut }),
    k(3.1, fit(a, 1.18), { curve: ease.quintInOut }),
    k(4.45, fit(a, 1.18)),
    k(5.2, { x: ctr(rows)[0], y: ctr(rows)[1], zoom: 1.35 }, { curve: ease.cubicInOut }),
    k(6.4, { x: R.per.x + R.per.w / 2 - 80, y: R.per.y, zoom: 1.32 }, { curve: ease.sineInOut }),
    k(7.15, fit(union(a, cal), 1.06), { curve: ease.expoInOut }),
    k(7.6, fit(union(a, cal), 1.06)),
    k(8.6, fit(union(rows, cells), 1.08), { curve: ease.cubicInOut }),
    k(9.35, fit(cells, 1.8), { curve: ease.cubicInOut }),
    k(9.7, fit(cells, 1.8)),
    k(10.5, fit(union(R.per, cal), 1.06), { curve: ease.cubicInOut }),
    k(11.0, fit(union(R.slot, union(...FLY)), 1.25), { curve: ease.cubicInOut }),
    k(11.35, fit(union(R.slot, union(...FLY)), 1.25)),
    k(12.9, fit(union(cells, capBox), 1.12), { curve: ease.cubicInOut }),
    k(13.6, fit(union(cells, capBox), 1.12)),
    k(14.45, fit(union(cal, map), 1.06), { curve: ease.cubicInOut }),
    k(14.95, fit(union(cal, map), 1.06)),
    k(15.7, fit(union(a, cal), 1.05), { curve: ease.quintInOut }),
    k(17.3, fit(union(a, cal), 1.05)),
    k(17.95, fit(cal, 1.08), { curve: ease.cubicInOut }),
    k(19.85, fit(cal, 1.08)),
    k(20.6, fit(union(cal, map), 1.06), { curve: ease.cubicInOut }),
    k(G + 0.1, fit(union(cal, map), 1.06)),
    k(G_MORPH1 + 0.05, { x: ICON.x, y: endY, zoom: 1 }, { curve: ease.expoInOut }),
  ];
}
const camAt = t => OM.camera(t, KEYS);

// ── the rain: every other show in the grid, in date order, arriving ever faster ──
let RAIN = [], SHOWN = [];
function planRain() {
  const order = CELLS.flatMap(c => c.shows.map((s, k) => ({ s, c, k }))).filter(o => !o.s.ann);
  order.forEach((o, i) => { o.t = T.RAIN0 + (T.RAIN1 - T.RAIN0) * Math.pow(i / Math.max(1, order.length - 1), 0.62); });
  RAIN = order;
  CHIPS.forEach(ch => { ch.t = ch.ann ? T.LAND[FLY.indexOf(ch)] : order.find(o => o.s === ch.s).t; });
  SHOWN = [...order.map(o => ({ t: o.t, s: o.s, c: o.c })), ...FLY.map((f, i) => ({ t: T.LAND[i], s: f.s, c: f.cell }))];
}
const monthCount = t => SHOWN.filter(o => o.t <= t && o.s.date.startsWith(D.month)).length;
const hiddenIn = (c, t) => SHOWN.filter(o => o.c === c && o.t <= t).length - Math.min(2, c.shows.length);

// ── prefecture counts: the four landings, then the rest of the month ──
let PEV = {};
function planPrefs() {
  PEV = {};
  const add = (p, t, v) => (PEV[p] = PEV[p] || []).push({ t, v });
  const run = {};
  PILLS.forEach((p, i) => { run[p.pref] = (run[p.pref] || 0) + 1; add(p.pref, T.PLAND[i], run[p.pref]); });
  Object.keys(D.counts).sort((a, b) => D.counts[b] - D.counts[a]).forEach((p, i) => {
    if (!PREF[p]) return; const t = T.PREF0 + i * T.PREF_DT;
    if (!PEV[p] || D.counts[p] !== run[p]) add(p, t, D.counts[p]); });
}
const prefVal = (p, t) => { let v = 0; for (const e of PEV[p] || []) v = lerp(v, e.v, ssstep(e.t, e.t + 0.3, t)); return v; };
const prefNum = (p, t) => { let n = 0, t0 = -1; for (const e of PEV[p] || []) if (t >= e.t) { n = e.v; t0 = e.t; } return { n, t0 }; };
const prefFill = v => { if (v <= 0.01) return '#E5DDD2'; const k = clamp(Math.sqrt(v / 11), 0, 1);
  const a = [247, 196, 168], b = [226, 84, 52]; return `rgb(${a.map((x, i) => Math.round(lerp(x, b[i], k))).join(',')})`; };
