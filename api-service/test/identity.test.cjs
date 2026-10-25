const {test}=require('node:test');
const assert=require('node:assert/strict');
const {canonical,digest}=require('../dist/services/identity');
test('request hashes ignore object key order while preserving array order',()=>{
 assert.equal(digest(canonical({z:1,p:{b:2,a:1}})),digest(canonical({p:{a:1,b:2},z:1})));
 assert.notEqual(digest(canonical([1,2])),digest(canonical([2,1])));
});
