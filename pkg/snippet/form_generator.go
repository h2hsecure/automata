package snippet

import (
	"fmt"
	"strings"
)

// GenerateForm produces a self-contained JavaScript snippet that renders the
// form identified by (contextID, slug) and posts submissions to the tracking
// server. The snippet fetches the form definition from the same origin at
// /form/definition/<contextID>/<slug> and posts to /form/submit/<slug> with
// the X-Context-ID header.
func GenerateForm(contextID, slug string) (string, error) {
	if contextID == "" {
		return "", fmt.Errorf("contextID required")
	}
	if slug == "" {
		return "", fmt.Errorf("slug required")
	}

	// Inline the full URLs so the snippet does not need to know the host:
	// it is served from the same origin that hosts the form endpoints.
	definitionURL := "/form/definition/" + contextID + "/" + slug
	submitURL := "/form/submit/" + slug

	js := `/* Automata form snippet (self-rendering) */
(function(){
var contextId=__AUTOMATA_CONTEXT_ID__;
var slug=__AUTOMATA_FORM_SLUG__;
var definitionUrl=__AUTOMATA_DEFINITION_URL__;
var submitUrl=__AUTOMATA_SUBMIT_URL__;
function _getCookie(n){var m=n+"=";var c=document.cookie.split(";");for(var i=0;i<c.length;i++){var v=c[i].trim();if(v.indexOf(m)===0)return v.substring(m.length,v.length)}return null}
var visitorId=_getCookie("automata_visitor");
function container(){
var el=document.querySelector('[data-automata-form="'+slug+'"]');
if(el)return el;
el=document.getElementById("automata-form");
if(el)return el;
return document.body;
}
function notAvailable(){
var div=document.createElement("div");
div.className="automata-form";
div.style.cssText="font-family:sans-serif;max-width:480px;margin:0 auto;padding:16px;border:1px solid #ddd;border-radius:8px;background:#fff;color:#555;font-size:14px";
div.textContent="This form is not available right now.";
container().appendChild(div);
}
function render(def){
var host=container();
var wrap=document.createElement("div");
wrap.className="automata-form";
wrap.setAttribute("data-automata-form",slug);
wrap.style.cssText="font-family:sans-serif;max-width:480px;margin:0 auto;padding:16px;border:1px solid #ddd;border-radius:8px;background:#fff";
var title=document.createElement("h3");
title.textContent=def.name||"Form";
title.style.cssText="margin:0 0 12px;font-size:18px";
wrap.appendChild(title);
var form=document.createElement("form");
form.setAttribute("novalidate","");
var fields=def.fields||[];
for(var i=0;i<fields.length;i++){
var f=fields[i];
var group=document.createElement("div");
group.style.cssText="margin-bottom:12px";
var label=document.createElement("label");
label.style.cssText="display:block;margin-bottom:4px;font-size:14px";
label.textContent=f.label||f.slug;
var input;
if(f.type==="textarea"){
input=document.createElement("textarea");
input.rows=4;
}else if(f.type==="select"){
input=document.createElement("select");
var opt=document.createElement("option");
opt.value="";
opt.textContent="Select\u2026";
input.appendChild(opt);
}else{
input=document.createElement("input");
input.type=f.type||"text";
}
input.name=f.slug;
if(f.required)input.required=true;
input.style.cssText="width:100%;box-sizing:border-box;padding:8px;border:1px solid #ccc;border-radius:4px;font-size:14px";
group.appendChild(label);
group.appendChild(input);
form.appendChild(group);
}
var status=document.createElement("div");
status.style.cssText="font-size:14px;margin-bottom:8px";
form.appendChild(status);
var btn=document.createElement("button");
btn.type="submit";
btn.textContent="Submit";
btn.style.cssText="padding:8px 16px;background:#2563eb;color:#fff;border:none;border-radius:4px;font-size:14px;cursor:pointer";
form.appendChild(btn);
form.addEventListener("submit",function(e){
e.preventDefault();
var data={};
for(var j=0;j<fields.length;j++){
var el=form.elements[fields[j].slug];
if(el)data[fields[j].slug]=el.value;
}
btn.disabled=true;
status.textContent="Sending\u2026";
status.style.color="#555";
fetch(submitUrl,{
method:"POST",
headers:{"Content-Type":"application/json","X-Context-ID":contextId},
body:JSON.stringify({data:data,visitor_id:visitorId})
}).then(function(res){
return res.text().then(function(t){return{ok:res.ok,text:t};});
}).then(function(out){
if(out.ok){
status.textContent="Thank you! Your submission was received.";
status.style.color="#16a34a";
form.reset();
}else{
var msg="Sorry, something went wrong.";
try{var p=JSON.parse(out.text);if(p.error)msg=p.error;}catch(_){}
status.textContent=msg;
status.style.color="#dc2626";
}
btn.disabled=false;
}).catch(function(){
status.textContent="Sorry, something went wrong.";
status.style.color="#dc2626";
btn.disabled=false;
});
});
wrap.appendChild(form);
host.appendChild(wrap);
}
fetch(definitionUrl).then(function(res){
if(!res.ok)throw new Error("load failed");
return res.json();
}).then(render).catch(function(){
notAvailable();
});
})();`

	js = strings.Replace(js, "__AUTOMATA_CONTEXT_ID__", quote(contextID), 1)
	js = strings.Replace(js, "__AUTOMATA_FORM_SLUG__", quote(slug), 1)
	js = strings.Replace(js, "__AUTOMATA_DEFINITION_URL__", quote(definitionURL), 1)
	js = strings.Replace(js, "__AUTOMATA_SUBMIT_URL__", quote(submitURL), 1)

	return js, nil
}

// quote returns a double-quoted JavaScript string literal with escaping.
func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return `"` + s + `"`
}