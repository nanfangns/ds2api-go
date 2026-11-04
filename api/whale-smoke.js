'use strict';
function readConfig() {
  const raw = String(process.env.DS2API_CONFIG_JSON || '').trim();
  for (const value of [raw, Buffer.from(raw, 'base64').toString('utf8')]) {
    try { return JSON.parse(value); } catch (_) {}
  }
  return {};
}
function apiKey(c) {
  for (const k of (Array.isArray(c.keys) ? c.keys : [])) if (typeof k === 'string' && k.trim()) return k.trim();
  for (const item of (Array.isArray(c.api_keys) ? c.api_keys : [])) if (item && typeof item.key === 'string' && item.key.trim()) return item.key.trim();
  return '';
}
function classify(m) {
  m = String(m || '').toLowerCase();
  if (/device fingerprint rejected|risk_device_detected/.test(m)) return 'device_risk';
  if (/password|credential/.test(m)) return 'account_credentials';
  if (/deployment protection|vercel authentication/.test(m)) return 'vercel_protection';
  if (/account token is invalid|invalid token/.test(m)) return 'upstream_invalid_token';
  if (/login failed/.test(m)) return 'account_login_failed';
  if (/unauthorized/.test(m)) return 'unauthorized';
  if (/account/.test(m)) return 'account_error';
  return m ? 'other_error' : 'no_error_message';
}
module.exports = async function(req, res) {
  res.setHeader('Content-Type', 'application/json');
  res.setHeader('Cache-Control', 'no-store');
  if (req.method !== 'GET') { res.statusCode = 405; return res.end(JSON.stringify({ok:false})); }
  const key = apiKey(readConfig());
  if (!key) { res.statusCode = 503; return res.end(JSON.stringify({ok:false, stage:'configuration', reason:'no managed API key'})); }
  const requestURL = new URL(req.url, 'https://' + req.headers.host);
  const bypass = String(req.headers['x-vercel-protection-bypass'] || requestURL.searchParams.get('x-vercel-protection-bypass') || '');
  const target = new URL('/v1/chat/completions', 'https://' + req.headers.host);
  const headers = {'Content-Type':'application/json', 'Authorization':'Bearer ' + key};
  if (bypass) { headers['x-vercel-protection-bypass'] = bypass; target.searchParams.set('x-vercel-protection-bypass', bypass); }
  const body = {model:'deepseek-flash', messages:[{role:'user',content:'Reply with OK.'}]};
  const result = {};
  try {
    const plain = await fetch(target, {method:'POST',headers,body:JSON.stringify({...body,stream:false}),signal:AbortSignal.timeout(90000)});
    const plainText = await plain.text();
    let json = {}; try { json=JSON.parse(plainText); } catch (_) {}
    const content = json.choices && json.choices[0] && json.choices[0].message && json.choices[0].message.content;
    result.nonstream = {status:plain.status, ok:plain.ok && typeof content === 'string' && content.trim().length > 0, error:json.error && (json.error.code || json.error.type), reason:classify(json.error && json.error.message)};
    if (!result.nonstream.ok) {
      const directTarget = new URL(target);
      directTarget.searchParams.set('__go', '1');
      const direct = await fetch(directTarget, {method:'POST',headers,body:JSON.stringify({...body,stream:false}),signal:AbortSignal.timeout(90000)});
      let directJSON={}; try { directJSON=JSON.parse(await direct.text()); } catch (_) {}
      result.direct_go = {status:direct.status, reason:classify(directJSON.error && directJSON.error.message)};
      const prod = await fetch('https://nanfangai.eu.cc/v1/chat/completions', {method:'POST',headers:{'Content-Type':'application/json','Authorization':'Bearer '+key},body:JSON.stringify({...body,stream:false}),signal:AbortSignal.timeout(90000)});
      let prodJSON={}; try { prodJSON=JSON.parse(await prod.text()); } catch (_) {}
      result.production_baseline = {status:prod.status, reason:classify(prodJSON.error && prodJSON.error.message)};
    }
    if (result.nonstream.ok) {
      const stream = await fetch(target, {method:'POST',headers,body:JSON.stringify({...body,stream:true}),signal:AbortSignal.timeout(90000)});
      const text = await stream.text();
      result.stream = {status:stream.status, ok:stream.ok && text.includes('data:') && text.includes('[DONE]') && text.includes('"content"')};
    }
    result.ok = Boolean(result.nonstream.ok && result.stream && result.stream.ok);
    res.statusCode = result.ok ? 200 : 502;
    return res.end(JSON.stringify(result));
  } catch (_) {
    res.statusCode=502;
    return res.end(JSON.stringify({...result,ok:false,reason:'request failed or timed out'}));
  }
};
