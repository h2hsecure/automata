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

	snippet := fmt.Sprintf(`(function(){var s=[%q,%q,%q],q=[],v;function g(n){var m=n+"=";var c=document.cookie.split(";");for(var i=0;i<c.length;i++){var t=c[i].trim();if(t.indexOf(m)===0)return t.substring(m.length)}return null}v=g(s[2]);if(!v){v="xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g,function(c){var r=Math.random()*16|0;return(c==="x"?r:(r&3|8)).toString(16)});document.cookie=s[2]+"="+v+"; path=/; expires="+new Date(Date.now()+31536e6).toUTCString()+"; SameSite=Lax"}function f(){if(!q.length)return;var e=q;q=[];for(var i=0;i<e.length;i++){try{fetch(s[0]+s[1],{method:"POST",headers:{"Content-Type":"text/plain"},body:JSON.stringify(e[i]),keepalive:true})}catch(_){}}}function d(t,u,r,p){q.push({v:v,t:t,u:u||location.href,r:r||document.referrer,p:p||{}});if(q.length>=%d)f()}setInterval(f,%d*1000);d("pv",location.href,document.referrer,{title:document.title});window.addEventListener("pagehide",f);window.addEventListener("beforeunload",f)})()`,
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

// Sanitize removes any leading/trailing whitespace from the generated snippet.
func Sanitize(snippet string) string {
	return strings.TrimSpace(snippet)
}
