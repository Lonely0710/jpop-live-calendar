// Builds the world once from window.DATA: the announcement card (A), the
// calendar (C), the map (M) and the end lockup, all in world px. Measured in
// measure() after fonts load, with the world untransformed.
const Q = new URLSearchParams(location.search);
const LANG = ['zh', 'ja', 'en'].includes(Q.get('lang')) ? Q.get('lang') : 'zh';
const S = STR[LANG];
document.documentElement.lang = LANG === 'zh' ? 'zh' : LANG;
const D = window.DATA, ANN = D.announcement;
const $ = id => document.getElementById(id);
const el = (tag, cls, parent, html) => { const e = document.createElement(tag); if (cls) e.className = cls; if (html != null) e.innerHTML = html; (parent || WORLD).appendChild(e); return e; };
const WORLD = $('world');

// ── layout (world px) ──
const AX = -1500, AY = -430, AW = 1020;
const CX = 0, CY = -380, CW = 868, CH = 828, COLW = 123, ROWH = 116, GRID_X = 4, GRID_Y = 116;
const MX = 940, MW = 760, MH = Math.round(MW * D.map.h / D.map.w), MY = -360, MS = MW / D.map.w;
const ICON = { x: 434, y: 470, size: 300 };     // the lockup's icon centre and size, world px

// ── A · the announcement, parsed from its own raw text ──
const raw = ANN.raw.split('\n').map(s => s.trim());
const lead = raw[1];
const ROWS = raw.filter(l => /^\d+\/\d+\(.\)\s/.test(l)).map(l => {
  const m = l.match(/^(\S+)\s+(\S+)\s+(.+?)\s+Open (\d+:\d+) \/ Start (\d+:\d+)$/);
  return { date: m[1], city: m[2], venue: m[3], open: m[4], start: m[5] };
});
const perLine = raw.find(l => l.includes('受付期間'));
const perLab = perLine.match(/<(.+?)>/)[1], perTxt = perLine.match(/(受付期間:.+?23:59)/)[1];
const A = $('world').appendChild(Object.assign(document.createElement('div'), { id: 'ann' }));
A.style.left = AX + 'px'; A.style.top = AY + 'px';
A.innerHTML = `<div class="mask"><div class="src"><img src="build/sony.svg" alt="">${ANN.source}</div></div>
  <div class="title"><span id="ttl"></span><i id="caret"></i></div>
  <div class="mask"><div class="lead">${lead}</div></div>
  <div class="mask"><div class="lab">&lt;日程・会場&gt; 2026年</div></div>
  ${ROWS.map((r, i) => `<div class="mask"><div class="row" id="row${i}"><span class="d">${r.date}</span> ${r.city} <span>${r.venue}</span>　Open ${r.open} / Start <span>${r.start}</span></div></div>`).join('')}
  <div class="mask"><div class="lab">&lt;${perLab}&gt;</div></div>
  <div class="mask"><div class="per"><span id="per">${perTxt}</span></div></div>
  <div class="mask"><div class="foot"><span>${S.meta(ANN.published, ANN.fetched, ANN.revision)}</span><span>${ANN.url.replace(/^https?:\/\/(www\.)?/, '')}</span></div></div>`;
const MASKS = [...A.querySelectorAll('.mask>div')];
const RING = el("div"); RING.id = "ring";

// ── C · the calendar ──
const CAL = el('div'); CAL.id = 'cal';
const CIN = el('div', null, CAL); CIN.id = 'calIn';
const HEAD = el('div', null, CAL); HEAD.id = 'capHead';
CIN.innerHTML = `<div class="hd">${S.month}<small><b id="cnt">0</b> ${S.live}</small></div><div id="slot"></div>` +
  S.week.map((w, i) => `<div class="wk${i === 5 ? ' sat' : i === 6 ? ' sun' : ''}" style="left:${GRID_X + i * COLW}px">${w}</div>`).join('');
const day0 = new Date(Date.UTC(2026, 8, 28));          // the Monday on or before 1 Oct
const iso = d => d.toISOString().slice(0, 10);
const CELLS = [];
for (let i = 0; i < 42; i++) {
  const d = new Date(day0.getTime() + i * 864e5), c = el('div', 'cell' + (d.getUTCMonth() === 9 ? '' : ' out'), CIN);
  const x = GRID_X + (i % 7) * COLW, y = GRID_Y + Math.floor(i / 7) * ROWH;
  c.style.left = x + 'px'; c.style.top = y + 'px';
  c.innerHTML = `<div class="flash"></div><div class="n">${d.getUTCDate()}</div><div class="more"></div>`;
  CELLS.push({ el: c, date: iso(d), x: CX + x, y: CY + y, i, flash: c.firstChild, more: c.lastChild, shows: [] });
}
const cellOf = date => CELLS.find(c => c.date === date);

// ── chips: every show in the grid; the announcement's four fly in, the rest rain ──
const mix = (hex, k) => { const n = parseInt(hex.slice(1), 16), r = n >> 16, g = (n >> 8) & 255, b = n & 255, P = [255, 250, 242];
  return `rgb(${Math.round(r * k + P[0] * (1 - k))},${Math.round(g * k + P[1] * (1 - k))},${Math.round(b * k + P[2] * (1 - k))})`; };
const line2 = s => [s.start || S.tba, s.venue ? `${s.pref ? s.pref + ' ' : ''}${s.venue}` : S.venueTba].join(' ');
D.shows.forEach(s => { const c = cellOf(s.date); if (c) c.shows.push(s); });
CELLS.forEach(c => c.shows.sort((a, b) => (!!b.ann - !!a.ann) || (a.start || '99').localeCompare(b.start || '99')));
const CHIPS = [];
CELLS.forEach(c => c.shows.forEach((s, k) => {
  if (k > 1) return;
  const color = s.color || '#8C847B', e = el('div', 'chip' + (s.status === 'cancelled' ? ' x' : ''));
  e.style.background = mix(color, 0.16);
  e.innerHTML = `<div class="bar" style="background:${color}"></div><div class="a">${s.status === 'cancelled' ? S.cancelled + ' · ' : ''}${s.artist}</div><div class="b">${line2(s)}</div>`;
  CHIPS.push({ el: e, s, cell: c, k, x: c.x + 4, y: c.y + 28 + k * 38, w: 115, h: 36, ann: s.ann });
}));
const FLY = ROWS.map((r, i) => { const s = D.shows.filter(x => x.ann)[i]; const ch = CHIPS.find(c => c.s === s);
  const tok = el('div', 'tok', ch.el, r.date); return Object.assign(ch, { row: i, tok }); });

// ── the ticket round → the stub in the header slot ──
const RD = D.rounds[0], STUB = el('div'); STUB.id = 'stub';
const fmt = m => m.slice(5).replace('-', '/').replace(/^0/, '');
STUB.innerHTML = `<div class="bar"></div><div class="a">${RD.name.replace(/\(.+\)/, '')}<em>${RD.lottery ? S.lottery : S.first}</em></div>
  <div class="b">${fmt(RD.opens)} → ${fmt(RD.closes)}</div><div class="tok">${perTxt}</div>`;

// ── M · the map, in its own units scaled to MW ──
const MAP = el('div'); MAP.id = 'map';
const svgNS = 'http://www.w3.org/2000/svg';
const msvg = document.createElementNS(svgNS, 'svg'); MAP.appendChild(msvg);
msvg.setAttribute('width', MW); msvg.setAttribute('height', MH); msvg.setAttribute('viewBox', `0 0 ${D.map.w} ${D.map.h}`);
const [ix, iy, iw, ih] = D.map.inset;
msvg.innerHTML = `<rect x="${ix}" y="${iy}" width="${iw}" height="${ih}" rx="40" fill="none" stroke="#D8D1C7" stroke-width="6" stroke-dasharray="22 18"/>`;
const PREF = {};
D.map.prefs.forEach(p => { const path = document.createElementNS(svgNS, 'path');
  path.setAttribute('d', p.rings.map(r => { let d = ''; for (let i = 0; i < r.length; i += 2) d += (i ? 'L' : 'M') + r[i].toFixed(1) + ' ' + r[i + 1].toFixed(1); return d + 'Z'; }).join(''));
  path.style.strokeWidth = (1.1 / MS).toFixed(2); msvg.appendChild(path);
  PREF[p.name] = { path, cap: [MX + p.capital[0] * MS, MY + p.capital[1] * MS], badge: null }; });
Object.keys(D.counts).forEach(n => { if (!PREF[n]) return; const b = el('div', 'badge'); PREF[n].badge = b; });

// ── threads: token → chip (×4), period → stub, chip → map (×4) ──
const TH = document.createElementNS(svgNS, 'svg'); TH.id = 'threads'; WORLD.appendChild(TH);
TH.setAttribute('width', 1); TH.setAttribute('height', 1); TH.style.overflow = 'visible';
const thread = () => { const p = document.createElementNS(svgNS, 'path'), c = document.createElementNS(svgNS, 'circle'); TH.appendChild(p); TH.appendChild(c); return { p, c }; };
const PILLS = FLY.map(f => ({ fly: f, pref: f.s.pref, e: el('div', 'ppill', null, f.s.pref) }));

// ── end lockup ──
const ICONEL = el('img'); ICONEL.id = 'icon'; ICONEL.src = 'build/icon.svg';
const MARK = el('div'); MARK.id = 'mark';
const MARKL = [...'Live Calendar'].map(ch => { const s = document.createElement('span'); s.textContent = ch === ' ' ? ' ' : ch; MARK.appendChild(s); return s; });
const TAG = el('div', null, null, S.tagline); TAG.id = 'tag';
const META = el('div', null, null, S.platforms(D.version)); META.id = 'meta';
const URL_ = el('div', null, null, 'github.com/Lonely0710/jpop-live-calendar'); URL_.id = 'url';
