package widget

// widgetHTMLTemplate is the Mini-HTML template complying with Telegram Widget Profile 1.
// Strictly optimized to keep total compressed size well under 4096 bytes even for heavy schedules.
const widgetHTMLTemplate = `<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no"><style>
:root{--bg:var(--tg-theme-bg-color,#fff);--txt:var(--tg-theme-text-color,#1c1c1e);--h:var(--tg-theme-hint-color,#8e8e93);--b:var(--tg-theme-button-color,#2481cc);--bt:var(--tg-theme-button-text-color,#fff);--s:var(--tg-theme-secondary-bg-color,#f2f2f7);--bd:rgba(128,128,128,0.18);--a:#10b981}
*{box-sizing:border-box;margin:0;padding:0;font-family:sans-serif}
body{background:var(--bg);color:var(--txt);padding:6px 8px 16px 8px;font-size:12px;line-height:1.25;user-select:none}
.hdr{display:flex;justify-content:space-between;align-items:center;margin-bottom:5px;padding-bottom:4px;border-bottom:1px solid var(--bd)}
.t{font-size:14px;font-weight:700}
.sub{font-size:11px;color:var(--h)}
.vb{background:var(--s);color:var(--txt);border:none;border-radius:6px;padding:3px 8px;font-size:11px;font-weight:600;cursor:pointer}
.vb.act{background:var(--b);color:var(--bt)}
.tabs,.sb{display:flex;gap:4px;margin-bottom:4px}
.tab,.sg{flex:1;background:var(--s);border:none;font-weight:600;cursor:pointer}
.tab{min-width:32px;padding:4px 2px;color:var(--txt);border-radius:7px;font-size:11.5px;text-align:center}
.sg{padding:3px 0;color:var(--h);border-radius:6px;font-size:11px}
.tab.act,.sg.act{background:var(--b);color:var(--bt)}
.lv{background:rgba(16,185,129,0.1);border:1px solid rgba(16,185,129,0.3);border-radius:7px;padding:4px 8px;margin-bottom:4px;font-size:10.5px;display:none}
.lv.show{display:block}
.lt{display:flex;justify-content:space-between;font-weight:600;color:var(--a);margin-bottom:2px}
.lb{height:3px;background:rgba(16,185,129,0.2);border-radius:2px;overflow:hidden}
.lp{height:100%%;background:var(--a);width:0%%;transition:width .4s}
.list{display:flex;flex-direction:column;gap:5px;padding-bottom:10px}
.c{background:var(--s);border:1px solid var(--bd);border-radius:7px;overflow:hidden;border-left:3px solid transparent}
.c.now{border-left-color:var(--a);background:rgba(16,185,129,0.06)}
.c.empty-pair{opacity:0.45;background:transparent;border:1px dashed var(--bd)}
.ct,.cb{padding:3px 8px}
.ct{display:flex;justify-content:space-between;align-items:center;background:rgba(128,128,128,0.06);border-bottom:1px solid var(--bd)}
.c.empty-pair .ct{border-bottom:none;background:transparent}
.cb{display:flex;flex-direction:column;gap:3px;padding-top:4px;padding-bottom:4px}
.nt{font-size:10.5px;color:var(--h);font-weight:600}
.lr{display:flex;justify-content:space-between;align-items:flex-start;gap:6px}
.lm{flex:1;min-width:0}
.sn{font-size:12px;font-weight:600;word-break:break-word;line-height:1.25}
.m{font-size:10.5px;color:var(--h);line-height:1.2;margin-top:1px}
.bgs{display:flex;gap:3px;flex-shrink:0;align-items:center}
.bg{font-size:10px;padding:1.5px 5px;border-radius:4px;background:rgba(128,128,128,0.14);color:var(--txt);font-weight:500}
.bg.r{background:rgba(36,129,204,0.15);color:var(--b)}
.sp{height:1px;background:var(--bd);margin:3px 0}
.et{color:var(--h);font-weight:400;padding:2px 8px 4px 8px;font-size:11.5px}
.em{text-align:center;padding:16px 0;color:var(--h);font-size:12px}
.cr{display:flex;justify-content:space-between;padding:5px 8px;background:var(--s);border-radius:6px;font-size:11.5px;border:1px solid var(--bd)}
.cr b{color:var(--b)}
</style></head><body>
<div class="hdr"><div><div class="t" id="tTitle"></div><div class="sub" id="tSub"></div></div><button class="vb" id="bToggle">⏰ Звонки</button></div>
<div id="schedView"><div class="tabs" id="dTabs"></div><div class="sb" id="sgBar" style="display:none"><button class="sg act" onclick="setSG(0)">Все</button><button class="sg" onclick="setSG(1)">1 п/г</button><button class="sg" onclick="setSG(2)">2 п/г</button></div><div class="lv" id="lvBox"><div class="lt"><span id="lvTxt"></span><span id="lvLeft"></span></div><div class="lb"><div class="lp" id="lvProg"></div></div></div><div class="list" id="lList"></div></div>
<div id="callsView" style="display:none"><div class="list" id="cList"></div></div>
<script>
const D=%s,$=i=>document.getElementById(i),$$=s=>document.querySelectorAll(s),DN=['Пн','Вт','Ср','Чт','Пт','Сб'];
let curDay=D.ad>=0&&D.ad<6?D.ad:0,curSG=0,vMode='s';
const C=[['09:00','09:45','09:55','10:40'],['10:50','11:35','11:55','12:40'],['13:00','13:45','13:55','14:40'],['14:50','15:35','15:45','16:30'],['16:40','17:25','17:35','18:20'],['18:30','19:15','19:25','20:10']];
const tm=s=>{const p=s.split(':');return p[0]*60+(+p[1])};
function hp(){try{window.TelegramWidget?.hapticFeedback?.impactOccurred?.('light')}catch(e){}}
function nowM(){const d=new Date(),m=new Date(d.getTime()+d.getTimezoneOffset()*60000+10800000);return{w:(m.getDay()+6)%%7,min:m.getHours()*60+m.getMinutes()}}
function init(){
$('tTitle').textContent=D.t;$('tSub').textContent=D.ws+' — '+D.we;
if(D.g)$('sgBar').style.display='flex';
$('dTabs').innerHTML=DN.map((s,i)=>'<button class="tab '+(i===curDay?'act':'')+'" onclick="setDay('+i+')">'+s+'</button>').join('');
$('bToggle').onclick=toggleV;
renderC();renderDay();updateLv();setInterval(updateLv,15000);
}
function setDay(i){hp();curDay=i;$$('#dTabs .tab').forEach((b,idx)=>b.classList.toggle('act',idx===i));renderDay();updateLv()}
function setSG(sg){hp();curSG=sg;$$('#sgBar .sg').forEach((b,idx)=>b.classList.toggle('act',idx===sg));renderDay()}
function toggleV(){
hp();const b=$('bToggle'),sv=$('schedView'),cv=$('callsView');
if(vMode==='s'){vMode='c';b.textContent='📅 Уроки';b.classList.add('act');sv.style.display='none';cv.style.display='block'}
else{vMode='s';b.textContent='⏰ Звонки';b.classList.remove('act');sv.style.display='block';cv.style.display='none'}
}
function renderDay(){
const day=D.d[curDay],list=$('lList');
if(!day||!day.length){list.innerHTML='<div class="em">🎉 В этот день пар нет</div>';return}
const filtered=day.filter(l=>curSG===0||!l.sg||l.sg===curSG);
if(!filtered.length){list.innerHTML='<div class="em">🎉 Для этой подгруппы пар нет</div>';return}
const pMap={};let maxP=0;
for(const l of filtered){if(!pMap[l.n])pMap[l.n]=[];pMap[l.n].push(l);if(l.n>maxP)maxP=l.n}
const nm=nowM(),isToday=nm.w===curDay,bg=(c,v)=>v?'<span class="bg '+(c||'')+'">'+v+'</span>':'';let out='';
for(let n=1;n<=maxP;n++){
const items=pMap[n],c=C[n-1]||['','','',''];
const tStr=c[0]?c[0]+' - '+c[3]:'';
if(!items||!items.length){out+='<div class="c empty-pair"><div class="ct"><span class="nt">'+n+' пара • '+tStr+'</span></div><div class="et">—</div></div>';continue}
const sMin=tm(c[0]),eMin=tm(c[3]),isNow=isToday&&nm.min>=sMin&&nm.min<=eMin,sMap={},sOrder=[];
for(const l of items){if(!sMap[l.s]){sMap[l.s]=[];sOrder.push(l.s)}sMap[l.s].push(l)}
let rows='';
for(let i=0;i<sOrder.length;i++){
const name=sOrder[i],sItems=sMap[name];
if(i>0)rows+='<div class="sp"></div>';
const who=sItems.map(l=>l.th||l.g).filter((v,idx,arr)=>v&&arr.indexOf(v)===idx).join(', ');
const rms=sItems.map(l=>l.r).filter((v,idx,arr)=>v&&arr.indexOf(v)===idx).join(', ');
const bHtml=bg('r',rms)+bg('',sItems.length===1&&sItems[0].sg?sItems[0].sg+' п/г':'')+bg('',sItems[0].t);
rows+='<div class="lr"><div class="lm"><div class="sn">'+name+'</div>'+(who?'<div class="m">'+who+'</div>':'')+'</div>'+(bHtml?'<div class="bgs">'+bHtml+'</div>':'')+'</div>';
}
out+='<div class="c '+(isNow?'now':'')+'"><div class="ct"><span class="nt">'+n+' пара • '+tStr+'</span></div><div class="cb">'+rows+'</div></div>';
}
list.innerHTML=out;
}
function updateLv(){
const nm=nowM(),box=$('lvBox');
if(nm.w!==curDay){box.classList.remove('show');return}
const day=D.d[curDay];if(!day||!day.length)return;
let act=null,ci=null;
for(const l of day){const c=C[l.n-1];if(c&&nm.min>=tm(c[0])&&nm.min<=tm(c[3])){act=l;ci=c;break}}
if(!act||!ci){box.classList.remove('show');return}
box.classList.add('show');
const sMin=tm(ci[0]),eMin=tm(ci[3]),tot=eMin-sMin,pct=Math.min(100,Math.max(0,((nm.min-sMin)/tot)*100));
$('lvTxt').textContent='🟢 '+act.n+' пара: '+act.s;
$('lvLeft').textContent='Осталось '+(eMin-nm.min)+' мин';
$('lvProg').style.width=pct+'%%';
}
function renderC(){$('cList').innerHTML=C.map((c,i)=>'<div class="cr"><span><b>'+(i+1)+' пара:</b> '+c[0]+'-'+c[1]+' | '+c[2]+'-'+c[3]+'</span><span>'+c[0]+' - '+c[3]+'</span></div>').join('');}
init();
</script></body></html>`
