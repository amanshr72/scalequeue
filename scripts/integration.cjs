// Requires a migrated disposable DB and Redis. Starts API/worker in this process's network namespace.
const {spawn}=require('node:child_process');
const assert=require('node:assert/strict');
const mysql=require('../api-service/node_modules/mysql2/promise');
const Redis=require('../api-service/node_modules/ioredis');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const pause=ms=>new Promise(r=>setTimeout(r,ms));
const base=process.env.TEST_API_URL||'http://127.0.0.1:13000';
const key=process.env.DEMO_API_KEY||'test-demo-key';
let children=[];
function startWorker(){const p=spawn(process.env.WORKER_BINARY||'/tmp/scalequeue-worker',[],{cwd:root,env:{...process.env,WORKER_COUNT:'2',LEASE_DURATION:'500ms',RECOVERY_IDLE:'200ms',RECOVERY_INTERVAL:'100ms',SHUTDOWN_TIMEOUT:'2s',HTTP_ADDR:':19090'},stdio:['ignore','pipe','pipe']});p.stderr.on('data',b=>process.stderr.write(b));p.stdout.on('data',b=>process.stdout.write(b));children.push(p);return p;}
async function request(route,body,extra={}){const response=await fetch(base+'/api/v1/jobs'+route,{method:body?'POST':'GET',headers:{'Content-Type':'application/json','x-api-key':key,...extra},body:body?JSON.stringify(body):undefined});return {status:response.status,body:await response.json()};}
async function waitFor(fn,label,timeout=12000){const until=Date.now()+timeout;while(Date.now()<until){const result=await fn();if(result)return result;await pause(50);}throw Error('Timed out: '+label);}
(async()=>{
 const d=await mysql.createConnection({host:process.env.DB_HOST||'127.0.0.1',port:Number(process.env.DB_PORT||13306),user:process.env.DB_USER||'root',password:process.env.DB_PASSWORD||'',database:process.env.DB_NAME||'scalequeue'});
 const redis=new Redis(process.env.REDIS_URL);
 try {
  await d.execute('INSERT INTO api_clients(id,name,api_key) VALUES(UUID(),?,?)',['test',key]);
  const api=spawn(process.execPath,['dist/server.js'],{cwd:path.join(root,'api-service'),env:{...process.env,PORT:'13000'},stdio:['ignore','pipe','pipe']});api.stderr.on('data',b=>process.stderr.write(b));api.stdout.on('data',b=>process.stdout.write(b));children.push(api);
  await waitFor(async()=>{try{return (await fetch(base+'/health')).ok}catch{return false}},'API ready');
  assert.equal((await fetch(base+'/api/v1/jobs')).status,401);
  const payload={type:'send_email',payload:{to:'demo@example.test'},max_retries:2};
  const accepted=await Promise.all(Array.from({length:10},()=>request('/',payload,{'Idempotency-Key':'same'})));
  accepted.forEach(x=>assert.equal(x.status,202));const id=accepted[0].body.data.id;assert.equal(new Set(accepted.map(x=>x.body.data.id)).size,1);
  assert.equal((await request('/',{...payload,payload:{to:'other@example.test'}},{'Idempotency-Key':'same'})).status,409);
  const [[{n}]]=await d.query('SELECT COUNT(*) AS n FROM job_outbox WHERE job_id=?',[id]);assert.equal(n,1);
  // Redis outage at acceptance: enqueue still commits with a durable outbox.
  await redis.call('CLIENT','PAUSE',400,'ALL');
  const outage=await request('/',payload);assert.equal(outage.status,202);
  let worker=startWorker();
  await waitFor(async()=>{const x=await request('/'+id);return x.body.data.status==='COMPLETED'},'outbox dispatch and completion');
  await waitFor(async()=>(await request('/'+outage.body.data.id)).body.data.status==='COMPLETED','outage acceptance');
  const [[first]]=await d.query('SELECT attempt_count FROM jobs WHERE id=?',[id]);assert.equal(first.attempt_count,1);
  const failed=await request('/',{type:'fail_job',payload:{reason:'test'},max_retries:1});const failId=failed.body.data.id;
  await waitFor(async()=>(await request('/'+failId)).body.data.status==='DEAD','retry exhausted');
  const [[dead]]=await d.query('SELECT attempt_count FROM jobs WHERE id=?',[failId]);assert.equal(dead.attempt_count,2);
  // Concurrent replay: exactly one resets the generation; the other is rejected.
  const replay=await Promise.all([request('/'+failId+'/requeue',{}),request('/'+failId+'/requeue',{})]);assert.deepEqual(replay.map(x=>x.status).sort(),[200,400]);
  const [[generation]]=await d.query('SELECT generation FROM jobs WHERE id=?',[failId]);assert.equal(generation.generation,2);
  await redis.xadd('jobs:queue','*','job_id',failId,'generation','1');
  await waitFor(async()=>(await request('/'+failId)).body.data.status==='DEAD','replayed job');
  const [[replayed]]=await d.query('SELECT attempt_count FROM jobs WHERE id=?',[failId]);assert.equal(replayed.attempt_count,2);
  // Stop before enqueue, then kill a worker during the simulated 500ms handler.
  worker.kill('SIGTERM');await new Promise(r=>worker.once('exit',r));
  const crash=await request('/',payload);const crashId=crash.body.data.id;
  worker=startWorker();
  await waitFor(async()=>(await request('/'+crashId)).body.data.status==='PROCESSING','worker acquired crash job');
  worker.kill('SIGKILL');await new Promise(r=>worker.once('exit',r));
  worker=startWorker();
  await waitFor(async()=>(await request('/'+crashId)).body.data.status==='COMPLETED','crashed worker recovered');
  const [[recovered]]=await d.query('SELECT attempt_count FROM jobs WHERE id=?',[crashId]);assert.equal(recovered.attempt_count,2);
  const malformed=await request('/',{type:'send_email',payload:{missing:'to'},max_retries:5});
  await waitFor(async()=>(await request('/'+malformed.body.data.id)).body.data.status==='DEAD','permanent payload failure');
  assert.equal((await request('/dead')).status,200);assert.equal((await request('/'+id+'/logs')).status,200);
  const metrics=await fetch('http://127.0.0.1:19090/metrics');assert.equal(metrics.status,200);assert.match(await metrics.text(),/scalequeue_jobs/);
  if (process.env.RUN_BENCHMARK === '1') {
    const output=require('node:child_process').execFileSync(process.execPath,['scripts/benchmark.cjs'],{cwd:root,env:{...process.env,API_URL:base,API_KEY:key,JOBS:'20',CONCURRENCY:'4'},encoding:'utf8'});
    require('node:fs').writeFileSync(path.join(root,'docs/benchmark-local.json'),output);
    console.log('BENCHMARK',output);
  }
  const started=Date.now();worker.kill('SIGTERM');await new Promise(r=>worker.once('exit',r));assert.ok(Date.now()-started<5000);
  console.log('PASS: API auth, concurrent enqueue deduplication, payload conflicts, outbox acceptance, retries/DLQ, concurrent replay, stale generation, worker kill/recovery, invalid payload, logs, metrics, bounded shutdown');
 } finally {
  for(const p of children)if(p.exitCode===null)p.kill('SIGTERM');
  await redis.quit();await d.end();
 }
})().catch(e=>{console.error(e);process.exitCode=1;});
