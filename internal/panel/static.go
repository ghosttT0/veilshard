package panel

// loginPageHTML is the ONLY thing unauthenticated visitors receive: a bare
// password form, no panel markup, no comments, no identifying data.
const loginPageHTML = `<!DOCTYPE html>
<html lang="zh"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="referrer" content="no-referrer"><title>·</title>
<style>body{background:#0a0e14;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;font-family:system-ui}
.f{background:#11161f;border:1px solid #2a3344;border-radius:12px;padding:30px;width:min(400px,92vw)}
h2{color:#c9d1d9;font-size:15px;margin:0 0 4px;font-weight:600}p{color:#7d8590;font-size:12px;margin:0 0 14px}
input{width:100%;box-sizing:border-box;background:#0d1117;color:#c9d1d9;border:1px solid #2a3344;border-radius:6px;padding:10px;font-size:13px;font-family:ui-monospace,monospace;margin-bottom:12px}
button{width:100%;background:#238636;color:#fff;border:0;border-radius:6px;padding:10px;font-size:14px;cursor:pointer}
</style></head><body><div class="f"><h2>🔒 管理员登录</h2><p>输入管理密钥继续</p>
<form method="POST" action="/panel/login"><input type="password" name="key" autocomplete="off" autofocus required><button type="submit">解锁</button></form>
</div></body></html>`

// panelHTML is the admin console, served only to authenticated sessions.
// Session state rides an HttpOnly SameSite=Strict cookie; the page itself
// holds no secrets and carries no comments.
const panelHTML = `<!DOCTYPE html>
<html lang="zh"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="referrer" content="no-referrer"><title>P</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}body{background:#0a0e14;color:#c9d1d9;font-family:system-ui,-apple-system,"Segoe UI",sans-serif;padding:22px;min-height:100vh}
h1{font-size:19px;letter-spacing:.5px}#meta{color:#7d8590;font-size:12px;margin:6px 0 16px}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px;margin-bottom:16px}
.card{background:#11161f;border:1px solid #1f2733;border-radius:10px;padding:14px 16px;overflow-x:auto}
.card .v{font-size:22px;font-weight:700;margin-top:4px}.card .l{font-size:12px;color:#7d8590}
.g{color:#3fb950}.y{color:#d29922}.r{color:#f85149}.b{color:#58a6ff}
table{border-collapse:collapse;width:100%;font-size:13px;min-width:900px}
th,td{text-align:left;padding:9px 10px;border-bottom:1px solid #1a2130;white-space:nowrap}
th{color:#7d8590;font-weight:600;font-size:12px}
.badge{padding:2px 9px;border-radius:10px;font-size:11px}
.active{background:#12261a;color:#3fb950}.disabled{background:#2a2107;color:#d29922}
.revoked,.expired{background:#2d1215;color:#f85149}
.bar{width:110px;height:6px;background:#1a2130;border-radius:4px;display:inline-block;vertical-align:middle;margin-right:6px;overflow:hidden}
.bar i{display:block;height:100%;border-radius:4px}
.link{font-family:ui-monospace,monospace;font-size:11px;color:#58a6ff;max-width:200px;display:inline-block;overflow:hidden;text-overflow:ellipsis;vertical-align:middle}
button{background:#1a2130;color:#c9d1d9;border:1px solid #2a3344;border-radius:6px;padding:4px 9px;font-size:12px;cursor:pointer;margin-left:4px}
button:hover{background:#2a3344}.danger:hover{background:#b62324;color:#fff;border-color:#b62324}
.primary{background:#238636;border-color:#238636;color:#fff}.primary:hover{background:#2ea043}
input{background:#0d1117;color:#c9d1d9;border:1px solid #2a3344;border-radius:6px;padding:7px 9px;margin-right:8px;font-size:13px}
label{font-size:12px;color:#7d8590;margin-right:4px}
#toast{position:fixed;bottom:20px;right:20px;background:#238636;color:#fff;padding:10px 16px;border-radius:8px;display:none;font-size:13px;z-index:9}
#toast.e{background:#b62324}
#qrbox{position:fixed;inset:0;background:rgba(4,7,12,.85);display:none;justify-content:center;align-items:center;z-index:8}
#qrbox .box{background:#11161f;border:1px solid #2a3344;border-radius:12px;padding:20px;max-width:92vw;overflow:auto}
pre{font-size:7px;line-height:1.05;margin-top:8px}.uri{font-family:ui-monospace,monospace;font-size:11px;color:#7d8590;word-break:break-all;user-select:all;margin-top:6px}
#logout{float:right}
</style></head><body>
<button id="logout" onclick="lg()">退出</button>
<h1>◈ Node Panel</h1><div id="meta">—</div>
<div class="cards" id="cards"></div>
<div class="card"><b style="font-size:14px">添加用户</b><br><br>
<form id="addf"><label>用户名</label><input id="f-name" placeholder="alice" required style="width:130px">
<label>配额</label><input id="f-quota" placeholder="GB 留空不限" style="width:110px">
<label>有效期</label><input id="f-days" placeholder="天 留空永久" style="width:110px">
<button class="primary" type="submit">创建并下发</button></form></div>
<div class="card" style="padding:0"><table><thead><tr>
<th>用户</th><th>状态</th><th>用量 / 配额</th><th>↑</th><th>↓</th><th>到期</th><th>订阅链接</th><th>操作</th>
</tr></thead><tbody id="tb"></tbody></table></div>
<div id="qrbox" onclick="this.style.display='none'"><div class="box"><b id="qrname"></b><div class="uri" id="qruri"></div><pre id="qrart"></pre></div></div>
<div id="toast"></div>
<script>
function hdr(){return{'Content-Type':'application/json'}}
function toast(m,e){var t=document.getElementById('toast');t.textContent=m;t.className=e?'e':'';t.style.display='block';setTimeout(function(){t.style.display='none'},2800)}
function esc(s){return String(s).replace(/[&<>"']/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})}
function fmtT(u){if(!u)return'永久';var r=u-Date.now()/1e3;if(r<0)return'<span class="badge expired">已过期</span>';
return new Date(u*1e3).toLocaleDateString()+' <span style="color:#7d8590">('+Math.floor(r/86400)+'天)</span>'}
function bar(u,q){if(q<=0)return'<span class="bar"><i style="width:'+(u>0?3:0)+'%;background:#58a6ff"></i></span>不限';
var p=Math.min(100,u/q*100),c=p<70?'#3fb950':(p<90?'#d29922':'#f85149');
return'<span class="bar"><i style="width:'+p+'%;background:'+c+'"></i></span>'+p.toFixed(0)+'%'}
function fmtBytes(b){if(b<1024)return b+' B';var u=['KB','MB','GB','TB'],i=-1;do{b/=1024;i++}while(b>=1024&&i<3);return b.toFixed(2)+' '+u[i]}
function load(){fetch('/api/users',{headers:hdr()}).then(function(r){
if(r.status===401){location.reload();throw'auth'}return r.json()}).then(function(d){
if(d.error){toast(d.error,1);return}
var nu=0,used=0;d.users.forEach(function(u){if(u.status==='active')nu++;used+=u.used});
document.getElementById('meta').textContent=d.users.length+' 用户 · '+new Date(d.now*1e3).toLocaleTimeString()+' 更新 · 10s 自动刷新';
document.getElementById('cards').innerHTML=
'<div class="card"><div class="l">用户总数</div><div class="v">'+d.users.length+'</div></div>'
+'<div class="card"><div class="l">活跃用户</div><div class="v g">'+nu+'</div></div>'
+'<div class="card"><div class="l">累计流量</div><div class="v b">'+fmtBytes(used)+'</div></div>'
+'<div class="card"><div class="l">节点状态</div><div class="v '+(d.node_online?'g':'r')+'">'+(d.node_online?'● 在线':'○ 离线')+'</div></div>';
var tb=document.getElementById('tb');tb.innerHTML='';
d.users.forEach(function(u){var tr=document.createElement('tr');
tr.innerHTML='<td><b>'+esc(u.name)+'</b></td>'
+'<td><span class="badge '+u.status+'">'+u.status+'</span></td>'
+'<td>'+bar(u.used,u.quota)+' '+u.used_human+' / '+u.quota_human+'</td>'
+'<td class="g">'+u.up_human+'</td><td class="b">'+u.down_human+'</td>'
+'<td>'+fmtT(u.expires)+'</td>'
+'<td><span class="link" title="'+esc(u.sub_url)+'">'+esc(u.sub_url)+'</span><button onclick="cp(\''+esc(u.sub_url)+'\')">复制</button></td>'
+'<td><button onclick="qr(\''+esc(u.name)+'\')">二维码</button>'
+'<button onclick="sq(\''+esc(u.name)+'\')">配额</button>'
+'<button onclick="rt(\''+esc(u.name)+'\')">重置链接</button>'
+(u.status==='disabled'?'<button onclick="tg(\''+esc(u.name)+'\',true)">启用</button>':'<button onclick="tg(\''+esc(u.name)+'\',false)">停用</button>')
+'<button class="danger" onclick="del(\''+esc(u.name)+'\')">删除</button></td>';
tb.appendChild(tr)})}).catch(function(e){if(e!=='auth')toast('加载失败',1)})}
function post(p,b,ok){fetch(p,{method:'POST',headers:hdr(),body:JSON.stringify(b)}).then(function(r){
if(r.status===401){location.reload();throw'auth'}return r.json()}).then(function(d){
if(d.error){toast('❌ '+d.error,1)}else{toast(ok);load()}}).catch(function(e){if(e!=='auth')toast('请求失败',1)})}
document.getElementById('addf').onsubmit=function(e){e.preventDefault();
post('/api/user/add',{name:document.getElementById('f-name').value.trim(),quota_gb:parseFloat(document.getElementById('f-quota').value)||0,days:parseInt(document.getElementById('f-days').value)||0},'✅ 已创建并下发到节点');
this.reset();return false};
function del(n){if(confirm('删除 '+n+'？订阅链接立即失效且不可恢复。'))post('/api/user/remove',{name:n},'✅ 已删除并回收')}
function tg(n,en){post('/api/user/toggle',{name:n,enabled:en},en?'✅ 已启用':'⏸ 已停用')}
function rt(n){if(confirm('重置 '+n+' 的订阅链接？旧链接立即作废。'))post('/api/user/reset-token',{name:n},'✅ 新链接已生成')}
function sq(n){var v=prompt('设置 '+n+' 的配额（GB，0 = 不限）');if(v===null)return;post('/api/user/quota',{name:n,quota_gb:parseFloat(v)||0},'✅ 配额已更新')}
function cp(t){(navigator.clipboard?navigator.clipboard.writeText(t):Promise.reject()).then(function(){toast('📋 已复制')}).catch(function(){
var i=document.createElement('textarea');i.value=t;document.body.appendChild(i);i.select();document.execCommand('copy');i.remove();toast('📋 已复制')})}
function qr(n){fetch('/api/user/qr?name='+encodeURIComponent(n),{headers:hdr()}).then(function(r){return r.json()}).then(function(d){
if(d.error){toast(d.error,1);return}
document.getElementById('qrname').textContent='📱 '+n;
document.getElementById('qruri').textContent=d.uri;
document.getElementById('qrart').textContent=d.qr;
document.getElementById('qrbox').style.display='flex'})}
function lg(){fetch('/panel/logout',{method:'POST'}).then(function(){location='/panel'})}
load();setInterval(load,10000);
</script></body></html>`
