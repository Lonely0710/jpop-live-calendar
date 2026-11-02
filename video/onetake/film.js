// measure() once, then seek(t): every style written from t alone.
const $lat = $('lat').getContext('2d');
const show = (e, op) => { e.style.display = op > 0.002 ? '' : 'none'; e.style.opacity = op.toFixed(3); };
const box = (e, b) => { e.style.transform = `translate(${b.x.toFixed(2)}px,${b.y.toFixed(2)}px)`; e.style.width = b.w.toFixed(2) + 'px'; e.style.height = b.h.toFixed(2) + 'px'; };
const wb = e => { const r = e.getBoundingClientRect(); return { x: r.left, y: r.top, w: r.width, h: r.height }; };
const pad = (b, p) => ({ x: b.x - p, y: b.y - p, w: b.w + 2 * p, h: b.h + 2 * p });
let TH_ = [], PULSES = [];

function measure() {
  WORLD.style.transform = 'none';
  $('ttl').textContent = ANN.title;
  R.ann = wb(A); R.title = wb($('ttl'));
  R.rows = ROWS.map((_, i) => wb($('row' + i)));
  R.spans = ROWS.map((_, i) => [...$('row' + i).querySelectorAll('span')]);
  R.tok = R.spans.map(s => pad(wb(s[0]), 0));
  R.per = wb($('per'));
  R.foot = wb(MASKS[MASKS.length - 1]);
  $('ttl').textContent = '';
  CAL.style.transform = `translate(${CX}px,${CY}px)`;
  const sl = wb($('slot')); R.slot = sl;
  MAP.style.transform = `translate(${MX}px,${MY}px)`;
  // the icon's own geometry (resources/icon.svg: 1024 units, inner group at translate(60.56 80.36) scale(.72))
  const u = ICON.size / 1024, ox = ICON.x - ICON.size / 2, oy = ICON.y - ICON.size / 2;
  const g = (x, y) => [ox + (60.56 + x * 0.72) * u, oy + (80.36 + y * 0.72) * u], s = 0.72 * u;
  const [bx, by] = g(280, 296), [tx, ty] = g(908, 852), [nx, ny] = g(488, 650);
  R.body = { x: bx, y: by, w: 694 * s, h: 685 * s, r: 95 * s, head: 156 * s, stroke: 32 * s };
  R.ticket = { x: tx - 158 * s, y: ty - 158 * s, w: 316 * s, h: 316 * s };
  R.note = { x: nx, y: ny, w: 132 * s, h: 128 * s, r: 32 * s };
  // threads: token → chip, period → stub, chip → capital
  TH_ = [...FLY.map((f, i) => ({ ...thread(), from: () => ctr(R.tok[i]), to: t => ctr(flyRect(i, t)), t0: T.FLY[i], t1: T.LAND[i] })),
    { ...thread(), from: () => ctr(R.per), to: t => ctr(stubRect(Math.min(t, T.STUB1))), t0: T.STUB0, t1: T.STUB1 },
    ...PILLS.map((p, i) => ({ ...thread(), from: () => ctr(FLY[i]), to: t => ctr(pillRect(i, t)), t0: T.PILL[i], t1: T.PLAND[i] }))];
  // the trace runs backwards: capital → chip, then chip → token, then slot → period
  PULSES = [...PILLS.map((_, i) => ({ th: 5 + i, t0: T.PULSE0 + i * T.PULSE_DT })),
    ...FLY.map((_, i) => ({ th: i, t0: T.PULSE0 + 0.45 + i * T.PULSE_DT })),
    { th: 4, t0: T.PULSE0 + 0.45 + 4 * T.PULSE_DT }];
}
const ARC = (a, b) => [(a[0] + b[0]) / 2, Math.min(a[1], b[1]) - 60 - Math.hypot(b[0] - a[0], b[1] - a[1]) * 0.12];
const qpt = (a, c, b, u) => [(1 - u) ** 2 * a[0] + 2 * (1 - u) * u * c[0] + u * u * b[0], (1 - u) ** 2 * a[1] + 2 * (1 - u) * u * c[1] + u * u * b[1]];
const PULSE_DUR = 0.42;
const pulseU = (p, t) => ease.cubicInOut(seg(t, p.t0, p.t0 + PULSE_DUR));
const arrive = i => { const p = PULSES.find(q => q.th === i); return p ? p.t0 + PULSE_DUR : 1e9; };

function lattice(cam) {
  const L = OM.lattice(cam, { depth: 1.2, spacing: 56, px: 1.3 });
  $lat.setTransform(1, 0, 0, 1, 0, 0); $lat.clearRect(0, 0, 1920, 1080);
  $lat.setTransform(...L.matrix); $lat.fillStyle = 'rgba(140,132,123,.20)'; $lat.beginPath();
  for (let x = L.x0; x <= L.x1; x += L.spacing) for (let y = L.y0; y <= L.y1; y += L.spacing) { $lat.moveTo(x + L.r, y); $lat.arc(x, y, L.r, 0, 6.2832); }
  $lat.fill();
}

const CAPS = [[0.4, 4.4, 0], [6.6, 9.7, 1], [10.25, 11.3, 2], [11.5, 13.9, 3], [14.6, 17.3, 4], [17.9, 22.2, 5]];
let capNow = -1;
function caption(t) {
  const c = CAPS.find(c => t >= c[0] && t < c[1] + 0.3), div = $('cap').firstChild;
  if (!c) { div.style.opacity = 0; return; }
  if (capNow !== c[2]) { capNow = c[2]; div.innerHTML = `<b>${String(c[2] + 1).padStart(2, '0')}</b>${S.caps[c[2]]}`; }
  const k = spring(t - c[0], 0.75, 16), out = ssstep(c[1], c[1] + 0.3, t);
  div.style.opacity = (clamp(k * 1.5) * (1 - out)).toFixed(3);
  div.style.transform = `translateY(${((1 - k) * 64 - out * 64).toFixed(1)}px)`;
}
