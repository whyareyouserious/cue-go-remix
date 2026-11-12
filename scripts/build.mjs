import { spawnSync } from 'node:child_process';
import { cpSync, mkdirSync, existsSync } from 'node:fs';
import { dirname, resolve } from 'node:path';

const target = process.argv[2] || process.platform;
const arch = process.argv[3] || (target === 'windows' ? 'amd64' : process.arch === 'arm64' ? 'arm64' : 'amd64');
const configuredLicenseURL = (process.env.CUE_LICENSE_URL || '').trim();
let licenseURL = '';
if (configuredLicenseURL) {
  let parsed;
  try { parsed = new URL(configuredLicenseURL); } catch { /* 下面统一报告配置错误，不输出原始地址。 */ }
  const deployedHTTP = parsed?.href.replace(/\/+$/, '') === 'http://43.142.63.76/api';
  if (!parsed || (parsed.protocol !== 'https:' && !deployedHTTP) || !parsed.hostname || parsed.username || parsed.password ||
      /[\s\u0000-\u001f\u007f'"\\?#]/u.test(configuredLicenseURL)) {
    throw new Error('CUE_LICENSE_URL 必须是 HTTPS API 地址或已部署的 http://43.142.63.76/api，且不能包含账号、密码、查询参数、片段、空白或引号');
  }
  licenseURL = parsed.href.replace(/\/+$/, '');
} else {
  console.log('未覆盖 CUE_LICENSE_URL：使用代码中已部署的默认授权 API 入口。');
}
const linkerFlags = ['-s', '-w', ...(licenseURL ? ['-X', `cue-go-remix/internal/config.LicenseBaseURL=${licenseURL}`] : [])];
function run(command, args, env = {}) {
  const result = spawnSync(command, args, { stdio: 'inherit', env: { ...process.env, ...env } });
  if (result.error || result.status !== 0) throw result.error || new Error(`${command} 构建失败`);
}
run(process.execPath, ['scripts/prepare-sherpa.mjs', target, arch]);
// Windows 的 npm.cmd 需要 shell；直接调用随 Node 安装的 npm CLI，构建参数不经 shell 展开。
if (process.platform === 'win32') {
  const npmCLI = resolve(dirname(process.execPath), 'node_modules/npm/bin/npm-cli.js');
  if (!existsSync(npmCLI)) throw new Error('未找到 npm CLI，请使用包含 npm 的 Node.js 安装重新构建');
  run(process.execPath, [npmCLI, '--prefix', 'frontend', 'run', 'build']);
} else run('npm', ['--prefix', 'frontend', 'run', 'build']);
if (target === 'darwin') {
  if (process.platform !== 'darwin') throw new Error('macOS 版本必须在 macOS 构建');
  const contents = resolve('build/bin/Cue.app/Contents');
  mkdirSync(`${contents}/MacOS`, { recursive: true });
  mkdirSync(`${contents}/Helpers`, { recursive: true });
  mkdirSync(`${contents}/Resources`, { recursive: true });
  run('go', ['build', '-tags', 'production,private_mac_apis', '-trimpath', `-ldflags=${linkerFlags.join(' ')}`, '-o', `${contents}/MacOS/Cue`, '.'], { GOOS: 'darwin', GOARCH: arch, CGO_ENABLED: '1', MACOSX_DEPLOYMENT_TARGET: '13.3', CGO_CFLAGS: '-O2 -g -mmacosx-version-min=13.3', CGO_CXXFLAGS: '-O2 -g -mmacosx-version-min=13.3', CGO_LDFLAGS: '-mmacosx-version-min=13.3' });
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
  run('go', ['build', '-tags', 'production', '-trimpath', `-ldflags=${[...linkerFlags, '-H', 'windowsgui'].join(' ')}`, '-o', `${dir}/Cue.exe`, '.'], { GOOS: 'windows', GOARCH: arch, CGO_ENABLED: '0' });
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
