// Run only against a disposable local stack: this creates real synthetic jobs.
const { spawnSync, execFileSync } = require('node:child_process');
const { writeFileSync } = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const runs = [];
for (let i = 0; i < 3; i++) {
  const result = spawnSync(process.execPath, [path.join(__dirname, 'benchmark.cjs')], {
    env: { ...process.env, JOBS: process.env.JOBS || '30', CONCURRENCY: process.env.CONCURRENCY || '4' },
    encoding: 'utf8', timeout: 180000,
  });
  if (result.status !== 0) {
    process.stderr.write(result.stderr || String(result.error || 'Benchmark failed'));
    process.exit(1);
  }
  runs.push(JSON.parse(result.stdout));
}
const report = {
  description: 'Synthetic notification jobs; handler waits 300ms; includes enqueue, execution and 50ms status polling. Three sequential trials, no warmup. Not production capacity.',
  commit: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
  recorded_at: new Date().toISOString(),
  environment: { os: os.release(), arch: os.arch(), cpu: os.cpus()[0]?.model, memory_bytes: os.totalmem(), node: process.version,
    containers: execFileSync('docker', ['compose', 'images', '--format', 'json'], { encoding: 'utf8' }).trim() },
  runs,
};
writeFileSync('benchmark-results.json', JSON.stringify(report, null, 2) + '\n');
console.log(JSON.stringify(report, null, 2));
