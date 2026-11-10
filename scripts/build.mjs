import { spawnSync } from 'node:child_process';
import { cpSync, mkdirSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

const target = process.argv[2] || process.platform;
const arch = process.argv[3] || (target === 'windows' ? 'amd64' : process.arch === 'arm64' ? 'arm64' : 'amd64');
function run(command, args, env = {}) {
  const result = spawnSync(command, args, { stdio: 'inherit', env: { ...process.env, ...env } });
  if (result.error || result.status !== 0) throw result.error || new Error(`${command} 构建失败`);
}
run(process.execPath, ['scripts/prepare-sherpa.mjs', target, arch]);
if (process.platform === 'win32') run('cmd.exe', ['/d', '/s', '/c', 'npm --prefix frontend run build']);
else run('npm', ['--prefix', 'frontend', 'run', 'build']);
if (target === 'darwin') {
  if (process.platform !== 'darwin') throw new Error('macOS 版本必须在 macOS 构建');
  const contents = resolve('build/bin/Cue.app/Contents');
  mkdirSync(`${contents}/MacOS`, { recursive: true });
  mkdirSync(`${contents}/Helpers`, { recursive: true });
  mkdirSync(`${contents}/Resources`, { recursive: true });
  run('go', ['build', '-tags', 'production,private_mac_apis', '-trimpath', '-ldflags=-s -w', '-o', `${contents}/MacOS/Cue`, '.'], { GOOS: 'darwin', GOARCH: arch, CGO_ENABLED: '1', MACOSX_DEPLOYMENT_TARGET: '13.3', CGO_CFLAGS: '-O2 -g -mmacosx-version-min=13.3', CGO_CXXFLAGS: '-O2 -g -mmacosx-version-min=13.3', CGO_LDFLAGS: '-mmacosx-version-min=13.3' });
  run('bash', ['native/macos/build.sh', arch === 'arm64' ? 'arm64' : 'x86_64', `${contents}/Helpers/cue-audio`]);
  cpSync('build/macos/Info.plist', `${contents}/Info.plist`);
  const whisper = `build/bin/whisper-darwin-${arch}`;
  if (existsSync(whisper)) cpSync(whisper, `${contents}/Resources/whisper`, { recursive: true });
  cpSync(`build/bin/sherpa-darwin-${arch}`, `${contents}/Resources/sherpa`, { recursive: true });
  for (const file of ['readme.md', 'LICENSE', 'THIRD_PARTY_NOTICES.md']) cpSync(file, `${contents}/Resources/${file}`);
  cpSync('licenses', `${contents}/Resources/licenses`, { recursive: true });
  cpSync('docs', `${contents}/Resources/docs`, { recursive: true });
  run('codesign', ['--force', '--deep', '--sign', '-', 'build/bin/Cue.app']);
  console.log('已生成 build/bin/Cue.app');
} else if (target === 'windows' || target === 'win32') {
  const dir = resolve(`build/bin/windows-${arch}`);
  mkdirSync(dir, { recursive: true });
  run('go', ['build', '-tags', 'production', '-trimpath', '-ldflags=-s -w -H windowsgui', '-o', `${dir}/Cue.exe`, '.'], { GOOS: 'windows', GOARCH: arch, CGO_ENABLED: '0' });
  const whisper = `build/bin/whisper-windows-${arch}`;
  if (existsSync(whisper)) cpSync(whisper, `${dir}/whisper`, { recursive: true });
  cpSync(`build/bin/sherpa-windows-${arch}`, `${dir}/sherpa`, { recursive: true });
  for (const file of ['readme.md', 'LICENSE', 'THIRD_PARTY_NOTICES.md']) cpSync(file, `${dir}/${file}`);
  cpSync('licenses', `${dir}/licenses`, { recursive: true });
  cpSync('docs', `${dir}/docs`, { recursive: true });
  console.log(`已生成 ${dir}/Cue.exe；首次启动需要 Microsoft Edge WebView2 Runtime。`);
} else {
  throw new Error('当前打包入口支持 darwin / windows');
}
