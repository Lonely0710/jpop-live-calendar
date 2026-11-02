// seek(t) and the onetake contract (__seek, __ready, __meta, __track, __motion).
let CAM = null, CNT = -1;
function seek(t) {
  CAM = camAt(t);
  const M = OM.view(CAM);
  WORLD.style.transform = `matrix(${M.map(v => v.toFixed(5)).join(',')})`;
  lattice(CAM);
  const px = 1 / CAM.zoom;                                   // one screen px in world units
  const gOut = ssstep(G, G + 0.35, t);

  // ── A · the announcement ──
  show(A, 1 - gOut);
  const REV = [0.05, T.BODY, T.BODY + 0.1, ...ROWS.map((_, i) => T.BODY + 0.22 + i * 0.09), T.BODY + 0.6, T.BODY + 0.68, T.BODY + 0.8];
  MASKS.forEach((m, i) => { const k = spring(t - REV[i], 0.72, 17); m.style.transform = `translateY(${((1 - k) * 110).toFixed(1)}%)`; m.style.opacity = clamp(k * 2).toFixed(3); });
  const ty = OM.typeOn(t, T.TYPE0, ANN.title, { cps: T.CPS });
  if ($('ttl').textContent !== ty.str) $('ttl').textContent = ty.str;
  $('caret').style.opacity = (ty.done ? ty.caret * (1 - sstep(3.0, 3.3, t)) : 1).toFixed(2);
  const dim = sstep(T.HIT0 - 0.15, T.HIT0 + 0.25, t) * (1 - sstep(T.TRACE_OUT, T.TRACE_OUT + 0.5, t));
  ROWS.forEach((_, i) => { $('row' + i).style.color = `rgb(${Math.round(lerp(46, 150, dim))},${Math.round(lerp(42, 142, dim))},${Math.round(lerp(40, 134, dim))})`; });
  HITS.forEach(h => {
    const sp = h.per ? $('per') : R.spans[h.r][h.k];
    const k = ease.expoOut(seg(t, h.t, h.t + 0.22));
    const back = arrive(h.per ? 4 : h.r), pr = OM.press(t, h.t, { depth: 0.1 }), pb = OM.press(t, back, { depth: 0.14 });
    sp.style.color = 'var(--ink)';
    sp.style.background = `linear-gradient(var(--hl),var(--hl)) 0 0/${(k * 100).toFixed(1)}% 100% no-repeat`;
    sp.style.transform = `scale(${(pr.s * pb.s).toFixed(4)})`;
  });
  { const rb = pad(union(...R.rows, R.per), 14), k = spring(t - T.RING, 0.6, 15), op = clamp(k * 2) * (1 - sstep(T.TRACE_OUT - 0.1, T.TRACE_OUT + 0.3, t));
    show(RING, t >= T.RING ? op : 0);
    const s = 0.94 + 0.06 * k; RING.style.transform = `translate(${rb.x}px,${rb.y}px) scale(${s.toFixed(4)})`; RING.style.transformOrigin = '50% 50%';
    RING.style.width = rb.w + 'px'; RING.style.height = rb.h + 'px'; RING.style.borderWidth = (3 * px).toFixed(2) + 'px'; }

  // ── C · the calendar, and at the gather the icon's body ──
  { const pin = OM.popIn(t, 6.55, { rise: 60 }), r = calRect(t), mk = ease.expoInOut(seg(t, G_MORPH0, G_MORPH1));
    show(CAL, t >= 6.5 ? pin.op * (1 - sstep(G_SWAP, G_SWAP1, t)) : 0);
    CAL.style.transform = `translate(${r.x.toFixed(2)}px,${(r.y + pin.dy).toFixed(2)}px)`;
    CAL.style.width = r.w.toFixed(2) + 'px'; CAL.style.height = r.h.toFixed(2) + 'px';
    CAL.style.borderRadius = lerp(26, R.body.r, mk).toFixed(2) + 'px';
    CAL.style.border = `${lerp(1, R.body.stroke, mk).toFixed(2)}px solid ${mk > 0.5 ? '#2E2A28' : mk > 0 ? '#8C847B' : '#E6DED2'}`;
    CAL.style.background = mk > 0 ? '#F7F3EB' : 'var(--card)';
    CIN.style.opacity = (1 - sstep(G + 0.05, G + 0.3, t)).toFixed(3);
    show(HEAD, sstep(0.25, 0.6, mk));
    HEAD.style.height = (R.body.head * (r.w / R.body.w)).toFixed(2) + 'px'; HEAD.style.width = '100%';
    HEAD.style.borderBottom = `${(R.body.stroke * mk).toFixed(2)}px solid #2E2A28`; }
  CELLS.forEach(c => { const col = c.i % 7, row = Math.floor(c.i / 7);
    c.el.style.opacity = clamp((t - 6.62 - (col + row) * 0.025) / 0.18).toFixed(3);
    const fi = FLY.findIndex(f => f.cell === c); c.flash.style.opacity = fi >= 0 ? (0.75 * Math.exp(-4 * Math.max(0, t - T.LAND[fi])) * (t >= T.LAND[fi] ? 1 : 0)).toFixed(3) : 0;
    const h = t >= T.RAIN0 ? hiddenIn(c, t) : 0, txt = h > 0 ? S.more(h) : ''; if (c.more.textContent !== txt) c.more.textContent = txt; });
  { const n = monthCount(t); if (n !== CNT) { CNT = n; $('cnt').textContent = n; } }

  // ── chips: the four that fly, then the rain ──
  const chipOut = i => 1 - ssstep(G - 0.05 + (i % 9) * 0.012, G + 0.25 + (i % 9) * 0.012, t);
  CHIPS.forEach((ch, i) => {
    if (ch.ann) { const f = FLY.indexOf(ch), on = t >= T.FLY[f];
      show(ch.el, on ? chipOut(i) : 0); if (!on) return;
      const r = flyRect(f, t), im = OM.impact(t, T.LAND[f], { sqx: 0.08, sqy: 0.1 });
      box(ch.el, r); ch.el.style.transform += ` scale(${im.sx.toFixed(4)},${im.sy.toFixed(4)})`;
      ch.tok.style.opacity = (1 - sstep(0.35, 0.7, r.k)).toFixed(3);
      ch.el.style.boxShadow = r.k > 0 && r.k < 1 ? `0 ${(18 * Math.sin(Math.PI * r.k)).toFixed(1)}px 30px -12px rgba(46,42,40,.35)` : 'none';
      return; }
    const p = OM.popIn(t, ch.t, { rise: 14, s0: 0.8, ds: 0.2 });
    show(ch.el, t >= ch.t ? p.op * chipOut(i) : 0);
    if (t >= ch.t) ch.el.style.transform = `translate(${ch.x}px,${(ch.y + p.dy).toFixed(2)}px) scale(${p.s.toFixed(4)})`;
  });

  seekRest(t, px, gOut);
}
