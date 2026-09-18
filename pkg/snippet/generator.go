package snippet

import (
	"fmt"
	"strings"
)

// Generate produces a self-contained JavaScript tracking snippet for the given tenant.
func Generate(tenantID string, options map[string]interface{}) (string, error) {
	if tenantID == "" {
		return "", fmt.Errorf("tenantID required")
	}

	apiHost, _ := options["apiHost"].(string)
	if apiHost == "" {
		apiHost = "https://api.automata.io"
	}

	trackEndpoint, _ := options["trackEndpoint"].(string)
	if trackEndpoint == "" {
		trackEndpoint = "/track"
	}

	batchSize, _ := options["batchSize"].(float64)
	if batchSize <= 0 {
		batchSize = 20
	}

	batchInterval, _ := options["batchInterval"].(float64)
	if batchInterval <= 0 {
		batchInterval = 30
	}

	snippet := fmt.Sprintf(`(function(){var a=window.Automata=a||{};a.tenantId="%s";a.apiHost="%s";a.trackEndpoint="%s";a._events=[];a._batchSize=%d;a._batchInterval=%d;a._cookieName="automata_visitor";function _generateId(){return"xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g,function(c){var r=Math.random()*16|0;return(c==="x"?r:r&0x3|0x8).toString(16)})}function _getCookie(n){var m=n+"=";var c=document.cookie.split(";");for(var i=0;i<c.length;i++){var v=c[i].trim();if(v.indexOf(m)===0)return v.substring(m.length,v.length)}return null}function _setCookie(n,v,d){var e="";if(d){var o=new Date;o.setTime(o.getTime()+d*24*60*60*1000);e="; expires="+o.toUTCString()}document.cookie=n+"="+v+e+"; path=/; SameSite=Lax"}a._visitorId=_getCookie(a._cookieName);if(!a._visitorId){a._visitorId=_generateId();_setCookie(a._cookieName,a._visitorId,365)}function _sendEvent(evt){var url=a.apiHost+a.trackEndpoint;var body={tenantId:a.tenantId,visitorId:a._visitorId,eventType:evt.eventType,pageURL:evt.pageURL||document.location.href,referrer:evt.referrer||document.referrer,properties:evt.properties||{}};try{fetch(url,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body),keepalive:true})}catch(e){}},a._flush=function(){if(a._events.length===0)return;var events=a._events.slice();a._events=[];for(var i=0;i<events.length;i++){_sendEvent(events[i])}};a.track=function(eventName,properties){var evt={eventType:eventName,properties:properties||{}};a._events.push(evt);if(a._events.length>=a._batchSize){a._flush()}};a.trackPageView=function(referrer){var evt={eventType:"page_view",pageURL:document.location.href,referrer:referrer||document.referrer,properties:{pageTitle:document.title}};a._events.push(evt);if(a._events.length>=a._batchSize){a._flush()}};setInterval(function(){a._flush()},a._batchInterval*1000);a.trackPageView()})();`,
		tenantID,
		apiHost,
		trackEndpoint,
		int(batchSize),
		int(batchInterval),
	)

	return snippet, nil
}

// GenerateMinified is an alias for Generate that returns the snippet ready for inline use.
func GenerateMinified(tenantID string, options map[string]interface{}) (string, error) {
	return Generate(tenantID, options)
}

// GenerateBannerSnippet produces a self-contained JavaScript banner snippet for the given tenant.
func GenerateBannerSnippet(tenantID string, options map[string]interface{}) (string, error) {
	if tenantID == "" {
		return "", fmt.Errorf("tenantID required")
	}

	serverHost, _ := options["serverHost"].(string)
	if serverHost == "" {
		serverHost = "https://banners.automata.io"
	}

	snippet := fmt.Sprintf(`(function(){var b=window.AutomataBanner=b||{};b.tenantId="%s";b.serverHost="%s";b.placements=[];b._initialized=false;function _loadPlacements(){if(b._initialized)return;b._initialized=true;var url=b.serverHost+"/api/banners/placements";fetch(url,{headers:{"X-Tenant-ID":b.tenantId}}).then(function(r){return r.json()}).then(function(data){if(!data||!data.placements)return;data.placements.forEach(function(p){p.banners.forEach(function(banner){var el=document.querySelector(p.css_selector);if(!el)return;var wrapper=document.createElement("div");wrapper.innerHTML=banner.html;var link=document.createElement("a");link.href=banner.click_url||"#";link.target="_blank";link.rel="noopener noreferrer";var img=wrapper.querySelector("img");if(img){var src=img.getAttribute("src");if(src&&src.indexOf("track/banner")===-1){var q=new URLSearchParams({banner_id:banner.id,visitor_id:b._visitorId,type:"impression"});img.src=src+(src.indexOf("?")===-1?"?":"&")+q.toString()}}var visitorId=b._visitorId;link.addEventListener("click",function(){var t=new URLSearchParams({banner_id:banner.id,visitor_id:visitorId,type:"click"});var u=b.serverHost+"/track/banner?"+t.toString();fetch(u,{method:"GET",keepalive:true})})el.appendChild(wrapper)})})}).catch(function(){});function _generateId(){return"xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g,function(c){var r=Math.random()*16|0;return(c==="x"?r:r&0x3|0x8).toString(16)})}function _getCookie(n){var m=n+"=";var c=document.cookie.split(";");for(var i=0;i<c.length;i++){var v=c[i].trim();if(v.indexOf(m)===0)return v.substring(m.length,v.length)}return null}function _setCookie(n,v,d){var e="";if(d){var o=new Date;o.setTime(o.getTime()+d*24*60*60*1000);e="; expires="+o.toUTCString()}document.cookie=n+"="+v+e+"; path=/; SameSite=Lax"}b._visitorId=_getCookie("automata_banner_visitor");if(!b._visitorId){b._visitorId=_generateId();_setCookie("automata_banner_visitor",b._visitorId,365)}b.loadPlacements=_loadPlacements;b.loadPlacements()})();`,
		tenantID,
		serverHost,
	)

	return snippet, nil
}

// Sanitize removes any leading/trailing whitespace from the generated snippet.
func Sanitize(snippet string) string {
	return strings.TrimSpace(snippet)
}
