// Shows a badge as it will come off the laser: its engraving files burned
// into a plywood card, in 3D. Click (or Enter) flips it over; it leans
// toward the pointer. Pinch or ctrl/⌘-scroll zooms, around the pointer, as
// do the buttons below it and + - 0; drag or scroll to look around when
// zoomed in. A #back in the URL starts it flipped. The laser files themselves are untouched: this
// restyles a copy of each side in the browser.
(() => {
  const NS = 'http://www.w3.org/2000/svg';
  const CARD_WIDTH = 53.98; // mm
  const THICKNESS = 3; // mm of plywood
  const PLIES = 12; // layers drawn to give the card its edge
  const SCORE = '#3a2214'; // scored lines
  const FILL = '#24140a'; // fill-engraved codes, burned deeper
  const EDGE = ['#4a2a15', '#57331b']; // laser-cut edges char dark
  const MIN_ZOOM = 0.6;
  const MAX_ZOOM = 8; // enough to read the 0.6mm microtext
  const ZOOM_STEP = 1.5;
  let uid = 0;

  const el = (name, attrs = {}, parent) => {
    const e = document.createElementNS(NS, name);
    for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
    if (parent) parent.appendChild(e);
    return e;
  };

  async function fetchSVG(url) {
    const res = await fetch(url, { credentials: 'same-origin' });
    if (!res.ok) throw new Error(`${url}: ${res.status}`);
    const svg = new DOMParser().parseFromString(await res.text(), 'image/svg+xml').documentElement;
    if (svg.nodeName !== 'svg') throw new Error(`${url}: not an SVG`);
    return svg;
  }

  // The card's shape, outline and lanyard slot: the cut layer on the front,
  // the alignment outline on the back.
  function shapeOf(svg) {
    const g = svg.querySelector('#cut, #align');
    return [...g.querySelectorAll('path')].map((p) => p.getAttribute('d')).join(' ');
  }

  // face draws one side: wood, grain, then the engraving as burn marks.
  function face(src, seed) {
    const id = `badge3d-${++uid}`;
    const viewBox = src.getAttribute('viewBox');
    const [, , w, h] = viewBox.split(/[\s,]+/).map(Number);
    const shape = shapeOf(src);
    const svg = el('svg', { viewBox, 'aria-hidden': 'true' });
    const defs = el('defs', {}, svg);
    el('path', { d: shape, 'clip-rule': 'evenodd' }, el('clipPath', { id: `${id}-shape` }, defs));

    // Grain runs down the card: noise stretched along it, mapped to brown
    // streaks, over a slower wash of figure in the wood
    const noise = (name, freq, octaves, s, alpha, rgb) => {
      const f = el('filter', { id: `${id}-${name}`, x: 0, y: 0, width: w, height: h, filterUnits: 'userSpaceOnUse' }, defs);
      el('feTurbulence', { type: 'fractalNoise', baseFrequency: freq, numOctaves: octaves, seed: s }, f);
      el('feColorMatrix', {
        type: 'matrix',
        values: `0 0 0 0 ${rgb[0]}  0 0 0 0 ${rgb[1]}  0 0 0 0 ${rgb[2]}  ${alpha}`,
      }, f);
    };
    noise('figure', '0.05 0.006', 2, seed + 7, '1.3 0 0 0 -0.5', [0.55, 0.33, 0.15]);
    noise('grain', '0.9 0.02', 3, seed, '2.2 0 0 0 -0.9', [0.42, 0.25, 0.11]);

    // A burn is a dark line with a faint scorched halo around it
    const burn = el('filter', { id: `${id}-burn`, x: '-5%', y: '-5%', width: '110%', height: '110%' }, defs);
    el('feGaussianBlur', { in: 'SourceAlpha', stdDeviation: 0.16, result: 'spread' }, burn);
    el('feFlood', { 'flood-color': '#6b3d1c', 'flood-opacity': 0.4 }, burn);
    el('feComposite', { in2: 'spread', operator: 'in', result: 'halo' }, burn);
    el('feGaussianBlur', { in: 'SourceGraphic', stdDeviation: 0.025, result: 'line' }, burn);
    const merge = el('feMerge', {}, burn);
    el('feMergeNode', { in: 'halo' }, merge);
    el('feMergeNode', { in: 'line' }, merge);

    // Light catching the surface, moved with the pointer
    const gloss = el('radialGradient', { id: `${id}-gloss`, gradientUnits: 'userSpaceOnUse', cx: w * 0.3, cy: h * 0.2, r: w * 0.9 }, defs);
    el('stop', { offset: 0, 'stop-color': '#fff6e8', 'stop-opacity': 0.45 }, gloss);
    el('stop', { offset: 1, 'stop-color': '#fff6e8', 'stop-opacity': 0 }, gloss);

    const card = el('g', { 'clip-path': `url(#${id}-shape)` }, svg);
    el('rect', { width: w, height: h, fill: '#dbb27c' }, card);
    el('rect', { width: w, height: h, filter: `url(#${id}-figure)` }, card);
    el('rect', { width: w, height: h, filter: `url(#${id}-grain)`, opacity: 0.6 }, card);

    const marks = el('g', { filter: `url(#${id}-burn)` }, card);
    for (const g of src.children) {
      if (g.nodeName !== 'g' || g.id === 'cut' || g.id === 'align') continue;
      const copy = document.importNode(g, true);
      copy.removeAttribute('id');
      if ((copy.getAttribute('fill') || 'none') !== 'none') copy.setAttribute('fill', FILL);
      if ((copy.getAttribute('stroke') || 'none') !== 'none') {
        copy.setAttribute('stroke', SCORE);
        copy.setAttribute('stroke-width', '0.12'); // about a kerf
      }
      marks.appendChild(copy);
    }
    // The cut chars the wood along its edges
    el('path', { d: shape, fill: 'none', stroke: '#2e180b', 'stroke-width': 0.8, opacity: 0.45, filter: `url(#${id}-burn)` }, card);
    el('rect', { width: w, height: h, fill: `url(#${id}-gloss)`, style: 'mix-blend-mode: soft-light' }, card);

    return { svg, gloss, w, h };
  }

  function ply(shape, viewBox, color) {
    const svg = el('svg', { viewBox, class: 'badge3d-ply', 'aria-hidden': 'true' });
    el('path', { d: shape, fill: color, 'fill-rule': 'evenodd' }, svg);
    return svg;
  }

  async function build(root) {
    const [frontSrc, backSrc] = await Promise.all([fetchSVG(root.dataset.front), fetchSVG(root.dataset.back)]);
    const seed = Number(root.dataset.seed) || 1;
    const front = face(frontSrc, seed);
    const back = face(backSrc, seed + 101);

    const card = document.createElement('div');
    card.className = 'badge3d-card';
    const shape = shapeOf(frontSrc);
    const viewBox = frontSrc.getAttribute('viewBox');
    for (let i = 0; i < PLIES; i++) {
      const layer = ply(shape, viewBox, EDGE[i % 2]);
      layer.style.transform = `translateZ(calc(var(--badge3d-t) * ${(i / (PLIES - 1) - 0.5).toFixed(3)}))`;
      card.appendChild(layer);
    }
    for (const [side, f] of [['front', front], ['back', back]]) {
      const div = document.createElement('div');
      div.className = `badge3d-face badge3d-${side}`;
      div.appendChild(f.svg);
      card.appendChild(div);
    }

    const shadow = document.createElement('div');
    shadow.className = 'badge3d-shadow';
    // The stage holds the card at the zoomed size: zooming lays the SVGs
    // out larger, so they redraw sharp, rather than scaling up a picture
    const stage = document.createElement('div');
    stage.className = 'badge3d-stage';
    stage.append(shadow, card);
    root.replaceChildren(stage);

    const caption = root.parentElement.querySelector('.badge3d-caption');
    const label = caption ? caption.textContent.trim() : '';
    let flipped = window.location.hash === '#back';
    let tiltX = 0;
    let tiltY = 0;
    let zoom = 1;
    let panX = 0;
    let panY = 0;

    const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));
    const layout = () => {
      const w = root.clientWidth;
      const h = root.clientHeight;
      const sw = w * zoom;
      const sh = h * zoom;
      // Keep some of the card in view however far it's dragged
      const mx = Math.max(0, (sw - w) / 2);
      const my = Math.max(0, (sh - h) / 2);
      panX = clamp(panX, -mx, mx);
      panY = clamp(panY, -my, my);
      Object.assign(stage.style, {
        width: `${sw}px`,
        height: `${sh}px`,
        left: `${(w - sw) / 2 + panX}px`,
        top: `${(h - sh) / 2 + panY}px`,
        perspective: `${sw * 4}px`,
      });
      root.style.setProperty('--badge3d-t', `${(sw * THICKNESS) / CARD_WIDTH}px`);
      root.classList.toggle('badge3d-zoomed', zoom > 1.001);
      if (zoomLabel) zoomLabel.textContent = `${Math.round(zoom * 100)}%`;
    };
    const apply = () => {
      card.style.transform = `rotateX(${tiltX}deg) rotateY(${(flipped ? 180 : 0) + tiltY}deg)`;
      if (caption) caption.textContent = `${label} · ${flipped ? 'back' : 'front'} · click to flip`;
    };
    const flip = () => {
      flipped = !flipped;
      apply();
    };
    // zoomTo keeps the point (cx, cy), relative to the viewer's center,
    // where it is on screen
    const zoomTo = (z, cx = 0, cy = 0) => {
      const next = clamp(z, MIN_ZOOM, MAX_ZOOM);
      panX = cx - (cx - panX) * (next / zoom);
      panY = cy - (cy - panY) * (next / zoom);
      zoom = next;
      if (zoom <= 1) {
        panX = 0;
        panY = 0;
      }
      layout();
    };
    const centerOffset = (e) => {
      const r = root.getBoundingClientRect();
      return [e.clientX - r.left - r.width / 2, e.clientY - r.top - r.height / 2];
    };

    // Controls sit below the viewer, not inside it: it is itself a button
    const controls = document.createElement('div');
    controls.className = 'badge3d-controls';
    const button = (text, title, onClick) => {
      const b = document.createElement('button');
      b.type = 'button';
      b.textContent = text;
      b.title = title;
      b.setAttribute('aria-label', title);
      b.addEventListener('click', onClick);
      controls.appendChild(b);
      return b;
    };
    button('−', 'Zoom out', () => zoomTo(zoom / ZOOM_STEP));
    const zoomLabel = button('100%', 'Reset zoom', () => zoomTo(1));
    button('+', 'Zoom in', () => zoomTo(zoom * ZOOM_STEP));
    root.after(controls);

    // A drag pans a zoomed card; only a click that didn't move flips it
    let drag = null;
    root.addEventListener('pointerdown', (e) => {
      if (e.button !== 0) return;
      drag = { x: e.clientX, y: e.clientY, panX, panY, moved: false };
    });
    root.addEventListener('pointermove', (e) => {
      if (drag && (e.buttons & 1)) {
        const dx = e.clientX - drag.x;
        const dy = e.clientY - drag.y;
        if (!drag.moved && Math.hypot(dx, dy) > 4) {
          drag.moved = true;
          root.setPointerCapture(e.pointerId);
        }
        if (drag.moved && zoom > 1) {
          panX = drag.panX + dx;
          panY = drag.panY + dy;
          layout();
        }
        if (drag.moved) return;
      }
      const r = root.getBoundingClientRect();
      const x = (e.clientX - r.left) / r.width - 0.5;
      const y = (e.clientY - r.top) / r.height - 0.5;
      // Lean less when zoomed in, where it gets in the way of reading
      tiltY = (x * 26) / zoom;
      tiltX = (-y * 18) / zoom;
      for (const f of [front, back]) {
        f.gloss.setAttribute('cx', f.w * (0.5 - x));
        f.gloss.setAttribute('cy', f.h * (0.5 - y));
      }
      apply();
    });
    root.addEventListener('pointerleave', () => {
      if (drag && drag.moved) return;
      tiltX = 0;
      tiltY = 0;
      apply();
    });
    root.addEventListener('click', () => {
      const moved = drag && drag.moved;
      drag = null;
      if (!moved) flip();
    });
    root.addEventListener('pointerup', () => {
      if (drag && drag.moved) setTimeout(() => { drag = null; });
    });

    // Pinch on a trackpad (a wheel event with ctrlKey) or ctrl/⌘-scroll
    // zooms; once zoomed, scrolling pans. Otherwise the page scrolls.
    root.addEventListener('wheel', (e) => {
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault();
        const [cx, cy] = centerOffset(e);
        zoomTo(zoom * Math.exp(-e.deltaY * 0.01), cx, cy);
      } else if (zoom > 1) {
        e.preventDefault();
        panX -= e.deltaX;
        panY -= e.deltaY;
        layout();
      }
    }, { passive: false });

    root.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        flip();
      } else if (e.key === '+' || e.key === '=') {
        zoomTo(zoom * ZOOM_STEP);
      } else if (e.key === '-') {
        zoomTo(zoom / ZOOM_STEP);
      } else if (e.key === '0') {
        zoomTo(1);
      }
    });

    new ResizeObserver(layout).observe(root);
    layout();
    apply();
  }

  for (const root of document.querySelectorAll('.badge3d')) {
    build(root).catch((err) => {
      console.error('badge preview:', err);
      const img = document.createElement('img');
      img.src = root.dataset.front;
      img.alt = root.getAttribute('aria-label') || 'Badge';
      img.className = 'badge3d-fallback';
      root.replaceChildren(img);
    });
  }
})();
