package snippet

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// cookieName derives a short, opaque per-context cookie name so the snippet
// carries no human-readable identifiers.
func cookieName(contextID string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(contextID))
	return fmt.Sprintf("a%x", h.Sum32()&0xffff)
}

// Generate produces a self-contained, obfuscated JavaScript tracking snippet
// for the given context. It exposes no window globals (closure only), keeps
// its config in a packed string array, and posts compact events
// ({"v","t","u","r","p"}) to POST /track/<contextID> with a text/plain body
// so cross-origin requests skip the CORS preflight.
func Generate(contextID string, trackingURL string, options map[string]interface{}) (string, error) {
	if contextID == "" {
		return "", fmt.Errorf("contextID required")
	}

	apiHost := trackingURL
	if apiHost == "" {
		apiHost = "http://localhost:8081"
	}

	trackEndpoint, _ := options["trackEndpoint"].(string)
	if trackEndpoint == "" {
		trackEndpoint = "/track/" + contextID
	}

	batchSize, _ := options["batchSize"].(float64)
	if batchSize <= 0 {
		batchSize = 20
	}

	batchInterval, _ := options["batchInterval"].(float64)
	if batchInterval <= 0 {
		batchInterval = 30
	}

	snippet := fmt.Sprintf(`(function(){var s=[%q,%q,%q],q=[],v;function g(n){var m=n+"=";var c=document.cookie.split(";");for(var i=0;i<c.length;i++){var t=c[i].trim();if(t.indexOf(m)===0)return t.substring(m.length)}return null}v=g(s[2]);if(!v){v="xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g,function(c){var r=Math.random()*16|0;return(c==="x"?r:(r&3|8)).toString(16)});document.cookie=s[2]+"="+v+"; path=/; expires="+new Date(Date.now()+31536e6).toUTCString()+"; SameSite=Lax"}function f(){if(!q.length)return;var e=q;q=[];for(var i=0;i<e.length;i++){try{fetch(s[0]+s[1],{method:"POST",headers:{"Content-Type":"text/plain"},body:JSON.stringify(e[i]),keepalive:true})}catch(_){}}}function d(t,u,r,p){q.push({v:v,t:t,u:u||location.href,r:r||document.referrer,p:p||{}});if(q.length>=%d)f()}setInterval(f,%d*1000);d("pv",location.href,document.referrer,{title:document.title})})()`,
		apiHost,
		trackEndpoint,
		cookieName(contextID),
		int(batchSize),
		int(batchInterval),
	)

	return snippet, nil
}

// GenerateMinified is an alias for Generate that returns the snippet ready for inline use.
func GenerateMinified(contextID string, trackingURL string, options map[string]interface{}) (string, error) {
	return Generate(contextID, trackingURL, options)
}

// GenerateBannerSnippet produces a self-contained JavaScript banner snippet for the given context.
func GenerateBannerSnippet(contextID string, options map[string]interface{}) (string, error) {
	if contextID == "" {
		return "", fmt.Errorf("contextID required")
	}

	serverHost, _ := options["serverHost"].(string)
	if serverHost == "" {
		serverHost = "https://banners.automata.io"
	}

	snippet := fmt.Sprintf(`(function(){var b=window.AutomataBanner=b||{};b.contextId="%s";b.serverHost="%s";b.placements=[];b._initialized=false;function _loadPlacements(){if(b._initialized)return;b._initialized=true;var url=b.serverHost+"/api/banners/placements";fetch(url,{headers:{"X-Context-ID":b.contextId}}).then(function(r){return r.json()}).then(function(data){if(!data||!data.placements)return;data.placements.forEach(function(p){p.banners.forEach(function(banner){var el=document.querySelector(p.css_selector);if(!el)return;var wrapper=document.createElement("div");wrapper.innerHTML=banner.html;var link=document.createElement("a");link.href=banner.click_url||"#";link.target="_blank";link.rel="noopener noreferrer";var img=wrapper.querySelector("img");if(img){var src=img.getAttribute("src");if(src&&src.indexOf("track/banner")===-1){var q=new URLSearchParams({banner_id:banner.id,visitor_id:b._visitorId,type:"impression"});img.src=src+(src.indexOf("?")===-1?"?":"&")+q.toString()}}var visitorId=b._visitorId;link.addEventListener("click",function(){var t=new URLSearchParams({banner_id:banner.id,visitor_id:visitorId,type:"click"});var u=b.serverHost+"/track/banner?"+t.toString();fetch(u,{method:"GET",keepalive:true})})el.appendChild(wrapper)})})}).catch(function(){});function _generateId(){return"xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g,function(c){var r=Math.random()*16|0;return(c==="x"?r:r&0x3|0x8).toString(16)})}function _getCookie(n){var m=n+"=";var c=document.cookie.split(";");for(var i=0;i<c.length;i++){var v=c[i].trim();if(v.indexOf(m)===0)return v.substring(m.length,v.length)}return null}function _setCookie(n,v,d){var e="";if(d){var o=new Date;o.setTime(o.getTime()+d*24*60*60*1000);e="; expires="+o.toUTCString()}document.cookie=n+"="+v+e+"; path=/; SameSite=Lax"}b._visitorId=_getCookie("automata_banner_visitor");if(!b._visitorId){b._visitorId=_generateId();_setCookie("automata_banner_visitor",b._visitorId,365)}b.loadPlacements=_loadPlacements;b.loadPlacements()})();`,
		contextID,
		serverHost,
	)

	return snippet, nil
}

// Sanitize removes any leading/trailing whitespace from the generated snippet.
func Sanitize(snippet string) string {
	return strings.TrimSpace(snippet)
}
