import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { copyFileSync, existsSync, mkdirSync, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const project = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const manifest = JSON.parse(readFileSync(join(project, 'internal/sherpa/artifacts.json'), 'utf8'));
const platform = process.argv[2] === 'win32' ? 'windows' : process.argv[2] || (process.platform === 'win32' ? 'windows' : process.platform);
const arch = process.argv[3] || (platform === 'windows' ? 'amd64' : process.arch === 'arm64' ? 'arm64' : 'amd64');
const target = `${platform}-${arch}`;
const runtime = manifest.runtimes[target];
if (!runtime) throw new Error(`不支持的本地转写运行时平台：${target}`);
const cache = join(project, 'build/cache/sherpa-onnx');
const output = join(project, `build/bin/sherpa-${target}`);

function run(command, args) {
  const result = spawnSync(command, args, { stdio: 'inherit' });
  if (result.error || result.status !== 0) throw result.error || new Error(`${command} 执行失败`);
}
function verify(path, artifact) {
  return existsSync(path) && statSync(path).size === artifact.bytes && createHash('sha256').update(readFileSync(path)).digest('hex') === artifact.sha256;
}
function unpack(artifact, directory) {
  mkdirSync(directory, { recursive: true });
  const archive = join(directory, artifact.filename);
  if (!verify(archive, artifact)) {
    const partial = `${archive}.part`;
    console.log(`下载 ${artifact.filename}（${Math.round(artifact.bytes / 1e6)} MB）`);
    run(process.platform === 'win32' ? 'curl.exe' : 'curl', ['--location', '--fail', '--retry', '2', '--continue-at', '-', '--output', partial, artifact.url]);
    if (!verify(partial, artifact)) throw new Error(`${artifact.filename} 大小或 SHA-256 不匹配，已保留下载文件供检查`);
    renameSync(partial, archive);
  }
  const root = join(directory, artifact.root);
  // 仅在已校验过的固定官方归档中解压所需资源。
  run('tar', ['-xf', archive, '-C', directory]);
  return root;
}

const runtimeRoot = unpack(runtime, join(cache, manifest.version));
const modelRoot = unpack(manifest.model, cache);
for (const file of manifest.model.files) {
  if (!verify(join(modelRoot, file.filename), file)) throw new Error(`模型文件 ${file.filename} 校验失败，请删除对应缓存目录后重新准备`);
}
mkdirSync(output, { recursive: true });
mkdirSync(join(output, 'model'), { recursive: true });
for (const name of runtime.libraries) {
  const source = join(runtimeRoot, 'lib', name);
  if (!existsSync(source)) throw new Error(`官方运行时缺少 ${name}`);
  copyFileSync(source, join(output, name));
}
for (const file of manifest.model.files) copyFileSync(join(modelRoot, file.filename), join(output, 'model', file.filename));
copyFileSync(join(modelRoot, 'README.md'), join(output, 'model', 'README.md'));
writeFileSync(join(output, 'model', 'manifest.json'), JSON.stringify(manifest.model, null, 2) + '\n');
writeFileSync(join(output, 'runtime.json'), JSON.stringify({ version: manifest.version, onnxruntimeVersion: manifest.onnxruntimeVersion, target, source: runtime.url, archiveSHA256: runtime.sha256 }, null, 2) + '\n');
// 不带测试录音与 WebSocket 服务程序；权重来源记录保存在发行包通知中。
console.log(`已准备中文流式转写：${output}`);
