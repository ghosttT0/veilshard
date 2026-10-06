package panel

// loginPageHTML is served when the admin key is missing or wrong.
const loginPageHTML = `<!DOCTYPE html>
<html lang="zh"><head><meta charset="utf-8"><title>vpnctl 面板</title>
<style>body{background:#0d1117;color:#c9d1d9;font-family:system-ui;display:flex;justify-content:center;padding-top:18vh}
.card{background:#161b22;border:1px solid #30363d;border-radius:10px;padding:28px;width:340px}
input{width:100%;box-sizing:border-box;background:#0d1117;color:#c9d1d9;border:1px solid #30363d;border-radius:6px;padding:9px;margin:10px 0}
button{width:100%;background:#238636;color:#fff;border:0;border-radius:6px;padding:9px;font-size:14px;cursor:pointer}
h2{margin:0 0 6px;font-size:17px}p{font-size:12px;color:#8b949e;margin:4px 0 14px}</style></head>
<body><div class="card"><h2>🔐 vpnctl 管理面板</h2><p>输入管理员密钥（服务器上的 /etc/vpnctl/admin_token）</p>
<input id="k" type="password" placeholder="admin key" autofocus><button onclick="go()">进入面板</button></div>
<script>function go(){var k=document.getElementById('k').value.trim();if(k){location='/panel?key='+encodeURIComponent(k)}}</script>
</body></html>`

// panelHTML is the main admin UI (single page, no external assets).
const panelHTML = `<!DOCTYPE html>
<html lang="zh"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>vpnctl 多用户面板</title>
<style>
*{box-sizing:border-box}body{background:#0d1117;color:#c9d1d9;font-family:system-ui;margin:0;padding:24px}
h1{font-size:20px;margin:0 0 4px}#meta{color:#8b949e;font-size:12px;margin-bottom:18px}
.card{background:#161b22;border:1px solid #30363d;border-radius:10px;padding:16px;margin-bottom:18px;overflow-x:auto}
table{border-collapse:collapse;width:100%;font-size:13px;min-width:860px}
th,td{text-align:left;padding:8px 10px;border-bottom:1px solid #21262d;white-space:nowrap}
th{color:#8b949e;font-weight:600}
.badge{padding:2px 8px;border-radius:10px;font-size:11px}
.active{background:#1a4721;color:#3fb950}.disabled{background:#3d2e00;color:#d29922}
.revoked,.expired{background:#4a1216;color:#f85149}
.link{font-family:monospace;font-size:11px;color:#58a6ff;max-width:210px;display:inline-block;overflow:hidden;text-overflow:ellipsis;vertical-align:middle}
button{background:#21262d;color:#c9d1d9;border:1px solid #30363d;border-radius:6px;padding:4px 9px;font-size:12px;cursor:pointer;margin-left:4px}
button:hover{background:#30363d}.danger:hover{background:#da3633;color:#fff;border-color:#da3633}
.primary{background:#238636;border-color:#238636;color:#fff}
input{background:#0d1117;color:#c9d1d9;border:1px solid #30363d;border-radius:6px;padding:7px 9px;margin-right:8px;font-size:13px}
label{font-size:12px;color:#8b949e;margin-right:4px}
#toast{position:fixed;bottom:20px;right:20px;background:#238636;color:#fff;padding:10px 16px;border-radius:8px;display:none;font-size:13px}
#qrbox{position:fixed;inset:0;background:rgba(0,0,0,.7);display:none;justify-content:center;align-items:center}
#qrbox .card{width:auto;max-width:92vw}pre{margin:8px 0 0;font-size:7px;line-height:1.05}
.uri{font-family:monospace;font-size:11px;color:#8b949e;word-break:break-all;user-select:all}
</style></head><body>
<h1>🛡️ vpnctl 多用户管理</h1><div id="meta">加载中…</div>

<div class="card">
<b style="font-size:14px">添加用户</b><br><br>
<form onsubmit="return addUser(event)">
<label>用户名</label><input id="f-name" placeholder="alice" required style="width:140px">
<label>配额</label><input id="f-quota" placeholder="GB，留空不限" style="width:120px">
<label>有效期</label><input id="f-days" placeholder="天，留空永久" style="width:120px">
<button class="primary" type="submit">＋ 创建并下发到节点</button>
</form>
</div>

<div class="card" style="padding:0">
<table id="tbl">
<thead><tr><th>用户</th><th>状态</th><th>已用 / 配额</th><th>↑上行</th><th>↓下行</th><th>到期</th><th>订阅链接</th><th>操作</th></tr></thead>
<tbody></tbody>
</table>
</div>

<div id="qrbox" onclick="this.style.display='none'"><div class="card">
<b id="qrname"></b><div class="uri" id="qruri"></div><pre id="qrart" style="color:#c9d1d9"></pre>
</div></div>
<div id="toast"></div>

<script>
var KEY=new URLSearchParams(location.search).get('key');
if(KEY){localStorage.setItem('panel_key',KEY);history.replaceState(null,'','/panel')}else{KEY=localStorage.getItem('panel_key')}
function hdr(){var h={'Content-Type':'application/json'};if(KEY)h['X-Admin-Key']=KEY;return h}
function toast(m){var t=document.getElementById('toast');t.textContent=m;t.style.display='block';setTimeout(function(){t.style.display='none'},2600)}
function esc(s){return String(s).replace(/[&<>"']/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})}
function fmtTime(u){if(!u)return '永久';var d=new Date(u*1000),rem=u-Date.now()/1000;if(rem<0)return '<span class="badge expired">已过期</span>';
var dd=Math.floor(rem/86400),hh=Math.floor(rem%86400/3600);return d.toLocaleDateString()+' <span style="color:#8b949e">('+dd+'天'+hh+'时)</span>'}
function load(){fetch('/api/users',{headers:hdr()}).then(function(r){return r.json()}).then(function(d){
if(d.error){toast(d.error);return}
document.getElementById('meta').textContent='节点 198.200.39.208 · '+d.users.length+' 个用户 · '+new Date(d.now*1000).toLocaleTimeString()+' 刷新 · 10s 自动刷新';
var tb=document.querySelector('#tbl tbody');tb.innerHTML='';
d.users.forEach(function(u){
var tr=document.createElement('tr');
tr.innerHTML='<td><b>'+esc(u.name)+'</b></td>'
+'<td><span class="badge '+u.status+'">'+u.status+'</span></td>'
+'<td>'+u.used_human+' / '+u.quota_human+'</td>'
+'<td style="color:#3fb950">'+u.up+'</td>'
+'<td style="color:#58a6ff">'+u.down+'</td>'
+'<td>'+fmtTime(u.expires)+'</td>'
+'<td><span class="link" title="'+esc(u.sub_url)+'">'+esc(u.sub_url)+'</span>'
+'<button onclick="copyTxt(\''+esc(u.sub_url)+'\')">复制</button></td>'
+'<td>'
+'<button onclick="qr(\''+esc(u.name)+'\')">二维码</button>'
+'<button onclick="quota(\''+esc(u.name)+'\')">配额</button>'
+'<button onclick="rtok(\''+esc(u.name)+'\')">重置链接</button>'
+(u.status==='disabled'?'<button onclick="toggle(\''+esc(u.name)+'\',true)">启用</button>':'<button onclick="toggle(\''+esc(u.name)+'\',false)">停用</button>')
+'<button class="danger" onclick="del(\''+esc(u.name)+'\')">删除</button>'
+'</td>';
tb.appendChild(tr)})}).catch(function(e){toast('加载失败：'+e)})}
function post(path,body,ok){fetch(path,{method:'POST',headers:hdr(),body:JSON.stringify(body)}).then(function(r){return r.json()}).then(function(d){
if(d.error){toast('❌ '+d.error)}else{toast(ok||'✅ 已生效');load()}}).catch(function(e){toast('请求失败：'+e)})}
function addUser(e){e.preventDefault();
var q=parseFloat(document.getElementById('f-quota').value)||0;
var dd=parseInt(document.getElementById('f-days').value)||0;
post('/api/user/add',{name:document.getElementById('f-name').value.trim(),quota_gb:q,days:dd},'✅ 用户已创建并下发到节点');
document.getElementById('f-name').value='';document.getElementById('f-quota').value='';document.getElementById('f-days').value='';return false}
function del(n){if(confirm('确定删除用户 '+n+'？其订阅链接立即失效。'))post('/api/user/remove',{name:n},'✅ 已删除')}
function toggle(n,en){post('/api/user/toggle',{name:n,enabled:en},en?'✅ 已启用':'⏸ 已停用')}
function rtok(n){if(confirm('重置 '+n+' 的订阅链接？旧链接立即失效。'))post('/api/user/reset-token',{name:n},'✅ 链接已重置，请重新复制')}
function quota(n){var v=prompt('设置 '+n+' 的配额（GB，0 或留空 = 不限）');if(v===null)return;
post('/api/user/quota',{name:n,quota_gb:parseFloat(v)||0},'✅ 配额已更新')}
function copyTxt(t){(navigator.clipboard?navigator.clipboard.writeText(t):Promise.reject()).then(function(){toast('📋 已复制')}).catch(function(){
var i=document.createElement('input');i.value=t;document.body.appendChild(i);i.select();document.execCommand('copy');i.remove();toast('📋 已复制')})}
function qr(n){fetch('/api/user/qr?name='+encodeURIComponent(n),{headers:hdr()}).then(function(r){return r.json()}).then(function(d){
if(d.error){toast(d.error);return}
document.getElementById('qrname').textContent='📱 '+n+' 的 vless:// 链接';
document.getElementById('qruri').textContent=d.uri;
document.getElementById('qrart').textContent=d.qr;
document.getElementById('qrbox').style.display='flex'})}
load();setInterval(load,10000);
</script></body></html>`
