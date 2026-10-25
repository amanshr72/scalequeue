// Run against an already-started local stack. Measures this demo, not production capacity.
const {performance}=require('node:perf_hooks');
const base=process.env.API_URL||'http://127.0.0.1:3000';
const count=Number(process.env.JOBS||100),concurrency=Number(process.env.CONCURRENCY||10);
const key=process.env.API_KEY||'local-demo-key-change-me';
if(!Number.isInteger(count)||count<1||!Number.isInteger(concurrency)||concurrency<1)throw Error('JOBS and CONCURRENCY must be positive integers');
const headers={'Content-Type':'application/json','x-api-key':key};
const latencies=[];let next=0,errors=0;const start=performance.now();
async function run(){while(next<count){next++;const began=performance.now();try{
 const response=await fetch(base+'/api/v1/jobs',{method:'POST',headers,body:JSON.stringify({type:'send_notification',payload:{user_id:'benchmark',channel:'in_app',message:'synthetic'}})});
 if(response.status!==202)throw Error('enqueue '+response.status);const {data}=await response.json();
 const deadline=performance.now()+120000;
 for(;;){const r=await fetch(base+'/api/v1/jobs/'+data.id,{headers});if(!r.ok)throw Error('status '+r.status);const {data:j}=await r.json();if(j.status==='COMPLETED')break;if(j.status==='DEAD'||performance.now()>deadline)throw Error('completion timeout/failure');await new Promise(r=>setTimeout(r,50));}
 latencies.push(performance.now()-began);
 }catch(e){errors++;console.error(e.message);}}}
(async()=>{await Promise.all(Array.from({length:concurrency},run));const seconds=(performance.now()-start)/1000;latencies.sort((a,b)=>a-b);const percentile=p=>latencies.length?latencies[Math.min(latencies.length-1,Math.ceil(latencies.length*p)-1)]:null;console.log(JSON.stringify({jobs:count,client_concurrency:concurrency,completed:latencies.length,errors,duration_seconds:seconds,jobs_per_second:latencies.length/seconds,end_to_end_p50_ms:percentile(.5),end_to_end_p95_ms:percentile(.95),node:process.version,platform:process.platform,cpus:require('node:os').availableParallelism()},null,2));if(errors)process.exitCode=1;})();
