// The card's seal of authenticity: a foil disc engraved with guilloche drawn
// from the member's seed, so no two members' seals match. It catches the
// light as the phone tilts (or as the pointer moves, on a desktop), and its
// clock and second hand keep time, which a screenshot or a printout can't.
(() => {
  const NS = 'http://www.w3.org/2000/svg';
  const FONT = 'Helvetica Neue, Helvetica, Arial, sans-serif';
  const TAU = Math.PI * 2;

  const seal = document.querySelector('.mp-seal');
  if (!seal) return;

  const seedHex = seal.dataset.seed || '0000000000000000';
  const seed = [];
  for (let i = 0; i < seedHex.length; i += 2) seed.push(parseInt(seedHex.slice(i, i + 2), 16) || 0);
  const s = (i) => seed[i % seed.length];

  // --- Geometry --------------------------------------------------------

  const f = (v) => (Math.round(v * 100) / 100).toString();

  // A closed curve as path data
  const curve = (pts) => 'M' + pts.map(([x, y]) => f(x) + ' ' + f(y)).join('L') + 'Z';

  const circle = (r) => `M${f(r)} 0A${f(r)} ${f(r)} 0 1 0 ${f(-r)} 0A${f(r)} ${f(r)} 0 1 0 ${f(r)} 0Z`;

  // Phase-shifted copies of ρ(θ) = r + a·sin(nθ + φ) + b·sin(2nθ − φ),
  // overlaid: the woven lattice banknote rosettes are cut from. The same
  // curve as the wooden badge's rosette.
  function lobedRing(r, a, b, lobes, copies, twist, perLobe) {
    const steps = lobes * perLobe;
    let d = '';
    for (let k = 0; k < copies; k++) {
      const phase = (TAU * k) / copies + twist;
      const pts = [];
      for (let i = 0; i < steps; i++) {
        const th = (TAU * i) / steps;
        const n = lobes * th;
        const rho = r + a * Math.sin(n + phase) + b * Math.sin(2 * n - phase);
        pts.push([rho * Math.cos(th), rho * Math.sin(th)]);
      }
      d += curve(pts);
    }
    return d;
  }

  // Reeding around the rim, like a coin's milled edge
  function reeds(r0, r1, count) {
    let d = '';
    for (let i = 0; i < count; i++) {
      const th = (TAU * i) / count;
      const c = Math.cos(th), sn = Math.sin(th);
      d += `M${f(r0 * c)} ${f(r0 * sn)}L${f(r1 * c)} ${f(r1 * sn)}`;
    }
    return d;
  }

  const twist = (s(4) / 255) * TAU;
  const lobes1 = 18 + (s(0) % 7), copies1 = 18 + (s(1) % 6);
  const lobes2 = 10 + (s(2) % 5), copies2 = 12 + (s(3) % 4);
  const lobesM = 48 + (s(5) % 13);

  const rings = [circle(99.6), circle(97.2), reeds(97.4, 99.4, 240), circle(90.4), circle(89.2),
    circle(78.6), circle(77.6), circle(51.2), circle(41.2), circle(40.2), circle(35.6)].join('');
  const guilloche = [
    lobedRing(93.8, 2.2, 0, 96, 5, twist, 10), // rope border
    lobedRing(72, 4, 0, lobesM, 3, -twist, 12), // fine moiré
    lobedRing(63, 10, 1.5, lobes1, copies1, twist, 20), // outer rosette
    lobedRing(46, 4.5, 0.6, lobes2, copies2, -twist, 20), // inner rosette
  ].join('');

  // Text set on a circle, repeated to fill it and stretched to close exactly
  function ringText(id, r, size, text, attrs) {
    const len = TAU * r;
    const unit = text.length * size * 0.66;
    const run = text.repeat(Math.max(1, Math.round(len / unit)));
    // Start at the top, clockwise
    const path = `<path id="${id}" d="M0 ${f(-r)}A${f(r)} ${f(r)} 0 1 1 0 ${f(r)}A${f(r)} ${f(r)} 0 1 1 0 ${f(-r)}"/>`;
    const t = `<text font-family="${FONT}" font-size="${size}" ${attrs} textLength="${f(len - size * 0.4)}" lengthAdjust="spacing">` +
      `<textPath href="#${id}" xlink:href="#${id}">${esc(run)}</textPath></text>`;
    return { path, t };
  }
  function esc(t) {
    return t.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  const name = (seal.dataset.text || 'MEMBER').trim();
  const outer = ringText('mp-ring-outer', 82, 6.2, `${name} ✦ SEAL OF AUTHENTICITY ✦ `, 'font-weight="700" letter-spacing="0.6" dominant-baseline="central"');
  const micro = ringText('mp-ring-micro', 37.8, 1.9, `${(seal.dataset.micro || '').trim()} · `, 'font-weight="600" dominant-baseline="central"');

  function svgMarkup(inkAttrs, lineWidth, ringWidth) {
    return `<defs>${outer.path}${micro.path}</defs>` +
      `<g fill="none" stroke-linecap="round" stroke-linejoin="round" ${inkAttrs}>` +
      `<path d="${rings}" stroke-width="${ringWidth}"/>` +
      `<path d="${guilloche}" stroke-width="${lineWidth}"/></g>` +
      `<g ${inkAttrs.replace('stroke=', 'fill=')}>${outer.t}${micro.t}</g>`;
  }

  // The engraving, in ink
  const ink = seal.querySelector('.mp-seal-ink');
  ink.innerHTML = svgMarkup('stroke="currentColor"', 0.32, 0.45);

  // The same engraving as a mask, so the foil only lights up along its lines
  const maskSVG = `<svg xmlns="${NS}" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="-100 -100 200 200">${svgMarkup('stroke="#000"', 0.6, 0.7)}</svg>`;
  const maskURL = `url("data:image/svg+xml;charset=utf-8,${encodeURIComponent(maskSVG)}")`;
  const holo = seal.querySelector('.mp-seal-holo');
  holo.style.webkitMaskImage = maskURL;
  holo.style.maskImage = maskURL;

  // --- Clock -----------------------------------------------------------

  const clock = seal.querySelector('[data-clock]');
  const date = seal.querySelector('[data-date]');
  const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'];
  const pad = (n) => String(n).padStart(2, '0');
  function tick() {
    const now = new Date();
    clock.textContent = `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`;
    date.textContent = `${now.getDate()} ${MONTHS[now.getMonth()]} ${now.getFullYear()}`;
    setTimeout(tick, 1000 - now.getMilliseconds() + 5);
  }
  tick();

  // A second hand rides the microtext ring, in step with the clock
  const sweep = document.createElement('div');
  sweep.className = 'mp-seal-sweep';
  sweep.setAttribute('aria-hidden', 'true');
  const n = new Date();
  sweep.style.animationDelay = `-${n.getSeconds() + n.getMilliseconds() / 1000}s`;
  seal.querySelector('.mp-seal-tilt').appendChild(sweep);

  // --- Light -----------------------------------------------------------

  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const clamp = (v) => Math.max(-1, Math.min(1, v));
  let target = { x: 0, y: 0 };
  let cur = { x: 0, y: 0 };
  let lastInput = -Infinity;
  let sensing = false;

  // Tilt is measured from where the phone has lately been held, so the seal
  // answers any movement, however it's held
  let base = null;
  function onOrientation(e) {
    if (e.beta == null || e.gamma == null) return;
    sensing = true;
    let b = e.beta, g = e.gamma;
    if (!base) base = { b, g };
    base.b += (b - base.b) * 0.006;
    base.g += (g - base.g) * 0.006;
    let dx = (g - base.g) / 18, dy = (b - base.b) / 18;
    // In landscape the axes turn with the screen
    const angle = ((screen.orientation && screen.orientation.angle) || window.orientation || 0) * Math.PI / 180;
    const x = dx * Math.cos(angle) + dy * Math.sin(angle);
    const y = dy * Math.cos(angle) - dx * Math.sin(angle);
    target = { x: clamp(x), y: clamp(y) };
    lastInput = performance.now();
  }

  function listen() {
    // Adding the same listener twice is a no-op
    window.addEventListener('deviceorientation', onOrientation);
  }

  // iOS asks first, and only from a tap. Other browsers that have the same
  // call (desktop Chrome does) send readings without asking, if they have a
  // sensor at all, so only offer the tap on a touch screen that stays quiet.
  const motionButton = document.querySelector('[data-motion]');
  if ('DeviceOrientationEvent' in window) listen();
  if (typeof DeviceOrientationEvent !== 'undefined' &&
      typeof DeviceOrientationEvent.requestPermission === 'function' &&
      window.matchMedia('(pointer: coarse)').matches) {
    const ask = () => {
      DeviceOrientationEvent.requestPermission().then((state) => {
        if (state === 'granted') {
          listen();
          motionButton.classList.add('hidden');
        }
      }).catch(() => {});
    };
    setTimeout(() => {
      if (sensing) return;
      motionButton.classList.remove('hidden');
      motionButton.addEventListener('click', ask);
      seal.addEventListener('click', ask);
    }, 700);
  }

  // Without a gyroscope, the pointer moves the light; on a touch screen,
  // dragging across the seal does
  function fromPointer(e) {
    if (sensing) return;
    // A finger moving is scrolling the page, not aiming the light
    if (e.type === 'pointermove' && e.pointerType !== 'mouse') return;
    const r = seal.getBoundingClientRect();
    const reach = Math.max(r.width, 320);
    target = {
      x: clamp((e.clientX - (r.left + r.width / 2)) / reach),
      y: clamp((e.clientY - (r.top + r.height / 2)) / reach),
    };
    lastInput = performance.now();
  }
  window.addEventListener('pointermove', fromPointer, { passive: true });
  seal.addEventListener('pointerdown', fromPointer);

  let frame = 0;
  function draw(t) {
    // Left alone, the light drifts on its own so the foil still lives
    if (t - lastInput > 1800) {
      target = reduceMotion ? { x: 0, y: 0 } : {
        x: 0.42 * Math.sin(t / 1900) + 0.12 * Math.sin(t / 530),
        y: 0.34 * Math.sin(t / 2700 + 1.3),
      };
    }
    const k = t - lastInput > 1800 ? 0.04 : 0.16;
    cur.x += (target.x - cur.x) * k;
    cur.y += (target.y - cur.y) * k;
    seal.style.setProperty('--tx', cur.x.toFixed(4));
    seal.style.setProperty('--ty', cur.y.toFixed(4));
    frame = requestAnimationFrame(draw);
  }
  frame = requestAnimationFrame(draw);

  document.addEventListener('visibilitychange', () => {
    cancelAnimationFrame(frame);
    if (!document.hidden) frame = requestAnimationFrame(draw);
  });
})();
