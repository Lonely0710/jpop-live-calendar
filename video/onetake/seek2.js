// The rest of seek(t): stub, map, threads, lockup, captions; then the contract.
function seekRest(t, px, gOut) {
  // ── the ticket round: period → header stub → the icon's ticket ──
  { const r = stubRect(t), on = t >= T.STUB0, mk = ease.expoInOut(seg(t, G_MORPH0 + 0.1, G_MORPH1));
    show(STUB, on ? 1 - sstep(G_SWAP, G_SWAP1, t) : 0);
    if (on) { box(STUB, r); const im = OM.impact(t, T.STUB1, { sqx: 0.06, sqy: 0.08 });
      STUB.style.transform += ` scale(${im.sx.toFixed(4)},${im.sy.toFixed(4)})`;
      const lk = seg(t, T.STUB0, T.STUB1); STUB.querySelector('.tok').style.opacity = (1 - sstep(0.3, 0.7, ease.quintInOut(lk))).toFixed(3);
      STUB.style.borderRadius = (mk > 0 ? lerp(12, r.w / 2, mk) : 12).toFixed(2) + 'px';
      STUB.style.background = mk > 0 ? '#FD8260' : '#FFF1EA';
      STUB.style.borderColor = mk > 0 ? '#2E2A28' : '#F5B9A6';
      STUB.style.borderWidth = lerp(1.5, R.body.stroke, mk).toFixed(2) + 'px';
      for (const c of STUB.querySelectorAll('.a,.b,.bar')) c.style.opacity = (1 - sstep(0, 0.3, mk)).toFixed(3); } }

  // ── M · the map ──
  const mapOp = sstep(T.MAP_IN, T.MAP_IN + 0.5, t) * (1 - sstep(G + 0.05, G + 0.35, t));
  show(MAP, mapOp); MAP.style.transform = `translate(${MX}px,${(MY + (1 - sstep(T.MAP_IN, T.MAP_IN + 0.6, t)) * 40).toFixed(1)}px)`;
  for (const [name, p] of Object.entries(PREF)) {
    p.path.style.fill = prefFill(prefVal(name, t));
    if (!p.badge) continue;
    const { n, t0 } = prefNum(name, t), first = (PEV[name] || [{ t: 1e9 }])[0].t;
    const isTokyo = name === '東京', g = isTokyo ? noteRect(t) : null, mk = isTokyo ? ease.expoInOut(seg(t, G_MORPH0 + 0.05, G_MORPH1 + 0.05)) : 0;
    const op = t >= first ? (isTokyo ? 1 - sstep(G_SWAP, G_SWAP1, t) : 1 - sstep(G + 0.05, G + 0.35, t)) : 0;
    show(p.badge, op); if (!op) continue;
    const pop = spring(t - first, 0.55, 18), bump = OM.press(t, t0, { depth: 0.22 }).s;
    const s = (0.6 + 0.4 * pop) * bump * clamp(1 / CAM.zoom, 0.8, 1.6);
    if (mk > 0) { box(p.badge, g); p.badge.style.margin = '0'; p.badge.style.borderRadius = lerp(22, R.note.r, mk).toFixed(2) + 'px';
      p.badge.style.background = '#FC7E5D'; p.badge.style.color = `rgba(255,255,255,${(1 - sstep(0, 0.3, mk)).toFixed(3)})`;
      p.badge.style.boxShadow = 'none'; }
    else { p.badge.style.margin = ''; p.badge.style.width = ''; p.badge.style.height = ''; p.badge.style.borderRadius = ''; p.badge.style.background = ''; p.badge.style.color = ''; p.badge.style.boxShadow = '';
      p.badge.style.transform = `translate(${p.cap[0].toFixed(2)}px,${p.cap[1].toFixed(2)}px) scale(${s.toFixed(4)})`; }
    if (p.badge.textContent !== String(n)) p.badge.textContent = n;
  }
  // the four prefecture pills: out of the chip, onto the capital, where the badge takes over
  PILLS.forEach((p, i) => { const on = t >= T.PILL[i] && t < T.PLAND[i] + 0.02;
    show(p.e, on ? 1 : 0); if (!on) return;
    const r = pillRect(i, t); box(p.e, r); p.e.style.borderRadius = (r.h / 2).toFixed(2) + 'px';
    p.e.style.fontSize = lerp(18, 13, r.k).toFixed(2) + 'px'; p.e.style.lineHeight = r.h.toFixed(2) + 'px';
    p.e.style.padding = `0 ${lerp(14, 0, r.k).toFixed(2)}px`; p.e.style.textAlign = 'center'; });

  // ── threads and the trace ──
  TH_.forEach((h, i) => { const a = h.from(), b = h.to(t), c = ARC(a, b), on = t >= h.t0;
    const op = !on ? 0 : t < h.t1 ? 0.85 : lerp(0.85, 0.35, sstep(h.t1, h.t1 + 0.5, t));
    const pl = PULSES.find(p => p.th === i), glow = pl ? Math.sin(Math.PI * seg(t, pl.t0, pl.t0 + PULSE_DUR + 0.2)) : 0;
    const out = 1 - sstep(T.TRACE_OUT, T.TRACE_OUT + 0.4, t);
    h.p.setAttribute('d', `M${a[0].toFixed(1)} ${a[1].toFixed(1)}Q${c[0].toFixed(1)} ${c[1].toFixed(1)} ${b[0].toFixed(1)} ${b[1].toFixed(1)}`);
    h.p.style.opacity = (Math.min(1, op + 0.6 * glow) * out).toFixed(3);
    h.p.style.strokeWidth = ((2 + 1.5 * glow) * px).toFixed(2);
    const u = pl ? pulseU(pl, t) : 0, live = pl && t >= pl.t0 && u < 1;
    const q = qpt(b, c, a, u); h.c.setAttribute('cx', q[0].toFixed(1)); h.c.setAttribute('cy', q[1].toFixed(1));
    h.c.setAttribute('r', (7 * px).toFixed(2)); h.c.style.opacity = live ? Math.sin(Math.PI * Math.min(1, u * 1.15)).toFixed(3) : 0;
  });

  // ── the lockup: the icon, then the name ──
  { const on = t >= G_SWAP, im = OM.impact(t, G_IMPACT, { sqx: 0.07, sqy: 0.09 });
    show(ICONEL, on ? sstep(G_SWAP, G_SWAP1, t) : 0);
    ICONEL.style.width = ICONEL.style.height = ICON.size + 'px';
    ICONEL.style.transform = `translate(${ICON.x - ICON.size / 2}px,${ICON.y - ICON.size / 2}px) scale(${im.sx.toFixed(4)},${im.sy.toFixed(4)})`;
    show(MARK, t >= G_MARK ? 1 : 0);
    MARK.style.transform = `translate(${(ICON.x - R.markW / 2).toFixed(1)}px,${ICON.y + ICON.size / 2 + 26}px)`;
    MARKL.forEach((s, i) => { const d = OM.letterDrop(t, G_MARK, i, { stagger: 0.045 });
      s.style.opacity = d.op.toFixed(3); s.style.transform = `translateY(${d.y.toFixed(1)}px) scale(${d.sx.toFixed(4)},${d.sy.toFixed(4)})`; });
    [[TAG, G_MARK + 0.75, R.tagW, 175], [META, G_MARK + 1.2, R.metaW, 236], [URL_, G_MARK + 1.55, R.urlW, 284]].forEach(([e, t0, w, dy]) => {
      const r = OM.wordRise(t, t0, { zeta: 0.75 }); show(e, t >= t0 ? r.op : 0);
      e.style.transform = `translate(${(ICON.x - w / 2).toFixed(1)}px,${(ICON.y + ICON.size / 2 + dy + r.y).toFixed(1)}px) scale(${r.s.toFixed(4)})`; }); }

  caption(t);
  $('hud').textContent = 't=' + t.toFixed(3);
}

// ── contract ──
window.__meta = { dur: DUR, fps: 30, w: 1920, h: 1080, cuts: [G_SWAP + 0.1],
  inFrame: { icon: [G_SWAP1, DUR, 'whole'], mark: [G_MARK + 0.6, DUR, 'whole'], url: [G_MARK + 1.8, DUR, 'whole'] } };
window.__track = t => { const V = OM.view(camAt(t)), P = b => OM.projectBox(V, b), o = {};
  if (t < G + 0.35) o.ann = P(R.ann);
  if (t >= 6.5 && t < G_SWAP1) o.cal = P(calRect(t));
  if (t >= T.MAP_IN && t < G + 0.35) o.map = P({ x: MX, y: MY, w: MW, h: MH });
  FLY.forEach((f, i) => { if (t >= T.FLY[i] && t < G + 0.25) o['show' + i] = P(flyRect(i, t)); });
  if (t >= T.STUB0 && t < G_SWAP1) o.stub = P(stubRect(t));
  PILLS.forEach((p, i) => { if (t >= T.PILL[i] && t < T.PLAND[i]) o['pill' + i] = P(pillRect(i, t)); });
  if (t >= PEV['東京'][0].t && t < G_SWAP1) o.tokyo = P(t >= G_MORPH0 ? noteRect(t) : { x: PREF['東京'].cap[0] - 22, y: PREF['東京'].cap[1] - 22, w: 44, h: 44 });
  if (t >= G_SWAP) o.icon = P({ x: ICON.x - ICON.size / 2, y: ICON.y - ICON.size / 2, w: ICON.size, h: ICON.size });
  if (t >= G_MARK) o.mark = P({ x: ICON.x - R.markW / 2, y: ICON.y + ICON.size / 2 + 26, w: R.markW, h: 130 });
  if (t >= G_MARK + 1.55) o.url = P({ x: ICON.x - R.urlW / 2, y: ICON.y + ICON.size / 2 + 284, w: R.urlW, h: 30 });
  return o; };
window.__motion = (t0, t1) => { const tm = (t0 + t1) / 2; let d = OM.screenTravel(camAt(t0), camAt(t1), camAt(tm));
  const a = __track(t0), b = __track(t1);
  for (const k in b) if (a[k]) d = Math.max(d, Math.abs(a[k].x - b[k].x), Math.abs(a[k].y - b[k].y), Math.abs(a[k].x + a[k].w - b[k].x - b[k].w), Math.abs(a[k].y + a[k].h - b[k].y - b[k].h));
  if (t1 > T.RAIN0 && t0 < T.RAIN1 + 0.4) d = Math.max(d, 14 * camAt(tm).zoom);
  if (t1 > G_MARK && t0 < G_MARK + 1.2) d = Math.max(d, 30);
  return d; };
window.__seek = async t => { seek(t); return true; };
window.__ready = document.fonts.load("112px 'Pacifico'").then(() => document.fonts.ready).then(() => Promise.all([...document.images].map(i => i.decode().catch(() => {}))))
  .then(() => { measure(); R.markW = MARK.getBoundingClientRect().width; R.tagW = TAG.getBoundingClientRect().width; R.metaW = META.getBoundingClientRect().width; R.urlW = URL_.getBoundingClientRect().width;
    planRain(); planPrefs(); KEYS = cameraKeys(); seek(0); });
if (Q.has('hud')) $('hud').style.display = 'block';
if (Q.has('t')) __ready.then(() => seek(parseFloat(Q.get('t'))));
if (Q.has('play')) __ready.then(() => { const t0 = performance.now(); (function loop() { seek(((performance.now() - t0) / 1000) % DUR); requestAnimationFrame(loop); })(); });
