import { spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';

const target = process.argv[2] || process.platform;
const arch = process.argv[3] || (target === 'windows' || target === 'win32' ? 'amd64' : process.arch === 'arm64' ? 'arm64' : 'amd64');
const windows = target === 'windows' || target === 'win32';
const source = resolve(windows ? `build/bin/windows-${arch}` : 'build/bin/Cue.app');
const archive = resolve(`build/bin/Cue-${windows ? 'windows' : 'macos'}-${arch}.zip`);
if (!existsSync(source)) throw new Error('请先执行 scripts/build.mjs 构建对应平台');
let result;
if (process.platform === 'darwin') result = spawnSync('ditto', ['-c', '-k', ...(windows ? ['--norsrc', '--noextattr'] : ['--sequesterRsrc']), '--keepParent', source, archive], { stdio: 'inherit' });
else if (process.platform === 'win32' && windows) {
  const quote = value => `'${value.replaceAll("'", "''")}'`;
  result = spawnSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', `Compress-Archive -LiteralPath ${quote(source)} -DestinationPath ${quote(archive)} -Force`], { stdio: 'inherit' });
} else throw new Error('当前打包脚本支持 macOS 与 Windows 主机');
if (result.error || result.status !== 0) throw result.error || new Error('创建压缩包失败');
console.log(`已打包：${archive}`);
