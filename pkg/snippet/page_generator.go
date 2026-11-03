package snippet

import (
	"fmt"
	"strings"
)

// GeneratePageConfigSnippet produces a self-contained JavaScript snippet that
// renders positioned pages (banners) for the given context. The snippet is
// served from the same origin as the page endpoints, so all URLs are relative.
//
// Behavior:
//   - Reads/creates the `automata_visitor` cookie (16-hex, 1 year).
//   - Fetches /page/config/<ctx>?visitor_id=... and renders each page with a
//     supported `position` (top | bottom | left | right | full) that the
//     visitor has not already decided on (cookie `ab_<pageId>`).
//   - Button clicks POST the decision as text/plain (no CORS preflight),
//     set `ab_<pageId>=<action>` for 365 days (SameSite=Lax; Path=/),
//     dismiss the page when the action is dismiss_accept/dismiss_decline
//     (full pages with only link buttons stay blocking), and navigate to
//     /page/<ctx>/<slug> when the button `href` starts with "/".
func GeneratePageConfigSnippet(contextID string) (string, error) {
	if contextID == "" {
		return "", fmt.Errorf("contextID required")
	}

	configURL := "/page/config/" + contextID
	decisionURL := "/page/decision/" + contextID

	js := `/* Automata page snippet (self-rendering) */
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
if(p==="left")return "position:fixed;left:0;top:0;bottom:0;width:320px;z-index:2147483000;";
if(p==="right")return "position:fixed;right:0;top:0;bottom:0;width:320px;z-index:2147483000;";
if(p==="full")return "position:fixed;top:0;left:0;right:0;bottom:0;z-index:2147483000;background:rgba(15,15,15,0.92);display:flex;align-items:center;justify-content:center;";
return"";
}
function renderPage(p){
if(!positionCss(p.position))return;
if(_getCookie("ab_"+p.id))return;
var wrap=document.createElement("div");
wrap.setAttribute("data-automata-page",p.id);
wrap.setAttribute("data-position",p.position);
wrap.style.cssText=positionCss(p.position);
var card=document.createElement("div");
if(p.position==="full"){
card.style.cssText="max-width:480px;width:90%;background:#fff;border-radius:8px;padding:24px;font-family:sans-serif;box-sizing:border-box;";
}else if(p.position==="left"||p.position==="right"){
card.style.cssText="width:100%;height:100%;overflow-y:auto;padding:24px;background:#fff;font-family:sans-serif;box-sizing:border-box;box-shadow:0 0 16px rgba(0,0,0,.2);";
}else{
card.style.cssText="font-family:sans-serif;padding:12px 16px;border:1px solid #ddd;box-shadow:0 2px 8px rgba(0,0,0,.15);box-sizing:border-box;";
}
wrap.appendChild(card);
var content=document.createElement("div");
content.innerHTML=p.content+(p.page?p.page:"");
card.appendChild(content);
var btns=document.createElement("div");
btns.style.cssText="margin-top:12px;display:flex;gap:8px;flex-wrap:wrap;";
var buttons=p.buttons||[];
for(var i=0;i<buttons.length;i++){
(function(btn){
var el=document.createElement("button");
el.textContent=btn.label||btn.action;
var style="padding:8px 16px;border:none;border-radius:4px;font-size:14px;cursor:pointer;background:#2563eb;color:#fff;";
if(btn.variant==="secondary"){style="padding:8px 16px;border-radius:4px;font-size:14px;cursor:pointer;background:#fff;color:#2563eb;border:1px solid #2563eb;";}
el.style.cssText=style;
el.addEventListener("click",function(){
fetch(decisionUrl,{method:"POST",headers:{"Content-Type":"text/plain"},body:JSON.stringify({page_id:p.id,decision:btn.action,visitor_id:visitorId})}).catch(function(){});
_setCookie("ab_"+p.id,btn.action,365);
if(btn.action==="dismiss_accept"||btn.action==="dismiss_decline"){if(wrap.parentNode)wrap.parentNode.removeChild(wrap);}
if(btn.href&&btn.href.indexOf("/")==0){location.href="/page/"+contextId+btn.href.slice(1);}
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
var pages=cfg.pages||[];
for(var i=0;i<pages.length;i++){renderPage(pages[i]);}
}).catch(function(){});
})();`

	js = strings.Replace(js, "__AUTOMATA_CONTEXT_ID__", quote(contextID), 1)
	js = strings.Replace(js, "__AUTOMATA_CONFIG_URL__", quote(configURL), 1)
	js = strings.Replace(js, "__AUTOMATA_DECISION_URL__", quote(decisionURL), 1)

	return js, nil
}
