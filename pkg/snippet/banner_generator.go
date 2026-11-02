package snippet

import (
	"fmt"
	"strings"
)

// GenerateBannerSnippet produces a self-contained JavaScript snippet that
// renders configurable banners for the given context. The snippet is served
// from the same origin as the banner endpoints, so all URLs are relative.
//
// Behavior:
//   - Reads/creates the `automata_visitor` cookie (16-hex, 1 year).
//   - Fetches /banner/config/<ctx>?visitor_id=... and renders each banner
//     with a non-empty `position` (top | bottom | fullpage) that the
//     visitor has not already decided on (cookie `ab_<bannerId>`).
//   - Button clicks POST the decision as text/plain (no CORS preflight),
//     set `ab_<bannerId>=<action>` for 365 days (SameSite=Lax; Path=/),
//     dismiss the banner when `dismisses` is true, and navigate to
//     /page/<ctx>/<slug> when `link_url` starts with "/".
func GenerateBannerConfigSnippet(contextID string) (string, error) {
	if contextID == "" {
		return "", fmt.Errorf("contextID required")
	}

	configURL := "/banner/config/" + contextID
	decisionURL := "/banner/decision/" + contextID

	js := `/* Automata banner snippet (self-rendering) */
(function(){
var contextId=__AUTOMATA_CONTEXT_ID__;
var configUrl=__AUTOMATA_CONFIG_URL__;
var decisionUrl=__AUTOMATA_DECISION_URL__;
function _getCookie(n){var m=n+"=";var c=document.cookie.split(";");for(var i=0;i<c.length;i++){var v=c[i].trim();if(v.indexOf(m)===0)return v.substring(m.length,v.length)}return null}
function _setCookie(n,v,days){var d=new Date();d.setTime(d.getTime()+days*24*60*60*1000);document.cookie=n+"="+v+";expires="+d.toUTCString()+";SameSite=Lax;Path=/";}
var visitorId=_getCookie("automata_visitor");
if(!visitorId){var _h="";for(var _i=0;_i<16;_i++){_h+="0123456789abcdef"[Math.floor(Math.random()*16)];}visitorId=_h;_setCookie("automata_visitor",visitorId,365);}
function positionCss(p){
if(p==="top")return "position:fixed;top:0;left:0;right:0;z-index:2147483000;";
if(p==="bottom")return "position:fixed;bottom:0;left:0;right:0;z-index:2147483000;";
return "position:fixed;top:0;left:0;right:0;bottom:0;z-index:2147483000;background:rgba(15,15,15,0.92);display:flex;align-items:center;justify-content:center;";
}
function renderBanner(b){
if(!b.position)return;
if(_getCookie("ab_"+b.id))return;
var wrap=document.createElement("div");
wrap.setAttribute("data-automata-banner",b.id);
wrap.style.cssText=positionCss(b.position);
var card=document.createElement("div");
if(b.position==="fullpage"){
card.style.cssText="max-width:480px;width:90%;background:#fff;border-radius:8px;padding:24px;font-family:sans-serif;box-sizing:border-box;";
}else{
card.style.cssText="font-family:sans-serif;padding:12px 16px;border:1px solid #ddd;box-shadow:0 2px 8px rgba(0,0,0,.15);box-sizing:border-box;";
}
wrap.appendChild(card);
var content=document.createElement("div");
content.innerHTML=b.content+(b.page?b.page:"");
card.appendChild(content);
var btns=document.createElement("div");
btns.style.cssText="margin-top:12px;display:flex;gap:8px;flex-wrap:wrap;";
var buttons=b.buttons||[];
for(var i=0;i<buttons.length;i++){
(function(btn){
var el=document.createElement("button");
el.textContent=btn.label||btn.action;
el.style.cssText="padding:8px 16px;border:none;border-radius:4px;font-size:14px;cursor:pointer;background:#2563eb;color:#fff;";
el.addEventListener("click",function(){
fetch(decisionUrl,{method:"POST",headers:{"Content-Type":"text/plain"},body:JSON.stringify({banner_id:b.id,action:btn.action,visitor_id:visitorId})}).catch(function(){});
_setCookie("ab_"+b.id,btn.action,365);
if(btn.dismisses){if(wrap.parentNode)wrap.parentNode.removeChild(wrap);}
if(btn.link_url&&btn.link_url.indexOf("/")==0){location.href="/page/"+contextId+btn.link_url.slice(1);}
});
btns.appendChild(el);
})(buttons[i]);
}
card.appendChild(btns);
document.body.appendChild(wrap);
}
fetch(configUrl+"?visitor_id="+encodeURIComponent(visitorId),{headers:{"Content-Type":"text/plain"}}).then(function(res){
if(!res.ok)throw new Error("load failed");
return res.json();
}).then(function(cfg){
var banners=cfg.banners||[];
for(var i=0;i<banners.length;i++){renderBanner(banners[i]);}
}).catch(function(){});
})();`

	js = strings.Replace(js, "__AUTOMATA_CONTEXT_ID__", quote(contextID), 1)
	js = strings.Replace(js, "__AUTOMATA_CONFIG_URL__", quote(configURL), 1)
	js = strings.Replace(js, "__AUTOMATA_DECISION_URL__", quote(decisionURL), 1)

	return js, nil
}
