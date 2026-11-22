import { spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// 将可编辑 SVG 导出为 PNG、macOS ICNS 和 Windows ICO；仅在修改图标时运行。
if (process.platform !== 'darwin') throw new Error('图标导出需要 macOS 的 sips 和 iconutil；正常构建直接使用已导出的图标');
const project = fileURLToPath(new URL('..', import.meta.url));
const output = join(project, 'build/icons');
const temporary = mkdtempSync(join(tmpdir(), 'cue-icon-'));
const iconset = join(temporary, 'cue.iconset');
const { chromium } = await import('../frontend/node_modules/playwright/index.mjs');

function run(command, args) {
  const result = spawnSync(command, args, { stdio: 'pipe', encoding: 'utf8' });
  if (result.error || result.status !== 0) throw result.error || new Error(result.stderr || `${command} 执行失败`);
}

mkdirSync(iconset);
try {
  // SVG 在浏览器原尺寸渲染，保留图标四角的透明通道。
  const browser = await chromium.launch({ executablePath: process.env.CUE_TEST_BROWSER || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });
  try {
    const page = await browser.newPage({ viewport: { width: 1024, height: 1024 }, deviceScaleFactor: 1 });
    await page.setContent(`<style>html,body{margin:0;background:transparent}svg{display:block}</style>${readFileSync(join(output, 'cue.svg'), 'utf8')}`);
    await page.screenshot({ path: join(output, 'cue.png'), omitBackground: true });
  } finally {
    await browser.close();
  }
  for (const size of [16, 32, 128, 256, 512]) {
    for (const scale of [1, 2]) {
      const name = `icon_${size}x${size}${scale === 2 ? '@2x' : ''}.png`;
      run('sips', ['-z', String(size * scale), String(size * scale), join(output, 'cue.png'), '--out', join(iconset, name)]);
    }
  }
  run('iconutil', ['-c', 'icns', iconset, '-o', join(output, 'cue.icns')]);

  // Windows 10/11 支持 PNG 图像条目；同时提供小尺寸以保持任务栏与资源管理器清晰。
  const sizes = [16, 24, 32, 48, 64, 128, 256];
  const images = sizes.map(size => {
    const path = join(temporary, `${size}.png`);
    run('sips', ['-z', String(size), String(size), join(output, 'cue.png'), '--out', path]);
    return readFileSync(path);
  });
  const directory = Buffer.alloc(6 + sizes.length * 16);
  directory.writeUInt16LE(1, 2);
  directory.writeUInt16LE(sizes.length, 4);
  let offset = directory.length;
  images.forEach((data, index) => {
    const entry = 6 + index * 16;
    directory[entry] = sizes[index] % 256;
    directory[entry + 1] = sizes[index] % 256;
    directory.writeUInt16LE(1, entry + 4);
    directory.writeUInt16LE(32, entry + 6);
    directory.writeUInt32LE(data.length, entry + 8);
    directory.writeUInt32LE(offset, entry + 12);
    offset += data.length;
  });
  writeFileSync(join(output, 'cue.ico'), Buffer.concat([directory, ...images]));
  console.log(`已生成软件图标：${resolve(output)}`);
} finally {
  rmSync(temporary, { recursive: true, force: true });
}
