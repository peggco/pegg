import { useEffect, useRef } from 'react';

const WIDTH = 86;
const HEIGHT = 40;
const RAMP = '.,-~:;=!*#$@';
const TAU = Math.PI * 2;

function normalize(v: number[]): number[] {
  const m = Math.hypot(...v);
  return v.map((x) => x / m);
}

function rotate(x: number, y: number, z: number, ax: number, ay: number, az: number): number[] {
  let c = Math.cos(ax);
  let s = Math.sin(ax);
  let ny = y * c - z * s;
  let nz = y * s + z * c;
  y = ny;
  z = nz;
  c = Math.cos(ay);
  s = Math.sin(ay);
  const nx = x * c + z * s;
  nz = -x * s + z * c;
  x = nx;
  z = nz;
  c = Math.cos(az);
  s = Math.sin(az);
  ny = x * s + y * c;
  return [x * c - y * s, ny, z];
}

function center(t: number): number[] {
  return [2.35 * Math.sin(t), 1.23 * Math.sin(t) * Math.cos(t), 0];
}

function tangent(t: number): number[] {
  return normalize([2.35 * Math.cos(t), 1.23 * Math.cos(2 * t), 0]);
}

function frame(t: number): [number[], number[]] {
  const tn = tangent(t);
  return [normalize([-tn[1], tn[0], 0]), [0, 0, 1]];
}

function draw(time: number, screen: HTMLPreElement) {
  const pixels = Array<string>(WIDTH * HEIGHT).fill(' ');
  const depth = new Float64Array(WIDTH * HEIGHT).fill(-Infinity);

  const ax = 0.15 + Math.sin(time * 0.0004) * 0.05;
  const ay = Math.sin(time * 0.00035) * 0.12;
  const az = Math.cos(time * 0.0003) * 0.02;
  const radius = 0.32;

  for (let i = 0; i < 380; i++) {
    const t = (TAU * i) / 380;
    const c = center(t);
    const [n, b] = frame(t);

    for (let j = 0; j < 30; j++) {
      const u = (TAU * j) / 30;
      const cu = Math.cos(u);
      const su = Math.sin(u);
      const p = [
        c[0] + radius * (n[0] * cu + b[0] * su),
        c[1] + radius * (n[1] * cu + b[1] * su),
        c[2] + radius * (n[2] * cu + b[2] * su),
      ];
      const normal = [
        n[0] * cu + b[0] * su,
        n[1] * cu + b[1] * su,
        n[2] * cu + b[2] * su,
      ];

      const [x, y, z] = rotate(p[0], p[1], p[2], ax, ay, az);
      const [nx, ny, nz] = rotate(normal[0], normal[1], normal[2], ax, ay, az);

      const invZ = 1 / (6.8 - z);
      const sx = Math.floor(WIDTH / 2 + x * 88 * invZ);
      const sy = Math.floor(HEIGHT / 2 - y * 45 * invZ);
      if (sx < 0 || sx >= WIDTH || sy < 0 || sy >= HEIGHT) continue;

      const k = sy * WIDTH + sx;
      if (invZ <= depth[k]) continue;
      depth[k] = invZ;

      const diffuse = Math.max(0, nx * 0.35 + ny * -0.55 + nz * 0.76);
      const baseBrightness = Math.min(1, 0.12 + diffuse * 0.58);
      const phase = t - time * 0.00115;
      const wave = Math.pow((Math.cos(phase) + 1) * 0.5, 5);
      const brightness = Math.min(1, baseBrightness + wave * 0.38);

      pixels[k] = RAMP[Math.min(RAMP.length - 1, Math.floor(brightness * (RAMP.length - 1)))];
    }
  }

  let output = '';
  for (let y = 0; y < HEIGHT; y++) {
    output += pixels.slice(y * WIDTH, (y + 1) * WIDTH).join('') + (y < HEIGHT - 1 ? '\n' : '');
  }
  screen.textContent = output;
}

export function PeggForever() {
  const ref = useRef<HTMLPreElement>(null);

  useEffect(() => {
    const screen = ref.current;
    if (!screen) return;
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const startedAt = performance.now();
    let frameID = 0;
    let last = 0;

    const tick = (now: number) => {
      if (now - last >= 80) {
        last = now;
        draw(now - startedAt, screen);
      }
      frameID = requestAnimationFrame(tick);
    };

    draw(0, screen);
    if (!reduceMotion) frameID = requestAnimationFrame(tick);

    const onVisibility = () => {
      if (document.hidden) {
        cancelAnimationFrame(frameID);
        frameID = 0;
      } else if (!reduceMotion && frameID === 0) {
        last = 0;
        frameID = requestAnimationFrame(tick);
      }
    };
    document.addEventListener('visibilitychange', onVisibility);

    return () => {
      cancelAnimationFrame(frameID);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, []);

  return (
    <div className="pegg-forever" aria-hidden="true">
      <pre ref={ref} />
    </div>
  );
}
