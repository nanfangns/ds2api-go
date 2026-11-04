'use strict';
const { chromium } = require('playwright-core');
function config() {
  const raw=String(process.env.DS2API_CONFIG_JSON || '').trim();
  for (const v of [raw,Buffer.from(raw,'base64').toString('utf8')]) { try { return JSON.parse(v); } catch (_) {} }
  return {};
}
module.exports=async function(req,res) {
  res.setHeader('Content-Type','application/json');
  res.setHeader('Cache-Control','no-store');
  if(req.method!=='GET'){res.statusCode=405;return res.end(JSON.stringify({ok:false}));}
  let browser; let stage='launch';
  try {
    const {default: chromiumBinary} = await import('@sparticuz/chromium');
    browser=await chromium.launch({executablePath:await chromiumBinary.executablePath(),headless:true,ignoreDefaultArgs:['--enable-automation'],args:[...chromiumBinary.args,'--disable-blink-features=AutomationControlled']});
    stage='page';
    const ctx=await browser.newContext({viewport:{width:1440,height:900},locale:'zh-CN',timezoneId:'Asia/Shanghai',serviceWorkers:'block'});
    await ctx.route(url => url.pathname.includes('/api/v0/users/'),route=>route.abort('blockedbyclient'));
    await ctx.addInitScript(()=>{Object.defineProperty(navigator,'webdriver',{get:()=>undefined});});
    const page=await ctx.newPage();
    const nav=await page.goto('https://chat.deepseek.com/sign_in',{waitUntil:'domcontentloaded',timeout:30000});
    stage='harvest';
    let token='';
    for(let n=0;n<120;n++){token=await page.evaluate(()=>{try{return window.SMSdk?.getDeviceId?.()||''}catch(_){return ''}}).catch(()=> '');if(typeof token==='string'&&token.startsWith('B'))break;await page.waitForTimeout(250);}
    if(!token.startsWith('B')){res.statusCode=502;return res.end(JSON.stringify({ok:false,stage,nav:nav?.status(),got_device_token:false}));}
    stage='login';
    const accounts=Array.isArray(config().accounts)?config().accounts:[];
    const acc=accounts.find(a=>a&&a.email&&a.password);
    if(!acc){res.statusCode=503;return res.end(JSON.stringify({ok:false,stage,got_device_token:true,reason:'no email account available'}));}
    const upstream=await fetch('https://chat.deepseek.com/api/v0/users/login',{method:'POST',headers:{'Content-Type':'application/json','Accept':'application/json','User-Agent':'DeepSeek/2.0.4 Android/35','x-client-platform':'android','x-client-version':'2.0.4','x-client-locale':'zh_CN'},body:JSON.stringify({email:acc.email,password:acc.password,device_id:token,os:'android'}),signal:AbortSignal.timeout(30000)});
    let body={};try{body=await upstream.json()}catch(_){}
    const bizCode=body?.data?.biz_code;
    const ok=upstream.ok&&Number(body.code)===0&&Number(bizCode)===0&&Boolean(body?.data?.biz_data?.user?.token);
    res.statusCode=ok?200:502;return res.end(JSON.stringify({ok,stage,got_device_token:true,http_status:upstream.status,biz_code:bizCode??null,outer_code:body.code??null}));
  } catch(err){res.statusCode=502;return res.end(JSON.stringify({ok:false,stage,error_type:err?.name||'Error'}));}
  finally {if(browser)await browser.close().catch(()=>{});}
};
